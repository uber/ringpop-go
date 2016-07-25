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
	"sort"
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/uber/ringpop-go/discovery/statichosts"
)

type JoinSenderTestSuite struct {
	suite.Suite
	tnode *testNode
	node  *Node
}

func (s *JoinSenderTestSuite) SetupTest() {
	// Explicitly set up a node with a fake address in the TEST-NET-1 range for
	// join tests where we are expecting certain hostport combinations given
	// a set of test data in the 192.0.2.1-255 range. TEST-NET-1 is defined
	// in http://tools.ietf.org/html/rfc5737.
	s.tnode = newChannelNodeWithHostPort(s.T(), "192.0.2.1:1")
	s.node = s.tnode.node
}

func (s *JoinSenderTestSuite) TearDownTest() {
	s.tnode.Destroy()
}

func (s *JoinSenderTestSuite) TestSelectGroup() {
	fakeHosts := fakeHostPorts(1, 1, 2, 3)
	bootstrapHosts := append(fakeHosts, s.node.Address())

	s.node.discoverProvider = statichosts.New(bootstrapHosts...)
	joiner, err := newJoinSender(s.node, nil)

	s.Require().NoError(err, "cannot have an error")
	s.Require().NotNil(joiner, "joiner cannot be nil")

	group := sort.StringSlice(joiner.SelectGroup([]string{}))
	group.Sort()

	s.EqualValues([]string{fakeHosts[0], fakeHosts[1]}, group)
}

func (s *JoinSenderTestSuite) TestSelectMultipleGroups() {
	bootstrapHosts := append(fakeHostPorts(1, 1, 2, 3), s.node.Address())
	expected := fakeHostPorts(1, 1, 2, 3)

	s.node.discoverProvider = statichosts.New(bootstrapHosts...)
	joiner, err := newJoinSender(s.node, nil)

	s.Require().NoError(err, "cannot have an error")
	s.Require().NotNil(joiner, "joiner cannot be nil")

	group1 := sort.StringSlice(joiner.SelectGroup(nil))
	group2 := sort.StringSlice(joiner.SelectGroup(nil))
	group1.Sort()
	group2.Sort()

	s.EqualValues(group1, group2)
	s.EqualValues(expected, group1)
	s.EqualValues(expected, group2)
}

func (s *JoinSenderTestSuite) TestSelectGroupExcludes() {
	bootstrapHosts := fakeHostPorts(1, 1, 1, 5)

	s.node.discoverProvider = statichosts.New(bootstrapHosts...)
	joiner, err := newJoinSender(s.node, nil)

	s.Require().NoError(err, "cannot have an error")
	s.Require().NotNil(joiner, "joiner cannot be nil")

	// fake hostports generated are in the 192.0.2.0/24 range. See
	// fakeHostPorts for more information.
	group := sort.StringSlice(joiner.SelectGroup([]string{"192.0.2.1:3", "192.0.2.1:5"}))
	group.Sort()

	s.EqualValues([]string{"192.0.2.1:2", "192.0.2.1:4"}, group)
}

func (s *JoinSenderTestSuite) TestSelectGroupPrioritizes() {
	bootstrapHosts := append(fakeHostPorts(1, 4, 1, 1), fakeHostPorts(1, 1, 2, 4)...)

	s.node.discoverProvider = statichosts.New(bootstrapHosts...)
	joiner, err := newJoinSender(s.node, &joinOpts{
		parallelismFactor: 1,
	})
	s.Require().NoError(err, "cannot have an error")
	s.Require().NotNil(joiner, "joiner cannot be nil")

	group := sort.StringSlice(joiner.SelectGroup(nil))
	group.Sort()

	s.EqualValues(fakeHostPorts(2, 4, 1, 1), group)
}

func (s *JoinSenderTestSuite) TestSelectGroupMixes() {
	bootstrapHosts := append(fakeHostPorts(1, 2, 1, 1), fakeHostPorts(1, 1, 2, 3)...)

	s.node.discoverProvider = statichosts.New(bootstrapHosts...)
	joiner, err := newJoinSender(s.node, &joinOpts{
		parallelismFactor: 1,
	})
	s.Require().NoError(err, "cannot have an error")
	s.Require().NotNil(joiner, "joiner cannot be nil")

	group := sort.StringSlice(joiner.SelectGroup(nil))
	group.Sort()

	s.EqualValues(append(fakeHostPorts(1, 1, 2, 3), fakeHostPorts(2, 2, 1, 1)...), group)
}

func (s *JoinSenderTestSuite) TestJoinDifferentApp() {
	peer := newChannelNode(s.T())
	peer.node.app = "different"
	defer peer.Destroy()

	bootstrapNodes(s.T(), s.tnode)
	bootstrapNodes(s.T(), peer)

	s.node.discoverProvider = statichosts.New(fakeHostPorts(1, 1, 1, 1)...)
	joiner, err := newJoinSender(s.node, nil)

	s.Require().NoError(err, "cannot have an error")
	s.Require().NotNil(joiner, "joiner cannot be nil")

	res, err := sendJoinRequest(s.node, peer.node.Address(), joiner.timeout)
	s.Nil(res, "expected err, not a response")
	s.Error(err, "expected join to fail for different apps")
}

func (s *JoinSenderTestSuite) TestJoinSelf() {
	// Set up a real listening channel
	s.tnode.Destroy()
	s.tnode = newChannelNode(s.T())
	s.node = s.tnode.node

	bootstrapNodes(s.T(), s.tnode)

	s.node.discoverProvider = statichosts.New(fakeHostPorts(1, 1, 1, 1)...)
	joiner, err := newJoinSender(s.node, nil)

	s.Require().NoError(err, "cannot have an error")
	s.Require().NotNil(joiner, "joiner cannot be nil")

	res, err := sendJoinRequest(s.node, s.node.Address(), joiner.timeout)
	s.Nil(res, "expected err, not a response")
	s.Error(err, "expected join to fail when joining yourself")
}

func (s *JoinSenderTestSuite) TestCustomDelayer() {
	delayer := &nullDelayer{}

	s.node.discoverProvider = statichosts.New(fakeHostPorts(1, 1, 1, 1)...)
	joiner, err := newJoinSender(s.node, &joinOpts{
		delayer: delayer,
	})

	s.NoError(err, "expected valid joiner")
	s.Equal(delayer, joiner.delayer, "custom delayer was set")
}

func TestJoinSenderTestSuite(t *testing.T) {
	suite.Run(t, new(JoinSenderTestSuite))
}

func (s *JoinSenderTestSuite) TestDontDisseminateJoinList() {
	tnodes := genChannelNodes(s.T(), 5)
	bootstrapNodes(s.T(), tnodes...)

	for i, tnode := range tnodes {
		s.Equal(i+1, tnodes[i].node.memberlist.CountMembers(ReachableMember), "expected that next node joined all previous ones")
		s.Equal(1, tnodes[i].node.disseminator.ChangesCount(), "expected to only disseminate yourself")

		_, ok := tnodes[i].node.disseminator.ChangesByAddress(tnode.node.Address())
		s.True(ok, "expected to only disseminate yourself")
	}
}
