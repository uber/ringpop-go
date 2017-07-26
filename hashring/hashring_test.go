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

package hashring

import (
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"testing"

	"github.com/uber/ringpop-go/events"
	eventsmocks "github.com/uber/ringpop-go/events/test/mocks"
	"github.com/uber/ringpop-go/membership"

	"github.com/dgryski/go-farm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

// fake event listener
type dummyListener struct {
	l      sync.Mutex
	events int
}

func (d *dummyListener) EventCount() int {
	d.l.Lock()
	events := d.events
	d.l.Unlock()
	return events
}

func (d *dummyListener) HandleEvent(event events.Event) {
	d.l.Lock()
	d.events++
	d.l.Unlock()
}

func TestAddMembers(t *testing.T) {
	ring := New(farm.Fingerprint32, 10)
	l := &dummyListener{}
	ring.AddListener(l)

	ring.AddMembers(fakeMember{address: "server1"})
	ring.AddMembers(fakeMember{address: "server2"})
	assert.Equal(t, 4, l.EventCount())
	assert.True(t, ring.HasServer("server1"), "expected server to be in ring")
	assert.True(t, ring.HasServer("server2"), "expected server to be in ring")
	assert.False(t, ring.HasServer("server3"), "expected server to not be in ring")

	ring.AddMembers(fakeMember{address: "server1"})
	assert.Equal(t, 4, l.EventCount())
	assert.True(t, ring.HasServer("server1"), "expected server to be in ring")
}

func TestRemoveMembers(t *testing.T) {
	ring := New(farm.Fingerprint32, 10)
	l := &dummyListener{}
	ring.AddListener(l)

	ring.AddMembers(fakeMember{address: "server1"})
	ring.AddMembers(fakeMember{address: "server2"})
	assert.Equal(t, 4, l.EventCount())
	assert.True(t, ring.HasServer("server1"), "expected server to be in ring")
	assert.True(t, ring.HasServer("server2"), "expected server to be in ring")

	ring.RemoveMembers(fakeMember{address: "server1"})
	assert.Equal(t, 6, l.EventCount())
	assert.False(t, ring.HasServer("server1"), "expected server to not be in ring")
	assert.True(t, ring.HasServer("server2"), "expected server to be in ring")

	ring.RemoveMembers(fakeMember{address: "server3"})
	assert.Equal(t, 6, l.EventCount())
	assert.False(t, ring.HasServer("server3"), "expected server to not be in ring")

	ring.RemoveMembers(fakeMember{address: "server1"})
	assert.Equal(t, 6, l.EventCount())
	assert.False(t, ring.HasServer("server3"), "expected server to not be in ring")
}

func TestConsistentLookupsOnDuplicates(t *testing.T) {
	ring1 := New(farm.Fingerprint32, 10)
	ring2 := New(farm.Fingerprint32, 10)

	member1 := fakeMember{
		address:  "server1",
		identity: "id",
	}
	member2 := fakeMember{
		address:  "server2",
		identity: "id",
	}

	ring1.AddMembers(member1)
	ring1.AddMembers(member2)

	// add in different order
	ring2.AddMembers(member2)
	ring2.AddMembers(member1)

	lookup1, _ := ring1.Lookup("id#1")
	lookup2, _ := ring2.Lookup("id#1")
	assert.Equal(t, lookup1, lookup2, "Order of adds does not affect lookups")
	assert.Equal(t, ring1.checksums["replica"], ring2.checksums["replica"], "Order of adds does not affect checksums")
}

func TestChecksumChanges(t *testing.T) {
	ring := New(farm.Fingerprint32, 10)
	checksum := ring.Checksum()

	ring.AddMembers(fakeMember{address: "server1"})
	ring.AddMembers(fakeMember{address: "server2"})
	assert.NotEqual(t, checksum, ring.Checksum(), "expected checksum to have changed on server add")

	checksum = ring.Checksum()
	ring.RemoveMembers(fakeMember{address: "server1"})

	assert.NotEqual(t, checksum, ring.Checksum(), "expected checksum to have changed on server remove")
}

func TestHashRing_Checksums(t *testing.T) {
	getSortedKeys := func(m map[string]uint32) []string {
		keys := make([]string, 0, len(m))
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		return keys
	}

	ring := New(farm.Fingerprint32, 10)
	ring.checksummers = map[string]Checksummer{
		"identity":     &identityChecksummer{},
		"address":      &addressChecksummer{},
		"replicaPoint": &replicaPointChecksummer{},
	}

	checkSummers := []string{"identity", "address", "replicaPoint"}
	sort.Strings(checkSummers)

	ring.AddMembers(fakeMember{address: "server1"})
	ring.AddMembers(fakeMember{address: "server2"})

	checksums := ring.Checksums()
	assert.Equal(t, checkSummers, getSortedKeys(checksums), "Expected all checksums to be computed")
	ring.RemoveMembers(fakeMember{address: "server1"})

	assert.NotEqual(t, checksums, ring.Checksums(), "expected checksums to have changed on server remove")
	assert.Equal(t, checkSummers, getSortedKeys(checksums), "Expected all checksums to be computed")
}

func TestServerCount(t *testing.T) {
	ring := New(farm.Fingerprint32, 10)
	assert.Equal(t, 0, ring.ServerCount(), "expected one server to be in ring")

	ring.AddMembers(fakeMember{address: "server1"})
	ring.AddMembers(fakeMember{address: "server2"})
	ring.AddMembers(fakeMember{address: "server3"})

	assert.Equal(t, 3, ring.ServerCount(), "expected three servers to be in ring")

	ring.RemoveMembers(fakeMember{address: "server1"})

	assert.Equal(t, 2, ring.ServerCount(), "expected two servers to be in ring")
}

func TestServers(t *testing.T) {
	ring := New(farm.Fingerprint32, 10)
	assert.Equal(t, 0, ring.ServerCount(), "expected one server to be in ring")

	ring.AddMembers(fakeMember{address: "server1"})
	ring.AddMembers(fakeMember{address: "server2"})
	ring.AddMembers(fakeMember{address: "server3"})

	servers := ring.Servers()
	sort.Strings(servers)

	assert.Equal(t, 3, len(servers), "expected three servers to be in ring")
	assert.Equal(t, "server1", servers[0], "expected server1")
	assert.Equal(t, "server2", servers[1], "expected server2")
	assert.Equal(t, "server3", servers[2], "expected server3")
}

func TestLookup(t *testing.T) {
	ring := New(farm.Fingerprint32, 10)
	ring.AddMembers(fakeMember{address: "server1"})
	ring.AddMembers(fakeMember{address: "server2"})

	_, ok := ring.Lookup("key")

	assert.True(t, ok, "expected Lookup to hash key to a server")

	ring.RemoveMembers(fakeMember{address: "server1"})
	ring.RemoveMembers(fakeMember{address: "server2"})

	_, ok = ring.Lookup("key")

	assert.False(t, ok, "expected Lookup to find no server for key to hash to")
}

func TestLookupDistribution(t *testing.T) {
	ring := New(farm.Fingerprint32, 5)
	members := genMembers(1, 1, 1000, false)
	ring.AddMembers(members...)

	keys := make([]string, 40)
	for i := range keys {
		keys[i] = fmt.Sprintf("%d", i)
	}

	servers := make(map[string]bool)
	for _, key := range keys {
		server, ok := ring.Lookup(key)
		assert.True(t, ok, "expected that the lookup is a success")
		servers[server] = true
	}

	// With this specific set of keys we don't expect any collisions
	assert.Len(t, servers, len(keys), "expected that all keys are owned by a different server")
}

// TestLookupNNoGaps tests the selected servers from LookupN form a contiguous
// section of all hashes in the red-black tree.
func TestLookupNNoGaps(t *testing.T) {
	ring := New(farm.Fingerprint32, 1)
	members := genMembers(1, 1, 100, false)
	ring.AddMembers(members...)
	key := "key with small hash"

	servers := ring.LookupN(key, 20)

	serversSet := make(map[string]struct{})
	for _, s := range servers {
		serversSet[s] = struct{}{}
	}

	// We are reconstructing the values under which the servers are stored in
	// the red-black tree. This approach is brittle but it gives us deeper
	// introspection into the internals of the tree. The hashring is configured
	// to only store one replica per server, if we didn't, it would be
	// impossible to find out which specific replica has been iterated over
	// by LookupN.
	hashes := make([]int, 0, len(servers))
	for _, s := range servers {
		// We are appending 0 to get the first (and only) replica of the server.
		hashes = append(hashes, ring.hashfunc(s+"0"))
	}

	min := hashes[0]
	for _, h := range hashes {
		if h < min {
			min = h
		}
	}
	max := hashes[0]
	for _, h := range hashes {
		if h > max {
			max = h
		}
	}

	// Here we are checking that the nodes that we lookup are part of a
	// contiguous series of the red-black tree nodes. All servers that
	// aren't part of the lookup should be stored under a value either smaller,
	// or larger than the values of the servers that are part of the lookup.
	allExcluded := true
	for _, member := range members {
		if _, ok := serversSet[member.GetAddress()]; ok {
			continue
		}
		adHash := ring.hashfunc(member.GetAddress() + "0")
		if adHash >= min && adHash <= max {
			allExcluded = false
		}
	}

	assert.True(t, allExcluded, "Expect addresses to be a contiguous section of the ring")
}

func TestLookupNOverflow(t *testing.T) {
	ring := New(farm.Fingerprint32, 10)
	members := genMembers(1, 1, 10, false)
	ring.AddMembers(members...)
	assert.Len(t, ring.LookupN("a random key", 20), 10, "expected that LookupN caps results when n is larger than number of servers")
}

func TestLookupNLoopAround(t *testing.T) {
	ring := New(farm.Fingerprint32, 1)
	members := genMembers(1, 1, 10, false)
	ring.AddMembers(members...)

	unique := make(map[valuetype]struct{})
	ring.tree.LookupNUniqueAt(1, replicaPoint{hash: 0}, unique)
	var firstInTree valuetype
	for server := range unique {
		firstInTree = server
		break
	}

	firstResult, ok := ring.Lookup("a random key")
	assert.True(t, ok, "expected to obtain server that owns key")
	assert.NotEqual(t, firstResult, firstInTree, "expected to test case where the key doesn't land at the first tree node")

	result := ring.LookupN("a random key", 9)
	assert.Contains(t, result, firstResult, "expected to have looped around the ring")
}

func TestLookupN(t *testing.T) {
	ring := New(farm.Fingerprint32, 10)
	servers := ring.LookupN("nil", 5)
	assert.Len(t, servers, 0, "expected no servers")

	unique := make(map[string]bool)

	members := genMembers(1, 1, 10, false)
	ring.AddMembers(members...)

	servers = ring.LookupN("key", 5)
	assert.Len(t, servers, 5, "expected five servers to be returned by lookup")
	for _, server := range servers {
		unique[server] = true
	}
	assert.Len(t, unique, 5, "expected to get five unique servers")

	unique = make(map[string]bool)

	servers = ring.LookupN("another key", 100)
	assert.Len(t, servers, 10, "expected to get max number of servers")
	for _, server := range servers {
		unique[server] = true
	}
	assert.Len(t, unique, 10, "expected to get 10 unique servers")

	unique = make(map[string]bool)

	ring.RemoveMembers(members[0])
	servers = ring.LookupN("yet another key", 10)
	assert.Len(t, servers, 9, "expected to get nine servers")
	for _, server := range servers {
		unique[server] = true
	}
	assert.Len(t, unique, 9, "expected to get nine unique servers")
}

func TestLookupNOrder(t *testing.T) {
	tests := []struct {
		n              int
		replicaPoints  int
		serverQuantity int
		label          string
	}{
		{
			n:              10,
			replicaPoints:  5,
			serverQuantity: 10,
			label:          "some_uuid_1",
		},
		{
			n:              5,
			replicaPoints:  3,
			serverQuantity: 10,
			label:          "some_uuid_2",
		},
		{
			n:              5,
			replicaPoints:  2,
			serverQuantity: 1,
			label:          "some_uuid_3",
		},
		{
			n:              100,
			replicaPoints:  7,
			serverQuantity: 10,
			label:          "some_uuid_4",
		},
	}

	for _, tt := range tests {
		ring := New(farm.Fingerprint32, tt.replicaPoints)
		members := genMembers(1, 1, tt.serverQuantity, true)
		ring.AddMembers(members...)

		unique := make(map[string]bool)

		key := replicaPoint{hash: ring.hashfunc(tt.label)}
		var expectedServers []string
		ring.tree.root.traverseWhile(func(node *redBlackNode) bool {
			server := node.value.(string)
			if node.key.Compare(key) >= 0 && !unique[server] {
				expectedServers = append(expectedServers, server)
				unique[server] = true
			}
			if len(expectedServers) >= tt.n {
				return false
			}
			return true
		})

		baseKey := replicaPoint{hash: 0}
		if len(expectedServers) < tt.n {
			ring.tree.root.traverseWhile(func(node *redBlackNode) bool {
				server := node.value.(string)
				if node.key.Compare(baseKey) >= 0 && !unique[server] {
					expectedServers = append(expectedServers, server)
					unique[server] = true
				}
				if len(expectedServers) >= tt.n {
					return false
				}
				return true
			})
		}

		assert.Equal(t, expectedServers, ring.LookupN(tt.label, tt.n), tt.label)
	}
}

type ProcessMembershipChangesSuite struct {
	suite.Suite
	ring    *HashRing
	members [4]fakeMember
	l       *eventsmocks.EventListener
}

func (s *ProcessMembershipChangesSuite) SetupSuite() {
	s.ring = New(farm.Fingerprint32, 10)
	s.members[0] = fakeMember{address: "192.0.2.0:1"}
	s.members[1] = fakeMember{address: "192.0.2.0:2"}
	s.members[2] = fakeMember{address: "192.0.2.0:3"}
	s.members[3] = fakeMember{address: "192.0.2.0:4"}
}

func (s *ProcessMembershipChangesSuite) SetupTest() {
	s.l = &eventsmocks.EventListener{}
	s.ring.AddListener(s.l)
}

func (s *ProcessMembershipChangesSuite) TearDownTest() {
	s.ring.RemoveListener(s.l)
}

func (s *ProcessMembershipChangesSuite) expectRingChangedEvent(event events.RingChangedEvent) {
	s.l.On("HandleEvent", mock.AnythingOfType("RingChecksumEvent"))
	s.l.On("HandleEvent", event)
}

func (s *ProcessMembershipChangesSuite) TestAddMember0() {
	s.expectRingChangedEvent(events.RingChangedEvent{
		ServersAdded: []string{s.members[0].GetAddress()},
	})

	s.ring.ProcessMembershipChanges([]membership.MemberChange{
		{After: s.members[0]},
	})
	mock.AssertExpectationsForObjects(s.T(), s.l.Mock)
	s.Equal(1, s.ring.ServerCount(), "unexpected count of members in ring")
}

func (s *ProcessMembershipChangesSuite) TestAddMember1() {
	s.expectRingChangedEvent(events.RingChangedEvent{
		ServersAdded: []string{s.members[1].GetAddress()},
	})

	s.ring.ProcessMembershipChanges([]membership.MemberChange{
		{After: s.members[1]},
	})
	mock.AssertExpectationsForObjects(s.T(), s.l.Mock)
	s.Equal(2, s.ring.ServerCount(), "unexpected count of members in ring")
}

func (s *ProcessMembershipChangesSuite) TestRemoveMember0AddMember2() {
	s.expectRingChangedEvent(events.RingChangedEvent{
		ServersAdded:   []string{s.members[2].GetAddress()},
		ServersRemoved: []string{s.members[0].GetAddress()},
	})

	s.ring.ProcessMembershipChanges([]membership.MemberChange{
		{After: s.members[2]},
		{Before: s.members[0]},
	})
	mock.AssertExpectationsForObjects(s.T(), s.l.Mock)
	s.Equal(2, s.ring.ServerCount(), "unexpected count of members in ring")
}

func (s *ProcessMembershipChangesSuite) TestNoopUpdate() {
	s.l.On("HandleEvent", mock.Anything).Return()
	s.ring.ProcessMembershipChanges([]membership.MemberChange{
		{Before: s.members[1], After: s.members[1]},
	})

	s.l.AssertNotCalled(s.T(), "HandleEvent", mock.AnythingOfType("RingChecksumEvent"))
	s.l.AssertNotCalled(s.T(), "HandleEvent", mock.AnythingOfType("RingChangedEvent"))

	s.Equal(2, s.ring.ServerCount(), "unexpected count of members in ring")
}

func (s *ProcessMembershipChangesSuite) TestChangeIdentityMember2() {
	s.expectRingChangedEvent(events.RingChangedEvent{
		ServersUpdated: []string{s.members[1].GetAddress()},
	})

	memberNewIdentity := fakeMember{
		address:  "192.0.2.0:2",
		identity: "new_identity",
	}
	s.ring.ProcessMembershipChanges([]membership.MemberChange{
		{Before: s.members[1], After: memberNewIdentity},
	})
	mock.AssertExpectationsForObjects(s.T(), s.l.Mock)
	s.Equal(2, s.ring.ServerCount(), "unexpected count of members in ring")
}

func (s *ProcessMembershipChangesSuite) TestRemoveNonExistingMember() {
	s.l.On("HandleEvent", mock.Anything).Return()
	s.ring.ProcessMembershipChanges([]membership.MemberChange{
		{Before: s.members[3]},
	})

	s.l.AssertNotCalled(s.T(), "HandleEvent", mock.AnythingOfType("RingChecksumEvent"))
	s.l.AssertNotCalled(s.T(), "HandleEvent", mock.AnythingOfType("RingChangedEvent"))

	s.Equal(2, s.ring.ServerCount(), "unexpected count of members in ring")
}

func (s *ProcessMembershipChangesSuite) TestUpdateNonExistingMember() {
	s.l.On("HandleEvent", mock.Anything).Return()
	s.ring.ProcessMembershipChanges([]membership.MemberChange{
		{Before: s.members[3], After: s.members[3]},
	})

	s.l.AssertNotCalled(s.T(), "HandleEvent", mock.AnythingOfType("RingChecksumEvent"))
	s.l.AssertNotCalled(s.T(), "HandleEvent", mock.AnythingOfType("RingChangedEvent"))

	s.Equal(2, s.ring.ServerCount(), "unexpected count of members in ring")
}

func TestNodeTestSuite(t *testing.T) {
	suite.Run(t, new(ProcessMembershipChangesSuite))
}

func TestLookupsWithIdentities(t *testing.T) {
	numReplicaPoints := 3
	numServers := 10
	ring := New(farm.Fingerprint32, numReplicaPoints)

	members := make([]membership.Member, 0, numServers)

	for i := 0; i < numServers; i++ {
		m := fakeMember{
			address:  fmt.Sprintf("127.0.0.1:%v", 3000+i),
			identity: fmt.Sprintf("identity%v", i),
		}
		members = append(members, m)
	}

	ring.AddMembers(members...)

	for _, m := range members {
		identity := m.Identity()

		for i := 0; i < numReplicaPoints; i++ {
			key := fmt.Sprintf("%v#%v", identity, i)

			value, has := ring.Lookup(key)
			assert.True(t, has)
			assert.Equal(t, m.GetAddress(), value)
		}
	}
}

func TestReplicaPointCompare(t *testing.T) {
	address := "127.0.0.1:3000"

	pointA := replicaPoint{hash: 100, address: address, index: 0}
	pointB := replicaPoint{hash: 200, address: address, index: 0}
	pointC := replicaPoint{hash: 300, address: address, index: 0}
	pointD := replicaPoint{hash: 200, address: address, index: 0}
	pointE := replicaPoint{hash: 200, address: address, index: 1}

	assert.True(t, pointB.Compare(pointA) > 0)
	assert.True(t, pointB.Compare(pointC) < 0)
	assert.True(t, pointB.Compare(pointB) == 0)
	assert.True(t, pointB.Compare(pointD) == 0)
	assert.True(t, pointB.Compare(pointE) < 0)
}

func genMembers(host, fromPort, toPort int, overrideIdentity bool) (members []membership.Member) {
	for i := fromPort; i <= toPort; i++ {
		member := fakeMember{
			address: fmt.Sprintf("127.0.0.%v:%v", host, 3000+i),
		}
		if overrideIdentity {
			member.identity = fmt.Sprintf("identity%v", i)
		}
		members = append(members, member)
	}
	return members
}

func BenchmarkHashRingLookupN(b *testing.B) {
	b.StopTimer()

	ring := New(farm.Fingerprint32, 100)

	members := genMembers(1, 1, 10, true)
	ring.AddMembers(members...)

	keys := make([]string, 100)
	for i := range keys {
		keys[i] = fmt.Sprintf("%d", rand.Int())
	}

	b.StartTimer()
	for n := 0; n < b.N; n++ {
		for _, key := range keys {
			_ = ring.LookupN(key, 10)
		}
	}
}
