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

package ringpop

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/uber/ringpop-go/swim"
	"github.com/uber/ringpop-go/test/mocks"
	"github.com/uber/tchannel-go"
)

type RingpopTestSuite struct {
	suite.Suite
	ringpop     *Ringpop
	channel     *tchannel.Channel
	mockRingpop *mocks.Ringpop
}

func (s *RingpopTestSuite) SetupTest() {

	ch, err := tchannel.NewChannel("test", nil)
	s.NoError(err, "channel must create successfully")
	s.channel = ch

	s.ringpop, err = New("test", Identity("127.0.0.1:3001"), Channel(ch))
	s.NoError(err, "Ringpop must create successfully")

	s.mockRingpop = &mocks.Ringpop{}
}

func (s *RingpopTestSuite) TearDownTest() {
	s.ringpop.Destroy()
}

func (s *RingpopTestSuite) TestCanAssignRingpopToRingpopInterface() {
	var ri Interface
	ri = s.ringpop

	s.Assert().Equal(ri, s.ringpop, "ringpop in the interface is not equal to ringpop")
}

func (s *RingpopTestSuite) TestHandlesMemberlistChangeEvent() {
	// Fake bootstrap
	s.ringpop.init()

	s.ringpop.HandleEvent(swim.MemberlistChangesAppliedEvent{
		Changes: genChanges(genAddresses(1, 1, 10), swim.Alive),
	})

	s.Len(s.ringpop.ring.GetServers(), 10)

	alive, faulty := genAddresses(1, 11, 15), genAddresses(1, 1, 5)
	s.ringpop.HandleEvent(swim.MemberlistChangesAppliedEvent{
		Changes: append(genChanges(alive, swim.Alive), genChanges(faulty, swim.Faulty)...),
	})

	s.Len(s.ringpop.ring.GetServers(), 10)
	for _, address := range alive {
		s.True(s.ringpop.ring.HasServer(address))
	}
	for _, address := range faulty {
		s.False(s.ringpop.ring.HasServer(address))
	}

	leave, suspect := genAddresses(1, 7, 10), genAddresses(1, 11, 15)
	s.ringpop.HandleEvent(swim.MemberlistChangesAppliedEvent{
		Changes: append(genChanges(leave, swim.Leave), genChanges(suspect, swim.Suspect)...),
	})
	for _, address := range leave {
		s.False(s.ringpop.ring.HasServer(address))
	}
	for _, address := range suspect {
		s.True(s.ringpop.ring.HasServer(address))
	}
}

func (s *RingpopTestSuite) TestHandleEvents() {
	// Fake bootstrap
	s.ringpop.init()

	stats := newDummyStats()
	s.ringpop.statter = stats

	listener := &dummyListener{}
	s.ringpop.RegisterListener(listener)

	s.ringpop.HandleEvent(swim.MemberlistChangesAppliedEvent{
		Changes: genChanges(genAddresses(1, 1, 10), swim.Alive),
	})
	s.Equal(int64(10), stats.vals["ringpop.127_0_0_1_3001.changes.apply"])
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.ring.checksum-computed"])
	// expected listener to record 3 events (forwarded swim event, checksum event,
	// and ring changed event)

	s.ringpop.HandleEvent(swim.MaxPAdjustedEvent{NewPCount: 100})
	s.Equal(int64(100), stats.vals["ringpop.127_0_0_1_3001.max-p"])
	// expected listener to record 1 event

	s.ringpop.HandleEvent(swim.JoinReceiveEvent{})
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.join.recv"])
	// expected listener to record 1 event

	s.ringpop.HandleEvent(swim.JoinCompleteEvent{Duration: time.Second})
	s.Equal(int64(1000), stats.vals["ringpop.127_0_0_1_3001.join"])
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.join.complete"])
	// expected listener to record 1 event

	s.ringpop.HandleEvent(swim.PingSendEvent{})
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.ping.send"])
	// expected listener to record 1 event

	s.ringpop.HandleEvent(swim.PingReceiveEvent{})
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.ping.recv"])
	// expected listener to record 1 event

	s.ringpop.HandleEvent(swim.PingRequestsSendEvent{Peers: genAddresses(1, 2, 5)})
	s.Equal(int64(4), stats.vals["ringpop.127_0_0_1_3001.ping-req.send"])
	// expected listener to record 1 event

	s.ringpop.HandleEvent(swim.PingRequestReceiveEvent{})
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.ping-req.recv"])
	// expected listener to record 1 event

	s.ringpop.HandleEvent(swim.PingRequestPingEvent{Duration: time.Second})
	s.Equal(int64(1000), stats.vals["ringpop.127_0_0_1_3001.ping-req.ping"])
	// expected listener to record 1 event

	time.Sleep(time.Millisecond) // sleep for a bit so that events can be recorded
	s.Equal(11, listener.EventCount(), "expected 11 total events to be recorded")
}

func (s *RingpopTestSuite) TestRingpopReady() {
	s.False(s.ringpop.Ready())
	// Create single node cluster.
	s.ringpop.Bootstrap(&swim.BootstrapOptions{
		Hosts: []string{"127.0.0.1:3001"},
	})
	s.True(s.ringpop.Ready())
}

func (s *RingpopTestSuite) TestRingpopNotReady() {
	// Ringpop should not be ready until bootstrapped
	s.False(s.ringpop.Ready())
}

// TestStateCreated tests that Ringpop is in a created state just after
// instantiating.
func (s *RingpopTestSuite) TestStateCreated() {
	s.Equal(created, s.ringpop.getState())
}

// TestStateInitialized tests that Ringpop is in an initialized state after
// a failed bootstrap attempt.
func (s *RingpopTestSuite) TestStateInitialized() {

	// Create channel and start listening so we can actually attempt to
	// bootstrap
	ch, _ := tchannel.NewChannel("test2", nil)
	ch.ListenAndServe("127.0.0.1:0")

	rp, err := New("test2", Channel(ch))
	s.NoError(err)
	s.NotNil(rp)

	// Bootstrap that will fail
	_, err = rp.Bootstrap(&BootstrapOptions{
		swim.BootstrapOptions{
			Hosts: []string{
				"127.0.0.1:9000",
				"127.0.0.1:9001",
			},
			// A MaxJoinDuration of 1 millisecond should fail immediately
			// without prolonging the test suite.
			MaxJoinDuration: time.Millisecond,
		},
	})
	s.Error(err)

	s.Equal(initialized, rp.getState())
}

// TestStateReady tests that Ringpop is ready after successful bootstrapping.
func (s *RingpopTestSuite) TestStateReady() {

	// Create channel and start listening so we can actually attempt to
	// bootstrap
	ch, _ := tchannel.NewChannel("test2", nil)
	ch.ListenAndServe("127.0.0.1:0")

	rp, err := New("test2", Channel(ch))
	s.NoError(err)
	s.NotNil(rp)

	// Create single node cluster.
	_, err = rp.Bootstrap(&BootstrapOptions{
		swim.BootstrapOptions{
			Hosts: []string{"127.0.0.1:3001"},
		},
	})
	s.NoError(err)

	s.Equal(ready, rp.state)
}

// TestStateDestroyed tests that Ringpop is in a destroyed state after calling
// Destroy().
func (s *RingpopTestSuite) TestStateDestroyed() {

	// Create single node cluster.
	_, err := s.ringpop.Bootstrap(&BootstrapOptions{
		swim.BootstrapOptions{
			Hosts: []string{"127.0.0.1:3001"},
		},
	})
	s.NoError(err)

	// Destroy
	s.ringpop.Destroy()
	s.Equal(destroyed, s.ringpop.state)
}

// TestDestroyIsIdempotent tests that Destroy() can be called multiple times.
func (s *RingpopTestSuite) TestDestroyIsIdempotent() {
	// Ringpop starts in the created state
	s.Equal(created, s.ringpop.state)

	// Should be destroyed straight away
	s.ringpop.Destroy()
	s.Equal(destroyed, s.ringpop.state)

	// Can destroy again
	s.ringpop.Destroy()
	s.Equal(destroyed, s.ringpop.state)
}

func TestRingpopTestSuite(t *testing.T) {
	suite.Run(t, new(RingpopTestSuite))
}
