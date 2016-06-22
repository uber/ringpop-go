// Copyright (c) 2016 Uber Technologies, Inc.
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
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"github.com/uber/ringpop-go/util"
)

type MemberTestSuite struct {
	suite.Suite
	states       []state
	localAddr    string
	nonLocalAddr string
}

type state struct {
	incNum int64
	status string
}

func (s *MemberTestSuite) SetupTest() {
	s.localAddr = "local address"
	s.nonLocalAddr = "non-local address"

	incNumStart := util.TimeNowMS()
	statuses := []string{Alive, Suspect, Faulty, Leave, Tombstone}

	// Add incNo, status combinations of ever increasing precedence.
	s.states = nil
	for i := int64(0); i < 4; i++ {
		for _, status := range statuses {
			s.states = append(s.states, state{incNumStart + i, status})
		}
	}
}

func newMember(addr string, s state) Member {
	return Member{
		Address:     addr,
		Status:      s.status,
		Incarnation: s.incNum,
	}
}

func newChange(addr string, s state) Change {
	return Change{
		Address:     addr,
		Status:      s.status,
		Incarnation: s.incNum,
	}
}

func (s *MemberTestSuite) TestNonLocalOverride() {
	// NonLocalOverride ignores the locallity and only cares about the
	// incarnation number and status of the members and changes. Since
	// the state (incNum, status pairs) slice is generated with ever
	// increasing precendence, changes with index j override members
	// with index i if and only if j > i.
	for i, s1 := range s.states {
		for j, s2 := range s.states {
			m := newMember(s.localAddr, s1)
			c := newChange(s.localAddr, s2)
			expected := j > i
			got := m.nonLocalOverride(c)
			s.Equal(expected, got, "expected override if and only if j > i")

			m = newMember(s.nonLocalAddr, s1)
			c = newChange(s.nonLocalAddr, s2)
			expected = j > i
			got = m.nonLocalOverride(c)
			s.Equal(expected, got, "expected override if and only if j > i")
		}
	}
}

func (s *MemberTestSuite) TestLocalOverride() {
	// LocalOverride marks updates as overrides when the change will be applied
	// to the status of this node. It follows the rules of SWIM with regards to
	// the incarnation number, but is hardcoded to states that the node will
	// never declare itself to. Meaning that it will allow the node to be in any
	// of Alive or Leave state.
	// The Update function reincarnates the node when LocalOverride returns true.
	for _, s1 := range s.states {
		for _, s2 := range s.states {
			m := newMember(s.localAddr, s1)
			c := newChange(s.localAddr, s2)
			expected := (c.Status == Suspect || c.Status == Faulty || c.Status == Tombstone) && c.Incarnation >= m.Incarnation
			got := m.localOverride(s.localAddr, c)
			s.Equal(expected, got, "expected override when change.Status is suspect or faulty")

			m = newMember(s.nonLocalAddr, s1)
			c = newChange(s.nonLocalAddr, s2)
			got = m.localOverride(s.localAddr, c)
			s.False(got, "expected no override since member is not local")
		}
	}
}

func TestMemberTestSuite(t *testing.T) {
	suite.Run(t, new(MemberTestSuite))
}

func TestChangeOmitTombstone(t *testing.T) {
	change := Change{
		Address:     "192.0.2.100:1234",
		Incarnation: 42,
		Status:      Alive,
	}

	data, err := json.Marshal(&change)
	require.NoError(t, err)

	parsedMap := make(map[string]interface{})
	json.Unmarshal(data, &parsedMap)
	_, has := parsedMap["tombstone"]
	assert.False(t, has, "don't expect the tombstone field to be serialized when it is")
}
