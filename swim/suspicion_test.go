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
	"github.com/uber/ringpop-go/swim/util"
)

type SuspicionTestSuite struct {
	suite.Suite
	node        *Node
	s           *suspicion
	m           *memberlist
	incarnation int64

	suspect Change
}

func (s *SuspicionTestSuite) SetupTest() {
	s.incarnation = util.TimeNowMS()
	s.node = NewNode("test", "127.0.0.1:3001", nil, &Options{
		SuspicionTimeout: 1000 * time.Second,
	})
	s.s = s.node.suspicion
	s.m = s.node.memberlist

	s.m.MakeAlive(s.node.Address(), s.incarnation)

	s.suspect = Change{
		Address:     "127.0.0.1:3002",
		Incarnation: s.incarnation,
	}
}

func (s *SuspicionTestSuite) TearDownTest() {
	s.node.Destroy()
}

func (s *SuspicionTestSuite) TestEnableDisableSuspicion() {
	s.True(s.s.enabled, "expected suspicion to be enabled")

	s.s.Disable()
	s.False(s.s.enabled, "expected suspicion to be disabled")

	s.s.Reenable()
	s.True(s.s.enabled, "expected suspicion to be enabled")

	s.s.Reenable()
	s.True(s.s.enabled, "expected suspicion to be enabled")
}

func (s *SuspicionTestSuite) TestSuspectMember() {
	s.s.Start(s.suspect)
	s.NotNil(s.s.Timer(s.suspect.Address), "expected suspicion timer to be set")
}

func (s *SuspicionTestSuite) TestSuspectLocal() {
	s.s.Start(*s.m.local)
	s.Nil(s.s.Timer(s.m.local.Address), "expected suspicion timer for local member to be nil")
}

func (s *SuspicionTestSuite) TestSuspectDisabled() {
	s.s.Disable()
	s.False(s.s.enabled, "expected suspicion to be disabled")

	s.s.Start(s.suspect)
	s.Nil(s.s.timers[s.suspect.Address], "expected suspicion timer to be nil")
}

func (s *SuspicionTestSuite) TestSuspectBecomesFaulty() {
	s.s.timeout = time.Millisecond

	s.m.MakeAlive(s.suspect.Address, s.suspect.Incarnation)
	member, _ := s.m.Member(s.suspect.Address)
	s.Require().NotNil(member, "expected cannot be nil")

	s.s.Start(*member)
	s.NotNil(s.s.Timer(member.Address), "expected suspicion timer to be set")

	time.Sleep(2 * time.Millisecond)
	s.Equal(Faulty, member.Status, "expected member to be faulty")
}

// TestTimerCreated tests that starting suspicion for a node creates a
// countdown timer that is responsible for marking a node as faulty at the end
// of the timeout.
func (s *SuspicionTestSuite) TestTimerCreated() {
	s.m.MakeAlive(s.suspect.Address, s.suspect.Incarnation)
	member, _ := s.m.Member(s.suspect.Address)
	s.Require().NotNil(member, "expected cannot be nil")

	old := s.s.Timer(member.Address)
	s.Require().Nil(old, "expected timer to be nil")

	// Start suspcision, which should create a timer
	s.s.Start(*member)

	s.NotEqual(old, s.s.Timer(member.Address), "expected timer to change")
}

func (s *SuspicionTestSuite) TestSuspicionDisableStopsTimers() {
	s.s.Start(Change{Address: "127.0.0.1:3002", Incarnation: s.incarnation})
	s.s.Start(Change{Address: "127.0.0.1:3003", Incarnation: s.incarnation})
	s.s.Start(Change{Address: "127.0.0.1:3004", Incarnation: s.incarnation})

	s.Len(s.s.timers, 3, "expected 3 timers to be started")

	s.s.Disable()
	s.False(s.s.enabled, "expected suspicion to be disabled")
	s.Empty(s.s.timers, "expected all timers to be cleared")
}

func TestSuspicionTestSuite(t *testing.T) {
	suite.Run(t, new(SuspicionTestSuite))
}
