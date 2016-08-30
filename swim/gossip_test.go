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
)

type GossipTestSuite struct {
	suite.Suite
	tnode       *testNode
	node        *Node
	g           *gossip
	m           *memberlist
	incarnation int64
}

func (s *GossipTestSuite) SetupTest() {
	s.tnode = newChannelNode(s.T())
	s.node = s.tnode.node

	s.g = s.node.gossip
	s.m = s.node.memberlist
}

func (s *GossipTestSuite) TearDownTest() {
	s.tnode.Destroy()
}

func (s *GossipTestSuite) TestStartStop() {
	s.True(s.g.Stopped(), "expected gossip to be stopped")

	s.g.Start()
	s.False(s.g.Stopped(), "expected gossip to be started")

	s.g.Stop()
	s.True(s.g.Stopped(), "expected gossip to be stopped")
}

func (s *GossipTestSuite) TestStartWhileRunning() {
	s.True(s.g.Stopped(), "expected gossip to be stopped")

	s.g.Start()
	s.False(s.g.Stopped(), "expected gossip to be started")

	s.g.Start()
	s.False(s.g.Stopped(), "expected gossip to still be started")
}

func (s *GossipTestSuite) StopWhileStopped() {
	s.True(s.g.Stopped(), "expected gossip to be stopped")

	s.g.Stop()
	s.True(s.g.Stopped(), "expected gossip to still be stopped")
}

func (s *GossipTestSuite) TestUpdatesArePropagated() {
	peer := newChannelNode(s.T())
	defer peer.Destroy()
	defer peer.channel.Close()

	bootstrapNodes(s.T(), s.tnode, peer)
	waitForConvergence(s.T(), 100, s.tnode, peer)
	s.True(s.g.Stopped())
	s.True(peer.node.gossip.Stopped())

	s.node.disseminator.ClearChanges()
	peer.node.disseminator.ClearChanges()

	peer.node.memberlist.Update([]Change{
		Change{Address: "127.0.0.1:3003", Incarnation: s.incarnation, Status: Alive},
		Change{Address: "127.0.0.1:3004", Incarnation: s.incarnation, Status: Faulty},
		Change{Address: "127.0.0.1:3005", Incarnation: s.incarnation, Status: Suspect},
		Change{Address: "127.0.0.1:3006", Incarnation: s.incarnation, Status: Leave},
	})

	s.Len(peer.node.disseminator.changes, 4)

	s.g.ProtocolPeriod()

	expectedMembers := []Member{
		Member{Address: "127.0.0.1:3003", Status: Alive},
		Member{Address: "127.0.0.1:3004", Status: Faulty},
		Member{Address: "127.0.0.1:3005", Status: Suspect},
		Member{Address: "127.0.0.1:3006", Status: Leave},
	}

	memberlistHasMembers(s.T(), s.node.memberlist, expectedMembers)
}

// TODO: move this test to node_test?
func (s *GossipTestSuite) TestSuspicionStarted() {
	peers := genChannelNodes(s.T(), 3)
	defer destroyNodes(peers...)

	bootstrapNodes(s.T(), append(peers, s.tnode)...)

	s.node.memberiter = new(dummyIter) // always returns a member at 127.0.0.1:3010

	s.g.ProtocolPeriod()

	s.NotNil(s.node.stateTransitions.timer("127.0.0.1:3010"), "expected state timer to be set")
}

func TestGossipTestSuite(t *testing.T) {
	suite.Run(t, new(GossipTestSuite))
}
