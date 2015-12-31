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
	"github.com/uber/tchannel-go/json"
)

func seedBootstrapHosts(node *Node, addresses []string) {
	node.seedBootstrapHosts(&BootstrapOptions{Hosts: addresses})
}

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

func (s *JoinSenderTestSuite) TestSendJoinNoBoostrapHosts() {
	joined, err := sendJoin(s.node, nil)

	s.Error(err, "expected error for no bootstrap hosts")
	s.Empty(joined, "expected no nodes to be joined")
}

func (s *JoinSenderTestSuite) TestJoinNoBootstrapHosts() {
	joiner, err := newJoinSender(s.node, nil)
	s.Error(err, "expected error for no bootstrap hosts")
	s.Nil(joiner, "expected joiner to be nil")
}

func (s *JoinSenderTestSuite) TestSelectGroup() {
	fakeHosts := fakeHostPorts(1, 1, 2, 3)
	bootstrapHosts := append(fakeHosts, s.node.Address())

	seedBootstrapHosts(s.node, bootstrapHosts)

	joiner, err := newJoinSender(s.node, nil)
	s.Require().NoError(err, "cannot have an error")
	s.Require().NotNil(joiner, "joiner cannot be nil")

	group := sort.StringSlice(joiner.SelectGroup([]string{}))
	group.Sort()

	s.EqualValues([]string{fakeHosts[0], fakeHosts[1]}, group)
}

func (s *JoinSenderTestSuite) TestSelectMultipleGroups() {
	seedBootstrapHosts(s.node, append(fakeHostPorts(1, 1, 2, 3), s.node.Address()))
	expected := fakeHostPorts(1, 1, 2, 3)

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
	seedBootstrapHosts(s.node, fakeHostPorts(1, 1, 1, 5))

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
	seedBootstrapHosts(s.node, append(fakeHostPorts(1, 4, 1, 1), fakeHostPorts(1, 1, 2, 4)...))

	joiner, err := newJoinSender(s.node, &joinOpts{parallelismFactor: 1})
	s.Require().NoError(err, "cannot have an error")
	s.Require().NotNil(joiner, "joiner cannot be nil")

	group := sort.StringSlice(joiner.SelectGroup(nil))
	group.Sort()

	s.EqualValues(fakeHostPorts(2, 4, 1, 1), group)
}

func (s *JoinSenderTestSuite) TestSelectGroupMixes() {
	seedBootstrapHosts(s.node, append(fakeHostPorts(1, 2, 1, 1), fakeHostPorts(1, 1, 2, 3)...))

	joiner, err := newJoinSender(s.node, &joinOpts{parallelismFactor: 1})
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

	joiner, err := newJoinSender(s.node, nil)
	s.Require().NoError(err, "cannot have an error")
	s.Require().NotNil(joiner, "joiner cannot be nil")

	ctx, cancel := json.NewContext(joiner.timeout)
	defer cancel()

	var res joinResponse
	select {
	case err := <-joiner.MakeCall(ctx, peer.node.Address(), &res):
		s.Error(err, "expected join to fail for different apps")
	case <-ctx.Done():
		s.Fail("expected join to not timeout")
	}
}

func (s *JoinSenderTestSuite) TestJoinSelf() {
	// Set up a real listening channel
	s.tnode = newChannelNode(s.T())
	s.node = s.tnode.node
	defer s.tnode.Destroy()

	bootstrapNodes(s.T(), s.tnode)

	joiner, err := newJoinSender(s.node, nil)
	s.Require().NoError(err, "cannot have an error")
	s.Require().NotNil(joiner, "joiner cannot be nil")

	ctx, cancel := json.NewContext(joiner.timeout)
	defer cancel()

	var res joinResponse
	select {
	case err := <-joiner.MakeCall(ctx, s.node.Address(), &res):
		s.Error(err, "expected join to fail for different apps")
	case <-ctx.Done():
		s.Fail("expected join to not timeout")
	}
}

func TestJoinSenderTestSuite(t *testing.T) {
	suite.Run(t, new(JoinSenderTestSuite))
}
