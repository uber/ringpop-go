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
	// ErrDuplicateHook is returned when a hook with the given name has already
	// been registered.
	ErrDuplicateHook = errors.New("expected hook name to be unique")

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
	// Name returns the name under which the eviction should be registered
	Name() string

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
	DisablePing bool
	PingRatio   float64
}

func mergeSelfEvictOptions(opt, def SelfEvictOptions) SelfEvictOptions {
	return SelfEvictOptions{
		DisablePing: util.SelectBool(opt.DisablePing, def.DisablePing),
		PingRatio:   util.SelectFloat(opt.PingRatio, def.PingRatio),
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
	node    *Node
	options SelfEvictOptions
	logger  bark.Logger
	phases  []*phase

	hooks map[string]SelfEvictHook
}

func newSelfEvict(node *Node, options SelfEvictOptions) *selfEvict {
	return &selfEvict{
		node:    node,
		options: options,
		logger:  node.logger,

		hooks: make(map[string]SelfEvictHook),
	}
}

// RegisterSelfEvictHook registers a named pre/post eviction hook pair.
func (s *selfEvict) RegisterSelfEvictHook(hook SelfEvictHook) error {
	if s.currentPhase() != nil {
		return ErrSelfEvictionInProgress
	}

	name := hook.Name()

	_, hasHook := s.hooks[name]
	if hasHook {
		return ErrDuplicateHook
	}

	s.hooks[name] = hook

	return nil
}

func (s *selfEvict) SelfEvict() error {
	if s.currentPhase() != nil {
		return ErrSelfEvictionInProgress
	}

	s.logger.Info("ringpop is initiating self eviction sequence")

	// these are the phases executed in order.
	s.preEvict()
	s.evict()
	s.postEvict()
	s.done()

	return nil
}

func (s *selfEvict) preEvict() {
	s.transitionTo(preEvict)
	s.runHooks(SelfEvictHook.PreEvict)
}

func (s *selfEvict) evict() {
	phase := s.transitionTo(evicting)
	s.node.memberlist.SetLocalStatus(Faulty)

	if s.options.DisablePing == false {
		numberOfPingableMembers := s.node.memberlist.NumPingableMembers()
		maxNumberOfPings := int(math.Ceil(float64(numberOfPingableMembers) * s.options.PingRatio))

		// final number of members to ping should not exceed any of:
		numberOfPings := util.Min(
			s.node.disseminator.maxP, // the piggyback counter
			numberOfPingableMembers,  // the number of members we can ping
			maxNumberOfPings,         // a configured percentage of members
		)

		phase.numberOfPings = numberOfPings

		// select the members we are going to ping
		targets := s.node.memberlist.RandomPingableMembers(numberOfPings, nil)

		s.logger.WithFields(bark.Fields{
			"numberOfPings": phase.numberOfPings,
			"targets":       targets,
		}).Debug("starting proactive gossip on self evict")

		var wg sync.WaitGroup
		wg.Add(len(targets))
		for _, target := range targets {
			go func(target *Member) {
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
}

func (s *selfEvict) postEvict() {
	s.transitionTo(postEvict)
	s.runHooks(SelfEvictHook.PostEvict)
}

func (s *selfEvict) done() {
	s.transitionTo(done)
	s.logger.Info("ringpop self eviction done")
	//TODO emit total timing
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
	for name, hook := range s.hooks {
		go func(name string, hook SelfEvictHook) {
			defer wg.Done()

			s.logger.WithField("hook", name).Debug("ringpop self eviction running hook")
			dispatch(hook)
			s.logger.WithField("hook", name).Debug("ringpop self eviction hook done")
		}(name, hook)
	}
	wg.Wait()
	s.logger.Debug("ringpop self eviction done running hooks")
}
