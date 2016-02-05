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

	"github.com/benbjohnson/clock"
	"github.com/stretchr/testify/suite"
	"github.com/uber/ringpop-go/util"
)

type RollupTestSuite struct {
	suite.Suite
	node        *Node
	r           *updateRollup
	incarnation int64
	updates     []Change
}

func (s *RollupTestSuite) SetupTest() {
	s.node = NewNode("test", "127.0.0.1:3001", nil, &Options{
		RollupFlushInterval: 10000 * time.Second,
		RollupMaxUpdates:    10000,
		Clock:               clock.NewMock(),
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

func (s *RollupTestSuite) TestFlushesAfterInterval() {
	s.r.flushInterval = time.Millisecond

	s.r.TrackUpdates([]Change{
		Change{Address: "one"},
		Change{Address: "two"},
	})

	s.NotNil(s.r.FlushTimer(), "expected timer to be set")
	s.Len(s.r.Buffer(), 2, "expected two updates to be in buffer")

	s.node.clock.(*clock.Mock).Add(2 * time.Millisecond)

	s.Empty(s.r.Buffer(), "expected buffer to be flushed")
	s.NotNil(s.r.FlushTimer(), "expected timer to be renewed")
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
