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
)

type PingTestSuite struct {
	suite.Suite
	tnode, tpeer *testNode
	node, peer   *Node
}

func (s *PingTestSuite) SetupTest() {
	s.tnode = newChannelNode(s.T(), "127.0.0.1:3001")
	s.node = s.tnode.node
	s.tpeer = newChannelNode(s.T(), "127.0.0.1:3002")
	s.peer = s.tpeer.node

	bootstrapNodes(s.T(), s.tnode, s.tpeer)
}

func (s *PingTestSuite) TearDownTest() {
	destroyNodes(s.tnode, s.tpeer)
}

func (s *PingTestSuite) TestPing() {
	res, err := sendPing(s.node, s.peer.Address(), time.Second)
	s.NoError(err, "expected a ping to succeed")
	s.NotNil(res, "expected a ping response")
}

func (s *PingTestSuite) TestPingFails() {
	res, err := sendPing(s.node, "127.0.0.1:3003", time.Second)
	s.Error(err, "expected ping to pail")
	s.Nil(res, "expected response to be nil")
}

func (s *PingTestSuite) TestPingTimesOut() {
	res, err := sendPing(s.node, "127.0.0.2:3001", time.Millisecond)
	s.Error(err, "expected ping to pail")
	s.Nil(res, "expected response to be nil")
}

func TestPingTestSuite(t *testing.T) {
	suite.Run(t, new(PingTestSuite))
}
