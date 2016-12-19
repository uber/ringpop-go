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
	"fmt"
	"sync"
	"time"

	"github.com/uber/ringpop-go/membership"
	"github.com/uber/ringpop-go/swim"
	"github.com/uber/ringpop-go/util"

	"github.com/uber-common/bark"
)

// fake membership.Member
type fakeMember struct {
	address  string
	identity string
}

func (f fakeMember) GetAddress() string {
	return f.address
}

func (f fakeMember) Label(key string) (value string, has bool) {
	return "", false
}

func (f fakeMember) Identity() string {
	if f.identity != "" {
		return f.identity
	}
	return f.address
}

// fake stats
type dummyStats struct {
	sync.RWMutex
	_vals map[string]int64
}

func newDummyStats() *dummyStats {
	return &dummyStats{
		_vals: make(map[string]int64),
	}
}

func (s *dummyStats) IncCounter(key string, tags bark.Tags, val int64) {
	s.Lock()
	defer s.Unlock()

	s._vals[key] += val
}

func (s *dummyStats) read(key string) int64 {
	s.RLock()
	defer s.RUnlock()

	return s._vals[key]
}

func (s *dummyStats) has(key string) bool {
	s.Lock()
	defer s.Unlock()

	_, has := s._vals[key]

	return has
}

func (s *dummyStats) clear() {
	s.Lock()
	defer s.Unlock()

	s._vals = make(map[string]int64)
}

func (s *dummyStats) UpdateGauge(key string, tags bark.Tags, val int64) {
	s.Lock()
	defer s.Unlock()

	s._vals[key] = val
}

func (s *dummyStats) RecordTimer(key string, tags bark.Tags, d time.Duration) {
	s.Lock()
	defer s.Unlock()

	s._vals[key] += util.MS(d)
}

func genAddresses(host, fromPort, toPort int) []string {
	var addresses []string

	for i := fromPort; i <= toPort; i++ {
		addresses = append(addresses, fmt.Sprintf("127.0.0.%v:%v", host, 3000+i))
	}

	return addresses
}

func genChanges(addresses []string, statuses ...string) (changes []swim.Change) {
	for _, address := range addresses {
		for _, status := range statuses {
			changes = append(changes, swim.Change{
				Address: address,
				Status:  status,
			})
		}
	}

	return changes
}

func genMembers(addresses []string) (members []membership.Member) {
	for _, address := range addresses {
		members = append(members, fakeMember{
			address: address,
		})
	}
	return
}

type MembershipChangeField int

const (
	BeforeMemberField MembershipChangeField = 1 << iota
	AfterMemberField                        = 1 << iota
)

func genMembershipChanges(members []membership.Member, fields MembershipChangeField) (changes []membership.MemberChange) {
	for _, member := range members {
		var change membership.MemberChange

		if fields&BeforeMemberField != 0 {
			change.Before = member
		}
		if fields&AfterMemberField != 0 {
			change.After = member
		}

		changes = append(changes, change)
	}
	return
}
