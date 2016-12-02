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
	"github.com/uber/ringpop-go/logging"
	"github.com/uber/ringpop-go/membership"

	"github.com/dgryski/go-farm"
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
	checksum  uint32

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

	if r.checksum != old {
		r.logger.WithFields(bark.Fields{
			"checksum":    r.checksum,
			"oldChecksum": old,
		}).Debug("ringpop ring computed new checksum")
	}

	r.EmitEvent(events.RingChecksumEvent{
		OldChecksum: old,
		NewChecksum: r.checksum,
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
		r.computeChecksumNoLock()
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
		r.computeChecksumNoLock()
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
	var added, removed []string
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
				r.removeServerNoLock(change.Before)
				r.addServerNoLock(change.After)
				changed = true
			}
		}
	}

	// recompute checksums on changes
	if changed {
		r.computeChecksumNoLock()
		r.EmitEvent(events.RingChangedEvent{
			ServersAdded:   added,
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
