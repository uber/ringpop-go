// Copyright (c) 2016 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package swim

import (
	"errors"
	"math"
	"sync"
	"sync/atomic"
	"time"

	"github.com/uber-common/bark"
	"github.com/uber/ringpop-go/util"
)

var (
	// ErrDuplicateHook is returned when a hook that has already been registered
	// is registered again
	ErrDuplicateHook = errors.New("hook already registered")

	// ErrSelfEvictionInProgress is returned when ringpop is already in the process
	// of evicting itself from the network.
	ErrSelfEvictionInProgress = errors.New("ringpop is already executing a self-eviction")
)

// SelfEvict defines the functions that interact with the self eviction of nodes
// from the membership prior to shutting down
type SelfEvict interface {
	// RegisterSelfEvictHook is used to register a SelfEvictHook interface to be
	// called during the shutting down of ringpop. Hooks can't be registered
	// after the self eviction has started.
	RegisterSelfEvictHook(hooks SelfEvictHook) error

	// SelfEvict should be called before shutting down the application to notify
	// the members of the membership that this node is going down and should not
	// receive reqeusts anymore.
	SelfEvict() error
}

// SelfEvictHook is an interface describing a module that can be registered to
// the self eviction hooks
type SelfEvictHook interface {
	// PreEvict is the hook that will be called before ringpop evicts itself
	// from the membership
	PreEvict()

	// PostEvict is the hook that will be called after ringpop has evicted
	// itself from them memership
	PostEvict()
}

type hookFn func(SelfEvictHook)

// SelfEvictOptions configures how self eviction should behave. Applications can
// configure if ringpop should proactively ping members of the network on self
// eviction and what percentage/ratio of the memberlist should be pinged at most
type SelfEvictOptions struct {
	PingRatio float64
}

func mergeSelfEvictOptions(opt, def SelfEvictOptions) SelfEvictOptions {
	return SelfEvictOptions{
		PingRatio: util.SelectFloat(opt.PingRatio, def.PingRatio),
	}
}

type evictionPhase int

const (
	preEvict evictionPhase = iota
	evicting
	postEvict
	done
)

type phase struct {
	phase evictionPhase
	start time.Time
	end   time.Time

	// phase specific information
	// phase: evicting
	numberOfPings           int
	numberOfSuccessfulPings int32
}

type selfEvict struct {
	// lock is used to gate access to state changing operations like registering
	// hooks and starting self eviction. After eviction has started all state
	// changing functions should return ErrSelfEvictionInProgress
	lock sync.Mutex

	node    *Node
	options SelfEvictOptions
	logger  bark.Logger
	phases  []*phase

	hooks []SelfEvictHook
}

func newSelfEvict(node *Node, options SelfEvictOptions) *selfEvict {
	return &selfEvict{
		node:    node,
		options: options,
		logger:  node.logger,
	}
}

// RegisterSelfEvictHook registers a pre/post eviction hook pair. If the hook
// has already been registered before it returns the ErrDuplicateHook and does
// not register it twice.
func (s *selfEvict) RegisterSelfEvictHook(hook SelfEvictHook) error {
	// lock the phases to prevent reading the current phase in a race. Unlocking
	// will happen when this function terminates to prevent self-eviction to
	// start while a hook is being added
	s.lock.Lock()
	defer s.lock.Unlock()

	if s.currentPhase() != nil {
		return ErrSelfEvictionInProgress
	}

	for _, h := range s.hooks {
		if h == hook {
			return ErrDuplicateHook
		}
	}

	s.hooks = append(s.hooks, hook)

	return nil
}

func (s *selfEvict) SelfEvict() error {
	// preEvict makes sure that SelfEvict will return ErrSelfEvictionInProgress
	// when eviction is already in progress
	err := s.preEvict()
	if err != nil {
		return err
	}
	s.evict()
	s.postEvict()
	s.done()

	return nil
}

func (s *selfEvict) preEvict() error {
	s.lock.Lock()
	if s.currentPhase() != nil {
		s.lock.Unlock()
		return ErrSelfEvictionInProgress
	}
	s.transitionTo(preEvict)
	s.lock.Unlock()

	s.logger.Info("ringpop is initiating self eviction sequence")
	s.runHooks(SelfEvictHook.PreEvict)

	return nil
}

func (s *selfEvict) evict() {
	s.lock.Lock()
	phase := s.transitionTo(evicting)
	s.lock.Unlock()
	s.node.memberlist.SetLocalStatus(Faulty)

	numberOfPingableMembers := s.node.memberlist.NumPingableMembers()
	maxNumberOfPings := int(math.Ceil(float64(numberOfPingableMembers) * s.options.PingRatio))

	// final number of members to ping should not exceed any of:
	numberOfPings := util.Min(
		s.node.disseminator.maxP, // the piggyback counter
		numberOfPingableMembers,  // the number of members we can ping
		maxNumberOfPings,         // a configured percentage of members
	)

	if numberOfPings <= 0 {
		// there are no nodes to be pinged, a value below 0 can be caused by a
		// negative ping ratio
		return
	}

	// select the members we are going to ping
	targets := s.node.memberlist.RandomPingableMembers(numberOfPings, nil)
	phase.numberOfPings = len(targets)

	s.logger.WithFields(bark.Fields{
		"numberOfPings": phase.numberOfPings,
		"targets":       targets,
	}).Debug("starting proactive gossip on self evict")

	var wg sync.WaitGroup
	wg.Add(len(targets))
	for _, target := range targets {
		go func(target Member) {
			defer wg.Done()
			_, err := sendPing(s.node, target.address(), s.node.pingTimeout)
			if err == nil {
				atomic.AddInt32(&phase.numberOfSuccessfulPings, 1)
			}
		}(target)
	}
	wg.Wait()

	s.logger.WithFields(bark.Fields{
		"numberOfPings":           phase.numberOfPings,
		"numberOfSuccessfulPings": phase.numberOfSuccessfulPings,
	}).Debug("finished proactive gossip on self evict")
}

func (s *selfEvict) postEvict() {
	s.lock.Lock()
	s.transitionTo(postEvict)
	s.lock.Unlock()

	s.runHooks(SelfEvictHook.PostEvict)
}

func (s *selfEvict) done() {
	s.lock.Lock()
	s.transitionTo(done)
	phasesCount := len(s.phases)
	firstPhase := s.phases[0].start
	lastPhase := s.phases[phasesCount-1]
	s.lock.Unlock()

	duration := lastPhase.end.Sub(firstPhase)

	s.logger.WithFields(bark.Fields{
		"phases":        s.phases,
		"totalDuration": duration,
	}).Info("ringpop self eviction done")

	s.node.EmitEvent(SelfEvictedEvent{
		PhasesCount: phasesCount,
		Duration:    duration,
	})
}

func (s *selfEvict) transitionTo(newPhase evictionPhase) *phase {
	p := &phase{
		phase: newPhase,
		start: s.node.clock.Now(),
	}

	previousPhase := s.currentPhase()
	s.phases = append(s.phases, p)

	if previousPhase != nil {
		previousPhase.end = p.start
	}

	s.logger.WithFields(bark.Fields{
		"newPhase": p,
		"oldPhase": previousPhase,
	}).Debug("ringpop self eviction phase-transitioning")

	return p
}

func (s *selfEvict) currentPhase() *phase {
	if len(s.phases) > 0 {
		return s.phases[len(s.phases)-1]
	}
	return nil
}

func (s *selfEvict) runHooks(dispatch hookFn) {
	var wg sync.WaitGroup

	wg.Add(len(s.hooks))
	for _, hook := range s.hooks {
		go func(hook SelfEvictHook) {
			defer wg.Done()

			s.logger.Debugf("ringpop self eviction running hook: %+v", hook)
			dispatch(hook)
			s.logger.Debugf("ringpop self eviction hook done: %+v", hook)
		}(hook)
	}
	wg.Wait()
	s.logger.Debug("ringpop self eviction done running hooks")
}
