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
	"strings"
	"sync"

	"github.com/uber/ringpop-go/events"
	"github.com/uber/ringpop-go/logging"
	"github.com/uber/ringpop-go/membership"

	"github.com/uber-common/bark"
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

// replicaPoint contains the address where a specific point in the hashring maps to
type replicaPoint struct {
	// hash of the point in the hashring
	hash int

	// identity of the member that owns this replicaPoint.
	identity string

	// address of the member that owns this replicaPoint.
	address string

	// index of the replica point for a member
	index int
}

func (r replicaPoint) Compare(other interface{}) (result int) {
	o := other.(replicaPoint)

	result = r.hash - o.hash
	if result != 0 {
		return
	}

	result = strings.Compare(r.address, o.address)
	if result != 0 {
		return
	}

	result = r.index - o.index
	return
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

	// legacyChecksum is the old checksum that is calculated only from the
	// identities being added.
	legacyChecksum uint32

	// checksummers is map of named Checksum calculators for the hashring
	checksummers map[string]Checksummer
	// checksums is a map containing the checksums that are representing this
	// hashring. The map should never be altered in place so it is safe to pass
	// a copy to components that need the checksums
	checksums map[string]uint32

	logger bark.Logger
}

// New instantiates and returns a new HashRing.
func New(hashfunc func([]byte) uint32, replicaPoints int) *HashRing {
	r := &HashRing{
		replicaPoints: replicaPoints,
		hashfunc: func(str string) int {
			return int(hashfunc([]byte(str)))
		},
		logger: logging.Logger("ring"),

		checksummers: map[string]Checksummer{
			"replica": &replicaPointChecksummer{},
		},
	}

	r.serverSet = make(map[string]struct{})
	r.tree = &redBlackTree{}
	return r
}

// Checksum returns the checksum of all stored servers in the HashRing
// Use this value to find out if the HashRing is mutated.
func (r *HashRing) Checksum() (checksum uint32) {
	r.RLock()
	checksum = r.legacyChecksum
	r.RUnlock()
	return
}

// Checksums returns a map of checksums named by the algorithm used to compute
// the checksum.
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
	changed := false
	// calculate all configured checksums
	for name, checksummer := range r.checksummers {
		oldChecksum := oldChecksums[name]
		newChecksum := checksummer.Checksum(r)
		r.checksums[name] = newChecksum

		if oldChecksum != newChecksum {
			changed = true
		}
	}

	// calculate the legacy identity only based checksum
	legacyChecksummer := identityChecksummer{}
	oldChecksum := r.legacyChecksum
	newChecksum := legacyChecksummer.Checksum(r)
	r.legacyChecksum = newChecksum

	if oldChecksum != newChecksum {
		changed = true
	}

	if changed {
		r.logger.WithFields(bark.Fields{
			"checksum":    r.legacyChecksum,
			"oldChecksum": oldChecksum,
			"checksums":   r.checksums,
		}).Debug("ringpop ring computed new checksum")
	}

	r.EmitEvent(events.RingChecksumEvent{
		OldChecksum:  oldChecksum,
		NewChecksum:  r.legacyChecksum,
		OldChecksums: oldChecksums,
		NewChecksums: r.checksums,
	})
}

func (r *HashRing) replicaPointForServer(server membership.Member, replica int) replicaPoint {
	identity := server.Identity()
	var replicaStr string
	if identity == server.GetAddress() {
		// If identity and address are the same, we need to be backwards compatible
		// this older replicaStr format will cause replica point collisions when there are
		// multiple instances running on the same host (e.g. on port 2100 and 21001).
		replicaStr = fmt.Sprintf("%s%v", identity, replica)
	} else {
		// This is the "new and improved" version.
		// Due to backwards compatibility it's only used when we got an identity.
		replicaStr = fmt.Sprintf("%s#%v", identity, replica)
	}
	return replicaPoint{
		hash:     r.hashfunc(replicaStr),
		identity: identity,
		address:  server.GetAddress(),
		index:    replica,
	}
}

// AddMembers adds multiple membership Member's and thus their replicas to the HashRing.
func (r *HashRing) AddMembers(members ...membership.Member) bool {
	r.Lock()
	changed := false
	var added []string
	for _, member := range members {
		if r.addMemberNoLock(member) {
			added = append(added, member.GetAddress())
			changed = true
		}
	}

	if changed {
		r.computeChecksumsNoLock()
		r.EmitEvent(events.RingChangedEvent{
			ServersAdded: added,
		})
	}
	r.Unlock()
	return changed
}

// This function isn't thread-safe, only call it when the HashRing is locked.
func (r *HashRing) addMemberNoLock(member membership.Member) bool {
	if _, ok := r.serverSet[member.GetAddress()]; ok {
		return false
	}
	r.serverSet[member.GetAddress()] = struct{}{}

	// add all replica points for the member
	for i := 0; i < r.replicaPoints; i++ {
		r.tree.Insert(
			r.replicaPointForServer(member, i),
			member.GetAddress(),
		)
	}

	return true
}

// RemoveMembers removes multiple membership Member's and thus their replicas from the HashRing.
func (r *HashRing) RemoveMembers(members ...membership.Member) bool {
	r.Lock()
	changed := false
	var removed []string
	for _, server := range members {
		if r.removeMemberNoLock(server) {
			removed = append(removed, server.GetAddress())
			changed = true
		}
	}

	if changed {
		r.computeChecksumsNoLock()
		r.EmitEvent(events.RingChangedEvent{
			ServersRemoved: removed,
		})
	}
	r.Unlock()
	return changed
}

// This function isn't thread-safe, only call it when the HashRing is locked.
func (r *HashRing) removeMemberNoLock(member membership.Member) bool {
	if _, ok := r.serverSet[member.GetAddress()]; !ok {
		return false
	}
	delete(r.serverSet, member.GetAddress())

	for i := 0; i < r.replicaPoints; i++ {
		r.tree.Delete(
			r.replicaPointForServer(member, i),
		)
	}

	return true
}

// ProcessMembershipChanges takes a slice of membership.MemberChange's and
// applies them to the hashring by adding and removing members accordingly to
// the changes passed in.
func (r *HashRing) ProcessMembershipChanges(changes []membership.MemberChange) {
	r.Lock()
	changed := false
	var added, updated, removed []string
	for _, change := range changes {
		if change.Before == nil && change.After != nil {
			// new member
			if r.addMemberNoLock(change.After) {
				added = append(added, change.After.GetAddress())
				changed = true
			}
		} else if change.Before != nil && change.After == nil {
			// remove member
			if r.removeMemberNoLock(change.Before) {
				removed = append(removed, change.Before.GetAddress())
				changed = true
			}
		} else {
			if change.Before.Identity() != change.After.Identity() {
				// identity has changed, member needs to be removed and readded
				r.removeMemberNoLock(change.Before)
				r.addMemberNoLock(change.After)
				updated = append(updated, change.After.GetAddress())
				changed = true
			}
		}
	}

	// recompute checksums on changes
	if changed {
		r.computeChecksumsNoLock()
		r.EmitEvent(events.RingChangedEvent{
			ServersAdded:   added,
			ServersUpdated: updated,
			ServersRemoved: removed,
		})
	}

	r.Unlock()
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
	hash := r.hashfunc(key)
	unique := make(map[valuetype]struct{})
	orderedUnique := make([]valuetype, 0, n)

	// lookup N unique servers from the red-black tree. If we have not
	// collected all the servers we want, we have reached the
	// end of the red-black tree and we need to loop around and inspect the
	// tree starting at 0.
	r.tree.LookupOrderedNUniqueAt(n, replicaPoint{hash: hash}, unique, &orderedUnique)
	if len(unique) < n {
		r.tree.LookupOrderedNUniqueAt(n, replicaPoint{hash: 0}, unique, &orderedUnique)
	}

	var servers []string
	for _, server := range orderedUnique {
		servers = append(servers, server.(string))
	}
	return servers
}
