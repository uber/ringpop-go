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

	"github.com/benbjohnson/clock"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/uber/ringpop-go/events"
	"github.com/uber/ringpop-go/forward"
	"github.com/uber/ringpop-go/swim"
	"github.com/uber/ringpop-go/test/mocks"
	"github.com/uber/tchannel-go"
)

type RingpopTestSuite struct {
	suite.Suite
	mockClock    *clock.Mock
	ringpop      *Ringpop
	channel      *tchannel.Channel
	mockRingpop  *mocks.Ringpop
	mockSwimNode *mocks.SwimNode
}

// createSingleNodeCluster is a helper function to create a single-node cluster
// during the tests
func createSingleNodeCluster(rp *Ringpop) error {
	// Bootstrapping with an empty list will created a single-node cluster.
	_, err := rp.Bootstrap(&swim.BootstrapOptions{
		Hosts: []string{},
	})

	return err
}

func (s *RingpopTestSuite) SetupTest() {
	s.mockClock = clock.NewMock()

	ch, err := tchannel.NewChannel("test", nil)
	s.NoError(err, "channel must create successfully")
	s.channel = ch

	s.ringpop, err = New("test", Identity("127.0.0.1:3001"), Channel(ch), Clock(s.mockClock))
	s.NoError(err, "Ringpop must create successfully")

	s.mockRingpop = &mocks.Ringpop{}
	s.mockSwimNode = &mocks.SwimNode{}

	s.mockSwimNode.On("Destroy").Return()
}

func (s *RingpopTestSuite) TearDownTest() {
	s.channel.Close()
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

	s.Equal(s.ringpop.ring.ServerCount(), 10)

	alive, faulty := genAddresses(1, 11, 15), genAddresses(1, 1, 5)
	s.ringpop.HandleEvent(swim.MemberlistChangesAppliedEvent{
		Changes: append(genChanges(alive, swim.Alive), genChanges(faulty, swim.Faulty)...),
	})

	s.Equal(s.ringpop.ring.ServerCount(), 10)
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
	s.Equal(int64(10), stats.vals["ringpop.127_0_0_1_3001.changes.apply"], "missing stats for applied changes")
	s.Equal(int64(10), stats.vals["ringpop.127_0_0_1_3001.updates"], "missing updates stats")
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.ring.checksum-computed"], "missing stats for checksums being computed")
	s.Equal(int64(10), stats.vals["ringpop.127_0_0_1_3001.membership-set.alive"], "missing stats for member being set to alive")
	s.Equal(int64(0 /* events are faked, ringpop still has 0 members */), stats.vals["ringpop.127_0_0_1_3001.num-members"], "missing num-members stats for member being set to alive")
	s.Equal(int64(10), stats.vals["ringpop.127_0_0_1_3001.membership-set.alive"], "missing stats for member being set to alive")
	// expected listener to record 3 events (forwarded swim event, checksum event, and ring changed event)

	s.ringpop.HandleEvent(swim.MemberlistChangesAppliedEvent{
		Changes: genChanges(genAddresses(1, 1, 1), swim.Faulty, swim.Leave, swim.Suspect),
	})
	s.Equal(int64(3), stats.vals["ringpop.127_0_0_1_3001.changes.apply"], "missing stats for applied changes for three status changes")
	s.Equal(int64(13), stats.vals["ringpop.127_0_0_1_3001.updates"], "missing updates stats for three status changes")
	s.Equal(int64(2 /* 1 + 1 from before */), stats.vals["ringpop.127_0_0_1_3001.ring.checksum-computed"], "missing stats for checksums being computed for three status changes")
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.membership-set.faulty"], "missing stats for member being set to faulty")
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.membership-set.leave"], "missing stats for member being set to leave")
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.membership-set.suspect"], "missing stats for member being set to suspect")
	s.Equal(int64(0 /* events are faked, ringpop still has 0 members */), stats.vals["ringpop.127_0_0_1_3001.num-members"], "missing num-members stats for three status changes")
	// expected listener to record 3 events (forwarded swim event, checksum event, and ring changed event)

	s.ringpop.HandleEvent(swim.MemberlistChangesAppliedEvent{
		Changes: genChanges(genAddresses(1, 1, 1), ""),
	})
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.changes.apply"], "missing stats for applied changes for unknown status change")
	s.Equal(int64(14), stats.vals["ringpop.127_0_0_1_3001.updates"], "missing updates stats for unknown status change")
	s.Equal(int64(2 /* 2 from before, no changes */), stats.vals["ringpop.127_0_0_1_3001.ring.checksum-computed"], "unexpected stats for checksums being computed for unknown status change")
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.membership-set.unknown"], "missing stats for member being set to unknown")
	s.Equal(int64(0 /* events are faked, ringpop still has 0 members */), stats.vals["ringpop.127_0_0_1_3001.num-members"], "missing num-members stats for member being set to unknown")
	// expected listener to record 3 events (forwarded swim event, checksum event, and ring changed event)

	s.ringpop.HandleEvent(swim.MaxPAdjustedEvent{NewPCount: 100})
	s.Equal(int64(100), stats.vals["ringpop.127_0_0_1_3001.max-piggyback"], "missing stats for piggyback adjustment")
	// expected listener to record 1 event

	s.ringpop.HandleEvent(swim.JoinReceiveEvent{})
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.join.recv"], "missing stats for joins received")
	// expected listener to record 1 event

	s.ringpop.HandleEvent(swim.JoinCompleteEvent{Duration: time.Second})
	s.Equal(int64(1000), stats.vals["ringpop.127_0_0_1_3001.join"], "missing stats for join initiated")
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.join.complete"], "missing stats for join completed")
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.join.succeeded"], "missing stats for join succeeded")
	// expected listener to record 1 event

	s.ringpop.HandleEvent(swim.PingSendEvent{})
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.ping.send"], "missing stats for sent pings")
	// expected listener to record 1 event

	s.ringpop.HandleEvent(swim.PingReceiveEvent{})
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.ping.recv"], "missing stats for received pings")
	// expected listener to record 1 event

	s.ringpop.HandleEvent(swim.PingRequestsSendEvent{Peers: genAddresses(1, 2, 5)})
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.ping-req.send"], "missing ping-req.send stats")
	s.Equal(int64(4), stats.vals["ringpop.127_0_0_1_3001.ping-req.other-members"], "missing ping-req.other-members stats")
	// expected listener to record 1 event

	s.ringpop.HandleEvent(swim.PingRequestReceiveEvent{})
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.ping-req.recv"], "missing stats for received ping-reqs")
	// expected listener to record 1 event

	s.ringpop.HandleEvent(swim.PingRequestPingEvent{Duration: time.Second})
	s.Equal(int64(1000), stats.vals["ringpop.127_0_0_1_3001.ping-req-ping"], "missing stats for ping-req pings executed")
	// expected listener to record 1 event

	s.ringpop.HandleEvent(swim.JoinFailedEvent{Reason: swim.Error})
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.join.failed.err"], "missing stats for join failed due to error")
	// expected listener to record 1 event

	s.ringpop.HandleEvent(swim.JoinFailedEvent{Reason: swim.Destroyed})
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.join.failed.destroyed"], "missing stats for join failed due to error")
	// expected listener to record 1 event

	s.ringpop.HandleEvent(swim.JoinTriesUpdateEvent{Retries: 1})
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.join.retries"], "missing stats for join retries")
	// expected listener to record 1 event

	s.ringpop.HandleEvent(swim.JoinTriesUpdateEvent{Retries: 2})
	s.Equal(int64(2), stats.vals["ringpop.127_0_0_1_3001.join.retries"], "join tries didn't update")
	// expected listener to record 1 event

	s.ringpop.HandleEvent(events.LookupEvent{
		Key:      "hello",
		Duration: time.Second,
	})
	s.Equal(int64(1000), stats.vals["ringpop.127_0_0_1_3001.lookup"], "missing lookup timer")
	// expected listener to record 1 event

	s.ringpop.HandleEvent(swim.MakeNodeStatusEvent{Status: swim.Alive})
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.make-alive"], "missing make-alive stat")
	// expected listener to record 1 event

	s.ringpop.HandleEvent(swim.MakeNodeStatusEvent{Status: swim.Faulty})
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.make-faulty"], "missing make-faulty stat")
	// expected listener to record 1 event

	s.ringpop.HandleEvent(swim.MakeNodeStatusEvent{Status: swim.Suspect})
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.make-suspect"], "missing make-suspect stat")
	// expected listener to record 1 event

	s.ringpop.HandleEvent(swim.MakeNodeStatusEvent{Status: swim.Leave})
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.make-leave"], "missing make-leave stat")
	// expected listener to record 1 event

	s.ringpop.HandleEvent(swim.ChecksumComputeEvent{
		Duration: 3 * time.Second,
		Checksum: 42,
	})
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.membership.checksum-computed"], "missing membership.checksum-computed stat")
	s.Equal(int64(42), stats.vals["ringpop.127_0_0_1_3001.checksum"], "missing checksum stat")
	s.Equal(int64(3000), stats.vals["ringpop.127_0_0_1_3001.compute-checksum"], "missing compute-checksum stat")
	// expected listener to record 1 event

	s.ringpop.HandleEvent(swim.RequestBeforeReadyEvent{Endpoint: swim.PingEndpoint})
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.not-ready.ping"], "missing not-ready.ping stat")
	// expected listener to record 1 event

	s.ringpop.HandleEvent(swim.RequestBeforeReadyEvent{Endpoint: swim.PingReqEndpoint})
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.not-ready.ping-req"], "missing not-ready.ping-req stat")
	// expected listener to record 1 event

	s.ringpop.HandleEvent(swim.RefuteUpdateEvent{})
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.refuted-update"], "missing refuted-update stat")

	// double check the counts before the event
	s.Equal(int64(10), stats.vals["ringpop.127_0_0_1_3001.ring.server-added"], "incorrect count for ring.server-added before RingChangedEvent")
	s.Equal(int64(2), stats.vals["ringpop.127_0_0_1_3001.ring.server-removed"], "incorrect count for ring.server-removed before RingChangedEvent")
	s.Equal(int64(2), stats.vals["ringpop.127_0_0_1_3001.ring.changed"], "incorrect count for ring.changed before RingChangedEvent")
	s.ringpop.HandleEvent(events.RingChangedEvent{
		ServersAdded:   genAddresses(1, 2, 5),
		ServersRemoved: genAddresses(1, 6, 8),
	})
	s.Equal(int64(14), stats.vals["ringpop.127_0_0_1_3001.ring.server-added"], "missing ring.server-added stat")
	s.Equal(int64(5), stats.vals["ringpop.127_0_0_1_3001.ring.server-removed"], "missing ring.server-removed stat")
	s.Equal(int64(3), stats.vals["ringpop.127_0_0_1_3001.ring.changed"], "missing ring.changed stat")

	s.ringpop.HandleEvent(forward.RequestForwardedEvent{})
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.requestProxy.egress"], "missing requestProxy.egress stat")
	// expected listener to record 1 event

	// forward.InflightRequestsChangedEvent:
	s.ringpop.HandleEvent(forward.InflightRequestsChangedEvent{Inflight: 5})
	s.Equal(int64(5), stats.vals["ringpop.127_0_0_1_3001.requestProxy.inflight"], "missing requestProxy.inflight stat")
	// expected listener to record 1 event

	// test an update on the Inflight requests count
	s.ringpop.HandleEvent(forward.InflightRequestsChangedEvent{Inflight: 4})
	s.Equal(int64(4), stats.vals["ringpop.127_0_0_1_3001.requestProxy.inflight"], "missing requestProxy.inflight stat (update)")
	// expected listener to record 1 event

	s.ringpop.HandleEvent(forward.InflightRequestsMiscountEvent{Operation: forward.InflightIncrement})
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.requestProxy.miscount.increment"], "missing requestProxy.miscount.increment stat")
	// expected listener to record 1 event

	s.ringpop.HandleEvent(forward.InflightRequestsMiscountEvent{Operation: forward.InflightDecrement})
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.requestProxy.miscount.decrement"], "missing requestProxy.miscount.decrement stat")
	// expected listener to record 1 event

	s.ringpop.HandleEvent(forward.SuccessEvent{})
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.requestProxy.send.success"], "missing requestProxy.send.success stat")
	// expected listener to record 1 event

	s.ringpop.HandleEvent(forward.FailedEvent{})
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.requestProxy.send.error"], "missing requestProxy.send.error stat")
	// expected listener to record 1 event

	s.ringpop.HandleEvent(forward.MaxRetriesEvent{MaxRetries: 3})
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.requestProxy.retry.failed"], "missing requestProxy.retry.failed stat")
	// expected listener to record 1 event

	s.ringpop.HandleEvent(forward.RetryAttemptEvent{})
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.requestProxy.retry.attempted"], "missing requestProxy.retry.attempted stat")
	// expected listener to record 1 event

	s.ringpop.HandleEvent(forward.RetryAbortEvent{})
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.requestProxy.retry.aborted"], "missing requestProxy.retry.aborted stat")
	// expected listener to record 1 event

	me, _ := s.ringpop.WhoAmI()
	s.ringpop.HandleEvent(forward.RerouteEvent{
		OldDestination: genAddresses(1, 1, 1)[0],
		NewDestination: me,
	})
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.requestProxy.retry.reroute.local"], "missing requestProxy.retry.reroute.local stat")
	// expected listener to record 1 event

	s.ringpop.HandleEvent(forward.RerouteEvent{
		OldDestination: genAddresses(1, 1, 1)[0],
		NewDestination: genAddresses(1, 2, 2)[0],
	})
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.requestProxy.retry.reroute.remote"], "missing requestProxy.retry.reroute.remote stat")
	// expected listener to record 1 event

	s.ringpop.HandleEvent(forward.RetrySuccessEvent{NumRetries: 1})
	s.Equal(int64(1), stats.vals["ringpop.127_0_0_1_3001.requestProxy.retry.succeeded"], "missing requestProxy.retry.reroute.remote stat")
	// expected listener to record 1 event

	time.Sleep(time.Millisecond) // sleep for a bit so that events can be recorded
	s.Equal(42, listener.EventCount(), "incorrect count for emitted events")
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
	defer ch.Close()

	rp, err := New("test2", Channel(ch))
	s.NoError(err)
	s.NotNil(rp)

	// Bootstrap that will fail
	_, err = rp.Bootstrap(&swim.BootstrapOptions{
		Hosts: []string{
			"127.0.0.1:9000",
			"127.0.0.1:9001",
		},
		// A MaxJoinDuration of 1 millisecond should fail immediately
		// without prolonging the test suite.
		MaxJoinDuration: time.Millisecond,
	})
	s.Error(err)

	s.Equal(initialized, rp.getState())
}

// TestStateReady tests that Ringpop is ready after successful bootstrapping.
func (s *RingpopTestSuite) TestStateReady() {
	// Bootstrap
	createSingleNodeCluster(s.ringpop)

	s.Equal(ready, s.ringpop.state)
}

// TestStateDestroyed tests that Ringpop is in a destroyed state after calling
// Destroy().
func (s *RingpopTestSuite) TestStateDestroyed() {
	// Bootstrap
	createSingleNodeCluster(s.ringpop)

	// Destroy
	s.ringpop.Destroy()
	s.Equal(destroyed, s.ringpop.state)
}

// TestDestroyFromCreated tests that Destroy() can be called straight away.
func (s *RingpopTestSuite) TestDestroyFromCreated() {
	// Ringpop starts in the created state
	s.Equal(created, s.ringpop.state)

	// Should be destroyed straight away
	s.ringpop.Destroy()
	s.Equal(destroyed, s.ringpop.state)
}

// TestDestroyFromInitialized tests that Destroy() can be called from the
// initialized state.
func (s *RingpopTestSuite) TestDestroyFromInitialized() {
	// Init
	s.ringpop.init()
	s.Equal(initialized, s.ringpop.state)

	s.ringpop.Destroy()
	s.Equal(destroyed, s.ringpop.state)
}

// TestDestroyIsIdempotent tests that Destroy() can be called multiple times.
func (s *RingpopTestSuite) TestDestroyIsIdempotent() {
	createSingleNodeCluster(s.ringpop)

	s.ringpop.Destroy()
	s.Equal(destroyed, s.ringpop.state)

	// Can destroy again
	s.ringpop.Destroy()
	s.Equal(destroyed, s.ringpop.state)
}

// TestWhoAmI tests that WhoAmI only operates when the Ringpop instance is in
// a ready state.
func (s *RingpopTestSuite) TestWhoAmI() {
	s.NotEqual(ready, s.ringpop.state)
	identity, err := s.ringpop.WhoAmI()
	s.Equal("", identity)
	s.Error(err)

	createSingleNodeCluster(s.ringpop)
	s.Equal(ready, s.ringpop.state)
	identity, err = s.ringpop.WhoAmI()
	s.NoError(err)
	s.Equal("127.0.0.1:3001", identity)
}

// TestUptime tests that Uptime only operates when the Ringpop instance is in
// a ready state.
func (s *RingpopTestSuite) TestUptime() {
	s.NotEqual(ready, s.ringpop.state)
	uptime, err := s.ringpop.Uptime()
	s.Zero(uptime)
	s.Error(err)

	createSingleNodeCluster(s.ringpop)
	s.Equal(ready, s.ringpop.state)
	uptime, err = s.ringpop.Uptime()
	s.NoError(err)
	s.NotZero(uptime)
}

// TestChecksum tests that Checksum only operates when the Ringpop instance is in
// a ready state.
func (s *RingpopTestSuite) TestChecksum() {
	s.NotEqual(ready, s.ringpop.state)
	checksum, err := s.ringpop.Checksum()
	s.Zero(checksum)
	s.Error(err)

	createSingleNodeCluster(s.ringpop)
	s.Equal(ready, s.ringpop.state)
	checksum, err = s.ringpop.Checksum()
	s.NoError(err)
	//s.NotZero(checksum)
}

// TestApp tests that App() returns the correct app name.
func (s *RingpopTestSuite) TestApp() {
	s.Equal("test", s.ringpop.App())
}

// TestLookupNotReady tests that Lookup fails when Ringpop is not ready.
func (s *RingpopTestSuite) TestLookupNotReady() {
	result, err := s.ringpop.Lookup("foo")
	s.Error(err)
	s.Empty(result)
}

// TestLookupNNotReady tests that LookupN fails when Ringpop is not ready.
func (s *RingpopTestSuite) TestLookupNNotReady() {
	result, err := s.ringpop.LookupN("foo", 3)
	s.Error(err)
	s.Nil(result)
}

// TestGetReachableMembersNotReady tests that GetReachableMembers fails when
// Ringpop is not ready.
func (s *RingpopTestSuite) TestGetReachableMembersNotReady() {
	result, err := s.ringpop.GetReachableMembers()
	s.Error(err)
	s.Nil(result)
}

// TestAddSelfToBootstrapList tests that Ringpop automatically adds its own
// identity to the bootstrap host list.
func (s *RingpopTestSuite) TestAddSelfToBootstrapList() {
	// Init ringpop, but then override the swim node with mock node
	s.ringpop.init()
	s.ringpop.node = s.mockSwimNode

	s.mockSwimNode.On("Bootstrap", mock.Anything).Return(
		[]string{"127.0.0.1:3001", "127.0.0.1:3002"},
		nil,
	)

	// Call Bootstrap with ourselves missing
	_, err := s.ringpop.Bootstrap(&swim.BootstrapOptions{
		Hosts: []string{"127.0.0.1:3002"},
	})

	// Test that self was added
	s.mockSwimNode.AssertCalled(s.T(), "Bootstrap", &swim.BootstrapOptions{
		Hosts: []string{"127.0.0.1:3002", "127.0.0.1:3001"},
	})
	s.Nil(err)
}

// TestDontAddSelfForFileBootstrap tests that Ringpop only adds itself to the
// bootstrap host list automatically, if Hosts is the bootstrap mode.
func (s *RingpopTestSuite) TestDontAddSelfForFileBootstrap() {
	// Init ringpop, but then override the swim node with mock node
	s.ringpop.init()
	s.ringpop.node = s.mockSwimNode

	s.mockSwimNode.On("Bootstrap", mock.Anything).Return(
		[]string{"127.0.0.1:3001", "127.0.0.1:3002"},
		nil,
	)

	// Call Bootstrap with File option and no Hosts
	_, err := s.ringpop.Bootstrap(&swim.BootstrapOptions{
		File: "./hosts.json",
	})

	// Test that Hosts is still empty
	s.mockSwimNode.AssertCalled(s.T(), "Bootstrap", &swim.BootstrapOptions{
		File: "./hosts.json",
	})
	s.Nil(err)
}

// TestEmptyJoinListCreatesSingleNodeCluster tests that when you call Bootstrap
// with no hosts or bootstrap file, a single-node cluster is created.
func (s *RingpopTestSuite) TestEmptyJoinListCreatesSingleNodeCluster() {
	createSingleNodeCluster(s.ringpop)
	s.Equal(ready, s.ringpop.state)
}

func (s *RingpopTestSuite) TestErrorOnChannelNotListening() {
	ch, err := tchannel.NewChannel("test", nil)
	s.Require().NoError(err)

	rp, err := New("test", Channel(ch))
	s.Require().NoError(err)

	nodesJoined, err := rp.Bootstrap(&swim.BootstrapOptions{})
	s.Exactly(err, ErrEphemeralIdentity)
	s.Nil(nodesJoined)
}

func TestRingpopTestSuite(t *testing.T) {
	suite.Run(t, new(RingpopTestSuite))
}

func (s *RingpopTestSuite) TestStartTimersIdempotance() {
	// starts empty. note that we use empty here to not overspecify --
	// whether it's nil or an empty slice isn't really relevant, so long as
	// it behaves well.
	s.Empty(s.ringpop.tickers)

	// init should default to have at least 1 timer --
	// RingChecksumStatPeriod
	s.ringpop.init()
	s.NotEmpty(s.ringpop.tickers)

	// validate idemopotence of startTimers
	s.ringpop.startTimers()

	// destroy works, and leaves it empty
	s.ringpop.Destroy()
	s.Empty(s.ringpop.tickers)

	// idempotent stop
	s.ringpop.stopTimers()
}

func (s *RingpopTestSuite) TestRingChecksumEmitTimer() {
	s.ringpop.init()
	stats := newDummyStats()
	s.ringpop.statter = stats
	s.mockClock.Add(5 * time.Second)
	_, ok := stats.vals["ringpop.127_0_0_1_3001.ring.checksum-periodic"]
	s.True(ok, "missing stats for checksums being computed")
	s.ringpop.Destroy()
}
