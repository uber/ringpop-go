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
	"math/rand"
	"time"
)

const (
	// Alive is the member "Alive" state
	Alive = "Alive"

	// Faulty is the member "Faulty" state
	Faulty = "Faulty"

	// Leave is the member "Leave" state
	Leave = "Leave"

	// Suspect is the memeber "Suspect" state
	Suspect = "Suspect"
)

// A Member is a member in the member list
type Member struct {
	Address     string
	Status      string
	Incarnation int64
}

func (m Member) address() string {
	return m.Address
}

func (m Member) incarnation() int64 {
	return m.Incarnation
}

// shuffles slice of members pseudo-randomly
func shuffle(members []*Member) []*Member {
	newMembers := make([]*Member, len(members), cap(members))
	newIndexes := rand.Perm(len(members))

	for o, n := range newIndexes {
		newMembers[n] = members[o]
	}

	return newMembers
}

func (m *Member) nonLocalOverride(change Change) bool {
	return m.aliveOverride(change) ||
		m.suspectOverride(change) ||
		m.faultyOverride(change) ||
		m.leaveOverride(change)
}

func (m *Member) aliveOverride(change Change) bool {
	return change.Status == Alive && change.Incarnation > m.Incarnation
}

func (m *Member) faultyOverride(change Change) bool {
	return change.Status == Faulty &&
		((m.Status == Suspect && change.Incarnation >= m.Incarnation) ||
			(m.Status == Faulty && change.Incarnation > m.Incarnation) ||
			(m.Status == Alive && change.Incarnation >= m.Incarnation))
}

func (m *Member) leaveOverride(change Change) bool {
	return change.Status == Leave &&
		m.Status != Leave && change.Incarnation >= m.Incarnation
}

func (m *Member) suspectOverride(change Change) bool {
	return change.Status == Suspect &&
		((m.Status == Suspect && change.Incarnation > m.Incarnation) ||
			(m.Status == Faulty && change.Incarnation > m.Incarnation) ||
			(m.Status == Alive && change.Incarnation >= m.Incarnation))
}

func (m *Member) localOverride(local string, change Change) bool {
	return m.localSuspectOverride(local, change) || m.localFaultyOverride(local, change)
}

func (m *Member) localFaultyOverride(local string, change Change) bool {
	return m.Address == local && change.Status == Faulty
}

func (m *Member) localSuspectOverride(local string, change Change) bool {
	return m.Address == local && change.Status == Suspect
}

// A Change is a change a member to be applied
type Change struct {
	Source            string    `json:"source"`
	SourceIncarnation int64     `json:"sourceIncarnation"`
	Address           string    `json:"address"`
	Incarnation       int64     `json:"incarnation"`
	Status            string    `json:"status"`
	Timestamp         time.Time `json:"timestamp"`
}

func (c Change) address() string {
	return c.Address
}

func (c Change) incarnation() int64 {
	return c.Incarnation
}
