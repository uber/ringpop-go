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

	log "github.com/uber/bark"
)

type suspect interface {
	address() string
	incarnation() int64
}

// Suspicion handles the suspicion sub-protocol of the SWIM protocol
type suspicion struct {
	node *Node

	timeout time.Duration
	timers  map[string]*time.Timer
	enabled bool

	// protects suspicion timers
	l sync.Mutex
}

// NewSuspicion returns a new suspicion SWIM sub-protocol with the given timeout
func newSuspicion(n *Node, timeout time.Duration) *suspicion {
	suspicion := &suspicion{
		node:    n,
		timeout: timeout,
		timers:  make(map[string]*time.Timer),
		enabled: true,
	}

	return suspicion
}

func (s *suspicion) Start(suspect suspect) {
	if !s.enabled {
		s.node.logger.WithField("local", s.node.Address()).
			Warn("cannot start suspect period while disabled")
		return
	}

	if s.node.Address() == suspect.address() {
		s.node.logger.WithField("local", s.node.Address()).
			Warn("cannot start suspect period for local member")
		return
	}

	s.Stop(suspect)

	s.l.Lock()
	defer s.l.Unlock()

	s.timers[suspect.address()] = time.AfterFunc(s.timeout, func() {
		s.node.logger.WithFields(log.Fields{
			"local":  s.node.Address(),
			"faulty": suspect.address(),
		}).Info("member declared faulty")

		s.node.memberlist.MakeFaulty(suspect.address(), suspect.incarnation())
	})

	s.node.logger.WithFields(log.Fields{
		"local":   s.node.Address(),
		"suspect": suspect.address(),
	}).Debug("started suspect period")
}

func (s *suspicion) Stop(suspect suspect) {
	s.l.Lock()
	defer s.l.Unlock()

	if timer, ok := s.timers[suspect.address()]; ok {
		timer.Stop()
		delete(s.timers, suspect.address())

		s.node.logger.WithFields(log.Fields{
			"local":   s.node.Address(),
			"suspect": suspect.address(),
		}).Debug("stopped member suspect timer")
	}
}

// reenable suspicion protocol
func (s *suspicion) Reenable() {
	s.l.Lock()
	defer s.l.Unlock()

	if s.enabled {
		s.node.logger.WithField("local", s.node.Address()).Warn("suspicion already enabled")
		return
	}

	s.enabled = true

	s.node.logger.WithField("local", s.node.Address()).Info("reenabled suspicion protocol")
}

// stop all suspicion timers and disables suspicion protocol
func (s *suspicion) Disable() {
	s.l.Lock()
	defer s.l.Unlock()

	if !s.enabled {
		s.node.logger.WithField("local", s.node.Address()).Warn("suspicion already disabled")
		return
	}

	s.enabled = false

	numTimers := len(s.timers)

	for address, timer := range s.timers {
		timer.Stop()
		delete(s.timers, address)
	}

	s.node.logger.WithFields(log.Fields{
		"local":         s.node.Address(),
		"timersStopped": numTimers,
	}).Info("disabled suspicion protocol")
}

// testing func to avoid data races
func (s *suspicion) Timer(address string) *time.Timer {
	s.l.Lock()
	defer s.l.Unlock()

	return s.timers[address]
}
