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
	destroyNodes(append(s.peers, s.tnode)...)
}

func (s *BootstrapTestSuite) TestBootstrapOk() {
	s.peers = genChannelNodes(s.T(), 5)
	bootstrapNodes(s.T(), false, append(s.peers, s.tnode)...)
	// Reachable members should be s.node + s.peers
	s.Equal(6, s.node.CountReachableMembers())
}

func (s *BootstrapTestSuite) TestBootstrapTimesOut() {
	_, err := s.node.Bootstrap(&BootstrapOptions{
		Hosts:           fakeHostPorts(1, 1, 1, 0),
		MaxJoinDuration: time.Millisecond,
	})

	s.Error(err, "expected bootstrap to exceed join duration")
}

func (s *BootstrapTestSuite) TestBootstrapJoinsTimeOut() {
	_, err := s.node.Bootstrap(&BootstrapOptions{
		Hosts:           append(fakeHostPorts(2, 2, 1, 5), s.node.Address()),
		MaxJoinDuration: time.Millisecond,
		JoinTimeout:     time.Millisecond / 2,
	})

	s.Error(err, "expected bootstrap to exceed join duration")
}

func (s *BootstrapTestSuite) TestBootstrapDestroy() {
	// Destroy node first to ensure there are no races
	// in how the goroutine below is scheduled.
	s.node.Destroy()

	errChan := make(chan error)

	go func() {
		_, err := s.node.Bootstrap(&BootstrapOptions{
			Hosts:       fakeHostPorts(1, 1, 1, 10),
			JoinTimeout: time.Millisecond,
		})
		errChan <- err
	}()

	// Block until the error is received from the bootstrap
	// goroutine above.
	chanErr := <-errChan
	s.EqualError(chanErr, "node destroyed while attempting to join cluster")
}

func (s *BootstrapTestSuite) TestJoinHandlerNotMakingAlive() {
	// get a bootstrapped cluster
	s.peers = genChannelNodes(s.T(), 3)
	bootstrapList := bootstrapNodes(s.T(), true, s.peers...)

	s.tnode.node.Bootstrap(&BootstrapOptions{
		Hosts: bootstrapList,
	})

	// test that there are no changes to disseminate after the bootrstrapping of a host
	for _, peer := range s.peers {
		s.Len(peer.node.disseminator.changes, 0)
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
		Hosts: []string{s.node.Address()},
	})
	s.Require().NoError(err, "unable to create single node cluster")

	s.False(s.node.gossip.Stopped())
}

// TestGossipStoppedOnBootstrap tests that a node bootstraps by doesn't ping
// when Stopped is set to True in the BootstrapOptions.
func (s *BootstrapTestSuite) TestGossipStoppedOnBootstrap() {
	_, err := s.node.Bootstrap(&BootstrapOptions{
		Hosts:   []string{s.node.Address()},
		Stopped: true,
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
		DiscoverProvider: &JSONFileHostList{file.Name()},
	})
	sort.Strings(nodesJoined)

	s.NoError(err, "error bootstrapping node")
	s.Equal(peerHostPorts, nodesJoined, "nodes joined should match hostports")
}

func (s *BootstrapTestSuite) TestInvalidJSONFile() {
	_, err := s.node.Bootstrap(&BootstrapOptions{
		File: "/invalid",
	})
	s.Error(err, "open /invalid: no such file or directory", "should fail to open file")
}

func (s *BootstrapTestSuite) TestMalformedJSONFile() {
	_, err := s.node.Bootstrap(&BootstrapOptions{
		File: "test/invalidhosts.json",
	})
	s.Error(err, "invalid character 'T' looking for beginning of value", "should fail to unmarhsal JSON")
}

func TestBootstrapTestSuite(t *testing.T) {
	suite.Run(t, new(BootstrapTestSuite))
}
