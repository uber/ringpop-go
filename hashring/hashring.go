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

// Package hashring provides a hashring implementation that uses a red-black
// Tree.
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

	serverSet map[string]struct{}
	tree      *redBlackTree
	checksum  uint32

	listeners []events.EventListener
}

func (r *HashRing) emit(event interface{}) {
	for _, listener := range r.listeners {
		listener.HandleEvent(event)
	}
}

// RegisterListener adds a listener that will listen for hashring events.
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

	r.serverSet = make(map[string]struct{})
	r.tree = &redBlackTree{}
	return r
}

// Checksum returns the checksum of all stored servers in the HashRing
// Use this value to find out if the HashRing is mutated.
func (r *HashRing) Checksum() uint32 {
	r.RLock()
	checksum := r.checksum
	r.RUnlock()
	return checksum
}

// computeChecksum computes checksum of all servers in the ring.
// This function isn't thread-safe, only call it when the HashRing is locked.
func (r *HashRing) computeChecksumNoLock() {
	addresses := r.copyServersNoLock()
	sort.Strings(addresses)
	bytes := []byte(strings.Join(addresses, ";"))
	old := r.checksum
	r.checksum = farm.Fingerprint32(bytes)

	r.emit(events.RingChecksumEvent{
		OldChecksum: old,
		NewChecksum: r.checksum,
	})
}

// AddServer adds a server and its replicas onto the HashRing.
func (r *HashRing) AddServer(address string) bool {
	r.Lock()
	ok := r.addServerNoLock(address)
	if ok {
		r.computeChecksumNoLock()
		r.emit(events.RingChangedEvent{
			ServersAdded:   []string{address},
			ServersRemoved: nil,
		})
	}
	r.Unlock()
	return ok
}

// This function isn't thread-safe, only call it when the HashRing is locked.
func (r *HashRing) addServerNoLock(address string) bool {
	if _, ok := r.serverSet[address]; ok {
		return false
	}

	r.addReplicasNoLock(address)
	return true
}

// This function isn't thread-safe, only call it when the HashRing is locked.
func (r *HashRing) addReplicasNoLock(server string) {
	r.serverSet[server] = struct{}{}
	for i := 0; i < r.replicaPoints; i++ {
		address := fmt.Sprintf("%s%v", server, i)
		r.tree.Insert(r.hashfunc(address), server)
	}
}

// RemoveServer removes a server and its replicas from the HashRing.
func (r *HashRing) RemoveServer(address string) bool {
	r.Lock()
	ok := r.removeServerNoLock(address)
	if ok {
		r.computeChecksumNoLock()
		r.emit(events.RingChangedEvent{
			ServersAdded:   nil,
			ServersRemoved: []string{address},
		})
	}
	r.Unlock()
	return ok
}

// This function isn't thread-safe, only call it when the HashRing is locked.
func (r *HashRing) removeServerNoLock(address string) bool {
	if _, ok := r.serverSet[address]; !ok {
		return false
	}

	r.removeReplicasNoLock(address)
	return true
}

// This function isn't thread-safe, only call it when the HashRing is locked.
func (r *HashRing) removeReplicasNoLock(server string) {
	delete(r.serverSet, server)
	for i := 0; i < r.replicaPoints; i++ {
		address := fmt.Sprintf("%s%v", server, i)
		r.tree.Delete(r.hashfunc(address))
	}
}

// AddRemoveServers adds and removes servers and all replicas associated to those
// servers to and from the HashRing. Returns whether the HashRing has changed.
func (r *HashRing) AddRemoveServers(add []string, remove []string) bool {
	r.Lock()
	result := r.addRemoveServersNoLock(add, remove)
	r.Unlock()
	return result
}

// This function isn't thread-safe, only call it when the HashRing is locked.
func (r *HashRing) addRemoveServersNoLock(add []string, remove []string) bool {
	changed := false

	for _, server := range add {
		if r.addServerNoLock(server) {
			changed = true
		}
	}

	for _, server := range remove {
		if r.removeServerNoLock(server) {
			changed = true
		}
	}

	if changed {
		r.computeChecksumNoLock()
		r.emit(events.RingChangedEvent{
			ServersAdded:   add,
			ServersRemoved: remove,
		})
	}
	return changed
}

// HasServer returns whether the HashRing contains the given server.
func (r *HashRing) HasServer(server string) bool {
	r.RLock()
	_, ok := r.serverSet[server]
	r.RUnlock()
	return ok
}

// Servers returns all servers contained in the HashRing.
func (r *HashRing) Servers() []string {
	r.RLock()
	servers := r.copyServersNoLock()
	r.RUnlock()
	return servers
}

// This function isn't thread-safe, only call it when the HashRing is locked.
func (r *HashRing) copyServersNoLock() []string {
	var servers []string
	for server := range r.serverSet {
		servers = append(servers, server)
	}
	return servers
}

// ServerCount returns the number of servers contained in the HashRing.
func (r *HashRing) ServerCount() int {
	r.RLock()
	count := len(r.serverSet)
	r.RUnlock()
	return count
}

// Lookup returns the owner of the given key and whether the HashRing contains
// the key at all.
func (r *HashRing) Lookup(key string) (string, bool) {
	strs := r.LookupN(key, 1)
	if len(strs) == 0 {
		return "", false
	}
	return strs[0], true
}

// LookupN returns the N servers that own the given key. Duplicates in the form
// of virtual nodes are skipped to maintain a list of unique servers. If there
// are less servers than N, we simply return all existing servers.
func (r *HashRing) LookupN(key string, n int) []string {
	r.RLock()
	servers := r.lookupNNoLock(key, n)
	r.RUnlock()
	return servers
}

// This function isn't thread-safe, only call it when the HashRing is locked.
func (r *HashRing) lookupNNoLock(key string, n int) []string {
	if n >= len(r.serverSet) {
		return r.copyServersNoLock()
	}

	hash := r.hashfunc(key)
	unique := make(map[string]struct{})

	// lookup N unique servers from the red-black tree. If we have not
	// collected all the servers we want, we have reached the
	// end of the red-black tree and we need to loop around and inspect the
	// tree starting at 0.
	r.tree.LookupNUniqueAt(n, hash, unique)
	if len(unique) < n {
		r.tree.LookupNUniqueAt(n, 0, unique)
	}

	var servers []string
	for server := range unique {
		servers = append(servers, server)
	}
	return servers
}
