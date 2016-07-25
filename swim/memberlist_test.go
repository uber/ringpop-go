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
	"sort"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/uber/ringpop-go/util"
)

type MemberlistTestSuite struct {
	suite.Suite
	node        *Node
	m           *memberlist
	incarnation int64
	changes     []Change
}

func (s *MemberlistTestSuite) SetupTest() {
	s.incarnation = util.TimeNowMS()
	s.node = NewNode("test", "127.0.0.1:3001", nil, nil)
	s.m = s.node.memberlist
	s.m.MakeAlive(s.node.Address(), s.incarnation)

	s.changes = []Change{
		Change{
			Address:     "127.0.0.1:3002",
			Status:      Alive,
			Incarnation: s.incarnation,
		},
		Change{
			Address:     "127.0.0.1:3003",
			Status:      Suspect,
			Incarnation: s.incarnation,
		},
		Change{
			Address:     "127.0.0.1:3004",
			Status:      Faulty,
			Incarnation: s.incarnation,
		},
		Change{
			Address:     "127.0.0.1:3005",
			Status:      Leave,
			Incarnation: s.incarnation,
		},
	}
}

func (s *MemberlistTestSuite) TestAddJoinList() {

	joinList := s.changes

	// We add all the members of this node to the joinList because we want
	// the joinList to contain the change for the local member.
	mems := s.node.disseminator.MembershipAsChanges()
	for _, mem := range mems {
		joinList = append(joinList, mem)
	}

	// We make sure that the joinList contains a change for the local member.
	var localMember *Change
	for _, mem := range joinList {
		if mem.Address == s.node.Address() {
			localMember = &mem
		}
	}
	s.NotNil(localMember, "expected joinList to contain a change for the local member")

	// Add join list to the membership and see that its only stored change is
	// for the local member.
	s.m.AddJoinList(joinList)
	s.Equal(1, s.node.disseminator.ChangesCount(), "expected to only have one change")
	_, ok := s.node.disseminator.ChangesByAddress(s.node.Address())
	s.True(ok, "expected to only have the local member as a change")
}

func (s *MemberlistTestSuite) TearDownTest() {
	s.node.Destroy()
}

func (s *MemberlistTestSuite) TestChecksumChanges() {
	old := s.m.Checksum()
	s.m.MakeAlive("127.0.0.1:3002", s.incarnation)
	s.NotEqual(old, s.m.Checksum(), "expected checksum to change")
}

func (s *MemberlistTestSuite) TestChecksumsEqual() {
	nodeA := NewNode("test", "127.0.0.1:3001", nil, nil)
	defer nodeA.Destroy()
	nodeB := NewNode("test", "127.0.0.1:3001", nil, nil)
	defer nodeB.Destroy()

	nodeA.memberlist.MakeAlive("127.0.0.1:3001", s.incarnation)
	nodeA.memberlist.MakeAlive("127.0.0.1:3002", s.incarnation)
	nodeA.memberlist.MakeAlive("127.0.0.1:3003", s.incarnation)
	nodeA.memberlist.MakeAlive("127.0.0.1:3004", s.incarnation)

	nodeB.memberlist.MakeAlive("127.0.0.1:3004", s.incarnation)
	nodeB.memberlist.MakeAlive("127.0.0.1:3001", s.incarnation)
	nodeB.memberlist.MakeAlive("127.0.0.1:3003", s.incarnation)
	nodeB.memberlist.MakeAlive("127.0.0.1:3002", s.incarnation)

	s.Equal(nodeA.memberlist.Checksum(), nodeB.memberlist.Checksum(),
		"expected checksums to be equal")
}

func (s *MemberlistTestSuite) TestLocalLeaveOverrideHigher() {
	s.Require().NotNil(s.m.local, "local member cannot be nil")

	s.m.MakeLeave(s.m.local.Address, s.incarnation+1)

	s.Equal(Leave, s.m.local.Status, "expected local member status to be leave")
}

func (s *MemberlistTestSuite) TestLocalLeaveOverrideEqual() {
	s.Require().NotNil(s.m.local, "local member cannot be nil")

	s.m.MakeLeave(s.m.local.Address, s.incarnation)

	s.Equal(Leave, s.m.local.Status, "expected local member status to be leave")
}

func (s *MemberlistTestSuite) TestLocalLeaveOverrideLower() {
	s.Require().NotNil(s.m.local, "local member cannot be nil")

	s.m.MakeLeave(s.m.local.Address, s.incarnation-1)

	s.Equal(Alive, s.m.local.Status, "expected local member status to be alive")
}

func (s *MemberlistTestSuite) TestLocalFaultyOverride() {
	s.Require().NotNil(s.m.local, "local member cannot be nil")

	s.m.MakeFaulty(s.m.local.Address, s.incarnation-1)
	s.Equal(Alive, s.m.local.Status, "expected local member status to be alive")

	s.m.MakeFaulty(s.m.local.Address, s.incarnation)
	s.Equal(Alive, s.m.local.Status, "expected local member status to be alive")

	s.m.MakeFaulty(s.m.local.Address, s.incarnation+1)
	s.Equal(Alive, s.m.local.Status, "expected local member status to be alive")
}

func (s *MemberlistTestSuite) TestLocalSuspectOverride() {
	s.Require().NotNil(s.m.local, "local member cannot be nil")

	s.m.MakeSuspect(s.m.local.Address, s.incarnation-1)
	s.Equal(Alive, s.m.local.Status, "expected local member status to be alive")

	s.m.MakeSuspect(s.m.local.Address, s.incarnation)
	s.Equal(Alive, s.m.local.Status, "expected local member status to be alive")

	s.m.MakeSuspect(s.m.local.Address, s.incarnation+1)
	s.Equal(Alive, s.m.local.Status, "expected local member status to be alive")
}

func (s *MemberlistTestSuite) TestMultipleUpdates() {
	applied := s.m.Update(s.changes)

	s.Len(applied, 4, "expected all updates to be applied")

	member, ok := s.m.Member("127.0.0.1:3002")
	s.NotNil(member, "expected member not to be nil")
	s.True(ok, "expected member to be found")
	s.Equal(Alive, member.Status, "expected member to be alive")

	member, ok = s.m.Member("127.0.0.1:3003")
	s.NotNil(member, "expected member not to be nil")
	s.True(ok, "expected member to be found")
	s.Equal(Suspect, member.Status, "expected member to be suspect")

	member, ok = s.m.Member("127.0.0.1:3004")
	s.NotNil(member, "expected member not to be nil")
	s.True(ok, "expected member to be found")
	s.Equal(Faulty, member.Status, "expected member to be faulty")

	member, ok = s.m.Member("127.0.0.1:3005")
	s.NotNil(member, "expected member not to be nil")
	s.True(ok, "expected member to be found")
	s.Equal(Leave, member.Status, "expected member to be leave")
}

func (s *MemberlistTestSuite) TestUpdateTriggersReincarnation() {
	source := "192.0.2.1:1234"
	s.NotEqual(source, s.m.local.Address, "this test relies on the source and the target of the change to be different")

	applied := s.m.Update([]Change{
		Change{
			Source:            source,
			SourceIncarnation: 1337,

			Address:     s.m.local.Address,
			Incarnation: s.m.local.Incarnation,
			Status:      Suspect,
		},
	})

	s.Len(applied, 1, "expected change to be applied")

	change := applied[0]
	s.NotNil(change, "expected change not to be nil")
	s.Equal(Alive, change.Status, "expected change to be overwritten to alive")
	s.Equal(s.m.local.Address, change.Source, "expected source to be the node that reincarnated its self")
	s.Equal(s.m.local.Incarnation, change.Incarnation, "expected the new incarnation number to be the same as the one that is stored on the local node")
	s.Equal(s.m.local.Incarnation, change.SourceIncarnation, "expected the source incarnation number to be the same as the one that is stored on the local node")
}

func (s *MemberlistTestSuite) TestAliveToFaulty() {
	s.m.MakeAlive("127.0.0.1:3002", s.incarnation)

	member, ok := s.m.Member("127.0.0.1:3002")
	s.NotNil(member, "expected member not to be nil")
	s.True(ok, "expected member to be found")
	s.Equal(Alive, member.Status, "expected member to be alive")

	s.m.MakeFaulty("127.0.0.1:3002", s.incarnation-1)
	s.Equal(Alive, member.Status, "expected member to be alive")

	s.m.MakeFaulty("127.0.0.1:3002", s.incarnation)
	s.Equal(Faulty, member.Status, "expected member to be faulty")

}

func (s *MemberlistTestSuite) TestString() {
	s.m.MakeAlive("127.0.0.1:3002", s.incarnation)
	s.m.MakeAlive("127.0.0.1:3003", s.incarnation)

	str := s.m.String()
	s.NotEqual("", str, "expected memberlist to be marshalled into JSON string")
}

func (s *MemberlistTestSuite) TestUpdateEmpty() {
	applied := s.m.Update([]Change{})
	s.Empty(applied, "expected no updates to be applied")
}

func (s *MemberlistTestSuite) TestUpdateWithTombstoneState() {
	target := "127.0.0.1:3002"
	s.m.MakeAlive(target, s.incarnation)

	s.m.Update([]Change{
		Change{
			Address:     target,
			Incarnation: s.incarnation,
			Status:      Tombstone,
		},
	})

	member := s.m.members.byAddress[target]

	s.Assert().Equal(Tombstone, member.Status, "expected the member with tombstone state")
}

func (s *MemberlistTestSuite) TestUpdateWithTombstoneFlag() {
	target := "127.0.0.1:3002"
	s.m.MakeAlive(target, s.incarnation)

	s.m.Update([]Change{
		Change{
			Address:     target,
			Incarnation: s.incarnation,
			Status:      Faulty,
			Tombstone:   true,
		},
	})

	member := s.m.members.byAddress[target]

	s.Assert().Equal(Tombstone, member.Status, "expected the member with tombstone state")
}

func (s *MemberlistTestSuite) TestRandomPingable() {
	s.m.MakeAlive("127.0.0.1:3002", testInc)
	s.m.MakeAlive("127.0.0.1:3003", testInc)
	s.m.MakeAlive("127.0.0.1:3004", testInc)

	excluding := map[string]bool{
		"127.0.0.1:3003": true,
	}

	members := s.m.RandomPingableMembers(4, excluding)
	s.Len(members, 2, "expected local and excluded member to be omitted")

	members = s.m.RandomPingableMembers(1, excluding)
	s.Len(members, 1, "expected only one member")
}

func (s *MemberlistTestSuite) TestGetReachableMembers() {
	nodeA := NewNode("test", "127.0.0.1:3001", nil, nil)
	defer nodeA.Destroy()

	nodeA.memberlist.MakeAlive("127.0.0.1:3001", s.incarnation)
	nodeA.memberlist.MakeAlive("127.0.0.1:3002", s.incarnation)
	nodeA.memberlist.MakeSuspect("127.0.0.1:3003", s.incarnation)
	nodeA.memberlist.MakeFaulty("127.0.0.1:3004", s.incarnation)

	activeMembers := nodeA.GetMembers(ReachableMember)
	activeAddresses := make([]string, 0, len(activeMembers))
	for _, member := range activeMembers {
		activeAddresses = append(activeAddresses, member.Address)
	}
	sort.Strings(activeAddresses)

	s.Equal([]string{
		"127.0.0.1:3001",
		"127.0.0.1:3002",
		"127.0.0.1:3003",
	}, activeAddresses, "expected a list of 3 specific nodes")
}

func (s *MemberlistTestSuite) TestCountReachableMembers() {
	nodeA := NewNode("test", "127.0.0.1:3001", nil, nil)
	defer nodeA.Destroy()

	nodeA.memberlist.MakeAlive("127.0.0.1:3001", s.incarnation)
	nodeA.memberlist.MakeAlive("127.0.0.1:3002", s.incarnation)
	nodeA.memberlist.MakeSuspect("127.0.0.1:3003", s.incarnation)
	nodeA.memberlist.MakeFaulty("127.0.0.1:3004", s.incarnation)

	reachableMemberCount := nodeA.CountMembers(ReachableMember)

	s.Equal(3, reachableMemberCount, "expected 3 reachable members")
}

func (s *MemberlistTestSuite) TestRemoveMember() {
	// seed the membership with more members
	s.m.Update(s.changes)

	var removed bool
	var count int

	count = s.m.NumMembers()
	// remove an unknown member
	removed = s.m.RemoveMember("192.0.2.123:1234")
	s.Assert().False(removed, "expect to not remove an unknown member")
	s.Assert().Equal(count, s.m.NumMembers(), "expected an unchanged count")

	// remove a known member
	removed = s.m.RemoveMember(s.changes[0].Address)
	s.Assert().True(removed, "expect to remove a member that was added before")
	s.Assert().Equal(count-1, s.m.NumMembers(), "expected that there is exactly one member less")
}

func (s *MemberlistTestSuite) TestApplyUnknownTombstone() {
	applied := s.m.MakeTombstone("192.0.2.123:1234", 42)
	s.Assert().Len(applied, 0, "expected that the declaration of a tombstone for an unknown member is not applied")
}

func TestMemberlistTestSuite(t *testing.T) {
	suite.Run(t, new(MemberlistTestSuite))
}
