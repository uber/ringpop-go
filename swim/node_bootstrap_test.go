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
	"encoding/json"
	"io/ioutil"
	"os"
	"sort"
	"testing"
	"time"

	"github.com/uber/ringpop-go/discovery/jsonfile"
	"github.com/uber/ringpop-go/discovery/statichosts"

	"github.com/stretchr/testify/suite"
)

type BootstrapTestSuite struct {
	suite.Suite
	tnode *testNode
	node  *Node
	peers []*testNode
}

func (s *BootstrapTestSuite) SetupTest() {
	s.tnode = newChannelNode(s.T())
	s.node = s.tnode.node
}

func (s *BootstrapTestSuite) TearDownTest() {
	destroyNodes(s.peers...)
	destroyNodes(s.tnode)
}

func (s *BootstrapTestSuite) TestBootstrapOk() {
	s.peers = genChannelNodes(s.T(), 5)
	bootstrapNodes(s.T(), append(s.peers, s.tnode)...)
	// Reachable members should be s.node + s.peers
	s.Equal(6, s.node.CountMembers(ReachableMember))
}

func (s *BootstrapTestSuite) TestBootstrapJoinsTimeOut() {
	hosts := append(fakeHostPorts(2, 2, 1, 5), s.node.Address())
	_, err := s.node.Bootstrap(&BootstrapOptions{
		DiscoverProvider: statichosts.New(hosts...),
		MaxJoinDuration:  time.Millisecond,
		JoinTimeout:      time.Millisecond / 2,
	})

	s.Error(err, "expected bootstrap to exceed join duration")
}

func (s *BootstrapTestSuite) TestBootstrapDestroy() {
	s.tnode.Destroy()

	_, err := s.node.Bootstrap(&BootstrapOptions{
		DiscoverProvider: statichosts.New(fakeHostPorts(1, 1, 1, 10)...),
	})

	s.EqualError(err, "node destroyed while attempting to join cluster")
}

func (s *BootstrapTestSuite) TestJoinHandlerNotMakingAlive() {
	// get a bootstrapped cluster
	s.peers = genChannelNodes(s.T(), 3)
	bootstrapList := bootstrapNodes(s.T(), s.peers...)
	waitForConvergence(s.T(), 500*time.Millisecond, s.peers...)

	s.tnode.node.Bootstrap(&BootstrapOptions{
		DiscoverProvider: statichosts.New(bootstrapList...),
	})

	// test that there are no changes to disseminate after the bootstrapping of a host
	for _, peer := range s.peers {
		_, hasMember := peer.node.memberlist.Member(s.tnode.node.Address())
		s.False(hasMember, "didn't expect the bootstapping node to appear in the member list of any peers")
		s.False(peer.node.HasChanges(), "didn't expect existing node to have changes")
	}

}

func (s *BootstrapTestSuite) TestBootstrapFailsWithNoChannel() {
	n := &Node{}
	_, err := n.Bootstrap(nil)
	s.EqualError(err, "channel required")
}

func (s *BootstrapTestSuite) TestBootstrapFailsWithNoBootstrapHosts() {
	_, err := s.node.Bootstrap(nil)
	s.EqualError(err, "no discover provider")
}

// TestGossipStartedByDefault tests that a node starts pinging immediately by
// default on successful bootstrap.
func (s *BootstrapTestSuite) TestGossipStartedByDefault() {
	_, err := s.node.Bootstrap(&BootstrapOptions{
		DiscoverProvider: statichosts.New(s.node.Address()),
	})
	s.Require().NoError(err, "unable to create single node cluster")

	s.False(s.node.gossip.Stopped())
}

// TestGossipStoppedOnBootstrap tests that a node bootstraps by doesn't ping
// when Stopped is set to True in the BootstrapOptions.
func (s *BootstrapTestSuite) TestGossipStoppedOnBootstrap() {
	_, err := s.node.Bootstrap(&BootstrapOptions{
		DiscoverProvider: statichosts.New(s.node.Address()),
		Stopped:          true,
	})
	s.Require().NoError(err, "unable to create single node cluster")

	s.True(s.node.gossip.Stopped())
}

func (s *BootstrapTestSuite) TestJSONFileHostList() {
	s.peers = genChannelNodes(s.T(), 5)

	// The expected nodesJoined from the bootstrap call will be all the peers,
	// excluding the one who actually calls Bootstrap.
	var peerHostPorts []string
	for _, n := range s.peers {
		peerHostPorts = append(peerHostPorts, n.node.Address())
	}
	sort.Strings(peerHostPorts)

	// Bootstrap file should contain ALL hostports
	bootstrapFileHostPorts := append(peerHostPorts, s.tnode.node.Address())

	// Write a list of node address to a temporary bootstrap file
	file, err := ioutil.TempFile(os.TempDir(), "bootstrap-hosts")
	defer os.Remove(file.Name())

	data, err := json.Marshal(bootstrapFileHostPorts)
	s.Require().NoError(err, "error marshalling JSON from hostports")

	file.Write(data)
	file.Close()

	// Bootstrap from the JSON file
	nodesJoined, err := s.node.Bootstrap(&BootstrapOptions{
		DiscoverProvider: jsonfile.New(file.Name()),
	})
	sort.Strings(nodesJoined)

	s.NoError(err, "error bootstrapping node")
	s.Equal(peerHostPorts, nodesJoined, "nodes joined should match hostports")
}

// Errs in returning Hosts with a predefined error message
type ParrotDiscoverProvider string

func (m ParrotDiscoverProvider) Error() string {
	return string(m)
}
func (m ParrotDiscoverProvider) Hosts() ([]string, error) {
	return nil, m
}

func (s *BootstrapTestSuite) TestFailingDiscoverProvider() {
	_, err := s.node.Bootstrap(&BootstrapOptions{
		DiscoverProvider: ParrotDiscoverProvider("no hosts today"),
	})
	s.Error(err, "no hosts today", "should propagate error message")
}

func (s *BootstrapTestSuite) TestDisseminationCounterAfterBootstrap() {
	s.peers = genChannelNodes(s.T(), 10)
	bootstrapList := bootstrapNodes(s.T(), s.peers...)
	waitForConvergence(s.T(), 5*time.Second, s.peers...)

	s.Equal(s.node.disseminator.pFactor, s.node.disseminator.maxP, "Initial maxP value should equal pFactor")
	s.node.Bootstrap(&BootstrapOptions{
		DiscoverProvider: statichosts.New(bootstrapList...),
		Stopped:          true,
	})
	waitForConvergence(s.T(), 5*time.Second, append(s.peers, s.tnode)...)

	// there are 11 nodes in total, log10(11) rounded up is 2
	expected := 2 * s.node.disseminator.pFactor
	s.Equal(expected, s.node.disseminator.maxP, "maxP should be a multiple of pFactor")
}

func TestBootstrapTestSuite(t *testing.T) {
	suite.Run(t, new(BootstrapTestSuite))
}
