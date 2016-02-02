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

type RollupTestSuite struct {
	suite.Suite
	node           *Node
	r              *updateRollup
	incarnation    int64
	updates        []Change
	mockafterclock *mockAfterClock
}

func (s *RollupTestSuite) SetupTest() {
	s.mockafterclock = &mockAfterClock{systemClock: systemClock{}}
	s.node = NewNode("test", "127.0.0.1:3001", nil, &Options{
		RollupFlushInterval: 10000 * time.Second,
		RollupMaxUpdates:    10000,
		Clock:               s.mockafterclock,
	})
	s.r = s.node.rollup
	s.incarnation = util.TimeNowMS()
	s.updates = []Change{
		Change{Address: "one"},
		Change{Address: "two"},
	}
}

func (s *RollupTestSuite) TearDownTest() {
	s.node.Destroy() // destroys rollup
}
func (s *RollupTestSuite) TestTrackUpdatesEmpty() {
	s.r.TrackUpdates([]Change{})

	s.Empty(s.r.Buffer(), "expected no updates to be tracked")
}

func (s *RollupTestSuite) TestUpdatesAreTracked() {
	s.node.memberlist.MakeAlive("127.0.0.1:3002", s.incarnation)
	s.node.memberlist.MakeAlive("127.0.0.1:3003", s.incarnation)

	s.Len(s.r.Buffer(), 2, "expected two updates to be tracked")
}

func (s *RollupTestSuite) TestFlushBuffer() {
	s.r.AddUpdates(s.updates)

	s.Len(s.r.Buffer(), 2, "expected two updates to be in buffer")

	s.r.FlushBuffer()

	s.Empty(s.r.Buffer(), "expeted buffer to be empty")
}

func (s *RollupTestSuite) TestTrackUpdatesFlushes() {
	s.r.buffer.Lock()
	s.r.buffer.updates["one"] = []Change{}
	s.r.buffer.updates["two"] = []Change{}
	s.r.buffer.Unlock()

	s.Len(s.r.Buffer(), 2, "expected two updates to be in buffer")

	s.r.timings.lastUpdate = util.TimeZero() // simulate time passing

	s.r.TrackUpdates([]Change{Change{Address: "new"}})

	s.Len(s.r.Buffer(), 1, "expected one change")
}

func (s *RollupTestSuite) TestFlushingEmptiesTheBuffer() {
	s.r.TrackUpdates([]Change{
		Change{Address: "one"},
		Change{Address: "two"},
	})
	s.Len(s.r.Buffer(), 2, "expected two updates to be in buffer")
	s.r.FlushBuffer()
	s.Empty(s.r.Buffer(), "expected buffer to be flushed")
}

// Enabling the background flusher flushes buffer after flushInterval.
//
// This is a bit higher-level test than TestFlushingEmptiesTheBuffer.
// In this test, we mock the timer to trigger buffer flushing.
func (s *RollupTestSuite) TestFlushesAfterInterval() {
	s.r.TrackUpdates([]Change{
		Change{Address: "one"},
		Change{Address: "two"},
	})

	s.NotNil(s.r.FlushTimer(), "expected timer to be set")
	s.Len(s.r.Buffer(), 2, "expected two updates to be in buffer")

	s.mockafterclock.f() // this tick triggers FlushTimer

	s.Empty(s.r.Buffer(), "expected buffer to be flushed")
	s.NotNil(s.r.FlushTimer(), "expected timer to be renewed")
}

// This facility is needed to mock 'AfterFunc', so we can capture it's arguments.
// We will use the captured function later to mimic a fired timer.
type mockAfterClock struct {
	systemClock
	f func() // Function that was passed to AfterFunc.
}

func (c *mockAfterClock) AfterFunc(t time.Duration, f func()) *time.Timer {
	c.f = f
	return time.AfterFunc(t, f)
}

func (s *RollupTestSuite) TestFlushTwice() {
	s.r.flushInterval = time.Millisecond

	s.r.AddUpdates(s.updates)

	s.Len(s.r.Buffer(), 2, "expected two updates in buffer")
	s.r.FlushBuffer()
	s.Empty(s.r.Buffer(), "expected buffer to be empty")

	s.r.AddUpdates(s.updates)

	s.r.FlushBuffer()
	s.Empty(s.r.Buffer(), "expected buffer to be empty")
}

func TestRollupTestSuite(t *testing.T) {
	suite.Run(t, new(RollupTestSuite))
}
