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

	"github.com/dgryski/go-farm"
	"github.com/uber/ringpop-go/swim/util"
)

// A memberlist contains the membership for a node
type memberlist struct {
	node     *Node
	mmap     map[string]*Member
	mlist    []*Member
	local    *Member
	checksum uint32

	// protects I/O operations on the member list
	l sync.RWMutex
}

// newMemberlist returns a new member list
func newMemberlist(n *Node) *memberlist {
	memberlist := &memberlist{
		node: n,
		mmap: make(map[string]*Member),
	}

	return memberlist
}

func (m *memberlist) Checksum() uint32 {
	m.l.RLock()
	defer m.l.RUnlock()

	return m.checksum
}

// computes membership checksum
func (m *memberlist) ComputeChecksum() {
	m.l.Lock()
	defer m.l.Unlock()

	m.checksum = farm.Hash32([]byte(m.GenChecksumString()))
}

// generates string to use when computing checksum
func (m *memberlist) GenChecksumString() string {
	var strings sort.StringSlice

	for _, member := range m.mmap {
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
	m.l.RLock()
	defer m.l.RUnlock()

	member, ok := m.mmap[address]
	return member, ok
}

func (m *memberlist) MemberAt(i int) *Member {
	m.l.RLock()
	defer m.l.RUnlock()

	return m.mlist[i]
}

// returns whether or not a member is pingable
func (m *memberlist) Pingable(member Member) bool {
	return member.Address != m.local.Address &&
		(member.Status == Alive || member.Status == Suspect)

}

// returns the number of pingable members in the memberlist
func (m *memberlist) NumPingableMembers() (n int) {
	m.l.RLock()
	defer m.l.RUnlock()

	for _, member := range m.mlist {
		if m.Pingable(*member) {
			n++
		}
	}
	return
}

// returns n pingable members in the member list
func (m *memberlist) RandomPingableMembers(n int, excluding map[string]bool) []*Member {
	var members []*Member

	m.l.RLock()
	defer m.l.RUnlock()

	for _, member := range m.mlist {
		if m.Pingable(*member) && !excluding[member.Address] {
			members = append(members, member)
		}
	}

	// shuffle members and take first n
	members = shuffle(members)

	if n > len(members) {
		return members
	}
	return members[:n]
}

// returns an immutable slice of members representing the current state of the membership
func (m *memberlist) GetMembers() (members []Member) {
	m.l.Lock()
	defer m.l.Unlock()

	for _, member := range m.mlist {
		members = append(members, *member)
	}

	return
}

func (m *memberlist) MakeAlive(address string, incarnation int64) []Change {
	return m.MakeChange(address, incarnation, Alive)
}

func (m *memberlist) MakeSuspect(address string, incarnation int64) []Change {
	return m.MakeChange(address, incarnation, Suspect)
}

func (m *memberlist) MakeFaulty(address string, incarnation int64) []Change {
	return m.MakeChange(address, incarnation, Faulty)
}

func (m *memberlist) MakeLeave(address string, incarnation int64) []Change {
	return m.MakeChange(address, incarnation, Leave)
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

	return m.Update([]Change{Change{
		Source:            m.local.Address,
		SourceIncarnation: m.local.Incarnation,
		Address:           address,
		Incarnation:       incarnation,
		Status:            status,
		Timestamp:         time.Now(),
	}})
}

// updates the member list with the slice of changes, applying selectively
func (m *memberlist) Update(changes []Change) (applied []Change) {
	if m.node.Stopped() || len(changes) == 0 {
		return nil
	}

	m.node.emit(MemberlistChangesReceivedEvent{changes})

	m.l.Lock()

	for _, change := range changes {
		member, ok := m.mmap[change.Address]

		// first time member has been seen, take change wholesale
		if !ok {
			m.Apply(change)
			applied = append(applied, change)
			continue
		}

		// if change is local override, reassert member is alive
		if localOverride(m.node.Address(), member, change) {
			overrideChange := Change{
				Source:            change.Source,
				SourceIncarnation: change.SourceIncarnation,
				Address:           change.Address,
				Incarnation:       util.TimeNowMS(),
				Status:            Alive,
				Timestamp:         time.Now(),
			}

			m.Apply(overrideChange)
			applied = append(applied, overrideChange)
			continue
		}

		// if non-local override, apply change wholesale
		if nonLocalOverride(member, change) {
			m.Apply(change)
			applied = append(applied, change)
		}
	}

	m.l.Unlock()

	if len(applied) > 0 {
		oldChecksum := m.Checksum()
		m.ComputeChecksum()
		m.node.emit(MemberlistChangesAppliedEvent{
			Changes:     applied,
			OldChecksum: oldChecksum,
			NewChecksum: m.Checksum(),
			NumMembers:  len(m.mlist),
		})
		m.node.handleChanges(applied)
		m.node.rollup.TrackUpdates(applied)
	}

	return
}

// gets a random position in [0, length of member list)
func (m *memberlist) getJoinPosition() int {
	l := len(m.mlist)
	if l == 0 {
		return l
	}
	return rand.Intn(len(m.mlist))
}

// applies a change directly to the member list
func (m *memberlist) Apply(change Change) {
	member, ok := m.mmap[change.Address]

	if !ok {
		member = &Member{
			Address:     change.Address,
			Status:      change.Status,
			Incarnation: change.Incarnation,
		}

		if member.Address == m.node.Address() {
			m.local = member
		}

		m.mmap[change.Address] = member
		i := m.getJoinPosition()
		m.mlist = append(m.mlist[:i], append([]*Member{member}, m.mlist[i:]...)...)
	}

	member.Status = change.Status
	member.Incarnation = change.Incarnation
}

// shuffles the member list
func (m *memberlist) Shuffle() {
	m.l.Lock()
	defer m.l.Unlock()

	m.mlist = shuffle(m.mlist)
}

// String returns a JSON string
func (m *memberlist) String() string {
	m.l.RLock()
	defer m.l.RUnlock()

	str, _ := json.Marshal(m.mlist) // will never return error (presumably)
	return string(str)
}

// Iter returns a MemberlistIter for the Memberlist
func (m *memberlist) Iter() *memberlistIter {
	return newMemberlistIter(m)
}
