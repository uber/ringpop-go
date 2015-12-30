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

// Package hashring provides a hashring implementation using a Red Black Tree.
package hashring

import (
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/uber/ringpop-go/events"

	"github.com/dgryski/go-farm"
)

// Configuration is a configuration struct that can be passed to the
// Ringpop constructor to customize hash ring options.
type Configuration struct {
	// ReplicaPoints is the number of positions a node will be assigned on the
	// ring. A bigger number will provide better key distribution, but require
	// more computation when building or traversing the ring (typically on
	// lookups or membership changes).
	ReplicaPoints int
}

// HashRing stores strings on a consistent hash ring. HashRing internally uses
// a Red-Black Tree to achieve O(log N) lookup and insertion time
type HashRing struct {
	hashfunc      func(string) int
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

// New instantiates and returns a new HashRing,
func New(hashfunc func([]byte) uint32, replicaPoints int) *HashRing {
	ring := &HashRing{
		replicaPoints: replicaPoints,
		hashfunc: func(str string) int {
			return int(hashfunc([]byte(str)))
		},
	}

	ring.servers.byAddress = make(map[string]bool)
	ring.servers.tree = &RedBlackTree{}
	return ring
}

// Checksum returns the checksum of all stored servers in the HashRing
// Use this value to find out if the HashRing is mutated.
func (r *HashRing) Checksum() uint32 {
	r.servers.RLock()
	defer r.servers.RUnlock()
	return r.servers.checksum
}

// computeChecksum computes checksum of all servers in the ring.
func (r *HashRing) computeChecksum() {
	addresses := r.GetServers()
	sort.Strings(addresses)
	bytes := []byte(strings.Join(addresses, ";"))

	r.servers.Lock()
	defer r.servers.Unlock()
	old := r.servers.checksum
	r.servers.checksum = farm.Fingerprint32(bytes)

	r.emit(events.RingChecksumEvent{
		OldChecksum: old,
		NewChecksum: r.servers.checksum,
	})
}

// AddServer adds all replicas of a server to the HashRing
func (r *HashRing) AddServer(address string) {
	if r.HasServer(address) {
		return
	}

	r.addReplicas(address)
	r.emit(events.RingChangedEvent{ServersAdded: []string{address}})
	r.computeChecksum()
}

func (r *HashRing) addReplicas(server string) {
	r.servers.Lock()
	defer r.servers.Unlock()

	r.servers.byAddress[server] = true
	for i := 0; i < r.replicaPoints; i++ {
		address := fmt.Sprintf("%s%v", server, i)
		r.servers.tree.Insert(r.hashfunc(address), server)
	}
}

// RemoveServer removes all replicas of a server from the HashRing
func (r *HashRing) RemoveServer(address string) {
	if !r.HasServer(address) {
		return
	}

	r.removeReplicas(address)
	r.emit(events.RingChangedEvent{ServersRemoved: []string{address}})
	r.computeChecksum()
}

func (r *HashRing) removeReplicas(server string) {
	r.servers.Lock()
	defer r.servers.Unlock()

	delete(r.servers.byAddress, server)
	for i := 0; i < r.replicaPoints; i++ {
		address := fmt.Sprintf("%s%v", server, i)
		r.servers.tree.Delete(r.hashfunc(address))
	}
}

// AddRemoveServers adds and removes all replicas of the given servers to the
// HashRing. This function only computes the new checksum once, and thus is an
// optimalization of AddServer and RemoveServer.
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
			r.removeReplicas(server)
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

// HasServer returns true if the server exists in the ring, false otherwise
func (r *HashRing) HasServer(address string) bool {
	r.servers.RLock()
	defer r.servers.RUnlock()
	return r.servers.byAddress[address]
}

// GetServers returns all servers stored in the HashRing
func (r *HashRing) GetServers() []string {
	r.servers.RLock()
	defer r.servers.RUnlock()

	var servers []string
	for server := range r.servers.byAddress {
		servers = append(servers, server)
	}
	return servers
}

// ServerCount returns the number of servers in the ring
func (r *HashRing) ServerCount() int {
	r.servers.RLock()
	defer r.servers.RUnlock()
	return len(r.servers.byAddress)
}

// Lookup return the owner of the given key
func (r *HashRing) Lookup(key string) (string, bool) {
	strs := r.LookupN(key, 1)
	if len(strs) == 0 {
		return "", false
	}
	return strs[0], true
}

// LookupN returns the N servers that own the given key. Duplicates in the form
// of virtual nodes are skipped to maintain a list of unique servers. If there
// are less servers then N, we simply return all existing servers.
func (r *HashRing) LookupN(key string, n int) []string {
	r.servers.RLock()
	defer r.servers.RUnlock()

	if n >= r.ServerCount() {
		return r.GetServers()
	}

	// Iterate over RB-tree and collect unique servers
	var unique = make(map[string]bool)
	hash := r.hashfunc(key)
	iter := NewIteratorAt(r.servers.tree, hash)
	for node := iter.Next(); len(unique) < n; node = iter.Next() {
		if node == nil {
			// reached end of rb-tree, loop around
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
	return servers
}
