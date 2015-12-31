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
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/uber/ringpop-go/swim/util"
)

type PingRequestTestSuite struct {
	suite.Suite
	tnode       *testNode
	node        *Node
	peers       []*testNode
	incarnation int64
}

func (s *PingRequestTestSuite) SetupTest() {
	s.incarnation = util.TimeNowMS()
	s.tnode = newChannelNode(s.T())
	s.node = s.tnode.node
	s.peers = genChannelNodes(s.T(), 2)
}

func (s *PingRequestTestSuite) TearDownTest() {
	destroyNodes(append(s.peers, s.tnode)...)
}

func (s *PingRequestTestSuite) TestOk() {
	bootstrapNodes(s.T(), append(s.peers, s.tnode)...)

	response := <-sendPingRequests(s.node, s.peers[0].node.Address(), 1, time.Second)
	switch res := response.(type) {
	case *pingResponse:
		s.True(res.Ok, "expected remote ping to succeed")
	default:
		s.Fail("expected response from ping request peer")
	}
}

func (s *PingRequestTestSuite) TestRemoteFail() {
	bootstrapNodes(s.T(), s.tnode, s.peers[0])

	response := <-sendPingRequests(s.node, "127.0.0.1:3005", 1, time.Second)
	switch res := response.(type) {
	case *pingResponse:
		s.False(res.Ok, "expected remote ping to fail")
	default:
		s.Fail("expected response from ping request peer")
	}
}

func (s *PingRequestTestSuite) TestRemoteTimesOut() {
	bootstrapNodes(s.T(), s.tnode, s.peers[0])

	s.peers[0].node.pingTimeout = time.Millisecond

	response := <-sendPingRequests(s.node, "127.0.0.2:3001", 1, time.Second)
	switch res := response.(type) {
	case *pingResponse:
		s.False(res.Ok, "expected remote ping to fail")
	default:
		s.Fail("expected response from ping request peer")
	}
}

func (s *PingRequestTestSuite) TestFail() {
	bootstrapNodes(s.T(), s.tnode)

	s.node.memberlist.MakeAlive("127.0.0.1:3005", s.incarnation) // peer to use for ping request

	response := <-sendPingRequests(s.node, "127.0.0.1:3003", 1, time.Second)
	switch res := response.(type) {
	case error:
		s.Error(res, "expected ping request to fail")
	default:
		s.Fail("expected ping request to fail")
	}
}

func (s *PingRequestTestSuite) TestTimesOut() {
	bootstrapNodes(s.T(), s.tnode)

	s.node.memberlist.MakeAlive("127.0.0.2:3001", s.incarnation) // peer to use for ping request

	response := <-sendPingRequests(s.node, "127.0.0.1:3002", 1, time.Millisecond)
	switch res := response.(type) {
	case error:
		s.Error(res, "expected ping request to fail")
	default:
		s.Fail("expected ping request to fail")
	}

}

func TestPingRequestTestSuite(t *testing.T) {
	suite.Run(t, new(PingRequestTestSuite))
}
