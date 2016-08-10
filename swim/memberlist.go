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
	sync.RWMutex
}

// newMemberlist returns a new member list
func newMemberlist(n *Node) *memberlist {
	m := &memberlist{
		node:   n,
		logger: logging.Logger("membership").WithField("local", n.address),

		// prepopulate the local member with its state
		local: &Member{
			Address:     n.Address(),
			Incarnation: nowInMillis(n.clock),
			Status:      Alive,
		},
	}

	m.members.byAddress = make(map[string]*Member)
	m.members.byAddress[m.local.Address] = m.local
	m.members.list = append(m.members.list, m.local)

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
	checksum := farm.Fingerprint32([]byte(m.genChecksumString()))
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
func (m *memberlist) genChecksumString() string {
	var strings sort.StringSlice
	var buffer bytes.Buffer

	for _, member := range m.members.list {
		// Don't include Tombstone nodes in the checksum to avoid
		// bringing them back to life through full syncs
		if member.Status == Tombstone {
			continue
		}

		// collect the string from the member and add it to the list of strings
		member.checksumString(&buffer)
		strings = append(strings, buffer.String())
		// the buffer is reused for the next member and collection below
		buffer.Reset()
	}

	strings.Sort()

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
	m.members.RLock()
	for _, member := range m.members.list {
		if m.Pingable(*member) {
			n++
		}
	}
	m.members.RUnlock()

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

// returns an slice of (copied) members representing the current state of the
// membership. The membership will be filtered by the predicates provided.
func (m *memberlist) GetMembers(predicates ...MemberPredicate) (members []Member) {
	m.members.RLock()
	members = make([]Member, 0, len(m.members.list))
	for _, member := range m.members.list {
		if TestMember(*member, predicates...) {
			members = append(members, *member)
		}
	}
	m.members.RUnlock()

	return
}

// bumpIncarnation will increase the incarnation number of the local member. It
// will also prepare the change needed to gossip the change to the rest of the
// network. This function does not update the checksum stored on the membership,
// this is the responsibility of the caller since more changes might be made at
// the same time.
func (m *memberlist) bumpIncarnation() Change {
	// reincarnate the local copy of the state of the node
	m.local.Incarnation = nowInMillis(m.node.clock)

	// create a change to disseminate around
	change := Change{}
	change.populateSource(m.local)
	change.populateSubject(m.local)

	return change
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

func (m *memberlist) SetLocalStatus(status string) {
	m.local.Status = status
	m.postLocalUpdate()
}

func (m *memberlist) SetLocalLabel(key, value string) error {
	// TODO implement a sane limit for the size of the labels to prevent users
	// from impacting the performance of the gossip protocol.

	// ensure that there is a labels map
	if m.local.Labels == nil {
		m.local.Labels = make(map[string]string)
	}

	// set the label
	m.local.Labels[key] = value
	m.postLocalUpdate()

	// there was no error during the manipulation of labels
	return nil
}

// GetLocalLabel returns the value of a label set on the local node. Its second
// argument indicates if the key was present on the node or not
func (m *memberlist) GetLocalLabel(key string) (string, bool) {
	value, has := m.local.Labels[key]
	return value, has
}

// LocalLabelsAsMap copies the labels set on the local node into a map for the
// callee to use. Changes to this map will not be reflected in the labels kept
// by this node.
func (m *memberlist) LocalLabelsAsMap() map[string]string {
	if len(m.local.Labels) == 0 {
		return nil
	}

	cpy := make(map[string]string)
	for k, v := range m.local.Labels {
		cpy[k] = v
	}
	return cpy
}

// SetLocalLabels updates multiple labels at once. It will take all the labels
// that are set in the map passed to this function and overwrite the value with
// the value in the map. Keys that are not present in the provided map will
// remain in the labels of this node.
func (m *memberlist) SetLocalLabels(labels map[string]string) {
	// ensure that there is a labels map
	if m.local.Labels == nil {
		m.local.Labels = make(map[string]string)
	}

	// copy the key-value pairs to our internal labels. By not setting the map
	// of labels to the Labels value of the local member we prevent removing labels
	// that the user did not specify in the new map.
	for key, value := range labels {
		m.local.Labels[key] = value
	}
	m.postLocalUpdate()
}

// Remove a label from the local map of labels. This will trigger a reincarnation
// of the member to gossip its labels around. It returns true if all labels have
// been removed.
func (m *memberlist) RemoveLocalLabel(keys ...string) bool {
	if len(m.local.Labels) == 0 || len(keys) == 0 {
		// nothing to delete
		return false
	}

	removed := true
	for _, key := range keys {
		_, has := m.local.Labels[key]
		delete(m.local.Labels, key)
		removed = removed && has
	}
	m.postLocalUpdate()
	return removed
}

// postLocalUpdate should be called after the local Member has been updated to
// make sure that its new state has a higher incarnation number and the change
// will be recorded as a change to gossip around.
func (m *memberlist) postLocalUpdate() {
	// bump our incarnation for this change to be accepted by all peers
	change := m.bumpIncarnation()

	// since we changed our local state we need to update our checksum
	m.ComputeChecksum()

	changes := []Change{change}

	// kick in our updating mechanism
	m.node.handleChanges(changes)
	m.node.rollup.TrackUpdates(changes)
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

	member, _ := m.Member(address)

	// create the new change based on information know to the memberlist
	var change Change
	change.populateSubject(member)
	change.populateSource(m.local)

	// Override values that are specific to the change we are making
	change.Address = address
	change.Incarnation = incarnation
	change.Status = status
	// Keep track of when the change was made
	change.Timestamp = util.Timestamp(time.Now())

	return m.ApplyChange(change)
}

func (m *memberlist) ApplyChange(c Change) []Change {
	applied := m.Update([]Change{c})

	if len(applied) > 0 {
		m.logger.WithFields(bark.Fields{
			"update": applied[0],
		}).Debugf("ringpop member declares other member %s", applied[0].Status)
	}

	return applied
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

	// run through all changes received and figure out if they need to be accepted
	m.members.Lock()
	for _, change := range changes {
		member, has := m.members.byAddress[change.Address]

		// transform the change into a member that we can test against existing
		// members
		gossip := Member{}
		gossip.populateFromChange(&change)

		// test to see if we need to process the gossip
		if shouldProcessGossip(member, &gossip) {
			// the gossip overwrites the know state about the member

			if gossip.Address == m.local.Address {
				// if the gossip is about the local member it needs to be
				// countered by increasing the incarnation number and gossip the
				// new state to the network.
				change = m.bumpIncarnation()
				m.node.emit(RefuteUpdateEvent{})
			} else {
				// otherwise it can be applied to the memberlist

				if !has {
					// if the member was not already present in the list we will
					// add it and assign it a random position in the list to ensure
					// guarantees for pinging
					m.members.byAddress[gossip.Address] = &gossip
					i := m.getJoinPosition()
					m.members.list = append(m.members.list[:i], append([]*Member{&gossip}, m.members.list[i:]...)...)
				} else {
					// copy the value of the gossip into the already existing
					// struct. This operation is by value, not by reference.
					// this is to keep both the list and byAddress map in sync
					// without tedious lookup operations.
					*member = gossip
				}

			}

			// keep track of the change that it has been applied
			applied = append(applied, change)

		}
	}
	m.members.Unlock()

	if len(applied) > 0 {
		// when there are changes applied we need to recalculate our checksum
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

// getJoinPosition picks a random position in [0, length of member list), this
// assumes the caller already has a read lock on the member struct to prevent
// concurrent access.
func (m *memberlist) getJoinPosition() int {
	l := len(m.members.list)
	if l == 0 {
		return l
	}
	return rand.Intn(l)
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

// CountMembers returns the number of members maintained by the swim membership
// protocol
func (m *memberlist) CountMembers(predicates ...MemberPredicate) int {
	count := 0

	m.members.RLock()
	for _, member := range m.members.list {
		if TestMember(*member, predicates...) {
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
