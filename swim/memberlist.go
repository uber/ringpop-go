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

	"github.com/uber/ringpop-go/membership"

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
func newMemberlist(n *Node, initialLabels LabelMap) *memberlist {
	m := &memberlist{
		node:   n,
		logger: logging.Logger("membership").WithField("local", n.address),

		// prepopulate the local member with its state
		local: &Member{
			Address:     n.Address(),
			Incarnation: nowInMillis(n.clock),
			Status:      Alive,
			Labels:      initialLabels,
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

	m.node.EmitEvent(ChecksumComputeEvent{
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
	var memberCopy *Member
	m.members.RLock()
	member, ok := m.members.byAddress[address]
	if member != nil {
		memberCopy = new(Member)
		*memberCopy = *member
	}
	m.members.RUnlock()

	return memberCopy, ok
}

// LocalMember returns a copy of the local Member in a thread safe way.
func (m *memberlist) LocalMember() (member Member) {
	m.members.Lock()
	// copy local member state
	member = *m.local
	m.members.Unlock()
	return
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
	member := new(Member)
	*member = *m.members.list[i]
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
func (m *memberlist) RandomPingableMembers(n int, excluding map[string]bool) []Member {
	members := make([]Member, 0, n)

	m.members.RLock()
	indices := rand.Perm(len(m.members.list))
	for _, index := range indices {
		member := m.members.list[index]
		if m.Pingable(*member) && !excluding[member.Address] {
			members = append(members, *member)
			if len(members) >= n {
				break
			}
		}
	}
	m.members.RUnlock()
	return members
}

// returns an slice of (copied) members representing the current state of the
// membership. The membership will be filtered by the predicates provided.
func (m *memberlist) GetMembers(predicates ...MemberPredicate) (members []Member) {
	m.members.RLock()
	members = make([]Member, 0, len(m.members.list))
	for _, member := range m.members.list {
		if MemberMatchesPredicates(*member, predicates...) {
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
	m.node.EmitEvent(MakeNodeStatusEvent{Alive})
	return m.MakeChange(address, incarnation, Alive)
}

func (m *memberlist) MakeSuspect(address string, incarnation int64) []Change {
	m.node.EmitEvent(MakeNodeStatusEvent{Suspect})
	return m.MakeChange(address, incarnation, Suspect)
}

func (m *memberlist) MakeFaulty(address string, incarnation int64) []Change {
	m.node.EmitEvent(MakeNodeStatusEvent{Faulty})
	return m.MakeChange(address, incarnation, Faulty)
}

func (m *memberlist) SetLocalStatus(status string) {
	m.updateLocalMember(func(member *Member) bool {
		member.Status = status
		return true
	})
}

// SetLocalLabel sets the label identified by key to the new value. This
// operation is validated against the configured limits for labels and will
// return an ErrLabelSizeExceeded in the case this operation would alter the
// labels of the node in such a way that the configured limits are exceeded.
func (m *memberlist) SetLocalLabel(key, value string) error {
	return m.SetLocalLabels(map[string]string{key: value})
}

// GetLocalLabel returns the value of a label set on the local node. Its second
// argument indicates if the key was present on the node or not
func (m *memberlist) GetLocalLabel(key string) (string, bool) {
	m.members.RLock()
	value, has := m.local.Labels[key]
	m.members.RUnlock()
	return value, has
}

// LocalLabelsAsMap copies the labels set on the local node into a map for the
// callee to use. Changes to this map will not be reflected in the labels kept
// by this node.
func (m *memberlist) LocalLabelsAsMap() map[string]string {
	m.members.RLock()
	defer m.members.RUnlock()
	if len(m.local.Labels) == 0 {
		return nil
	}

	cpy := make(map[string]string, len(m.local.Labels))
	for k, v := range m.local.Labels {
		cpy[k] = v
	}
	return cpy
}

// SetLocalLabels updates multiple labels at once. It will take all the labels
// that are set in the map passed to this function and overwrite the value with
// the value in the map. Keys that are not present in the provided map will
// remain in the labels of this node. The operation is guaranteed to succeed
// completely or not at all.
// Before any changes are made to the labels the input is validated against the
// configured limits on labels. This function will propagate any error that is
// returned by the validation of the label limits eg. ErrLabelSizeExceeded
func (m *memberlist) SetLocalLabels(labels map[string]string) error {
	if err := m.node.labelLimits.validateLabels(m.local.Labels, labels); err != nil {
		// the labels operation violates the label limits that has been configured
		return err
	}

	m.updateLocalMember(func(member *Member) bool {
		// ensure that there is a new copy of the labels to work with.
		labelsCopy := member.Labels.copy()

		// keep track if we made changes to the labels
		changes := false

		// copy the key-value pairs to our internal labels. By not setting the map
		// of labels to the Labels value of the local member we prevent removing labels
		// that the user did not specify in the new map.
		for key, value := range labels {
			old, had := labelsCopy[key]
			labelsCopy[key] = value

			if !had || old != value {
				changes = true
			}
		}

		if changes {
			// only if there are changes we put the copied labels on the member.
			member.Labels = labelsCopy
		}
		return changes
	})

	return nil
}

// RemoveLocalLabels removes the labels keyed by the keys from the local map of
// labels. When changes are made to the member state a reincarnation will be
// triggered and the new state of the member will be recorded in the disseminator
// and subsequently be gossiped around. It is a valid operation to remove non-
// existing keys. It returns true if all (and only all) labels have been removed.
func (m *memberlist) RemoveLocalLabels(keys ...string) bool {
	// keep track if all labels are removed, it will be set to false if a label
	// couldn't be removed.
	removed := true

	m.updateLocalMember(func(member *Member) bool {
		// ensure that there is a new copy of the labels to work
		// with.
		labelsCopy := member.Labels.copy()

		any := false // keep track if we at least removed one label
		for _, key := range keys {
			_, has := labelsCopy[key]
			delete(labelsCopy, key)
			removed = removed && has
			any = any || has
		}

		if any {
			// only if there are changes we put the copied labels on the member.
			member.Labels = labelsCopy
		}

		return any
	})

	return removed
}

// updateLocalMember takes an update function to upate the member passed in. The
// update function can make mutations to the member and should indicate if it
// made changes, only if changes are made the incarnation number will be bumped
// and the new state will be gossiped to the peers
func (m *memberlist) updateLocalMember(update func(*Member) bool) {
	m.members.Lock()

	before := *m.local
	didUpdate := update(m.local)

	// exit if the update didn't change anything
	if !didUpdate {
		m.members.Unlock()
		return
	}

	// bump incarnation number if the member has been updated
	change := m.bumpIncarnation()

	changes := []Change{change}

	after := *m.local

	m.members.Unlock()

	// since we changed our local state we need to update our checksum
	m.ComputeChecksum()

	// kick in our updating mechanism
	m.node.handleChanges(changes)

	// prepare a membership change event for observable state changes
	var memberChange membership.MemberChange
	if before.isReachable() {
		memberChange.Before = before
	}
	if after.isReachable() {
		memberChange.After = after
	}

	if memberChange.Before != nil || memberChange.After != nil {
		m.node.EmitEvent(membership.ChangeEvent{
			Changes: []membership.MemberChange{
				memberChange,
			},
		})
	}
}

// MakeTombstone declares the node with the provided address in the tombstone state
// on the given incarnation number. If the incarnation number in the local memberlist
// is already higher than the incartation number provided in this function it is
// essentially a no-op. The list of changes that is returned is the actual list of
// changes that have been applied to the memberlist. It can be used to test if the
// tombstone declaration has been executed atleast to the local memberlist.
func (m *memberlist) MakeTombstone(address string, incarnation int64) []Change {
	m.node.EmitEvent(MakeNodeStatusEvent{Tombstone})
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

	m.node.EmitEvent(MemberlistChangesReceivedEvent{changes})

	var memberChanges []membership.MemberChange

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
				m.node.EmitEvent(RefuteUpdateEvent{})
			} else {
				// otherwise it can be applied to the memberlist

				// prepare the change and collect if there is an outside
				// observable change eg. changes that involve active
				// participants of the membership (pingable)
				memberChange := membership.MemberChange{}
				if has && member.isReachable() {
					memberChange.Before = *member
				}
				if gossip.isReachable() {
					memberChange.After = gossip
				}
				if memberChange.Before != nil || memberChange.After != nil {
					memberChanges = append(memberChanges, memberChange)
				}

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

		m.node.EmitEvent(MemberlistChangesAppliedEvent{
			Changes:     applied,
			OldChecksum: oldChecksum,
			NewChecksum: m.Checksum(),
			NumMembers:  m.NumMembers(),
		})
		m.node.handleChanges(applied)

	}

	// if there are changes that are important for outside observers of the
	// membership emit those
	if len(memberChanges) > 0 {
		m.node.EmitEvent(membership.ChangeEvent{
			Changes: memberChanges,
		})
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
// protocol for all members that match the predicates
func (m *memberlist) CountMembers(predicates ...MemberPredicate) int {
	count := 0

	m.members.RLock()
	for _, member := range m.members.list {
		if MemberMatchesPredicates(*member, predicates...) {
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
