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

package swim

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/dgryski/go-farm"
	"github.com/uber-common/bark"
	"github.com/uber/ringpop-go/logging"
	"github.com/uber/ringpop-go/util"
)

// A memberlist contains the membership for a node
type memberlist struct {
	node  *Node
	local *Member

	members struct {
		list      []*Member
		byAddress map[string]*Member
		checksum  uint32
		sync.RWMutex
	}

	logger bark.Logger

	// TODO: rework locking in ringpop-go (see #113). Required for Update().

	// Updates to membership list and hash ring should happen atomically. We
	// could use members lock for that, but that introduces more deadlocks, so
	// making a short-term fix instead by adding another lock. Like said, this
	// is short-term, see github#113.
	sync.Mutex
}

// newMemberlist returns a new member list
func newMemberlist(n *Node) *memberlist {
	m := &memberlist{
		node:   n,
		logger: logging.Logger("membership").WithField("local", n.address),
	}

	m.members.byAddress = make(map[string]*Member)

	return m
}

func (m *memberlist) Checksum() uint32 {
	m.members.Lock()
	checksum := m.members.checksum
	m.members.Unlock()

	return checksum
}

// computes membership checksum
func (m *memberlist) ComputeChecksum() {
	startTime := time.Now()
	m.members.Lock()
	checksum := farm.Fingerprint32([]byte(m.GenChecksumString()))
	oldChecksum := m.members.checksum
	m.members.checksum = checksum
	m.members.Unlock()

	if oldChecksum != checksum {
		m.logger.WithFields(bark.Fields{
			"checksum":    checksum,
			"oldChecksum": oldChecksum,
		}).Debug("ringpop membership computed new checksum")
	}

	m.node.emit(ChecksumComputeEvent{
		Duration:    time.Now().Sub(startTime),
		Checksum:    checksum,
		OldChecksum: oldChecksum,
	})
}

// generates string to use when computing checksum
func (m *memberlist) GenChecksumString() string {
	var strings sort.StringSlice

	for _, member := range m.members.list {
		// Don't include Tombstone nodes in the checksum to avoid
		// bringing them back to life through full syncs
		if member.Status == Tombstone {
			continue
		}
		s := fmt.Sprintf("%s%s%v", member.Address, member.Status, member.Incarnation)
		strings = append(strings, s)
	}

	strings.Sort()

	buffer := bytes.NewBuffer([]byte{})
	for _, str := range strings {
		buffer.WriteString(str)
		buffer.WriteString(";")
	}

	return buffer.String()
}

// returns the member at a specific address
func (m *memberlist) Member(address string) (*Member, bool) {
	m.members.RLock()
	member, ok := m.members.byAddress[address]
	m.members.RUnlock()

	return member, ok
}

// RemoveMember removes the member from the membership list. If the membership has
// changed during this operation a new checksum will be computed.
func (m *memberlist) RemoveMember(address string) bool {
	m.members.Lock()
	member, hasMember := m.members.byAddress[address]
	if hasMember {
		delete(m.members.byAddress, address)
		for i, lMember := range m.members.list {
			if member == lMember {
				// a safe way to remove a pointer from a slice
				m.members.list, m.members.list[len(m.members.list)-1] = append(m.members.list[:i], m.members.list[i+1:]...), nil
				break
			}
		}
	}
	m.members.Unlock()

	if hasMember {
		// if we changed the membership recompute the actual checksum
		m.ComputeChecksum()
	}

	return hasMember
}

func (m *memberlist) MemberAt(i int) *Member {
	m.members.RLock()
	member := m.members.list[i]
	m.members.RUnlock()

	return member
}

func (m *memberlist) NumMembers() int {
	m.members.RLock()
	n := len(m.members.list)
	m.members.RUnlock()

	return n
}

// returns whether or not a member is pingable
func (m *memberlist) Pingable(member Member) bool {
	return member.Address != m.local.Address &&
		(member.Status == Alive || member.Status == Suspect)

}

// returns the number of pingable members in the memberlist
func (m *memberlist) NumPingableMembers() (n int) {
	m.members.Lock()
	for _, member := range m.members.list {
		if m.Pingable(*member) {
			n++
		}
	}
	m.members.Unlock()

	return n
}

// returns n pingable members in the member list
func (m *memberlist) RandomPingableMembers(n int, excluding map[string]bool) []*Member {
	var members []*Member

	m.members.RLock()
	for _, member := range m.members.list {
		if m.Pingable(*member) && !excluding[member.Address] {
			members = append(members, member)
		}
	}
	m.members.RUnlock()

	// shuffle members and take first n
	members = shuffle(members)

	if n > len(members) {
		return members
	}
	return members[:n]
}

// returns an immutable slice of members representing the current state of the membership
func (m *memberlist) GetMembers() (members []Member) {
	m.members.RLock()
	for _, member := range m.members.list {
		members = append(members, *member)
	}
	m.members.RUnlock()

	return
}

// Reincarnate sets the status of the node to Alive and updates the incarnation
// number. It adds the change to the disseminator as well.
func (m *memberlist) Reincarnate() []Change {
	return m.MakeAlive(m.node.address, nowInMillis(m.node.clock))
}

func (m *memberlist) MakeAlive(address string, incarnation int64) []Change {
	m.node.emit(MakeNodeStatusEvent{Alive})
	return m.MakeChange(address, incarnation, Alive)
}

func (m *memberlist) MakeSuspect(address string, incarnation int64) []Change {
	m.node.emit(MakeNodeStatusEvent{Suspect})
	return m.MakeChange(address, incarnation, Suspect)
}

func (m *memberlist) MakeFaulty(address string, incarnation int64) []Change {
	m.node.emit(MakeNodeStatusEvent{Faulty})
	return m.MakeChange(address, incarnation, Faulty)
}

func (m *memberlist) MakeLeave(address string, incarnation int64) []Change {
	m.node.emit(MakeNodeStatusEvent{Leave})
	return m.MakeChange(address, incarnation, Leave)
}

// MakeTombstone declares the node with the provided address in the tombstone state
// on the given incarnation number. If the incarnation number in the local memberlist
// is already higher than the incartation number provided in this function it is
// essentially a no-op. The list of changes that is returned is the actual list of
// changes that have been applied to the memberlist. It can be used to test if the
// tombstone declaration has been executed atleast to the local memberlist.
func (m *memberlist) MakeTombstone(address string, incarnation int64) []Change {
	m.node.emit(MakeNodeStatusEvent{Tombstone})
	return m.MakeChange(address, incarnation, Tombstone)
}

// Evict evicts a member from the memberlist. It prevents the local node to be evicted
// since that is undesired behavior.
func (m *memberlist) Evict(address string) {
	if m.local.Address == address {
		// We should not evict ourselves from the memberlist. This should not be reached, but we will make noise in the logs
		m.logger.Error("ringpop tried to evict the local member from the memberlist, action has been prevented")
		return
	}

	m.RemoveMember(address)
}

// makes a change to the member list
func (m *memberlist) MakeChange(address string, incarnation int64, status string) []Change {
	if m.local == nil {
		m.local = &Member{
			Address:     m.node.Address(),
			Incarnation: util.TimeNowMS(),
			Status:      Alive,
		}
	}

	changes := m.Update([]Change{Change{
		Source:            m.local.Address,
		SourceIncarnation: m.local.Incarnation,
		Address:           address,
		Incarnation:       incarnation,
		Status:            status,
		Timestamp:         util.Timestamp(time.Now()),
	}})

	if len(changes) > 0 {
		m.logger.WithFields(bark.Fields{
			"update": changes[0],
		}).Debugf("ringpop member declares other member %s", changes[0].Status)
	}

	return changes
}

// updates the member list with the slice of changes, applying selectively
func (m *memberlist) Update(changes []Change) (applied []Change) {
	if m.node.Stopped() || len(changes) == 0 {
		return nil
	}

	// validate incoming changes
	for i, change := range changes {
		changes[i] = change.validateIncoming()
	}

	m.node.emit(MemberlistChangesReceivedEvent{changes})

	m.Lock()
	m.members.Lock()

	for _, change := range changes {
		member, ok := m.members.byAddress[change.Address]

		// first time member has been seen, take change wholesale
		if !ok {
			if m.Apply(change) {
				applied = append(applied, change)
			}
			continue
		}

		// if change is local override, reassert member is alive
		if member.localOverride(m.node.Address(), change) {
			m.node.emit(RefuteUpdateEvent{})
			newIncNo := nowInMillis(m.node.clock)
			overrideChange := Change{
				Source:            m.node.Address(),
				SourceIncarnation: newIncNo,
				Address:           change.Address,
				Incarnation:       newIncNo,
				Status:            Alive,
				Timestamp:         util.Timestamp(time.Now()),
			}

			if m.Apply(overrideChange) {
				applied = append(applied, overrideChange)
			}

			continue
		}

		// if non-local override, apply change wholesale
		if member.nonLocalOverride(change) {
			if m.Apply(change) {
				applied = append(applied, change)
			}
		}
	}

	m.members.Unlock()

	if len(applied) > 0 {
		oldChecksum := m.Checksum()
		m.ComputeChecksum()

		for _, change := range applied {
			if change.Source != m.node.address {
				m.logger.WithFields(bark.Fields{
					"remote": change.Source,
				}).Debug("ringpop applied remote update")
			}
		}

		m.node.emit(MemberlistChangesAppliedEvent{
			Changes:     applied,
			OldChecksum: oldChecksum,
			NewChecksum: m.Checksum(),
			NumMembers:  m.NumMembers(),
		})
		m.node.handleChanges(applied)
		m.node.rollup.TrackUpdates(applied)
	}

	m.Unlock()
	return applied
}

// AddJoinList adds the list to the membership with the Update
// function. However, as a side effect, Update adds changes to
// the disseminator as well. Since we don't want to disseminate
// the potentially very large join lists, we clear all the
// changes from the disseminator, except for the one change
// that refers to the make-alive of this node.
func (m *memberlist) AddJoinList(list []Change) {
	applied := m.Update(list)
	for _, member := range applied {
		if member.Address == m.node.Address() {
			continue
		}
		m.node.disseminator.ClearChange(member.Address)
	}
}

// gets a random position in [0, length of member list)
func (m *memberlist) getJoinPosition() int {
	l := len(m.members.list)
	if l == 0 {
		return l
	}
	return rand.Intn(l)
}

// Apply tries to apply the change to the memberlsist. Returns true when the change was applied
func (m *memberlist) Apply(change Change) bool {
	member, ok := m.members.byAddress[change.Address]

	if !ok {
		// avoid indefinite tombstones by not creating new nodes
		// directly in this state
		if change.Status == Tombstone {
			return false
		}

		member = &Member{
			Address:     change.Address,
			Status:      change.Status,
			Incarnation: change.Incarnation,
		}

		if member.Address == m.node.Address() {
			m.local = member
		}

		m.members.byAddress[change.Address] = member
		i := m.getJoinPosition()
		m.members.list = append(m.members.list[:i], append([]*Member{member}, m.members.list[i:]...)...)
	}

	member.Lock()
	member.Status = change.Status
	member.Incarnation = change.Incarnation
	member.Unlock()

	return true
}

// shuffles the member list
func (m *memberlist) Shuffle() {
	m.members.Lock()
	m.members.list = shuffle(m.members.list)
	m.members.Unlock()
}

// String returns a JSON string
func (m *memberlist) String() string {
	m.members.RLock()
	str, _ := json.Marshal(m.members.list) // will never return error (presumably)
	m.members.RUnlock()
	return string(str)
}

// Iter returns a MemberlistIter for the Memberlist
func (m *memberlist) Iter() *memberlistIter {
	return newMemberlistIter(m)
}

func (m *memberlist) GetReachableMembers() []string {
	var active []string

	m.members.RLock()
	for _, member := range m.members.list {
		if member.isReachable() {
			active = append(active, member.Address)
		}
	}
	m.members.RUnlock()

	return active
}

func (m *memberlist) CountReachableMembers() int {
	count := 0

	m.members.RLock()
	for _, member := range m.members.list {
		if member.isReachable() {
			count++
		}
	}
	m.members.RUnlock()

	return count
}

// nowInMillis is a utility function that call Now on the clock and converts it
// to milliseconds.
func nowInMillis(c clock.Clock) int64 {
	return c.Now().UnixNano() / int64(time.Millisecond)
}
