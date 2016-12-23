// Copyright (c) 2015 Uber Technologies, Inc.
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
	"sync"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/uber-common/bark"
	"github.com/uber/ringpop-go/logging"
	"github.com/uber/ringpop-go/util"
)

// subject is an interface to define the subject (eg. member) to transition state for. This interface allows to pass in both a Member and a Change struct to the schedule function.
type subject interface {
	address() string
	incarnation() int64
}

type transitionTimer struct {
	*clock.Timer

	// state represents the state the subject was in when the transition was scheduled
	state string
}

// stateTransitions handles the timers for state transitions in SWIM
type stateTransitions struct {
	sync.Mutex

	node     *Node
	timeouts StateTimeouts
	timers   map[string]*transitionTimer
	enabled  bool

	logger bark.Logger
}

// StateTimeouts contains the configured timeouts for every state before transitioning to the new state
type StateTimeouts struct {
	// Suspect is the timeout it takes a node in suspect mode to transition to faulty
	Suspect time.Duration

	// Faulty is the timeout it takes a node in faulty mode to transition to tombstone
	Faulty time.Duration

	// Tombstone is the timeout it takes a node in tombstone mode to be evicted
	Tombstone time.Duration
}

func mergeStateTimeouts(one StateTimeouts, two StateTimeouts) StateTimeouts {
	return StateTimeouts{
		Suspect:   util.SelectDuration(one.Suspect, two.Suspect),
		Faulty:    util.SelectDuration(one.Faulty, two.Faulty),
		Tombstone: util.SelectDuration(one.Tombstone, two.Tombstone),
	}
}

// newStateTransitions returns a new state transition controller that can be used to schedule state transitions for nodes
func newStateTransitions(n *Node, timeouts StateTimeouts) *stateTransitions {
	return &stateTransitions{
		node:     n,
		timeouts: timeouts,
		timers:   make(map[string]*transitionTimer),
		enabled:  true,
		logger:   logging.Logger("stateTransitions").WithField("local", n.Address()),
	}
}

// ScheduleSuspectToFaulty starts the suspect timer. After the Suspect timeout the node will be declared faulty
func (s *stateTransitions) ScheduleSuspectToFaulty(subject subject) {
	s.Lock()
	s.schedule(subject, Suspect, s.timeouts.Suspect, func() {
		// transition the subject to faulty
		s.node.memberlist.MakeFaulty(subject.address(), subject.incarnation())
	})
	s.Unlock()
}

// ScheduleFaultyToTombstone starts the faulty timer. After the Faulty timeout the node will be declared tombstone
func (s *stateTransitions) ScheduleFaultyToTombstone(subject subject) {
	s.Lock()
	s.schedule(subject, Faulty, s.timeouts.Faulty, func() {
		// transition the subject to tombstone
		s.node.memberlist.MakeTombstone(subject.address(), subject.incarnation())
	})
	s.Unlock()
}

// ScheduleTombstoneToEvict starts the tombstone timer. After the Faulty timeout the node will be evicted
func (s *stateTransitions) ScheduleTombstoneToEvict(subject subject) {
	s.Lock()
	s.schedule(subject, Tombstone, s.timeouts.Tombstone, func() {
		// transition the subject to tombstone
		s.node.memberlist.Evict(subject.address())
	})
	s.Unlock()
}

func (s *stateTransitions) schedule(subject subject, state string, timeout time.Duration, transition func()) {
	if !s.enabled {
		s.logger.WithField("member", subject.address()).Warn("cannot schedule a state transition while disabled")
		return
	}

	if s.node.Address() == subject.address() {
		s.logger.WithField("member", subject.address()).Debug("cannot schedule a state transition for the local member")
		return
	}

	if timer, ok := s.timers[subject.address()]; ok {
		if timer.state == state {
			s.logger.WithFields(bark.Fields{
				"member": subject.address(),
				"state":  state,
			}).Warn("redundant call to schedule a state transition for member, ignored")
			return
		}
		// cancel the previously scheduled transition for the subject
		timer.Stop()
	}

	timer := s.node.clock.AfterFunc(timeout, func() {
		s.logger.WithFields(bark.Fields{
			"member": subject.address(),
			"state":  state,
		}).Info("executing scheduled transition for member")
		// execute the transition
		transition()
	})

	s.timers[subject.address()] = &transitionTimer{
		Timer: timer,
		state: state,
	}

	s.logger.WithFields(bark.Fields{
		"member": subject.address(),
		"state":  state,
	}).Debug("scheduled state transition for member")
}

// Cancel cancels the scheduled transition for the subject
func (s *stateTransitions) Cancel(subject subject) {
	s.Lock()

	if timer, ok := s.timers[subject.address()]; ok {
		timer.Stop()
		delete(s.timers, subject.address())
		s.logger.WithFields(bark.Fields{
			"member": subject.address(),
			"state":  timer.state,
		}).Debug("stopped scheduled state transition for member")
	}

	s.Unlock()
}

// Enable enables state transition controller. The transition controller needs to be in enabled state to allow transitions to be scheduled.
func (s *stateTransitions) Enable() {
	s.Lock()

	if s.enabled {
		s.logger.Warn("state transition controller already enabled")
		s.Unlock()
		return
	}

	s.enabled = true
	s.Unlock()
	s.logger.Info("enabled state transition controller")
}

// Disable cancels all scheduled state transitions and disables the state transition controller for further use
func (s *stateTransitions) Disable() {
	s.Lock()

	if !s.enabled {
		s.logger.Warn("state transition controller already disabled")
		s.Unlock()
		return
	}

	s.enabled = false

	numTimers := len(s.timers)
	for address, timer := range s.timers {
		timer.Stop()
		delete(s.timers, address)
	}

	s.Unlock()
	s.logger.WithField("timersStopped", numTimers).Info("disabled state transition controller")
}

// timer is a testing func to avoid data races
func (s *stateTransitions) timer(address string) *clock.Timer {
	s.Lock()
	t, ok := s.timers[address]
	s.Unlock()
	if !ok {
		return nil
	}
	return t.Timer
}
