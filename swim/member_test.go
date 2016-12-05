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

var shouldProcessGossipTests = []struct {
	member  *Member
	gossip  *Member
	process bool
	name    string
}{
	// test against unknown members
	{nil, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Alive}, true, "accept Alive gossip for an unknown member"},
	{nil, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Suspect}, true, "accept Suspect gossip for an unknown member"},
	{nil, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Faulty}, true, "accept Faulty gossip for an unknown member"},
	{nil, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Leave}, true, "accept Leave gossip for an unknown member"},
	{nil, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Tombstone}, false, "not accept Tombstone gossip for an unknown member"},

	// test against existing members with newer incarnation numbers
	{&Member{Address: "192.0.2.1:1234", Incarnation: 1337, Status: Alive}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Alive}, false, "not accept an Alive gossip with an older incarnation number"},
	{&Member{Address: "192.0.2.1:1234", Incarnation: 1337, Status: Alive}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Suspect}, false, "not accept a Suspect gossip with an older incarnation number"},
	{&Member{Address: "192.0.2.1:1234", Incarnation: 1337, Status: Alive}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Faulty}, false, "not accept a Faulty gossip with an older incarnation number"},
	{&Member{Address: "192.0.2.1:1234", Incarnation: 1337, Status: Alive}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Leave}, false, "not accept a Leave gossip with an older incarnation number"},
	{&Member{Address: "192.0.2.1:1234", Incarnation: 1337, Status: Alive}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Tombstone}, false, "not accept a Tombstone gossip with an older incarnation number"},

	// test against existing members with an older incarnation number
	{&Member{Address: "192.0.2.1:1234", Incarnation: 41, Status: Alive}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Alive}, true, "accept an Alive gossip with an newer incarnation number"},
	{&Member{Address: "192.0.2.1:1234", Incarnation: 41, Status: Alive}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Suspect}, true, "accept a Suspect gossip with an newer incarnation number"},
	{&Member{Address: "192.0.2.1:1234", Incarnation: 41, Status: Alive}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Faulty}, true, "accept a Faulty gossip with an newer incarnation number"},
	{&Member{Address: "192.0.2.1:1234", Incarnation: 41, Status: Alive}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Leave}, true, "accept a Leave gossip with an newer incarnation number"},
	{&Member{Address: "192.0.2.1:1234", Incarnation: 41, Status: Alive}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Tombstone}, true, "accept a Tombstone gossip with an newer incarnation number"},

	// test against existing members with the same incarnation number starting on Alive
	{&Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Alive}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Alive}, false, "not accept an Alive gossip with the same incarnation number when Alive"},
	{&Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Alive}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Suspect}, true, "accept a Suspect gossip with the same incarnation number when Alive"},
	{&Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Alive}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Faulty}, true, "accept a Faulty gossip with the same incarnation number when Alive"},
	{&Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Alive}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Leave}, true, "accept a Leave gossip with the same incarnation number when Alive"},
	{&Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Alive}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Tombstone}, true, "accept a Tombstone gossip with the same incarnation number when Alive"},

	// test against existing members with the same incarnation number starting on Suspect
	{&Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Suspect}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Alive}, false, "not accept an Alive gossip with the same incarnation number when Suspect"},
	{&Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Suspect}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Suspect}, false, "not accept a Suspect gossip with the same incarnation number when Suspect"},
	{&Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Suspect}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Faulty}, true, "accept a Faulty gossip with the same incarnation number when Suspect"},
	{&Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Suspect}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Leave}, true, "accept a Leave gossip with the same incarnation number when Suspect"},
	{&Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Suspect}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Tombstone}, true, "accept a Tombstone gossip with the same incarnation number when Suspect"},

	// test against existing members with the same incarnation number starting on Faulty
	{&Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Faulty}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Alive}, false, "not accept an Alive gossip with the same incarnation number when Faulty"},
	{&Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Faulty}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Suspect}, false, "not accept a Suspect gossip with the same incarnation number when Faulty"},
	{&Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Faulty}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Faulty}, false, "not accept a Faulty gossip with the same incarnation number when Faulty"},
	{&Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Faulty}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Leave}, true, "accept a Leave gossip with the same incarnation number when Faulty"},
	{&Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Faulty}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Tombstone}, true, "accept a Tombstone gossip with the same incarnation number when Faulty"},

	// test against existing members with the same incarnation number starting on Leave
	{&Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Leave}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Alive}, false, "not accept an Alive gossip with the same incarnation number when Leave"},
	{&Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Leave}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Suspect}, false, "not accept a Suspect gossip with the same incarnation number when Leave"},
	{&Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Leave}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Faulty}, false, "not accept a Faulty gossip with the same incarnation number when Leave"},
	{&Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Leave}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Leave}, false, "not accept a Leave gossip with the same incarnation number when Leave"},
	{&Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Leave}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Tombstone}, true, "accept a Tombstone gossip with the same incarnation number when Leave"},

	// test against existing members with the same incarnation number starting on Tombstone
	{&Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Tombstone}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Alive}, false, "not accept an Alive gossip with the same incarnation number when Tombstone"},
	{&Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Tombstone}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Suspect}, false, "not accept a Suspect gossip with the same incarnation number when Tombstone"},
	{&Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Tombstone}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Faulty}, false, "not accept a Faulty gossip with the same incarnation number when Tombstone"},
	{&Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Tombstone}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Leave}, false, "not accept a Leave gossip with the same incarnation number when Tombstone"},
	{&Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Tombstone}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Tombstone}, false, "not accept a Tombstone gossip with the same incarnation number when Tombstone"},

	// test members where only the labels are different
	{&Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Alive}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Alive, Labels: map[string]string{"hello": "world"}}, true, "should accept an Alive gossip with the same incarnation number when labels override exising state"},
	{&Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Alive, Labels: map[string]string{"hello": "world"}}, &Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Alive}, false, "should not accept an Alive gossip with the same incarnation number when labels don't override exising state"},
}

func TestAcceptGossip(t *testing.T) {
	for _, test := range shouldProcessGossipTests {
		assert.Equal(t, test.process, shouldProcessGossip(test.member, test.gossip), "expected to "+test.name)
	}
}

func TestIdentityWithoutLabel(t *testing.T) {
	m := Member{
		Address: "192.0.2.1:1234",
	}
	assert.Equal(t, "192.0.2.1:1234", m.Identity(), "Expected identity to be equal to address")
}

func TestIdentityWithIdentityLabel(t *testing.T) {
	m := Member{
		Address: "192.0.2.1:1234",
		Labels: LabelMap{
			"__identity": "identityFromLabel",
		},
	}
	assert.Equal(t, "identityFromLabel", m.Identity(), "Expected identity to be equal to address")
}
