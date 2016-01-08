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

func TestEvents(t *testing.T) {
	ring := New(farm.Fingerprint32, 10)
	l := &dummyListener{}
	ring.RegisterListener(l)
	ring.emit(struct{}{})
	assert.Equal(t, 1, l.events, "expected one event to be emitted")
}

func TestAddServer(t *testing.T) {
	ring := New(farm.Fingerprint32, 10)
	ring.AddServer("server1")
	ring.AddServer("server2")

	assert.True(t, ring.HasServer("server1"), "expected server to be in ring")
	assert.True(t, ring.HasServer("server2"), "expected server to be in ring")
	assert.False(t, ring.HasServer("server3"), "expected server to not be in ring")

	ring.AddServer("server1")
	assert.True(t, ring.HasServer("server1"), "expected server to be in ring")
}

func TestRemoveServer(t *testing.T) {
	ring := New(farm.Fingerprint32, 10)
	ring.AddServer("server1")
	ring.AddServer("server2")

	assert.True(t, ring.HasServer("server1"), "expected server to be in ring")
	assert.True(t, ring.HasServer("server2"), "expected server to be in ring")

	ring.RemoveServer("server1")

	assert.False(t, ring.HasServer("server1"), "expected server to not be in ring")
	assert.True(t, ring.HasServer("server2"), "expected server to be in ring")

	ring.RemoveServer("server3")

	assert.False(t, ring.HasServer("server3"), "expected server to not be in ring")
}

func TestChecksumChanges(t *testing.T) {
	ring := New(farm.Fingerprint32, 10)
	checksum := ring.Checksum()

	ring.AddServer("server1")
	ring.AddServer("server2")
	assert.NotEqual(t, checksum, ring.Checksum(), "expected checksum to have changed on server add")

	checksum = ring.Checksum()
	ring.RemoveServer("server1")

	assert.NotEqual(t, checksum, ring.Checksum(), "expected checksum to have changed on server remove")
}

func TestServerCount(t *testing.T) {
	ring := New(farm.Fingerprint32, 10)
	assert.Equal(t, 0, ring.ServerCount(), "expected one server to be in ring")

	ring.AddServer("server1")
	ring.AddServer("server2")
	ring.AddServer("server3")

	assert.Equal(t, 3, ring.ServerCount(), "expected three servers to be in ring")

	ring.RemoveServer("server1")

	assert.Equal(t, 2, ring.ServerCount(), "expected two servers to be in ring")
}

func TestServers(t *testing.T) {
	ring := New(farm.Fingerprint32, 10)
	assert.Equal(t, 0, ring.ServerCount(), "expected one server to be in ring")

	ring.AddServer("server1")
	ring.AddServer("server2")
	ring.AddServer("server3")

	servers := ring.Servers()
	sort.Strings(servers)

	assert.Equal(t, 3, len(servers), "expected three servers to be in ring")
	assert.Equal(t, "server1", servers[0], "expected server1")
	assert.Equal(t, "server2", servers[1], "expected server2")
	assert.Equal(t, "server3", servers[2], "expected server3")
}

func TestAddRemoveServers(t *testing.T) {
	ring := New(farm.Fingerprint32, 10)
	add := []string{"server1", "server2"}
	remove := []string{"server3", "server4"}

	ring.AddRemoveServers(remove, nil)

	assert.Equal(t, 2, ring.ServerCount(), "expected two servers to be in ring")

	oldChecksum := ring.Checksum()

	ring.AddRemoveServers(add, remove)

	assert.Equal(t, 2, ring.ServerCount(), "expected two servers to be in ring")

	assert.NotEqual(t, oldChecksum, ring.Checksum(), "expected checksum to change")
}

func TestLookup(t *testing.T) {
	ring := New(farm.Fingerprint32, 10)
	ring.AddServer("server1")
	ring.AddServer("server2")

	_, ok := ring.Lookup("key")

	assert.True(t, ok, "expected Lookup to hash key to a server")

	ring.RemoveServer("server1")
	ring.RemoveServer("server2")

	_, ok = ring.Lookup("key")

	assert.False(t, ok, "expected Lookup to find no server for key to hash to")
}

func TestLookupNOverflow(t *testing.T) {
	ring := New(farm.Fingerprint32, 10)
	addresses := genAddresses(1, 1, 10)
	ring.AddRemoveServers(addresses, nil)
	assert.Len(t, ring.LookupN("a random key", 20), 10, "expected that LookupN caps results when n is larger than number of servers")
}

func TestLookupNLoopAround(t *testing.T) {
	ring := New(farm.Fingerprint32, 1)
	addresses := genAddresses(1, 1, 10)
	ring.AddRemoveServers(addresses, nil)

	unique := make(map[string]struct{})
	ring.tree.LookupNUniqueAt(1, 0, unique)
	var firstInTree string
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

	addresses := genAddresses(1, 1, 10)
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

func genAddresses(host, fromPort, toPort int) []string {
	var addresses []string
	for i := fromPort; i <= toPort; i++ {
		addresses = append(addresses, fmt.Sprintf("127.0.0.%v:%v", host, 3000+i))
	}
	return addresses
}

func BenchmarkHashRingLookupN(b *testing.B) {
	b.StopTimer()

	ring := New(farm.Fingerprint32, 100)

	servers := make([]string, 1000)
	for i := range servers {
		servers[i] = fmt.Sprintf("%d", rand.Int())
	}
	ring.AddRemoveServers(servers, nil)

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
