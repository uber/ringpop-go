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

	"github.com/stretchr/testify/suite"
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
	s.d.ClearChange(fakeChange)
	s.Equal(4, s.d.ChangesCount(), "expected no problems deleting non-existent changes")

	changes := s.d.issueChanges()
	for i, c := range changes {
		s.d.ClearChange(c)
		s.Equal(4-i-1, s.d.ChangesCount(), "expected one change to be deleted")
	}
	s.Equal(0, s.d.ChangesCount(), "expected no change left")

	s.d.ClearChange(fakeChange)
	s.Equal(0, s.d.ChangesCount(), "expected no problems deleting non-existent changes")
}

func (s *DisseminatorTestSuite) TestFullSync() {
	addresses := fakeHostPorts(1, 1, 2, 4)

	for _, address := range addresses {
		s.m.MakeAlive(address, s.incarnation)
	}

	changes := s.d.FullSync()

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
	s.d.changes[suspectAddr].Incarnation = s.incarnation

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
	s.d.changes[suspectAddr].Incarnation = s.d.node.Incarnation()

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
