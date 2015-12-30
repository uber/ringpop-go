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
// a Red-Black Tree to achieve O(log N) lookup and insertion time.
type HashRing struct {
	sync.RWMutex

	hashfunc      func(string) int
	replicaPoints int

	servers  map[string]struct{}
	tree     *RedBlackTree
	checksum uint32

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

// New instantiates and returns a new HashRing.
func New(hashfunc func([]byte) uint32, replicaPoints int) *HashRing {
	r := &HashRing{
		replicaPoints: replicaPoints,
		hashfunc: func(str string) int {
			return int(hashfunc([]byte(str)))
		},
	}

	r.servers = make(map[string]struct{})
	r.tree = &RedBlackTree{}
	return r
}

// LookupN returns the N servers that own the given key. Duplicates in the form
// of virtual nodes are skipped to maintain a list of unique servers. If there
// are less servers then N, we simply return all existing servers.
func (r *HashRing) LookupN(key string, n int) []string {
	r.RLock()
	defer r.RUnlock()

	if n >= r.ServerCount() {
		return r.serversNoLock()
	}

	// Iterate over RB-tree and collect unique servers
	var unique = make(map[string]bool)
	hash := r.hashfunc(key)
	iter := NewIteratorAt(r.tree, hash)
	for node := iter.Next(); len(unique) < n; node = iter.Next() {
		if node == nil {
			// reached end of rb-tree, loop around
			iter = NewIterator(r.tree)
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

// AddRemoveServers adds and removes all replicas of the given servers to the
// HashRing. Returns wether or not the checksum has changed.
func (r *HashRing) AddRemoveServers(add []string, remove []string) bool {
	r.Lock()
	defer r.Unlock()

	changed := false

	for _, server := range add {
		if r.addServer(server) {
			changed = true
		}
	}

	for _, server := range remove {
		if r.removeServer(server) {
			changed = true
		}
	}

	if changed {
		r.emit(events.RingChangedEvent{add, remove})
		r.computeChecksum()
	}

	return changed
}

// addServer adds all replicas of a server to the HashRing.
func (r *HashRing) addServer(address string) bool {
	if _, ok := r.servers[address]; ok {
		return false
	}

	r.addReplicas(address)
	r.emit(events.RingChangedEvent{ServersAdded: []string{address}})
	return true
}

func (r *HashRing) addReplicas(server string) {
	r.servers[server] = struct{}{}
	for i := 0; i < r.replicaPoints; i++ {
		address := fmt.Sprintf("%s%v", server, i)
		r.tree.Insert(r.hashfunc(address), server)
	}
}

// removeServer removes all replicas of a server from the HashRing.
func (r *HashRing) removeServer(address string) bool {
	if _, ok := r.servers[address]; !ok {
		return false
	}

	r.removeReplicas(address)
	r.emit(events.RingChangedEvent{ServersRemoved: []string{address}})
	return true
}

func (r *HashRing) removeReplicas(server string) {
	delete(r.servers, server)
	for i := 0; i < r.replicaPoints; i++ {
		address := fmt.Sprintf("%s%v", server, i)
		r.tree.Delete(r.hashfunc(address))
	}
}

// Checksum returns the checksum of all stored servers in the HashRing
// Use this value to find out if the HashRing is mutated.
func (r *HashRing) Checksum() uint32 {
	r.RLock()
	defer r.RUnlock()
	return r.checksum
}

// computeChecksum computes checksum of all servers in the ring.
func (r *HashRing) computeChecksum() {
	addresses := r.serversNoLock()
	sort.Strings(addresses)
	bytes := []byte(strings.Join(addresses, ";"))
	old := r.checksum
	r.checksum = farm.Fingerprint32(bytes)

	r.emit(events.RingChecksumEvent{
		OldChecksum: old,
		NewChecksum: r.checksum,
	})
}

// Lookup returns the owner of the given key.
func (r *HashRing) Lookup(key string) (string, bool) {
	strs := r.LookupN(key, 1)
	if len(strs) == 0 {
		return "", false
	}
	return strs[0], true
}

// HasServer returns wether or not the server exists in the ring.
func (r *HashRing) HasServer(address string) bool {
	r.RLock()
	defer r.RUnlock()
	_, ok := r.servers[address]
	return ok
}

// Servers returns all servers stored in the HashRing.
func (r *HashRing) Servers() []string {
	r.RLock()
	defer r.RUnlock()
	return r.serversNoLock()
}

func (r *HashRing) serversNoLock() []string {
	var servers []string
	for server := range r.servers {
		servers = append(servers, server)
	}
	return servers
}

// ServerCount returns the number of servers in the ring.
func (r *HashRing) ServerCount() int {
	r.RLock()
	defer r.RUnlock()
	return len(r.hasServer)
}
