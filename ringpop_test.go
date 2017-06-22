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
	"sync"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/uber/ringpop-go/discovery/statichosts"
	"github.com/uber/ringpop-go/events"
	eventsmocks "github.com/uber/ringpop-go/events/test/mocks"
	"github.com/uber/ringpop-go/forward"
	"github.com/uber/ringpop-go/membership"
	"github.com/uber/ringpop-go/swim"
	"github.com/uber/ringpop-go/test/mocks"
	"github.com/uber/tchannel-go"
)

type destroyable interface {
	Destroy()
}

type destroyableChannel struct {
	*tchannel.Channel
}

func (c *destroyableChannel) Destroy() {
	if c.Channel != nil {
		c.Channel.Close()
	}
}

type RingpopTestSuite struct {
	suite.Suite
	mockClock    *clock.Mock
	ringpop      *Ringpop
	channel      *tchannel.Channel
	mockRingpop  *mocks.Ringpop
	mockSwimNode *mocks.SwimNode
	stats        *dummyStats

	destroyables []destroyable
}

func (s *RingpopTestSuite) makeNewRingpop() (rp *Ringpop, err error) {
	ch, err := tchannel.NewChannel("test", nil)
	s.NoError(err, "channel must create successfully")

	err = ch.ListenAndServe("127.0.0.1:0")
	s.NoError(err, "channel must listen successfully")

	rp, err = New("test", Channel(ch), Clock(s.mockClock))
	s.NoError(err, "Ringpop must create successfully")

	// collect ringpop and tchannel for destruction later
	s.destroyables = append(s.destroyables, &destroyableChannel{ch}, rp)

	return
}

// createSingleNodeCluster is a helper function to create a single-node cluster
// during the tests
func createSingleNodeCluster(rp *Ringpop) error {
	// Bootstrapping with an empty list will created a single-node cluster.
	_, err := rp.Bootstrap(&swim.BootstrapOptions{
		DiscoverProvider: statichosts.New(),
	})

	return err
}

func (s *RingpopTestSuite) SetupTest() {
	s.mockClock = clock.NewMock()

	ch, err := tchannel.NewChannel("test", nil)
	s.NoError(err, "channel must create successfully")
	s.channel = ch

	s.stats = newDummyStats()

	s.ringpop, err = New("test",
		Address("127.0.0.1:3001"),
		Channel(ch),
		Clock(s.mockClock),
		Statter(s.stats),

		// configure low limits for testing of enforcement and error propagation
		LabelLimitCount(1),
		LabelLimitKeySize(5),
		LabelLimitValueSize(5),
		RequiresAppInPing(true),
	)
	s.NoError(err, "Ringpop must create successfully")

	ch.ListenAndServe(":0")

	s.mockRingpop = &mocks.Ringpop{}
	s.mockSwimNode = &mocks.SwimNode{}

	s.mockSwimNode.On("Destroy").Return()
}

func (s *RingpopTestSuite) TearDownTest() {
	s.channel.Close()
	s.ringpop.Destroy()

	// clean up all the things
	for _, d := range s.destroyables {
		if d != nil {
			d.Destroy()
		}
	}
}

func (s *RingpopTestSuite) TestCanAssignRingpopToRingpopInterface() {
	var ri Interface
	ri = s.ringpop

	s.Assert().Equal(ri, s.ringpop, "ringpop in the interface is not equal to ringpop")
}

func (s *RingpopTestSuite) TestHandlesMemberlistChangeEvent() {
	// Fake bootstrap
	s.ringpop.init()

	s.ringpop.HandleEvent(membership.ChangeEvent{
		Changes: genMembershipChanges(genMembers(genAddresses(1, 1, 10)), AfterMemberField),
	})

	s.Equal(10, s.ringpop.ring.ServerCount())

	alive, faulty := genAddresses(1, 11, 15), genAddresses(1, 1, 5)
	s.ringpop.HandleEvent(membership.ChangeEvent{
		Changes: append(
			genMembershipChanges(genMembers(alive), AfterMemberField),
			genMembershipChanges(genMembers(faulty), BeforeMemberField)...,
		),
	})

	s.Equal(10, s.ringpop.ring.ServerCount())
	for _, address := range alive {
		s.True(s.ringpop.ring.HasServer(address))
	}
	for _, address := range faulty {
		s.False(s.ringpop.ring.HasServer(address))
	}
}

func (s *RingpopTestSuite) TestEventPropagation() {
	var wg sync.WaitGroup
	wg.Add(1)
	l := &eventsmocks.EventListener{}
	l.On("HandleEvent", struct{}{}).Run(func(args mock.Arguments) {
		wg.Done()
	})

	s.ringpop.AddListener(l)
	defer s.ringpop.RemoveListener(l)

	s.ringpop.HandleEvent(struct{}{})

	// wait for the event being fired asynchronous
	wg.Wait()

	l.AssertCalled(s.T(), "HandleEvent", struct{}{})
}

func (s *RingpopTestSuite) TestHandleEvents() {
	// Fake bootstrap
	s.ringpop.init()

	// remove stats from init phase
	s.stats.clear()

	alive := genAddresses(1, 1, 10)
	s.ringpop.HandleEvent(swim.MemberlistChangesAppliedEvent{
		Changes: genChanges(alive, swim.Alive),
	})
	s.ringpop.HandleEvent(membership.ChangeEvent{
		Changes: genMembershipChanges(genMembers(alive), AfterMemberField),
	})

	s.Equal(int64(10), s.stats.read("ringpop.127_0_0_1_3001.changes.apply"), "missing stats for applied changes")
	s.Equal(int64(10), s.stats.read("ringpop.127_0_0_1_3001.updates"), "missing updates stats")
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.ring.checksum-computed"), "missing stats for checksums being computed")
	s.Equal(int64(10), s.stats.read("ringpop.127_0_0_1_3001.membership-set.alive"), "missing stats for member being set to alive")
	s.Equal(int64(0 /* events are faked, ringpop still has 0 members */), s.stats.read("ringpop.127_0_0_1_3001.num-members"), "missing num-members stats for member being set to alive")
	s.Equal(int64(10), s.stats.read("ringpop.127_0_0_1_3001.membership-set.alive"), "missing stats for member being set to alive")

	s.ringpop.HandleEvent(swim.MemberlistChangesAppliedEvent{
		Changes: genChanges(alive[0:1], swim.Faulty, swim.Leave, swim.Suspect),
	})
	s.ringpop.HandleEvent(membership.ChangeEvent{
		Changes: genMembershipChanges(genMembers(alive[0:1]), BeforeMemberField),
	})
	s.Equal(int64(3), s.stats.read("ringpop.127_0_0_1_3001.changes.apply"), "missing stats for applied changes for three status changes")
	s.Equal(int64(13), s.stats.read("ringpop.127_0_0_1_3001.updates"), "missing updates stats for three status changes")
	s.Equal(int64(2 /* 1 + 1 from before */), s.stats.read("ringpop.127_0_0_1_3001.ring.checksum-computed"), "missing stats for checksums being computed for three status changes")
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.membership-set.faulty"), "missing stats for member being set to faulty")
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.membership-set.leave"), "missing stats for member being set to leave")
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.membership-set.suspect"), "missing stats for member being set to suspect")
	s.Equal(int64(0 /* events are faked, ringpop still has 0 members */), s.stats.read("ringpop.127_0_0_1_3001.num-members"), "missing num-members stats for three status changes")

	s.ringpop.HandleEvent(swim.MemberlistChangesAppliedEvent{
		Changes: genChanges(genAddresses(1, 1, 1), ""),
	})
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.changes.apply"), "missing stats for applied changes for unknown status change")
	s.Equal(int64(14), s.stats.read("ringpop.127_0_0_1_3001.updates"), "missing updates stats for unknown status change")
	s.Equal(int64(2 /* 2 from before, no changes */), s.stats.read("ringpop.127_0_0_1_3001.ring.checksum-computed"), "unexpected stats for checksums being computed for unknown status change")
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.membership-set.unknown"), "missing stats for member being set to unknown")
	s.Equal(int64(0 /* events are faked, ringpop still has 0 members */), s.stats.read("ringpop.127_0_0_1_3001.num-members"), "missing num-members stats for member being set to unknown")

	s.ringpop.HandleEvent(swim.FullSyncEvent{})
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.full-sync"), "missing stats for full sync")

	s.ringpop.HandleEvent(swim.StartReverseFullSyncEvent{})
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.full-sync.reverse"), "missing stats for reverse full sync")

	s.ringpop.HandleEvent(swim.OmitReverseFullSyncEvent{})
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.full-sync.reverse-omitted"), "missing stats for omitted reverse full sync")

	s.ringpop.HandleEvent(swim.RedundantReverseFullSyncEvent{})
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.full-sync.redundant-reverse"), "missing stats for redundant reverse full sync")

	s.ringpop.HandleEvent(swim.MaxPAdjustedEvent{NewPCount: 100})
	s.Equal(int64(100), s.stats.read("ringpop.127_0_0_1_3001.max-piggyback"), "missing stats for piggyback adjustment")

	s.ringpop.HandleEvent(swim.JoinReceiveEvent{})
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.join.recv"), "missing stats for joins received")

	s.ringpop.HandleEvent(swim.JoinCompleteEvent{Duration: time.Second})
	s.Equal(int64(1000), s.stats.read("ringpop.127_0_0_1_3001.join"), "missing stats for join initiated")
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.join.complete"), "missing stats for join completed")
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.join.succeeded"), "missing stats for join succeeded")

	s.ringpop.HandleEvent(swim.PingSendEvent{})
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.ping.send"), "missing stats for sent pings")

	s.ringpop.HandleEvent(swim.PingReceiveEvent{})
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.ping.recv"), "missing stats for received pings")

	s.ringpop.HandleEvent(swim.PingRequestsSendEvent{Peers: genAddresses(1, 2, 5)})
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.ping-req.send"), "missing ping-req.send stats")
	s.Equal(int64(4), s.stats.read("ringpop.127_0_0_1_3001.ping-req.other-members"), "missing ping-req.other-members stats")

	s.ringpop.HandleEvent(swim.PingRequestReceiveEvent{})
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.ping-req.recv"), "missing stats for received ping-reqs")

	s.ringpop.HandleEvent(swim.PingRequestPingEvent{Duration: time.Second})
	s.Equal(int64(1000), s.stats.read("ringpop.127_0_0_1_3001.ping-req-ping"), "missing stats for ping-req pings executed")

	s.ringpop.HandleEvent(swim.JoinFailedEvent{Reason: swim.Error})
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.join.failed.err"), "missing stats for join failed due to error")

	s.ringpop.HandleEvent(swim.JoinFailedEvent{Reason: swim.Destroyed})
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.join.failed.destroyed"), "missing stats for join failed due to error")

	s.ringpop.HandleEvent(swim.JoinTriesUpdateEvent{Retries: 1})
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.join.retries"), "missing stats for join retries")

	s.ringpop.HandleEvent(swim.JoinTriesUpdateEvent{Retries: 2})
	s.Equal(int64(2), s.stats.read("ringpop.127_0_0_1_3001.join.retries"), "join tries didn't update")

	s.ringpop.HandleEvent(swim.DiscoHealEvent{})
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.heal.triggered"), "missing stats for received pings")

	s.ringpop.HandleEvent(swim.AttemptHealEvent{})
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.heal.attempt"), "missing stats for received pings")

	s.ringpop.HandleEvent(swim.MakeNodeStatusEvent{Status: swim.Alive})
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.make-alive"), "missing make-alive stat")

	s.ringpop.HandleEvent(swim.MakeNodeStatusEvent{Status: swim.Faulty})
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.make-faulty"), "missing make-faulty stat")

	s.ringpop.HandleEvent(swim.MakeNodeStatusEvent{Status: swim.Suspect})
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.make-suspect"), "missing make-suspect stat")

	s.ringpop.HandleEvent(swim.MakeNodeStatusEvent{Status: swim.Leave})
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.make-leave"), "missing make-leave stat")

	s.ringpop.HandleEvent(swim.ChecksumComputeEvent{
		Duration: 3 * time.Second,
		Checksum: 42,
	})
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.membership.checksum-computed"), "missing membership.checksum-computed stat")
	s.Equal(int64(42), s.stats.read("ringpop.127_0_0_1_3001.checksum"), "missing checksum stat")
	s.Equal(int64(3000), s.stats.read("ringpop.127_0_0_1_3001.compute-checksum"), "missing compute-checksum stat")

	s.ringpop.HandleEvent(swim.RequestBeforeReadyEvent{Endpoint: swim.PingEndpoint})
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.not-ready.ping"), "missing not-ready.ping stat")

	s.ringpop.HandleEvent(swim.RequestBeforeReadyEvent{Endpoint: swim.PingReqEndpoint})
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.not-ready.ping-req"), "missing not-ready.ping-req stat")

	s.ringpop.HandleEvent(swim.RefuteUpdateEvent{})
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.refuted-update"), "missing refuted-update stat")

	// double check the counts before the event
	s.Equal(int64(9), s.stats.read("ringpop.127_0_0_1_3001.ring.server-added"), "incorrect count for ring.server-added before RingChangedEvent")
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.ring.server-removed"), "incorrect count for ring.server-removed before RingChangedEvent")
	s.Equal(int64(2), s.stats.read("ringpop.127_0_0_1_3001.ring.changed"), "incorrect count for ring.changed before RingChangedEvent")
	s.ringpop.HandleEvent(events.RingChangedEvent{
		ServersAdded:   genAddresses(1, 2, 5),
		ServersRemoved: genAddresses(1, 6, 8),
	})
	s.Equal(int64(13), s.stats.read("ringpop.127_0_0_1_3001.ring.server-added"), "missing ring.server-added stat")
	s.Equal(int64(4), s.stats.read("ringpop.127_0_0_1_3001.ring.server-removed"), "missing ring.server-removed stat")
	s.Equal(int64(3), s.stats.read("ringpop.127_0_0_1_3001.ring.changed"), "missing ring.changed stat")

	s.ringpop.HandleEvent(forward.RequestForwardedEvent{})
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.requestProxy.egress"), "missing requestProxy.egress stat")

	// forward.InflightRequestsChangedEvent:
	s.ringpop.HandleEvent(forward.InflightRequestsChangedEvent{Inflight: 5})
	s.Equal(int64(5), s.stats.read("ringpop.127_0_0_1_3001.requestProxy.inflight"), "missing requestProxy.inflight stat")

	// test an update on the Inflight requests count
	s.ringpop.HandleEvent(forward.InflightRequestsChangedEvent{Inflight: 4})
	s.Equal(int64(4), s.stats.read("ringpop.127_0_0_1_3001.requestProxy.inflight"), "missing requestProxy.inflight stat (update)")

	s.ringpop.HandleEvent(forward.InflightRequestsMiscountEvent{Operation: forward.InflightIncrement})
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.requestProxy.miscount.increment"), "missing requestProxy.miscount.increment stat")

	s.ringpop.HandleEvent(forward.InflightRequestsMiscountEvent{Operation: forward.InflightDecrement})
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.requestProxy.miscount.decrement"), "missing requestProxy.miscount.decrement stat")

	s.ringpop.HandleEvent(forward.SuccessEvent{})
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.requestProxy.send.success"), "missing requestProxy.send.success stat")

	s.ringpop.HandleEvent(forward.FailedEvent{})
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.requestProxy.send.error"), "missing requestProxy.send.error stat")

	s.ringpop.HandleEvent(forward.MaxRetriesEvent{MaxRetries: 3})
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.requestProxy.retry.failed"), "missing requestProxy.retry.failed stat")

	s.ringpop.HandleEvent(forward.RetryAttemptEvent{})
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.requestProxy.retry.attempted"), "missing requestProxy.retry.attempted stat")

	s.ringpop.HandleEvent(forward.RetryAbortEvent{})
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.requestProxy.retry.aborted"), "missing requestProxy.retry.aborted stat")

	me, _ := s.ringpop.WhoAmI()
	s.ringpop.HandleEvent(forward.RerouteEvent{
		OldDestination: genAddresses(1, 1, 1)[0],
		NewDestination: me,
	})
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.requestProxy.retry.reroute.local"), "missing requestProxy.retry.reroute.local stat")

	s.ringpop.HandleEvent(forward.RerouteEvent{
		OldDestination: genAddresses(1, 1, 1)[0],
		NewDestination: genAddresses(1, 2, 2)[0],
	})
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.requestProxy.retry.reroute.remote"), "missing requestProxy.retry.reroute.remote stat")

	s.ringpop.HandleEvent(forward.RetrySuccessEvent{NumRetries: 1})
	s.Equal(int64(1), s.stats.read("ringpop.127_0_0_1_3001.requestProxy.retry.succeeded"), "missing requestProxy.retry.reroute.remote stat")

	s.ringpop.HandleEvent(swim.SelfEvictedEvent{
		PhasesCount: 4,
		Duration:    time.Duration(1) * time.Second,
	})
	s.Equal(int64(1000), s.stats.read("ringpop.127_0_0_1_3001.self-eviction"), "missing self-eviction stat")

}

func (s *RingpopTestSuite) TestRingpopReady() {
	s.False(s.ringpop.Ready())
	// Create single node cluster.
	s.ringpop.Bootstrap(&swim.BootstrapOptions{
		DiscoverProvider: statichosts.New("127.0.0.1:3001"),
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
		DiscoverProvider: statichosts.New("127.0.0.1:9000", "127.0.0.1:9001"),
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
	err := createSingleNodeCluster(s.ringpop)
	s.Require().NoError(err, "unable to bootstrap single node cluster")

	s.Equal(ready, s.ringpop.state)
}

// TestStateDestroyed tests that Ringpop is in a destroyed state after calling
// Destroy().
func (s *RingpopTestSuite) TestStateDestroyed() {
	// Bootstrap
	err := createSingleNodeCluster(s.ringpop)
	s.Require().NoError(err, "unable to bootstrap single node cluster")

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
	err := createSingleNodeCluster(s.ringpop)
	s.Require().NoError(err, "unable to bootstrap single node cluster")

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
	address, err := s.ringpop.WhoAmI()
	s.Equal("", address)
	s.Error(err)

	err = createSingleNodeCluster(s.ringpop)
	s.Require().NoError(err, "unable to bootstrap single node cluster")

	s.Equal(ready, s.ringpop.state)
	address, err = s.ringpop.WhoAmI()
	s.NoError(err)
	s.Equal("127.0.0.1:3001", address)
}

// TestUptime tests that Uptime only operates when the Ringpop instance is in
// a ready state.
func (s *RingpopTestSuite) TestUptime() {
	s.NotEqual(ready, s.ringpop.state)
	uptime, err := s.ringpop.Uptime()
	s.Zero(uptime)
	s.Error(err)

	err = createSingleNodeCluster(s.ringpop)
	s.Require().NoError(err, "unable to bootstrap single node cluster")

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

	err = createSingleNodeCluster(s.ringpop)
	s.Require().NoError(err, "unable to bootstrap single node cluster")

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

func (s *RingpopTestSuite) TestLookupNoDestination() {
	err := createSingleNodeCluster(s.ringpop)
	s.Require().NoError(err, "unable to bootstrap single node cluster")

	address, _ := s.ringpop.address()
	member := fakeMember{
		address: address,
	}
	s.ringpop.ring.RemoveMembers(member)

	result, err := s.ringpop.Lookup("foo")
	s.Equal("", result)
	s.Error(err)
}

func (s *RingpopTestSuite) TestLookupEmitStat() {
	err := createSingleNodeCluster(s.ringpop)
	s.Require().NoError(err, "unable to bootstrap single node cluster")

	_, _ = s.ringpop.Lookup("foo")

	ok := s.stats.has("ringpop.127_0_0_1_3001.lookup")
	s.True(ok, "missing lookup timer")
}

func (s *RingpopTestSuite) TestLookupNEmitStat() {
	err := createSingleNodeCluster(s.ringpop)
	s.Require().NoError(err, "unable to bootstrap single node cluster")

	_, _ = s.ringpop.LookupN("foo", 3)
	_, _ = s.ringpop.LookupN("foo", 5)

	ok := s.stats.has("ringpop.127_0_0_1_3001.lookupn.3")
	s.True(ok, "missing lookupn.3 timer")

	ok = s.stats.has("ringpop.127_0_0_1_3001.lookupn.5")
	s.True(ok, "missing lookupn.5 timer")
}

// TestLookupNNotReady tests that LookupN fails when Ringpop is not ready.
func (s *RingpopTestSuite) TestLookupNNotReady() {
	result, err := s.ringpop.LookupN("foo", 3)
	s.Error(err)
	s.Nil(result)
}

func (s *RingpopTestSuite) TestLookupNNoDestinations() {
	err := createSingleNodeCluster(s.ringpop)
	s.Require().NoError(err, "unable to bootstrap single node cluster")

	address, _ := s.ringpop.address()
	member := fakeMember{
		address: address,
	}
	s.ringpop.ring.RemoveMembers(member)

	result, err := s.ringpop.LookupN("foo", 5)
	s.Empty(result)
	s.Error(err)
}

func (s *RingpopTestSuite) TestLookupN() {
	err := createSingleNodeCluster(s.ringpop)
	s.Require().NoError(err, "unable to bootstrap single node cluster")

	result, err := s.ringpop.LookupN("foo", 5)

	s.Nil(err)
	s.Equal(1, len(result), "LookupN returns not more results than number of nodes")

	members := genMembers(genAddresses(1, 10, 20))
	s.ringpop.ring.AddMembers(members...)

	result, err = s.ringpop.LookupN("foo", 5)
	s.Nil(err)
	s.Equal(5, len(result), "LookupN returns N number of results")
}

// TestGetReachableMembersNotReady tests that GetReachableMembers fails when
// Ringpop is not ready.
func (s *RingpopTestSuite) TestGetReachableMembersNotReady() {
	result, err := s.ringpop.GetReachableMembers()
	s.Error(err)
	s.Nil(result)
}

func (s *RingpopTestSuite) TestGetReachableMembers() {
	createSingleNodeCluster(s.ringpop)

	address, err := s.ringpop.WhoAmI()
	s.Require().NoError(err)

	result, err := s.ringpop.GetReachableMembers()
	s.NoError(err)
	s.Equal([]string{address}, result)
}

func (s *RingpopTestSuite) TestGetReachableMembersNotMePredicate() {
	createSingleNodeCluster(s.ringpop)

	address, err := s.ringpop.WhoAmI()
	s.Require().NoError(err)

	// get reachable members without me (non in this test)
	result, err := s.ringpop.GetReachableMembers(func(member swim.Member) bool {
		return member.Address != address
	})

	s.NoError(err)
	s.Equal([]string{}, result)
}

func (s *RingpopTestSuite) TestCountReachableMembersNotReady() {
	_, err := s.ringpop.CountReachableMembers()
	s.Error(err)
}

func (s *RingpopTestSuite) TestCountReachableMembers() {
	createSingleNodeCluster(s.ringpop)

	result, err := s.ringpop.CountReachableMembers()
	s.NoError(err)
	s.Equal(1, result)
}

func (s *RingpopTestSuite) TestCountReachableMembersNotMePredicate() {
	createSingleNodeCluster(s.ringpop)

	address, err := s.ringpop.WhoAmI()
	s.Require().NoError(err)

	// get reachable members without me (non in this test)
	result, err := s.ringpop.CountReachableMembers(func(member swim.Member) bool {
		return member.Address != address
	})

	s.NoError(err)
	s.Equal(0, result)
}

// TestAddSelfToBootstrapList tests that Ringpop automatically adds its own
// address to the bootstrap host list.
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
		DiscoverProvider: statichosts.New("127.0.0.1:3002"),
	})

	s.Nil(err)
}

// TestEmptyJoinListCreatesSingleNodeCluster tests that when you call Bootstrap
// with no hosts or bootstrap file, a single-node cluster is created.
func (s *RingpopTestSuite) TestEmptyJoinListCreatesSingleNodeCluster() {
	err := createSingleNodeCluster(s.ringpop)
	s.Require().NoError(err, "unable to bootstrap single node cluster")
	s.Equal(ready, s.ringpop.state)
}

func (s *RingpopTestSuite) TestErrorOnChannelNotListening() {
	ch, err := tchannel.NewChannel("test", nil)
	s.Require().NoError(err)

	rp, err := New("test", Channel(ch))
	s.Require().NoError(err)

	nodesJoined, err := rp.Bootstrap(&swim.BootstrapOptions{})
	s.Exactly(err, ErrEphemeralAddress)
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

func (s *RingpopTestSuite) TestInitialLabels() {
	s.ringpop.config.InitialLabels["key"] = "value"
	s.ringpop.init()
	value, has := s.ringpop.node.Labels().Get("key")
	s.Equal("value", value, "Label correctly set on local node")
	s.True(has, "Label correctly set on local node")
}

func (s *RingpopTestSuite) TestReadyEvent() {
	called := make(chan bool, 1)

	l := &eventsmocks.EventListener{}
	l.On("HandleEvent", events.Ready{}).Return().Once().Run(func(args mock.Arguments) {
		called <- true
	})
	s.ringpop.AddListener(l)
	defer s.ringpop.RemoveListener(l)

	s.ringpop.setState(ready)

	// block with timeout for event to be emitted
	select {
	case <-called:
	case <-time.After(100 * time.Millisecond):
	}

	l.AssertCalled(s.T(), "HandleEvent", events.Ready{})

	s.ringpop.setState(ready)
	l.AssertNumberOfCalls(s.T(), "HandleEvent", 1)
}

func (s *RingpopTestSuite) TestDestroyedEvent() {
	called := make(chan bool, 1)

	l := &eventsmocks.EventListener{}
	l.On("HandleEvent", events.Destroyed{}).Return().Once().Run(func(args mock.Arguments) {
		called <- true
	})
	s.ringpop.AddListener(l)
	defer s.ringpop.RemoveListener(l)

	s.ringpop.setState(destroyed)

	// block with timeout for event to be emitted
	select {
	case <-called:
	case <-time.After(100 * time.Millisecond):
	}

	l.AssertCalled(s.T(), "HandleEvent", events.Destroyed{})

	s.ringpop.setState(destroyed)
	l.AssertNumberOfCalls(s.T(), "HandleEvent", 1)
}

func (s *RingpopTestSuite) TestRingIsConstructedWhenStateReady() {
	called := make(chan bool, 1)

	rp1, err := s.makeNewRingpop()
	s.Require().NoError(err)

	rp2, err := s.makeNewRingpop()
	s.Require().NoError(err)

	err = createSingleNodeCluster(rp1)
	s.Require().NoError(err)

	me1, err := rp1.WhoAmI()
	s.Require().NoError(err)

	l := &eventsmocks.EventListener{}

	l.On("HandleEvent", events.Ready{}).Return().Run(func(args mock.Arguments) {
		s.True(rp2.Ready(), "expect ringpop to be ready when the ready event fires")

		me2, err := rp2.WhoAmI()
		s.Require().NoError(err)

		s.True(rp2.ring.HasServer(me1), "expected ringpop1 to be in the ring when ringpop fires the ready event")
		s.True(rp2.ring.HasServer(me2), "expected ringpop2 to be in the ring when ringpop fires the ready event")
		s.False(rp2.ring.HasServer("127.0.0.1:3001"), "didn't expect the mocked ringpop to be in the ring")

		called <- true
	})
	l.On("HandleEvent", mock.Anything).Return()

	rp2.AddListener(l)
	defer rp2.RemoveListener(l)

	_, err = rp2.Bootstrap(&swim.BootstrapOptions{
		DiscoverProvider: statichosts.New(me1),
	})
	s.Require().NoError(err)

	// block with timeout for event to be emitted
	select {
	case <-called:
	case <-time.After(100 * time.Millisecond):
	}
}

func (s *RingpopTestSuite) TestRingChecksumEmitTimer() {
	s.ringpop.init()
	s.mockClock.Add(5 * time.Second)
	ok := s.stats.has("ringpop.127_0_0_1_3001.membership.checksum-periodic")
	s.True(ok, "membership checksum stat")
	ok = s.stats.has("ringpop.127_0_0_1_3001.ring.checksum-periodic")
	s.True(ok, "ring checksum stat")
	s.ringpop.Destroy()
}

func (s *RingpopTestSuite) TestLabels() {
	err := createSingleNodeCluster(s.ringpop)
	s.Require().NoError(err, "expected no error in setting up cluster")

	labels, err := s.ringpop.Labels()
	s.Assert().NoError(err)
	s.Require().NotNil(labels)

	// label count: 0
	err = labels.Set("hellos", "world")
	s.Assert().Error(err, "expected an error when the key size is exceeded")

	// // label count: 0
	err = labels.Set("hello", "worlds")
	s.Assert().Error(err, "expected an error when the value size is exceeded")

	// label count: 0
	err = labels.Set("hello", "world")
	s.Assert().NoError(err, "doesnt expect an error when setting a label that is exactly the limit")

	// label count: 1
	err = labels.Set("foo", "bar")
	s.Assert().Error(err, "expected an error when you set more labels then allowed")

	// label count: 1
}

func (s *RingpopTestSuite) TestLabelsNotReady() {
	// todat labels are only supported after ringpop has been bootstrapped, this
	// test can be removed when we find an elegant way of setting labels before
	// bootstrapping
	labels, err := s.ringpop.Labels()
	s.Error(err)
	s.Nil(labels)
}

func (s *RingpopTestSuite) TestDontAllowBootstrapWithoutChannelListening() {
	ch, err := tchannel.NewChannel("test", nil)
	s.NoError(err, "channel must create successfully")
	s.channel = ch

	// Bug #146 meant that you could bootstrap ringpop without a listening
	// channel IF you provided the Address argument (or a custom Identity)
	// provider.
	s.ringpop, err = New("test", Channel(ch), Address("127.0.0.1:3001"))
	s.NoError(err, "Ringpop must create successfully")

	// Calls bootstrap
	err = createSingleNodeCluster(s.ringpop)

	s.Error(err, "expect error when tchannel is not listening")
}
