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
	"github.com/uber/ringpop-go/discovery/statichosts"
	"github.com/uber/ringpop-go/membership"
)

type NodeTestSuite struct {
	suite.Suite
	testNode *testNode
	peers    []*testNode
}

func (s *NodeTestSuite) SetupTest() {
	s.testNode = newChannelNode(s.T())
	s.peers = append(s.peers, s.testNode)
}

func (s *NodeTestSuite) TearDownTest() {
	// Destroy any nodes added to the peer list during testing
	for _, node := range s.peers {
		if node != nil {
			node.Destroy()
		}
	}
}

func (s *NodeTestSuite) TestAppName() {
	s.Equal("test", s.testNode.node.App())
}

func (s *NodeTestSuite) TestStartStop() {
	s.testNode.node.Bootstrap(&BootstrapOptions{
		DiscoverProvider: statichosts.New(),
	})

	s.testNode.node.Stop()

	s.True(s.testNode.node.gossip.Stopped(), "gossip should be stopped")
	s.True(s.testNode.node.Stopped(), "node should be stopped")
	s.False(s.testNode.node.stateTransitions.enabled, "suspicion should not be enabled")

	s.testNode.node.Start()

	s.True(s.testNode.node.stateTransitions.enabled, "suspicon should be enabled")
	s.False(s.testNode.node.Stopped(), "node should not be stopped")
	s.False(s.testNode.node.gossip.Stopped(), "gossip should not be stopped")
}

func (s *NodeTestSuite) TestStoppedBootstrapOption() {
	s.testNode.node.Bootstrap(&BootstrapOptions{
		DiscoverProvider: statichosts.New(),
		Stopped:          true,
	})

	s.True(s.testNode.node.gossip.Stopped(), "gossip should be stopped")
	// TODO: Should these also be stopped?
	//s.True(s.testNode.node.Stopped(), "node should be stopped")
	//s.False(s.testNode.node.stateTransitions.enabled, "suspicion should not be enabled")
}

func (s *NodeTestSuite) TestSetIdentity() {
	_, has := s.testNode.node.Labels().Get(membership.IdentityLabelKey)
	s.False(has, "Identity label not set")

	s.testNode.node.SetIdentity("new_identity")

	value, has := s.testNode.node.Labels().Get(membership.IdentityLabelKey)
	s.True(has, "Identity label set")
	s.Equal("new_identity", value, "Identity label contains identity")
}

func TestNodeTestSuite(t *testing.T) {
	suite.Run(t, new(NodeTestSuite))
}
