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

	"github.com/uber/ringpop-go/events"
	"github.com/uber/ringpop-go/membership"
	"github.com/uber/ringpop-go/util"

	"github.com/benbjohnson/clock"
	"github.com/stretchr/testify/suite"
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
	c := clock.NewMock()

	nodeA := NewNode("test", "127.0.0.1:3001", nil, &Options{
		Clock: c,
	})
	defer nodeA.Destroy()
	nodeB := NewNode("test", "127.0.0.1:3001", nil, &Options{
		Clock: c,
	})
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

	member, ok = s.m.Member("127.0.0.1:3002")
	s.NotNil(member, "expected member not to be nil")
	s.True(ok, "expected member to be found")
	s.Equal(Alive, member.Status, "expected member to be alive")

	s.m.MakeFaulty("127.0.0.1:3002", s.incarnation)

	member, ok = s.m.Member("127.0.0.1:3002")
	s.NotNil(member, "expected member not to be nil")
	s.True(ok, "expected member to be found")
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

	activeMembers := nodeA.GetReachableMembers()
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

	reachableMemberCount := nodeA.CountReachableMembers()

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

func (s *MemberlistTestSuite) TestSetLocalLabel() {
	// make sure there are no changes recorded before the test
	s.node.disseminator.ClearChanges()

	s.m.SetLocalLabel("hello", "world")
	value, has := s.m.GetLocalLabel("hello")

	s.Assert().True(has, "expected to have the local label hello after is has been set")
	s.Assert().Equal("world", value, "expected a value of world for the local label")
	s.Assert().Equal(1, s.node.disseminator.ChangesCount(), "expected to have 1 change recorded in the disseminator")

	// testing to overwrite the value
	s.m.SetLocalLabel("hello", "baz")
	value, has = s.m.GetLocalLabel("hello")

	s.Assert().True(has, "expected to have the local label hello after is has been overwritten")
	s.Assert().Equal("baz", value, "expected a value of baz for the local label")
	s.Assert().Equal(1, s.node.disseminator.ChangesCount(), "expected to have 1 change recorded in the disseminator after the labels are overwritten")

	// testing to overwrite with the same value, should not result in dissemination.
	s.node.disseminator.ClearChanges()
	s.m.SetLocalLabel("hello", "baz")
	s.Assert().Equal(0, s.node.disseminator.ChangesCount(), "expected to not have changes in the disseminator when overwriting a label with the same value")
}

func (s *MemberlistTestSuite) TestSetLocalLabels() {
	// make sure there are no changes recorded before the test
	s.node.disseminator.ClearChanges()

	s.m.SetLocalLabels(map[string]string{"hello": "world"})
	value, has := s.m.GetLocalLabel("hello")

	s.Assert().True(has, "expected to have the local label hello after is has been set via bulk operation")
	s.Assert().Equal("world", value, "expected a value of world for the local label")
	s.Assert().Equal(1, s.node.disseminator.ChangesCount(), "expected to have 1 change recorded in the disseminator")

	// test that the change will not be recorded if a single label does not change the value
	s.node.disseminator.ClearChanges()

	s.m.SetLocalLabels(map[string]string{"hello": "world"})
	value, has = s.m.GetLocalLabel("hello")

	s.Assert().True(has, "expected to have the local label hello after is has been set via bulk operation")
	s.Assert().Equal("world", value, "expected a value of world for the local label")
	s.Assert().Equal(0, s.node.disseminator.ChangesCount(), "expected to have 1 change recorded in the disseminator")

	// test an operation for two keys
	s.node.disseminator.ClearChanges()

	s.m.SetLocalLabels(map[string]string{
		"hello": "world",
		"foo":   "bar",
	})
	value, has = s.m.GetLocalLabel("hello")
	s.Assert().True(has, "expected to have the local label hello after is has been set via bulk operation")
	s.Assert().Equal("world", value, "expected a value of world for the local label")

	value, has = s.m.GetLocalLabel("foo")
	s.Assert().True(has, "expected to have the local label foo after is has been set via bulk operation")
	s.Assert().Equal("bar", value, "expected a value of bar for the local label")

	s.Assert().Equal(1, s.node.disseminator.ChangesCount(), "expected to have 1 change recorded in the disseminator")
}

func (s *MemberlistTestSuite) TestSetLocalLabel_empty() {
	// test an empty value
	s.m.SetLocalLabel("empty", "")
	value, has := s.m.GetLocalLabel("empty")

	s.Assert().True(has, "expected to have a local label with an empty value")
	s.Assert().Equal("", value, "expected an empty value")

	// test an empty key
	s.m.SetLocalLabel("", "empty")
	value, has = s.m.GetLocalLabel("")

	s.Assert().True(has, "expected to have a local label with an empty key")
	s.Assert().Equal("empty", value, "expected the value to be the string 'empty'")

	// empty key and label
	s.m.SetLocalLabel("", "")
	value, has = s.m.GetLocalLabel("")

	s.Assert().True(has, "expected to have a local label with an empty key")
	s.Assert().Equal("", value, "expected an empty value")
}

func (s *MemberlistTestSuite) TestGetLocalLabel() {
	s.m.SetLocalLabel("hello", "world")
	value, has := s.m.GetLocalLabel("hello")

	s.Assert().True(has, "expected to have the local label hello after is has been set")
	s.Assert().Equal("world", value, "expected a value of world for the local label")

	value, has = s.m.GetLocalLabel("foo")
	s.Assert().False(has, "expected to not have a label named foo")
}

func (s *MemberlistTestSuite) TestGetLocalLabelsAsMap() {
	m := s.m.LocalLabelsAsMap()
	s.Assert().Len(m, 0, "expected an empty map of labels")

	s.m.SetLocalLabel("hello", "world")

	m = s.m.LocalLabelsAsMap()
	s.Assert().Equal(map[string]string{"hello": "world"}, m, "expected a map with label 'hello' set to world")

	m["hack"] = "pwnd"
	value, has := s.m.GetLocalLabel("hack")
	s.Assert().False(has, "expected no label with key 'hack' since the label map should be a copy and not the live instance")
	s.Assert().NotEqual("pwnd", value, "expected an empty value and not the label we wrongfully put in the map returned")
}

func (s *MemberlistTestSuite) TestRemoveLocalLabels() {
	// make sure there are no changes recorded before the test
	s.node.disseminator.ClearChanges()

	removed := s.m.RemoveLocalLabels("hello")
	s.Assert().False(removed, "expected no removed labels")
	s.Assert().Equal(0, s.node.disseminator.ChangesCount(), "expected to have 0 change recorded in the disseminator after the removal of an unexisting label")

	// prepare with a single label
	s.m.SetLocalLabel("hello", "world")
	s.m.node.disseminator.ClearChanges()

	// test
	removed = s.m.RemoveLocalLabels("hello")
	s.Assert().True(removed, "expected to remove a label")
	s.Assert().Equal(1, s.node.disseminator.ChangesCount(), "expected to have 1 change recorded in the disseminator after the removal of an existing label")
}

// MembershipVisibleTransitions is a matrix of transitions which should and
// should not show up in the memberlist.ChangeEvent changes. This is used in the
// tests for remote and local updates to verify all events that should show up
// are there and all events that should not show up don't. Not all transactions
// are covered because their behavior is undefined and may change in the future.
// These are transitions that are non-observable for the membership incl. but
// not limited to incarnation number changes
var MembershipVisibleTransitions = map[string]struct {
	From string
	To   string

	Before bool
	After  bool

	notAllowed bool
}{
	// events that must emit
	"192.0.2.0:1": {From: Alive, To: Faulty, Before: true, After: false},
	"192.0.2.0:2": {From: Alive, To: Leave, Before: true, After: false},
	"192.0.2.0:3": {From: Alive, To: Tombstone, Before: true, After: false},

	"192.0.2.0:4": {From: Suspect, To: Faulty, Before: true, After: false},
	"192.0.2.0:5": {From: Suspect, To: Leave, Before: true, After: false},
	"192.0.2.0:6": {From: Suspect, To: Tombstone, Before: true, After: false},

	"192.0.2.0:7": {From: Faulty, To: Alive, Before: false, After: true},
	"192.0.2.0:8": {From: Faulty, To: Suspect, Before: false, After: true},

	"192.0.2.0:9":  {From: Leave, To: Alive, Before: false, After: true},
	"192.0.2.0:10": {From: Leave, To: Suspect, Before: false, After: true},

	"192.0.2.0:11": {From: Tombstone, To: Alive, Before: false, After: true},
	"192.0.2.0:12": {From: Tombstone, To: Suspect, Before: false, After: true},

	// events that must NOT emit
	"192.0.2.0:13": {From: Faulty, To: Leave, notAllowed: true},
	"192.0.2.0:14": {From: Faulty, To: Tombstone, notAllowed: true},

	"192.0.2.0:15": {From: Leave, To: Faulty, notAllowed: true},
	"192.0.2.0:16": {From: Leave, To: Tombstone, notAllowed: true},

	"192.0.2.0:17": {From: Tombstone, To: Faulty, notAllowed: true},
	"192.0.2.0:18": {From: Tombstone, To: Leave, notAllowed: true},
}

// TestMembershipEventsRemoteState uses MembershipVisibleTransitions to test
// transitions that are applied due to incomming gossip events generate the
// desired membership.ChangeEvent events.
func (s *MemberlistTestSuite) TestMembershipEventsRemoteState() {
	var initialChanges []Change
	var updateChanges []Change

	for address, test := range MembershipVisibleTransitions {
		initialChanges = append(initialChanges, Change{
			Address:     address,
			Incarnation: s.incarnation,
			Status:      test.From,
		})

		updateChanges = append(updateChanges, Change{
			Address:     address,
			Incarnation: s.incarnation + 1,
			Status:      test.To,
		})
	}

	// seed the memberlist with intial states
	s.m.Update(initialChanges)

	eventFired := false
	s.node.AddListener(on(membership.ChangeEvent{}, func(e events.Event) {
		changeEvent := e.(membership.ChangeEvent)

		seenMembers := make(map[string]struct{})

		for _, change := range changeEvent.Changes {
			var address string

			if change.Before != nil {
				address = change.Before.GetAddress()
			} else {
				address = change.After.GetAddress()
			}

			seenMembers[address] = struct{}{}

			test, has := MembershipVisibleTransitions[address]
			if !has {
				// change is not specified and therefore allowed
				s.Assert().Fail("allowed but not expected, logging during development")
				continue
			}

			if test.notAllowed {
				s.Assert().Fail("member %q is not allowed to show up in membership.ChangeEvent.Changes", address)
				continue
			}

			if test.Before {
				s.Assert().NotNil(change.Before, "Expected before to be set for member %q", address)
			} else {
				s.Assert().Nil(change.Before, "Expected before not to be set for member %q", address)
			}

			if test.After {
				s.Assert().NotNil(change.After, "Expected after to be set for member %q", address)
			} else {
				s.Assert().Nil(change.After, "Expected after not to be set for member %q", address)
			}
		}

		for address, test := range MembershipVisibleTransitions {
			_, seen := seenMembers[address]
			s.Assert().Equal(!test.notAllowed, seen, "membership update for member %q inconsistency", address)
		}

		eventFired = true
	}))

	// now send the updates that should trigger the Membership event
	s.m.Update(updateChanges)
	s.Assert().True(eventFired, "expected membership.ChangeEvent to fire during update")
}

// TestMembershipEventsLocalState uses MembershipVisibleTransitions to test
// transitions that are applied to the local node to generate the desired
// membership.ChangeEvent events.
func (s *MemberlistTestSuite) TestMembershipEventsLocalState() {
	for _, test := range MembershipVisibleTransitions {

		// listen for expected event
		fired := false
		l := on(membership.ChangeEvent{}, func(e events.Event) {
			fired = true

			changeEvent := e.(membership.ChangeEvent)
			change := changeEvent.Changes[0]

			if test.notAllowed {
				return
			}

			if test.Before {
				s.Assert().NotNil(change.Before, "Expected before to be set for change %s->%s", test.From, test.To)
			} else {
				s.Assert().Nil(change.Before, "Expected before not to be set for change %s->%s", test.From, test.To)
			}

			if test.After {
				s.Assert().NotNil(change.After, "Expected after to be set for change %s->%s", test.From, test.To)
			} else {
				s.Assert().Nil(change.After, "Expected after not to be set for member change %s->%s", test.From, test.To)
			}
		})

		s.m.SetLocalStatus(test.From)
		s.node.AddListener(l)
		s.m.SetLocalStatus(test.To)
		s.node.RemoveListener(l)

		s.Assert().Equal(!test.notAllowed, fired, "unexpected result for event that fired for change %s->%s", test.From, test.To)
	}
}

// MembershipVisibleLabelChangeStates contains a matrix of states and their
// Observability during label updates from the standpoint of the
// membership.ChangeEvent
var MembershipVisibleLabelChangeStates = map[string]struct {
	State      string
	Observable bool
}{
	"192.0.2.0:1": {State: Alive, Observable: true},
	"192.0.2.0:2": {State: Suspect, Observable: true},
	"192.0.2.0:3": {State: Faulty, Observable: false},
	"192.0.2.0:4": {State: Leave, Observable: false},
	"192.0.2.0:5": {State: Tombstone, Observable: false},
}

// TestMembershipEventsRemoteLabels uses MembershipVisibleLabelChangeStates to
// test for the correct events being emitted during label changes on members
// that are processed after a gossip
func (s *MemberlistTestSuite) TestMembershipEventsRemoteLabels() {
	labelsStart := map[string]string{
		"hello": "world",
	}

	labelsEnd := map[string]string{
		"hello": "universe",
		"foo":   "bar",
	}

	var initialChanges []Change
	var updateChanges []Change

	expectedHostsInUpdate := make(map[string]struct{})

	for address, test := range MembershipVisibleLabelChangeStates {
		initialChanges = append(initialChanges, Change{
			Address:     address,
			Incarnation: s.incarnation,
			Status:      test.State,
			Labels:      labelsStart,
		})

		updateChanges = append(updateChanges, Change{
			Address:     address,
			Incarnation: s.incarnation + 1,
			Status:      test.State,
			Labels:      labelsEnd,
		})

		if test.Observable {
			expectedHostsInUpdate[address] = struct{}{}
		}
	}

	s.m.Update(initialChanges)
	fired := false
	s.node.AddListener(on(membership.ChangeEvent{}, func(e events.Event) {
		fired = true

		changeEvent := e.(membership.ChangeEvent)

		for _, change := range changeEvent.Changes {
			var address string
			if change.Before != nil {
				address = change.Before.GetAddress()
			} else {
				address = change.After.GetAddress()
			}

			_, expected := expectedHostsInUpdate[address]
			s.Assert().True(expected, "the change for member %q was unexpected in the membership.ChangeEvent", address)
			delete(expectedHostsInUpdate, address)

			key := "hello"
			expectedValue := "world"
			value, has := change.Before.Label(key)
			s.Assert().True(has, "expected %q label to be in the before state for member %q", key, address)
			s.Assert().Equal(expectedValue, value, "expected the %q label to be set to %q in the before state for member %q", key, expectedValue, address)

			key = "foo"
			value, has = change.Before.Label(key)
			s.Assert().False(has, "expected %q label to not be in the before state for member %q", key, address)

			key = "hello"
			expectedValue = "universe"
			value, has = change.After.Label(key)
			s.Assert().True(has, "expected %q label to be in the before state for member %q", key, address)
			s.Assert().Equal(expectedValue, value, "expected the %q label to be set to %q in the before state for member %q", key, expectedValue, address)

			key = "foo"
			expectedValue = "bar"
			value, has = change.After.Label(key)
			s.Assert().True(has, "expected %q label to be in the before state for member %q", key, address)
			s.Assert().Equal(expectedValue, value, "expected the %q label to be set to %q in the before state for member %q", key, expectedValue, address)
		}
	}))
	s.m.Update(updateChanges)

	s.Assert().True(fired, "expected the membership.ChangeEvent to fire on the updates")
	s.Assert().Empty(expectedHostsInUpdate, "expected all expected hosts to be removed during the processing of the event, an event is missing")
}

// TestMembershipEventsLocalLabels uses MembershipVisibleLabelChangeStates to
// test for the correct events being emitted during label changes on the local
// node
func (s *MemberlistTestSuite) TestMembershipEventsLocalLabels() {
	labelsStart := map[string]string{
		"hello": "world",
	}

	labelsEnd := map[string]string{
		"hello": "universe",
		"foo":   "bar",
	}

	usedLabelKeys := []string{"hello", "foo"}

	for _, test := range MembershipVisibleLabelChangeStates {
		s.m.RemoveLocalLabels(usedLabelKeys...)
		s.m.SetLocalStatus(test.State)
		s.m.SetLocalLabels(labelsStart)

		fired := false
		l := on(membership.ChangeEvent{}, func(e events.Event) {
			fired = true

			changeEvent := e.(membership.ChangeEvent)
			change := changeEvent.Changes[0]

			key := "hello"
			expectedValue := "world"
			value, has := change.Before.Label(key)
			s.Assert().True(has, "expected %q label to be in the before state for member %q", key)
			s.Assert().Equal(expectedValue, value, "expected the %q label to be set to %q in the before state", key, expectedValue)

			key = "foo"
			value, has = change.Before.Label(key)
			s.Assert().False(has, "expected %q label to not be in the before state", key)

			key = "hello"
			expectedValue = "universe"
			value, has = change.After.Label(key)
			s.Assert().True(has, "expected %q label to be in the before state", key)
			s.Assert().Equal(expectedValue, value, "expected the %q label to be set to %q in the before state", key, expectedValue)

			key = "foo"
			expectedValue = "bar"
			value, has = change.After.Label(key)
			s.Assert().True(has, "expected %q label to be in the before state", key)
			s.Assert().Equal(expectedValue, value, "expected the %q label to be set to %q in the before state", key, expectedValue)
		})
		s.node.AddListener(l)

		s.m.SetLocalLabels(labelsEnd)

		s.node.RemoveListener(l)

		s.Assert().Equal(test.Observable, fired, "unexpected result for event being fired compared to observability of change")
	}
}

func TestMemberlistTestSuite(t *testing.T) {
	suite.Run(t, new(MemberlistTestSuite))
}
