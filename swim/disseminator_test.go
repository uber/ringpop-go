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
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/uber/ringpop-go/swim/util"
)

type DisseminatorTestSuite struct {
	suite.Suite
	node        *Node
	d           *disseminator
	m           *memberlist
	incarnation int64
}

func (s *DisseminatorTestSuite) SetupTest() {
	s.incarnation = util.TimeNowMS()
	s.node = NewNode("test", "127.0.0.1:3001", nil, nil)
	s.node.memberlist.MakeAlive(s.node.Address(), s.incarnation)
	s.d = s.node.disseminator
	s.m = s.node.memberlist
}

func (s *DisseminatorTestSuite) TearDownTest() {
	s.node.Destroy()
}

func (s *DisseminatorTestSuite) TestChangesAreRecorded() {
	addresses := genAddresses(1, 2, 4)

	for _, address := range addresses {
		s.m.MakeAlive(address, s.incarnation)
	}

	s.Len(s.d.changes, 4, "expected four changes to be recorded")
}

func (s *DisseminatorTestSuite) TestFullSync() {
	addresses := genAddresses(1, 2, 4)

	for _, address := range addresses {
		s.m.MakeAlive(address, s.incarnation)
	}

	changes := s.d.FullSync()

	s.Len(changes, 4, "expected to get change for each member")
}

func (s *DisseminatorTestSuite) TestIssueChangesAsSender() {
	s.d.ClearChanges()

	s.m.MakeAlive("127.0.0.1:3002", s.incarnation)
	s.m.MakeSuspect("127.0.0.1:3003", s.incarnation)
	s.m.MakeFaulty("127.0.0.1:3004", s.incarnation)

	changes := s.d.IssueAsSender()
	s.Len(changes, 3, "expected three changes to be issued")
}

func (s *DisseminatorTestSuite) TestIssueChangesAsReceiver() {
	s.d.ClearChanges()

	s.m.MakeAlive("127.0.0.1:3002", s.incarnation)
	s.m.MakeSuspect("127.0.0.1:3003", s.incarnation)
	s.m.MakeFaulty("127.0.0.1:3004", s.incarnation)

	changes, fs := s.d.IssueAsReceiver(s.node.Address(), s.node.Incarnation(), s.m.checksum)
	s.Len(changes, 0, "expected no changes to be issued for same sender/receiver")
	s.False(fs, "expected changes to not be a full sync")

	changes, fs = s.d.IssueAsReceiver("127.0.0.1:3002", s.incarnation, s.m.checksum)
	s.Len(changes, 3, "expected three changes to be issued")
	s.False(fs, "expected changes to not be a full sync")

	s.d.ClearChanges()

	changes, fs = s.d.IssueAsReceiver("127.0.0.1:3002", s.incarnation, s.m.checksum)
	s.Len(changes, 0, "expected to get no changes")
	s.False(fs, "expected changes to not be a full sync")

	changes, fs = s.d.IssueAsReceiver("127.0.0.1:3002", s.incarnation, s.m.checksum+1)
	s.Len(changes, 4, "expected change to be issued for each memeber in membership")
	s.True(fs, "expected changes to be a full sync")
}

func (s *DisseminatorTestSuite) TestChangesDeleted() {
	s.d.ClearChanges()
	s.d.maxP = 2
	s.d.pFactor = 2

	address := "127.0.0.1:3002"

	s.m.MakeAlive(address, s.incarnation)

	s.Equal(0, s.d.changes[address].p, "expected propogations for change to be 0")

	changes := s.d.IssueAsSender()
	s.Len(changes, 1, "expected one change to be issued")
	s.Equal(1, s.d.changes[address].p, "expected propogations for change to be 1")

	changes = s.d.IssueAsSender()
	s.Len(changes, 1, "expected one change to be issued")
	s.Equal(2, s.d.changes[address].p, "expected propogations for change to be 2")

	changes = s.d.IssueAsSender()
	s.Empty(changes, "expected no changes to be issued")

	_, ok := s.d.changes[address]
	s.False(ok, "expected change to be deleted")
}

func TestDisseminatorTestSuite(t *testing.T) {
	suite.Run(t, new(DisseminatorTestSuite))
}
