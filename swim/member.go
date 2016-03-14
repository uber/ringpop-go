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
	"sync"

	"github.com/uber/ringpop-go/util"
)

const (
	// Alive is the member "alive" state
	Alive = "alive"

	// Suspect is the member "suspect" state
	Suspect = "suspect"

	// Faulty is the member "faulty" state
	Faulty = "faulty"

	// Leave is the member "leave" state
	Leave = "leave"
)

// A Member is a member in the member list
type Member struct {
	sync.RWMutex
	Address     string `json:"address"`
	Status      string `json:"status"`
	Incarnation int64  `json:"incarnationNumber"`
}

// suspect interface
func (m Member) address() string {
	return m.Address
}

func (m Member) incarnation() int64 {
	return m.Incarnation
}

// shuffles slice of members pseudo-randomly, returns new slice
func shuffle(members []*Member) []*Member {
	newMembers := make([]*Member, len(members), cap(members))
	newIndexes := rand.Perm(len(members))

	for o, n := range newIndexes {
		newMembers[n] = members[o]
	}

	return newMembers
}

// nonLocalOverride returns wether a change should be applied to the member.
// This function assumes that the address of the member and the change are
// equal.
func (m *Member) nonLocalOverride(change Change) bool {
	// change is younger than current member
	if change.Incarnation > m.Incarnation {
		return true
	}

	// change is older than current member
	if change.Incarnation < m.Incarnation {
		return false
	}

	// If the incarnation numbers are equal, we look at the state to
	// determine wether the change overrides this member.
	return statePrecedence(change.Status) > statePrecedence(m.Status)
}

// localOverride returns whether a change should be applied to to the member
func (m *Member) localOverride(local string, change Change) bool {
	if m.Address != local {
		return false
	}
	return change.Status == Faulty || change.Status == Suspect
}

func statePrecedence(s string) int {
	switch s {
	case Alive:
		return 0
	case Suspect:
		return 1
	case Faulty:
		return 2
	case Leave:
		return 3
	default:
		panic("invalid state")
	}
}

func (m *Member) isReachable() bool {
	return m.Status == Alive || m.Status == Suspect
}

// A Change is a change a member to be applied
type Change struct {
	Source            string `json:"source"`
	SourceIncarnation int64  `json:"sourceIncarnationNumber"`
	Address           string `json:"address"`
	Incarnation       int64  `json:"incarnationNumber"`
	Status            string `json:"status"`
	// Use util.Timestamp for bi-direction binding to time encoded as
	// integer Unix timestamp in JSON
	Timestamp util.Timestamp `json:"timestamp"`
}

// suspect interface
func (c Change) address() string {
	return c.Address
}

func (c Change) incarnation() int64 {
	return c.Incarnation
}
