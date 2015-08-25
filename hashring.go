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

package ringpop

import (
	"bytes"
	"fmt"
	"sort"
	"sync"

	"github.com/uber/ringpop-go/rbtree"

	"github.com/dgryski/go-farm"
)

type hashRing struct {
	ringpop       *Ringpop
	tree          rbtree.RBTree
	servers       map[string]bool
	hashfunc      func([]byte) uint32
	checksum      uint32
	replicaPoints int
	l             sync.RWMutex
}

func newHashRing(ringpop *Ringpop, hashfunc func([]byte) uint32) *hashRing {
	return &hashRing{
		ringpop:       ringpop,
		tree:          rbtree.RBTree{},
		hashfunc:      hashfunc,
		servers:       make(map[string]bool),
		replicaPoints: 3,
	}
}

func (r *hashRing) Checksum() uint32 {
	r.l.RLock()
	defer r.l.RUnlock()

	return r.checksum
}

// computeChecksum computes checksum of all servers in the ring
func (r *hashRing) ComputeChecksum() {
	var addresses sort.StringSlice
	var buffer bytes.Buffer

	r.l.Lock()
	defer r.l.Unlock()

	for address := range r.servers {
		addresses = append(addresses, address)
	}

	addresses.Sort()

	for _, address := range addresses {
		buffer.WriteString(address)
		buffer.WriteString(";")
	}

	old := r.checksum
	r.checksum = farm.Hash32(buffer.Bytes())
	r.ringpop.ringEvent(RingChecksumEvent{
		OldChecksum: old,
		NewChecksum: r.checksum,
	})
}

func (r *hashRing) AddServer(address string) {
	if r.HasServer(address) {
		return
	}

	r.AddReplicas(address)
	r.ringpop.ringEvent(RingChangedEvent{ServersAdded: []string{address}})
	r.ComputeChecksum()
}

// inserts server replicas into ring
func (r *hashRing) AddReplicas(server string) {
	r.l.Lock()
	defer r.l.Unlock()

	r.servers[server] = true

	for i := 0; i < r.replicaPoints; i++ {
		address := fmt.Sprintf("%s%v", server, i)
		r.tree.Insert(int(r.hashfunc([]byte(address))), server)
	}
}

func (r *hashRing) RemoveServer(address string) {
	if !r.HasServer(address) {
		return
	}

	r.RemoveReplicas(address)
	r.ringpop.ringEvent(RingChangedEvent{ServersRemoved: []string{address}})
	r.ComputeChecksum()
}

func (r *hashRing) RemoveReplicas(server string) {
	r.l.Lock()
	defer r.l.Unlock()

	delete(r.servers, server)

	for i := 0; i < r.replicaPoints; i++ {
		address := fmt.Sprintf("%s%v", server, i)
		r.tree.Delete(int(r.hashfunc([]byte(address))))
	}
}

// adds and removes servers in a batch with a single checksum computation at the end
func (r *hashRing) AddRemoveServers(add []string, remove []string) bool {
	var changed, added, removed bool

	for _, server := range add {
		if !r.HasServer(server) {
			r.AddReplicas(server)
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
		r.ringpop.ringEvent(RingChangedEvent{add, remove})
		r.ComputeChecksum()
	}

	return changed
}

// hasServer returns true if the server exists in the ring, false otherwise
func (r *hashRing) HasServer(address string) bool {
	r.l.RLock()
	defer r.l.RUnlock()

	return r.servers[address]
}

// ServerCount returns the number of servers in the ring
func (r *hashRing) ServerCount() int {
	r.l.RLock()
	defer r.l.RUnlock()

	return len(r.servers)
}

func (r *hashRing) Lookup(key string) (string, bool) {
	r.l.RLock()
	defer r.l.RUnlock()

	iter := r.tree.IterAt(int(r.hashfunc([]byte(key))))
	if iter.Nil() {
		return "", false
	}
	return iter.Str(), true
}

func (r *hashRing) LookupN(key string, n int) []string {
	r.l.RLock()
	defer r.l.RUnlock()

	serverCount := r.ServerCount()
	if n > serverCount {
		n = serverCount
	}

	var servers []string
	var unique = make(map[string]bool)

	iter := r.tree.IterAt(int(r.hashfunc([]byte(key))))
	if iter.Nil() {
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

	return servers
}
