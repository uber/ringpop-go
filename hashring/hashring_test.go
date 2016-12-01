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

	"github.com/uber/ringpop-go/membership"

	"github.com/dgryski/go-farm"
	"github.com/uber/ringpop-go/events"

	"github.com/stretchr/testify/assert"
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

func TestAddServer(t *testing.T) {
	ring := New(farm.Fingerprint32, 10)
	l := &dummyListener{}
	ring.AddListener(l)

	ring.AddServer(fakeMember{address: "server1"})
	ring.AddServer(fakeMember{address: "server2"})
	assert.Equal(t, 4, l.EventCount())
	assert.True(t, ring.HasServer("server1"), "expected server to be in ring")
	assert.True(t, ring.HasServer("server2"), "expected server to be in ring")
	assert.False(t, ring.HasServer("server3"), "expected server to not be in ring")

	ring.AddServer(fakeMember{address: "server1"})
	assert.Equal(t, 4, l.EventCount())
	assert.True(t, ring.HasServer("server1"), "expected server to be in ring")
}

func TestRemoveServer(t *testing.T) {
	ring := New(farm.Fingerprint32, 10)
	l := &dummyListener{}
	ring.AddListener(l)

	ring.AddServer(fakeMember{address: "server1"})
	ring.AddServer(fakeMember{address: "server2"})
	assert.Equal(t, 4, l.EventCount())
	assert.True(t, ring.HasServer("server1"), "expected server to be in ring")
	assert.True(t, ring.HasServer("server2"), "expected server to be in ring")

	ring.RemoveServer(fakeMember{address: "server1"})
	assert.Equal(t, 6, l.EventCount())
	assert.False(t, ring.HasServer("server1"), "expected server to not be in ring")
	assert.True(t, ring.HasServer("server2"), "expected server to be in ring")

	ring.RemoveServer(fakeMember{address: "server3"})
	assert.Equal(t, 6, l.EventCount())
	assert.False(t, ring.HasServer("server3"), "expected server to not be in ring")

	ring.RemoveServer(fakeMember{address: "server1"})
	assert.Equal(t, 6, l.EventCount())
	assert.False(t, ring.HasServer("server3"), "expected server to not be in ring")
}

func TestChecksumChanges(t *testing.T) {
	ring := New(farm.Fingerprint32, 10)
	checksum := ring.Checksum()

	ring.AddServer(fakeMember{address: "server1"})
	ring.AddServer(fakeMember{address: "server2"})
	assert.NotEqual(t, checksum, ring.Checksum(), "expected checksum to have changed on server add")

	checksum = ring.Checksum()
	ring.RemoveServer(fakeMember{address: "server1"})

	assert.NotEqual(t, checksum, ring.Checksum(), "expected checksum to have changed on server remove")
}

func TestServerCount(t *testing.T) {
	ring := New(farm.Fingerprint32, 10)
	assert.Equal(t, 0, ring.ServerCount(), "expected one server to be in ring")

	ring.AddServer(fakeMember{address: "server1"})
	ring.AddServer(fakeMember{address: "server2"})
	ring.AddServer(fakeMember{address: "server3"})

	assert.Equal(t, 3, ring.ServerCount(), "expected three servers to be in ring")

	ring.RemoveServer(fakeMember{address: "server1"})

	assert.Equal(t, 2, ring.ServerCount(), "expected two servers to be in ring")
}

func TestServers(t *testing.T) {
	ring := New(farm.Fingerprint32, 10)
	assert.Equal(t, 0, ring.ServerCount(), "expected one server to be in ring")

	ring.AddServer(fakeMember{address: "server1"})
	ring.AddServer(fakeMember{address: "server2"})
	ring.AddServer(fakeMember{address: "server3"})

	servers := ring.Servers()
	sort.Strings(servers)

	assert.Equal(t, 3, len(servers), "expected three servers to be in ring")
	assert.Equal(t, "server1", servers[0], "expected server1")
	assert.Equal(t, "server2", servers[1], "expected server2")
	assert.Equal(t, "server3", servers[2], "expected server3")
}

func TestAddRemoveServers(t *testing.T) {
	ring := New(farm.Fingerprint32, 10)
	l := &dummyListener{}
	ring.AddListener(l)
	add := []membership.Member{
		fakeMember{address: "server1"},
		fakeMember{address: "server2"},
	}
	remove := []membership.Member{
		fakeMember{address: "server3"},
		fakeMember{address: "server4"},
	}

	ring.AddRemoveServers(remove, nil)
	assert.Equal(t, 2, l.EventCount())
	assert.Equal(t, 2, ring.ServerCount(), "expected two servers to be in ring")

	oldChecksum := ring.Checksum()

	ring.AddRemoveServers(add, remove)
	assert.Equal(t, 4, l.EventCount())
	assert.Equal(t, 2, ring.ServerCount(), "expected two servers to be in ring")

	assert.NotEqual(t, oldChecksum, ring.Checksum(), "expected checksum to change")
}

func TestLookup(t *testing.T) {
	ring := New(farm.Fingerprint32, 10)
	ring.AddServer(fakeMember{address: "server1"})
	ring.AddServer(fakeMember{address: "server2"})

	_, ok := ring.Lookup("key")

	assert.True(t, ok, "expected Lookup to hash key to a server")

	ring.RemoveServer(fakeMember{address: "server1"})
	ring.RemoveServer(fakeMember{address: "server2"})

	_, ok = ring.Lookup("key")

	assert.False(t, ok, "expected Lookup to find no server for key to hash to")
}

func TestLookupDistribution(t *testing.T) {
	ring := New(farm.Fingerprint32, 5)
	addresses := genMembers(1, 1, 1000)
	ring.AddRemoveServers(addresses, nil)

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
	members := genMembers(1, 1, 100)
	ring.AddRemoveServers(members, nil)
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
	addresses := genMembers(1, 1, 10)
	ring.AddRemoveServers(addresses, nil)
	assert.Len(t, ring.LookupN("a random key", 20), 10, "expected that LookupN caps results when n is larger than number of servers")
}

func TestLookupNLoopAround(t *testing.T) {
	ring := New(farm.Fingerprint32, 1)
	addresses := genMembers(1, 1, 10)
	ring.AddRemoveServers(addresses, nil)

	unique := make(map[valuetype]struct{})
	ring.tree.LookupNUniqueAt(1, replicaPoint(0), unique)
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

	addresses := genMembers(1, 1, 10)
	ring.AddRemoveServers(addresses, nil)

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

	ring.RemoveServer(addresses[0])
	servers = ring.LookupN("yet another key", 10)
	assert.Len(t, servers, 9, "expected to get nine servers")
	for _, server := range servers {
		unique[server] = true
	}
	assert.Len(t, unique, 9, "expected to get nine unique servers")
}

func genMembers(host, fromPort, toPort int) []membership.Member {
	var addresses []membership.Member
	for i := fromPort; i <= toPort; i++ {
		addresses = append(addresses, fakeMember{
			address: fmt.Sprintf("127.0.0.%v:%v", host, 3000+i),
		})
	}
	return addresses
}

func BenchmarkHashRingLookupN(b *testing.B) {
	b.StopTimer()

	ring := New(farm.Fingerprint32, 100)

	members := make([]membership.Member, 1000)
	for i := range members {
		members[i] = fakeMember{
			address: fmt.Sprintf("%d", rand.Int()),
		}
	}
	ring.AddRemoveServers(members, nil)

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
