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
	"sync"

	"github.com/uber-common/bark"
	"github.com/uber/ringpop-go/events"
	"github.com/uber/ringpop-go/logging"
	"github.com/uber/ringpop-go/membership"
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

type replicaPoint struct {
	point    int
	replica  int
	identity string
}

func (r replicaPoint) Compare(other interface{}) int {
	return r.point - other.(replicaPoint).point
}

// HashRing stores strings on a consistent hash ring. HashRing internally uses
// a Red-Black Tree to achieve O(log N) lookup and insertion time.
type HashRing struct {
	sync.RWMutex
	events.SyncEventEmitter

	hashfunc      func(string) int
	replicaPoints int

	serverSet map[string]struct{}
	tree      *redBlackTree

	// checksummers is map of named Checksum calculators for the hashring
	checksummers map[string]Checksum
	// checksums is a map containing the checksums that are representing this
	// hashring. The map should never be altered in place so it is safe to pass
	// a copy to components that need the checksums
	checksums map[string]uint32
	// defaultChecksum is the name of the Checksum calculator that is used as
	// the default checksum for the hashring. The default checksum is returned
	// when asked for only 1 checksum and is present for legacy reasons
	defaultChecksum string

	logger bark.Logger
}

// New instantiates and returns a new HashRing.
func New(hashfunc func([]byte) uint32, replicaPoints int) *HashRing {
	r := &HashRing{
		replicaPoints: replicaPoints,
		hashfunc: func(str string) int {
			return int(hashfunc([]byte(str)))
		},
		logger:       logging.Logger("ring"),
		checksummers: make(map[string]Checksum),
	}

	r.checksummers["address"] = &addressChecksum{}
	r.defaultChecksum = "address"

	r.serverSet = make(map[string]struct{})
	r.tree = &redBlackTree{}
	return r
}

// Checksum returns the checksum of all stored servers in the HashRing
// Use this value to find out if the HashRing is mutated.
func (r *HashRing) Checksum() uint32 {
	r.RLock()
	checksum := r.checksums[r.defaultChecksum]
	r.RUnlock()
	return checksum
}

func (r *HashRing) Checksums() (checksums map[string]uint32) {
	r.RLock()
	// even though the map is immutable the pointer to it is not so it requires
	// a readlock
	checksums = r.checksums
	r.RUnlock()
	return
}

// computeChecksumsNoLock re-computes all configured checksums for this hashring
// and updates the in memory map with a new map containing the new checksums.
func (r *HashRing) computeChecksumsNoLock() {
	oldChecksums := r.checksums
	r.checksums = make(map[string]uint32)

	// calculate all configured checksums
	for name, checksummer := range r.checksummers {
		r.checksums[name] = checksummer.Compute(r)
	}

	if r.checksums[r.defaultChecksum] != oldChecksums[r.defaultChecksum] {
		r.logger.WithFields(bark.Fields{
			"checksum":    r.checksums[r.defaultChecksum],
			"oldChecksum": oldChecksums[r.defaultChecksum],
		}).Debug("ringpop ring computed new checksum")
	}

	r.EmitEvent(events.RingChecksumEvent{
		OldChecksum: oldChecksums[r.defaultChecksum],
		NewChecksum: r.checksums[r.defaultChecksum],
	})
}

func (r *HashRing) replicaPointForServer(server membership.Member, replica int) replicaPoint {
	identity := fmt.Sprintf("%s%v", server.Identity(), replica)
	return replicaPoint{
		point:    r.hashfunc(identity),
		replica:  replica,
		identity: server.Identity(),
	}
}

// AddServer adds a server and its replicas onto the HashRing.
func (r *HashRing) AddServer(server membership.Member) bool {
	r.Lock()
	ok := r.addServerNoLock(server)
	if ok {
		r.computeChecksumsNoLock()
		r.EmitEvent(events.RingChangedEvent{
			ServersAdded:   []string{server.GetAddress()},
			ServersRemoved: nil,
		})
	}
	r.Unlock()
	return ok
}

// This function isn't thread-safe, only call it when the HashRing is locked.
func (r *HashRing) addServerNoLock(server membership.Member) bool {
	if _, ok := r.serverSet[server.GetAddress()]; ok {
		return false
	}

	r.addReplicasNoLock(server)
	return true
}

// This function isn't thread-safe, only call it when the HashRing is locked.
func (r *HashRing) addReplicasNoLock(server membership.Member) {
	r.serverSet[server.GetAddress()] = struct{}{}
	for i := 0; i < r.replicaPoints; i++ {
		r.tree.Insert(
			r.replicaPointForServer(server, i),
			server.GetAddress(),
		)
	}
}

// RemoveServer removes a server and its replicas from the HashRing.
func (r *HashRing) RemoveServer(server membership.Member) bool {
	r.Lock()
	ok := r.removeServerNoLock(server)
	if ok {
		r.computeChecksumsNoLock()
		r.EmitEvent(events.RingChangedEvent{
			ServersAdded:   nil,
			ServersRemoved: []string{server.GetAddress()},
		})
	}
	r.Unlock()
	return ok
}

// This function isn't thread-safe, only call it when the HashRing is locked.
func (r *HashRing) removeServerNoLock(server membership.Member) bool {
	if _, ok := r.serverSet[server.GetAddress()]; !ok {
		return false
	}

	r.removeReplicasNoLock(server)
	return true
}

// This function isn't thread-safe, only call it when the HashRing is locked.
func (r *HashRing) removeReplicasNoLock(server membership.Member) {
	delete(r.serverSet, server.GetAddress())
	for i := 0; i < r.replicaPoints; i++ {
		r.tree.Delete(
			r.replicaPointForServer(server, i),
		)
	}
}

func (r *HashRing) ProcessMembershipChangesServers(changes []membership.MemberChange) {
	r.Lock()
	changed := false
	for _, change := range changes {
		if change.Before == nil && change.After != nil {
			// new member
			r.addServerNoLock(change.After)
			changed = true
		} else if change.Before != nil && change.After == nil {
			// remove member
			r.removeServerNoLock(change.Before)
			changed = true
		} else {
			if change.Before.Identity() != change.After.Identity() {
				// identity has changed, member needs to be removed and readded
				r.removeServerNoLock(change.Before)
				r.addServerNoLock(change.After)
				changed = true
			}
		}
	}

	// recompute checksums on changes
	if changed {
		r.computeChecksumsNoLock()
	}

	r.Unlock()
}

// AddRemoveServers adds and removes servers and all replicas associated to those
// servers to and from the HashRing. Returns whether the HashRing has changed.
func (r *HashRing) AddRemoveServers(add []membership.Member, remove []membership.Member) bool {
	r.Lock()
	result := r.addRemoveServersNoLock(add, remove)
	r.Unlock()
	return result
}

// This function isn't thread-safe, only call it when the HashRing is locked.
func (r *HashRing) addRemoveServersNoLock(add []membership.Member, remove []membership.Member) bool {
	changed := false

	var added, removed []string
	for _, server := range add {
		if r.addServerNoLock(server) {
			added = append(added, server.GetAddress())
			changed = true
		}
	}

	for _, server := range remove {
		if r.removeServerNoLock(server) {
			removed = append(removed, server.GetAddress())
			changed = true
		}
	}

	if changed {
		r.computeChecksumsNoLock()
		r.EmitEvent(events.RingChangedEvent{
			ServersAdded:   added,
			ServersRemoved: removed,
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
	servers := make([]string, 0, len(r.serverSet))
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
	unique := make(map[valuetype]struct{})

	// lookup N unique servers from the red-black tree. If we have not
	// collected all the servers we want, we have reached the
	// end of the red-black tree and we need to loop around and inspect the
	// tree starting at 0.
	r.tree.LookupNUniqueAt(n, replicaPoint{point: hash}, unique)
	if len(unique) < n {
		r.tree.LookupNUniqueAt(n, replicaPoint{point: 0}, unique)
	}

	var servers []string
	for server := range unique {
		servers = append(servers, server.(string))
	}
	return servers
}
