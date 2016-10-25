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
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/uber/ringpop-go/events"
	"github.com/uber/ringpop-go/util"
)

type DisseminatorTestSuite struct {
	suite.Suite
	node        *Node
	d           *disseminator
	m           *memberlist
	incarnation int64
}

func (s *DisseminatorTestSuite) SetupTest() {
	s.incarnation = util.TimeNowMS()

	fakeAddress := fakeHostPorts(1, 1, 1, 1)[0]
	s.node = NewNode("test", fakeAddress, nil, nil)

	s.node.memberlist.MakeAlive(s.node.Address(), s.incarnation)
	s.d = s.node.disseminator
	s.m = s.node.memberlist
}

func (s *DisseminatorTestSuite) TearDownTest() {
	s.node.Destroy()
}

func (s *DisseminatorTestSuite) TestChangesAreRecorded() {
	addresses := fakeHostPorts(1, 1, 2, 4)

	for _, address := range addresses {
		s.m.MakeAlive(address, s.incarnation)
	}

	s.Len(s.d.changes, 4, "expected four changes to be recorded")
}

func (s *DisseminatorTestSuite) TestChangesCount() {
	addresses := fakeHostPorts(1, 1, 2, 4)
	for i, address := range addresses {
		s.Equal(i+1, s.d.ChangesCount(), "expected change to be recorded")

		s.m.MakeAlive(address, s.incarnation)
	}
	s.Equal(len(addresses)+1, s.d.ChangesCount(), "expected no changes to be recorded")
}

func (s *DisseminatorTestSuite) TestChangesByAddress() {
	addresses := fakeHostPorts(1, 1, 2, 4)

	c, ok := s.d.ChangesByAddress(addresses[0])
	s.False(ok, "expected changes does not contain this address yet")
	s.Equal(Change{}, c, "expected change is nil")

	for _, address := range addresses {
		s.m.MakeAlive(address, s.incarnation)

		c, ok := s.d.ChangesByAddress(address)
		s.True(ok, "expected changes contains this address")
		s.Equal(address, c.address(), "expected change has correct address")
		s.Equal(s.incarnation, c.incarnation(), "expected change has correct incarnation")
	}
}

func (s *DisseminatorTestSuite) TestChangesAreCleared() {
	addresses := fakeHostPorts(1, 1, 2, 4)

	for _, address := range addresses {
		s.m.MakeAlive(address, s.incarnation)
	}
	fakeChange := Change{}
	fakeChange.Address = "fake address"
	s.d.ClearChange(fakeChange.Address)
	s.Equal(4, s.d.ChangesCount(), "expected no problems deleting non-existent changes")

	changes := s.d.issueChanges()
	for i, c := range changes {
		s.d.ClearChange(c.Address)
		s.Equal(4-i-1, s.d.ChangesCount(), "expected one change to be deleted")
	}
	s.Equal(0, s.d.ChangesCount(), "expected no change left")

	s.d.ClearChange(fakeChange.Address)
	s.Equal(0, s.d.ChangesCount(), "expected no problems deleting non-existent changes")
}

func (s *DisseminatorTestSuite) TestMembershipAsChanges() {
	addresses := fakeHostPorts(1, 1, 2, 4)

	for _, address := range addresses {
		s.m.MakeAlive(address, s.incarnation)
	}

	changes := s.d.MembershipAsChanges()

	s.Len(changes, 4, "expected to get change for each member")
}

func (s *DisseminatorTestSuite) TestIssueChangesAsSender() {
	s.d.ClearChanges()

	aliveAddr := "127.0.0.1:3002"
	suspectAddr := "127.0.0.1:3003"
	faultyAddr := "127.0.0.1:3004"
	s.m.MakeAlive(aliveAddr, s.incarnation)
	s.m.MakeSuspect(suspectAddr, s.incarnation)
	s.m.MakeFaulty(faultyAddr, s.incarnation)

	changes, _ := s.d.IssueAsSender()
	s.Len(changes, 3, "expected three changes to be issued")
}

// TestIssueChangesAsSenderForTombstone tests that tombstones are issued as tombstone
// flags when changes are issued when the node is the sender of a ping(-req)
func (s *DisseminatorTestSuite) TestIssueChangesAsSenderForTombstone() {
	s.d.ClearChanges()

	tombstoneAddr := "127.0.0.1:3002"
	// tombstones are only applied when the member is in the list, via MakeAlive
	// we ensure that the member is in the list before we declare it as a tombstone
	s.m.MakeAlive(tombstoneAddr, s.incarnation)
	s.m.MakeTombstone(tombstoneAddr, s.incarnation)

	changes, _ := s.d.IssueAsSender()
	s.Require().Len(changes, 1, "required to only have the tombstone change")
	s.Assert().Equal("faulty", changes[0].Status, "expected tombstone to be gossiped as a faulty state")
	s.Assert().Equal(true, changes[0].Tombstone, "expected tombstone to be gossiped with a tombstone flag")
}

// TestIssueChangesAsReceiverForTombstone tests that tombstones are issued as tombstone
// flags when changes are issued when the node is the receiver of a ping(-req)
func (s *DisseminatorTestSuite) TestIssueChangesAsReceiverForTombstone() {
	s.d.ClearChanges()

	tombstoneAddr := "127.0.0.1:3002"
	otherAddr := "127.0.0.1:3003"
	// tombstones are only applied when the member is in the list, via MakeAlive
	// we ensure that the member is in the list before we declare it as a tombstone
	s.m.MakeAlive(tombstoneAddr, s.incarnation)
	s.m.MakeTombstone(tombstoneAddr, s.incarnation)

	changes, _ := s.d.IssueAsReceiver(otherAddr, s.node.Incarnation(), s.m.Checksum())
	s.Require().Len(changes, 1, "required to only have the tombstone change")
	s.Assert().Equal("faulty", changes[0].Status, "expected tombstone to be gossiped as a faulty state")
	s.Assert().Equal(true, changes[0].Tombstone, "expected tombstone to be gossiped with a tombstone flag")
}

func (s *DisseminatorTestSuite) TestIssueChangesAsReceiver() {
	s.d.ClearChanges()

	aliveAddr := "127.0.0.1:3002"
	suspectAddr := "127.0.0.1:3003"
	faultyAddr := "127.0.0.1:3004"
	s.m.MakeAlive(aliveAddr, s.incarnation)
	s.m.MakeSuspect(suspectAddr, s.incarnation)
	s.m.MakeFaulty(faultyAddr, s.incarnation)

	changes, fs := s.d.IssueAsReceiver(s.node.Address(), s.node.Incarnation(), s.m.Checksum())
	s.Len(changes, 0, "expected no changes to be issued for same sender/receiver")
	s.False(fs, "expected changes to not be a full sync")

	changes, fs = s.d.IssueAsReceiver(aliveAddr, s.incarnation, s.m.Checksum())
	s.Len(changes, 3, "expected three changes to be issued")
	s.False(fs, "expected changes to not be a full sync")

	s.d.ClearChanges()

	changes, fs = s.d.IssueAsReceiver(aliveAddr, s.incarnation, s.m.Checksum())
	s.Len(changes, 0, "expected to get no changes")
	s.False(fs, "expected changes to not be a full sync")

	changes, fs = s.d.IssueAsReceiver(aliveAddr, s.incarnation, s.m.Checksum()+1)
	s.Len(changes, 4, "expected change to be issued for each member in membership")
	s.True(fs, "expected changes to be a full sync")
}

func (s *DisseminatorTestSuite) TestBumpPiggybackAsSender() {
	s.d.ClearChanges()

	aliveAddr := "127.0.0.1:3002"
	suspectAddr := "127.0.0.1:3003"
	faultyAddr := "127.0.0.1:3004"
	s.m.MakeAlive(aliveAddr, s.incarnation)
	s.m.MakeSuspect(suspectAddr, s.incarnation)
	s.m.MakeFaulty(faultyAddr, s.incarnation)
	ac := s.d.changes[aliveAddr]
	sc := s.d.changes[suspectAddr]
	fc := s.d.changes[faultyAddr]

	s.Equal(ac.p, 0, "expected piggyback counter isn't bumped")
	s.Equal(sc.p, 0, "expected piggyback counter isn't bumped")
	s.Equal(fc.p, 0, "expected piggyback counter isn't bumped")

	_, bumpPiggybackCounters := s.d.IssueAsSender()
	s.Equal(ac.p, 0, "expected piggyback counter isn't bumped")
	s.Equal(sc.p, 0, "expected piggyback counter isn't bumped")
	s.Equal(fc.p, 0, "expected piggyback counter isn't bumped")
	bumpPiggybackCounters()
	s.Equal(ac.p, 1, "expected piggyback counter is bumped")
	s.Equal(sc.p, 1, "expected piggyback counter is bumped")
	s.Equal(fc.p, 1, "expected piggyback counter is bumped")
}

func (s *DisseminatorTestSuite) TestBumpPiggybackAsReceiver() {
	s.d.ClearChanges()

	aliveAddr := "127.0.0.1:3002"
	suspectAddr := "127.0.0.1:3003"
	faultyAddr := "127.0.0.1:3004"
	s.m.MakeAlive(aliveAddr, s.incarnation)
	s.m.MakeSuspect(suspectAddr, s.incarnation)
	s.m.MakeFaulty(faultyAddr, s.incarnation)
	ac := s.d.changes[aliveAddr]
	sc := s.d.changes[suspectAddr]
	fc := s.d.changes[faultyAddr]

	s.Equal(ac.p, 0, "expected piggyback counter isn't bumped")
	s.Equal(sc.p, 0, "expected piggyback counter isn't bumped")
	s.Equal(fc.p, 0, "expected piggyback counter isn't bumped")

	_, _ = s.d.IssueAsReceiver(aliveAddr, s.incarnation, s.m.Checksum())
	s.Equal(ac.p, 1, "expected piggyback counter is bumped")
	s.Equal(sc.p, 1, "expected piggyback counter is bumped")
	s.Equal(fc.p, 1, "expected piggyback counter is bumped")
}

func (s *DisseminatorTestSuite) TestChangesDeletedAsSender() {
	s.d.ClearChanges()
	s.d.maxP = 2
	s.d.pFactor = 2

	address := "127.0.0.1:3002"

	s.m.MakeAlive(address, s.incarnation)

	s.Equal(0, s.d.changes[address].p, "expected propagations for change to be 0")

	changes, bumpPiggybackCounters := s.d.IssueAsSender()
	oldBumpPBC := bumpPiggybackCounters
	bumpPiggybackCounters()
	s.Len(changes, 1, "expected one change to be issued")
	s.Equal(1, s.d.changes[address].p, "expected propagations for change to be 1")

	changes, bumpPiggybackCounters = s.d.IssueAsSender()
	bumpPiggybackCounters()
	s.Len(changes, 1, "expected one change to be issued")
	s.Empty(s.d.changes, "expected changes are cleared after 2 propagations")

	changes, bumpPiggybackCounters = s.d.IssueAsSender()
	bumpPiggybackCounters()
	s.Empty(changes, "expected no changes to be issued")
	s.Empty(s.d.changes, "expected changes are cleared after 2 propagations")

	_, ok := s.d.changes[address]
	s.False(ok, "expected change to be deleted")

	// make sure bumping pb counters of removed changes is ok
	oldBumpPBC()
	s.Empty(s.d.changes, "expected changes are cleared after 2 propagations")
}

func (s *DisseminatorTestSuite) TestChangesDeletedAsReceiver() {
	s.d.ClearChanges()
	s.d.maxP = 2
	s.d.pFactor = 2

	address := "127.0.0.1:3002"

	s.m.MakeAlive(address, s.incarnation)

	s.Equal(0, s.d.changes[address].p, "expected propagations for change to be 0")

	changes, _ := s.d.IssueAsReceiver(address, s.incarnation, s.m.Checksum())
	s.Len(changes, 1, "expected one change to be issued")
	s.Equal(1, s.d.changes[address].p, "expected propagations for change to be 1")

	changes, _ = s.d.IssueAsReceiver(address, s.incarnation, s.m.Checksum())
	s.Len(changes, 1, "expected one change to be issued")
	s.Empty(s.d.changes, "expected changes are cleared after 2 propagations")

	changes, _ = s.d.IssueAsReceiver(address, s.incarnation, s.m.Checksum())
	s.Empty(changes, "expected no changes to be issued")

	_, ok := s.d.changes[address]
	s.False(ok, "expected change to be deleted")
}

func (s *DisseminatorTestSuite) TestFilterChangesFromSender() {
	s.d.ClearChanges()
	aliveAddr := "127.0.0.1:3002"
	suspectAddr := "127.0.0.1:3003"
	faultyAddr := "127.0.0.1:3004"
	s.m.MakeAlive(aliveAddr, s.incarnation)
	s.m.MakeSuspect(suspectAddr, s.incarnation)
	s.m.MakeFaulty(faultyAddr, s.incarnation)

	// make aliveAddr the source of suspect change
	s.d.changes[suspectAddr].Source = aliveAddr
	s.d.changes[suspectAddr].SourceIncarnation = s.incarnation

	changes := s.d.issueChanges()
	s.Len(changes, 3, "expected three changes")

	cs1 := s.d.filterChangesFromSender(changes, aliveAddr, s.incarnation)

	// test that filterChangesFromSender only reorders but not modifies otherwise
	s.Len(changes, 3, "expected that changes is reordered but not shrunk")
	s.True(contains(changes, aliveAddr), "expected that changes is reordered correctly")
	s.True(contains(changes, suspectAddr), "expected that changes is reordered correctly")
	s.True(contains(changes, faultyAddr), "expected that changes is reordered correctly")

	// test that one change is filtered for aliveAddr
	s.Len(cs1, 2, "expected one change was filtered")
	s.True(contains(cs1, aliveAddr), "expected that alive change is not filtered")
	s.True(contains(cs1, faultyAddr), "expected that faulty change is not filtered")
	s.False(contains(cs1, suspectAddr), "expected that suspect change filtered")

	// test that two changes are filtered for local address
	cs1 = s.d.filterChangesFromSender(changes, s.d.node.Address(), s.d.node.Incarnation())
	s.Len(cs1, 1, "expected two changes were filtered")
	s.True(contains(cs1, suspectAddr), "expected that suspect change is not filtered")
	s.False(contains(cs1, aliveAddr), "expected that alive change is filtered")
	s.False(contains(cs1, faultyAddr), "expected that faulty change is filtered")

	// test that no changes are filtered when incarnation number doesn't match
	cs1 = s.d.filterChangesFromSender(changes, aliveAddr, s.incarnation-1)
	s.Len(cs1, 3, "expected no changes were filtered")
	s.True(contains(cs1, aliveAddr), "expected no nodes were filtered")
	s.True(contains(cs1, suspectAddr), "expected no nodes were filtered")
	s.True(contains(cs1, faultyAddr), "expected no nodes were filtered")

	// test that no changes are filtered when for suspectAddr
	cs1 = s.d.filterChangesFromSender(changes, suspectAddr, s.incarnation)
	s.Len(cs1, 3, "expected no changes were filtered")
	s.True(contains(cs1, aliveAddr), "expected no nodes were filtered")
	s.True(contains(cs1, suspectAddr), "expected no nodes were filtered")
	s.True(contains(cs1, faultyAddr), "expected no nodes were filtered")

	// test that no changes are filtered when for aliveAddr
	cs1 = s.d.filterChangesFromSender(changes, faultyAddr, s.incarnation)
	s.Len(cs1, 3, "expected no changes were filtered")
	s.True(contains(cs1, aliveAddr), "expected no nodes were filtered")
	s.True(contains(cs1, suspectAddr), "expected no nodes were filtered")
	s.True(contains(cs1, faultyAddr), "expected no nodes were filtered")

	// make change the source of suspect change back to local address
	s.d.changes[suspectAddr].Source = s.d.node.Address()
	s.d.changes[suspectAddr].SourceIncarnation = s.d.node.Incarnation()

	// test all changes are filtered
	changes = s.d.issueChanges()
	cs1 = s.d.filterChangesFromSender(changes, s.d.node.Address(), s.d.node.Incarnation())
	s.Len(changes, 3, "expected changes is reodered but not shrunk")
	s.Len(cs1, 0, "expected all changes were filtered")
}

func contains(cs []Change, address string) bool {
	for _, c := range cs {
		if c.address() == address {
			return true
		}
	}
	return false
}

func TestDisseminatorTestSuite(t *testing.T) {
	suite.Run(t, new(DisseminatorTestSuite))
}

// TestBidirectionalFullSync creates two nodes a and b. The nodes are set up
// such that a has b in its memberlist, but b doesn't have a in its memberlist.
// Next we let a ping b, b issues a full sync but that doesn't help much
// because a already knows about b. If bidirectional full syncs is implemented
// b will also become aware of a's memberlist and thus the ring converges
func TestBidirectionalFullSync(t *testing.T) {
	// create two nodes: a and b
	a := newChannelNode(t)
	defer a.Destroy()

	b := newChannelNode(t)
	defer b.Destroy()

	// After bootstrap, a's memberlist contains a and b, but b's membership
	// only contains b.
	bootstrapNodes(t, b, a)

	// clear changes so that a full sync will be issued
	a.node.disseminator.ClearChanges()
	b.node.disseminator.ClearChanges()

	// check that indeed b's memberlist doesn't contain a
	addrA := a.node.Address()
	_, has := b.node.memberlist.Member(addrA)
	assert.False(t, has, "expected that a is not yet part of b's membership")

	cont := make(chan struct{})

	// only when a reverse full sync is issued the membership of b changes
	// we listen for a membership change and continue the main thread of the test
	b.node.AddListener(on(MemberlistChangesAppliedEvent{}, func(e events.Event) {
		cont <- struct{}{}
	}))

	// this forces a to ping b, b will respond with a full sync and
	// perform a reverse full sync subsequently
	a.node.pingNextMember()

	// wait until the membership has changed and the cont channel is sent a signal
	select {
	case <-time.After(time.Second):
		assert.Fail(t, "Test timed out, memberships did not converge")
	case <-cont:
	}

	// check that b has indeed added a to its membership
	_, has = b.node.memberlist.Member(addrA)
	assert.True(t, has, "expected that a is part of b's membership now")
}

// TestThrottleBidirectionalFullSyncs tests if we throttle the maximum number
// of reverse full syncs running at the same time. In this test, we start
// maxReverseFullSyncJobs+1 reverse full syncs and we block these routines midway
// by waiting for the block channel. The last reverse full sync will be omitted
// because the max number of reverse full syncs is in process (albeit blocked).
// We wait and listen for the OmitReverseFullSyncEvent. When it fires, we
// continue the test and count if the number of started reverse full syncs is
// indeed equal to the maxBidirectionalFullSync variable.
func TestThrottleBidirectionalFullSyncs(t *testing.T) {
	tnode := newChannelNode(t)
	maxJobs := tnode.node.maxReverseFullSyncJobs

	tnodes := genChannelNodes(t, maxJobs+1)

	bootstrapNodes(t, append(tnodes, tnode)...)
	waitForConvergence(t, 100, append(tnodes, tnode)...)
	block := make(chan struct{})
	quit := make(chan struct{})
	total := int64(0)

	// Block the reverse full sync procedure to test if we throttle correctly.
	tnode.node.AddListener(on(StartReverseFullSyncEvent{}, func(e events.Event) {
		atomic.AddInt64(&total, 1)
		<-block
	}))

	// Listen for a reverse full sync that is omitted because the max number of
	// reverse full sync routines are running.
	tnode.node.AddListener(on(OmitReverseFullSyncEvent{}, func(e events.Event) {
		go func() {
			for i := 0; i < maxJobs; i++ {
				block <- struct{}{}
			}
			close(quit)
		}()
	}))

	// Start the reverse full syncs.
	for _, tn := range tnodes {
		target := tn.node.Address()
		tnode.node.disseminator.tryStartReverseFullSync(target, time.Second)
	}

	// The last tryStartReverseFullSync should fail to start a reverse full
	// sync because the max number of reverse full sync routines are running.
	// The OmitReverseFullSync event is handled above, when it fires the
	// handler unblocks all the running reverse full syncs, thereafter it
	// closes the quit channel so that the test continue and pass.
	select {
	case <-quit:
	case <-time.After(time.Second):
		assert.Fail(t, "test timed out, onOmitReverseFullSync listener didn't close quit channel")
	}

	assert.Equal(t, int64(maxJobs), total, "expected a throttled amount of concurrent jobs")
}

// TestReverseFullSync starts two partitions and checks if reverseFullSync
// merges them.
func TestReverseFullSync(t *testing.T) {
	A := genChannelNodes(t, 5)
	bootstrapNodes(t, A...)
	waitForConvergence(t, 100, A...)

	B := genChannelNodes(t, 5)
	bootstrapNodes(t, B...)
	waitForConvergence(t, 100, B...)

	target := B[0].node.Address()
	A[0].node.disseminator.reverseFullSync(target, time.Second)

	waitForConvergence(t, 100, append(A, B...)...)
}

// TestRedundantFullSync tests wether a RudundantReverseFullSyncEvent is fired
// when the reverseFullSync didn't apply any changes to the memberlist. We test
// this by starting a reverseFullSync on a converged healthy cluster.
func TestRedundantFullSync(t *testing.T) {
	tnodes := genChannelNodes(t, 5)
	bootstrapNodes(t, tnodes...)
	waitForConvergence(t, 100, tnodes...)

	quit := make(chan struct{})
	tnodes[0].node.AddListener(on(RedundantReverseFullSyncEvent{}, func(e events.Event) {
		close(quit)
	}))

	target := tnodes[1].node.Address()
	tnodes[0].node.disseminator.reverseFullSync(target, time.Second)

	select {
	case <-quit:
	case <-time.After(time.Second):
		assert.Fail(t, "test timed out")
	}
}

// TestReverseFullSyncJoinFailure test the code path for when the join in the
// reverseFullSync fails.
func TestReverseFullSyncJoinFailure(t *testing.T) {
	tnodes := genChannelNodes(t, 5)
	bootstrapNodes(t, tnodes...)
	waitForConvergence(t, 100, tnodes...)
	tnodes[0].node.disseminator.reverseFullSync("XXX", time.Second)
}
