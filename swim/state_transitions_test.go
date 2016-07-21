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
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/uber/ringpop-go/util"
)

type StateTransitionsSuite struct {
	suite.Suite
	node             *Node
	stateTransitions *stateTransitions
	m                *memberlist
	incarnation      int64

	suspect Change
}

func (s *StateTransitionsSuite) SetupTest() {
	s.incarnation = util.TimeNowMS()
	s.node = NewNode("test", "127.0.0.1:3001", nil, &Options{
		StateTimeouts: StateTimeouts{
			Suspect: 1000 * time.Second,
		},
	})
	s.stateTransitions = s.node.stateTransitions
	s.m = s.node.memberlist

	s.m.MakeAlive(s.node.Address(), s.incarnation)

	s.suspect = Change{
		Address:     "127.0.0.1:3002",
		Incarnation: s.incarnation,
	}
}

func (s *StateTransitionsSuite) TearDownTest() {
	s.node.Destroy()
}

func (s *StateTransitionsSuite) TestEnableDisableSuspicion() {
	s.True(s.stateTransitions.enabled, "expected suspicion to be enabled")

	s.stateTransitions.Disable()
	s.False(s.stateTransitions.enabled, "expected suspicion to be disabled")

	s.stateTransitions.Enable()
	s.True(s.stateTransitions.enabled, "expected suspicion to be enabled")

	s.stateTransitions.Enable()
	s.True(s.stateTransitions.enabled, "expected suspicion to be enabled")
}

func (s *StateTransitionsSuite) TestSuspectMember() {
	s.stateTransitions.ScheduleSuspectToFaulty(s.suspect)
	s.NotNil(s.stateTransitions.timer(s.suspect.Address), "expected suspicion timer to be set")
}

func (s *StateTransitionsSuite) TestStartingSuspectTimerTwice() {
	s.stateTransitions.ScheduleSuspectToFaulty(s.suspect)
	firstTimer := s.stateTransitions.timer(s.suspect.address())

	s.stateTransitions.ScheduleSuspectToFaulty(s.suspect)
	secondTimer := s.stateTransitions.timer(s.suspect.address())

	s.Equal(firstTimer, secondTimer, "expected the timer to remain the same when set twice")
}

func (s *StateTransitionsSuite) TestSuspectLocal() {
	s.stateTransitions.ScheduleSuspectToFaulty(*s.m.local)
	s.Nil(s.stateTransitions.timer(s.m.local.Address), "expected suspicion timer for local member to be nil")
}

func (s *StateTransitionsSuite) TestSuspectDisabled() {
	s.stateTransitions.Disable()
	s.False(s.stateTransitions.enabled, "expected suspicion to be disabled")

	s.stateTransitions.ScheduleSuspectToFaulty(s.suspect)
	s.Nil(s.stateTransitions.timers[s.suspect.Address], "expected suspicion timer to be nil")
}

func (s *StateTransitionsSuite) TestSuspectBecomesFaulty() {
	s.stateTransitions.timeouts.Suspect = time.Millisecond

	s.m.MakeAlive(s.suspect.Address, s.suspect.Incarnation)
	member, _ := s.m.Member(s.suspect.Address)
	s.Require().NotNil(member, "expected cannot be nil")

	s.stateTransitions.ScheduleSuspectToFaulty(*member)
	s.NotNil(s.stateTransitions.timer(member.Address), "expected suspicion timer to be set")

	time.Sleep(5 * time.Millisecond)
	s.Equal(Faulty, member.Status, "expected member to be faulty")
}

// TestTimerCreated tests that starting suspicion for a node creates a
// countdown timer that is responsible for marking a node as faulty at the end
// of the timeout.
func (s *StateTransitionsSuite) TestTimerCreated() {
	s.m.MakeAlive(s.suspect.Address, s.suspect.Incarnation)
	member, _ := s.m.Member(s.suspect.Address)
	s.Require().NotNil(member, "expected cannot be nil")

	old := s.stateTransitions.timer(member.Address)
	s.Require().Nil(old, "expected timer to be nil")

	// Start suspcision, which should create a timer
	s.stateTransitions.ScheduleSuspectToFaulty(*member)

	s.NotEqual(old, s.stateTransitions.timer(member.Address), "expected timer to change")
}

func (s *StateTransitionsSuite) TestSuspicionDisableStopsTimers() {
	s.stateTransitions.ScheduleSuspectToFaulty(Change{Address: "127.0.0.1:3002", Incarnation: s.incarnation})
	s.stateTransitions.ScheduleSuspectToFaulty(Change{Address: "127.0.0.1:3003", Incarnation: s.incarnation})
	s.stateTransitions.ScheduleSuspectToFaulty(Change{Address: "127.0.0.1:3004", Incarnation: s.incarnation})

	s.Len(s.stateTransitions.timers, 3, "expected 3 timers to be started")

	s.stateTransitions.Disable()
	s.False(s.stateTransitions.enabled, "expected suspicion to be disabled")
	s.Empty(s.stateTransitions.timers, "expected all timers to be cleared")
}

func TestStateTransitionsSuite(t *testing.T) {
	suite.Run(t, new(StateTransitionsSuite))
}
