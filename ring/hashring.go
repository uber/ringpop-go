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

package ring

import (
	"bytes"
	"fmt"
	"sort"
	"sync"

	"github.com/dgryski/go-farm"
    "github.com/uber/ringpop-go/shared"
)

type HashRingConfiguration struct {
	ReplicaPoints int
}

type hashRing struct {
	hashfunc      func([]byte) uint32
	replicaPoints int

	servers struct {
		byAddress map[string]bool
		tree      *RBTree
		checksum  uint32
		sync.RWMutex
	}

    listeners []shared.EventListener
}

func New(hashfunc func([]byte) uint32, replicaPoints int) *hashRing {
	ring := &hashRing{
		hashfunc:      hashfunc,
		replicaPoints: replicaPoints,
	}

	ring.servers.byAddress = make(map[string]bool)
	ring.servers.tree = &RBTree{}

	return ring
}

func (r *hashRing) Checksum() uint32 {
	r.servers.RLock()
	checksum := r.servers.checksum
	r.servers.RUnlock()

	return checksum
}

// computeChecksum computes checksum of all servers in the ring
func (r *hashRing) computeChecksum() {
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
	r.emit(RingChecksumEvent{
		OldChecksum: old,
		NewChecksum: r.servers.checksum,
	})

	r.servers.Unlock()
}

func (r *hashRing) AddServer(address string) {
	if r.HasServer(address) {
		return
	}

	r.addReplicas(address)
	r.emit(RingChangedEvent{ServersAdded: []string{address}})
	r.computeChecksum()
}

// inserts server replicas into ring
func (r *hashRing) addReplicas(server string) {
	r.servers.Lock()
	r.servers.byAddress[server] = true

	for i := 0; i < r.replicaPoints; i++ {
		address := fmt.Sprintf("%s%v", server, i)
		r.servers.tree.Insert(int(r.hashfunc([]byte(address))), server)
	}

	r.servers.Unlock()
}

func (r *hashRing) RemoveServer(address string) {
	if !r.HasServer(address) {
		return
	}

	r.removeReplicas(address)
	r.emit(RingChangedEvent{ServersRemoved: []string{address}})
	r.computeChecksum()
}

func (r *hashRing) removeReplicas(server string) {
	r.servers.Lock()

	delete(r.servers.byAddress, server)

	for i := 0; i < r.replicaPoints; i++ {
		address := fmt.Sprintf("%s%v", server, i)
		r.servers.tree.Delete(int(r.hashfunc([]byte(address))))
	}

	r.servers.Unlock()
}

// adds and removes servers in a batch with a single checksum computation at the end
func (r *hashRing) AddRemoveServers(add []string, remove []string) bool {
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
		r.emit(RingChangedEvent{add, remove})
		r.computeChecksum()
	}

	return changed
}

// hasServer returns true if the server exists in the ring, false otherwise
func (r *hashRing) HasServer(address string) bool {
	r.servers.RLock()
	server := r.servers.byAddress[address]
	r.servers.RUnlock()

	return server
}

func (r *hashRing) GetServers() (servers []string) {
	r.servers.RLock()
	for server := range r.servers.byAddress {
		servers = append(servers, server)
	}
	r.servers.RUnlock()

	return servers
}

// ServerCount returns the number of servers in the ring
func (r *hashRing) ServerCount() int {
	r.servers.RLock()
	count := len(r.servers.byAddress)
	r.servers.RUnlock()

	return count
}

func (r *hashRing) Lookup(key string) (string, bool) {
	r.servers.RLock()

	iter := r.servers.tree.IterAt(int(r.hashfunc([]byte(key))))
	if iter.Nil() {
		r.servers.RUnlock()
		return "", false
	}

	server := iter.Str()

	r.servers.RUnlock()

	return server, true
}

func (r *hashRing) LookupN(key string, n int) []string {
	serverCount := r.ServerCount()
	if n > serverCount {
		n = serverCount
	}

	r.servers.RLock()
	var servers []string
	var unique = make(map[string]bool)

	iter := r.servers.tree.IterAt(int(r.hashfunc([]byte(key))))
	if iter.Nil() {
		r.servers.RUnlock()
		return servers
	}

	firstVal := iter.Val()
	for {
		res := iter.Str()
		if !unique[res] {
			servers = append(servers, res)
			unique[res] = true
		}
		iter.Next()

		if len(servers) == n || iter.Val() == firstVal {
			break
		}
	}

	r.servers.RUnlock()

	return servers
}

func (h *hashRing) RegisterListener(listener shared.EventListener) {
    h.listeners = append(h.listeners, listener)
}

func (h *hashRing) emit(event interface{}) {
    for _, listener := range h.listeners {
        listener(event)
    }
}
