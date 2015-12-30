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
	"bytes"
	"fmt"
	"sort"
	"sync"

	"github.com/uber/ringpop-go/events"

	"github.com/dgryski/go-farm"
)

// HashRingConfiguration is a configuration struct that can be passed to the
// Ringpop constructor to customize hash ring options.
type HashRingConfiguration struct {
	// ReplicaPoints is the number of positions a node will be assigned on the
	// ring. A bigger number will provide better key distribution, but require
	// more computation when building or traversing the ring (typically on
	// lookups or membership changes).
	ReplicaPoints int
}

type HashRing struct {
	hashfunc      func([]byte) uint32
	replicaPoints int

	servers struct {
		byAddress map[string]bool
		tree      *RedBlackTree
		checksum  uint32
		sync.RWMutex
	}

	listeners []events.EventListener
}

func (r *HashRing) emit(event interface{}) {
	for _, listener := range r.listeners {
		listener.HandleEvent(event)
	}
}

// RegisterListener adds a listener that will be sent swim events.
func (r *HashRing) RegisterListener(l events.EventListener) {
	r.listeners = append(r.listeners, l)
}

func NewHashRing(hashfunc func([]byte) uint32, replicaPoints int) *HashRing {
	ring := &HashRing{
		hashfunc:      hashfunc,
		replicaPoints: replicaPoints,
	}

	ring.servers.byAddress = make(map[string]bool)
	ring.servers.tree = &RedBlackTree{}

	return ring
}

func (r *HashRing) Checksum() uint32 {
	r.servers.RLock()
	checksum := r.servers.checksum
	r.servers.RUnlock()

	return checksum
}

// computeChecksum computes checksum of all servers in the ring
func (r *HashRing) computeChecksum() {
	var addresses sort.StringSlice
	var buffer bytes.Buffer

	r.servers.Lock()
	for address := range r.servers.byAddress {
		addresses = append(addresses, address)
	}

	addresses.Sort()

	for _, address := range addresses {
		buffer.WriteString(address)
		buffer.WriteString(";")
	}

	old := r.servers.checksum
	r.servers.checksum = farm.Fingerprint32(buffer.Bytes())
	r.emit(events.RingChecksumEvent{
		OldChecksum: old,
		NewChecksum: r.servers.checksum,
	})

	r.servers.Unlock()
}

func (r *HashRing) AddServer(address string) {
	if r.HasServer(address) {
		return
	}

	r.addReplicas(address)
	r.emit(events.RingChangedEvent{ServersAdded: []string{address}})
	r.computeChecksum()
}

// inserts server replicas into ring
func (r *HashRing) addReplicas(server string) {
	r.servers.Lock()
	r.servers.byAddress[server] = true

	for i := 0; i < r.replicaPoints; i++ {
		address := fmt.Sprintf("%s%v", server, i)
		r.servers.tree.Insert(int(r.hashfunc([]byte(address))), server)
	}

	r.servers.Unlock()
}

func (r *HashRing) RemoveServer(address string) {
	if !r.HasServer(address) {
		return
	}

	r.RemoveReplicas(address)
	r.emit(events.RingChangedEvent{ServersRemoved: []string{address}})
	r.computeChecksum()
}

func (r *HashRing) RemoveReplicas(server string) {
	r.servers.Lock()

	delete(r.servers.byAddress, server)

	for i := 0; i < r.replicaPoints; i++ {
		address := fmt.Sprintf("%s%v", server, i)
		r.servers.tree.Delete(int(r.hashfunc([]byte(address))))
	}

	r.servers.Unlock()
}

// adds and removes servers in a batch with a single checksum computation at the end
func (r *HashRing) AddRemoveServers(add []string, remove []string) bool {
	var changed, added, removed bool

	for _, server := range add {
		if !r.HasServer(server) {
			r.addReplicas(server)
			added = true
		}
	}

	for _, server := range remove {
		if r.HasServer(server) {
			r.RemoveReplicas(server)
			removed = true
		}
	}

	changed = added || removed

	if changed {
		r.emit(events.RingChangedEvent{add, remove})
		r.computeChecksum()
	}

	return changed
}

// hasServer returns true if the server exists in the ring, false otherwise
func (r *HashRing) HasServer(address string) bool {
	r.servers.RLock()
	server := r.servers.byAddress[address]
	r.servers.RUnlock()

	return server
}

func (r *HashRing) GetServers() (servers []string) {
	r.servers.RLock()
	for server := range r.servers.byAddress {
		servers = append(servers, server)
	}
	r.servers.RUnlock()

	return servers
}

// ServerCount returns the number of servers in the ring
func (r *HashRing) ServerCount() int {
	r.servers.RLock()
	count := len(r.servers.byAddress)
	r.servers.RUnlock()

	return count
}

// Lookup returns the owner of the given key.
func (r *HashRing) Lookup(key string) (string, bool) {
	strs := r.LookupN(key, 1)
	if len(strs) == 0 {
		return "", false
	}
	return strs[0], true
}

func (r *HashRing) LookupN(key string, n int) []string {
	serverCount := r.ServerCount()
	if n > serverCount {
		n = serverCount
	}

	r.servers.RLock()

	// Iterate over RB-tree and collect unique servers
	var unique = make(map[string]bool)
	hash := r.hashfunc([]byte(key))
	iter := NewIteratorAt(r.servers.tree, int(hash))
	for node := iter.Next(); len(unique) < n; node = iter.Next() {
		if node == nil {
			// Reached end of rb-tree, loop around
			iter = NewIterator(r.servers.tree)
			node = iter.Next()
		}
		unique[node.Str()] = true
	}

	// Collect results
	var servers []string
	for server := range unique {
		servers = append(servers, server)
	}

	r.servers.RUnlock()

	return servers
}
