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

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/uber/ringpop-go/events/test/mocks"
	"github.com/uber/tchannel-go"
)

type PingTestSuite struct {
	suite.Suite
	tnode, tpeer *testNode
	node, peer   *Node
}

func (s *PingTestSuite) SetupSuite() {
	s.tnode = newChannelNode(s.T())
	s.node = s.tnode.node
	s.tpeer = newChannelNode(s.T())
	s.peer = s.tpeer.node

	bootstrapNodes(s.T(), s.tnode, s.tpeer)
}

func (s *PingTestSuite) TearDownSuite() {
	destroyNodes(s.tnode, s.tpeer)
}

func (s *PingTestSuite) TestPing() {
	res, err := sendPing(s.node, s.peer.Address(), time.Second)
	s.NoError(err, "expected a ping to succeed")
	s.NotNil(res, "expected a ping response")
}

func (s *PingTestSuite) TestPingFails() {
	// Create a channel with no handlers registered. Any requests to this
	// channel should result in an error being returned immediately.
	ch, err := tchannel.NewChannel("test", nil)
	ch.ListenAndServe("127.0.0.1:0")
	s.Require().NoError(err, "channel must create successfully")

	res, err := sendPing(s.node, ch.PeerInfo().HostPort, time.Second)
	s.Error(err, "expected ping to fail")
	s.Nil(res, "expected response to be nil")
}

func (s *PingTestSuite) TestPingTimesOut() {
	// Set the timeout so low that a ping response could never come back before
	// the timeout is reached.
	res, err := sendPing(s.node, s.peer.Address(), time.Nanosecond)
	s.Error(err, "expected ping to fail")
	s.Nil(res, "expected response to be nil")
}

func (s *PingTestSuite) TestPingBeforeReady() {
	testNode1 := newChannelNode(s.T())
	defer testNode1.Destroy()

	testNode2 := newChannelNode(s.T())
	defer testNode2.Destroy()

	// Register listener that should be fired with the correct event type
	listener := &mocks.EventListener{}
	listener.On("HandleEvent", mock.AnythingOfType("RequestBeforeReadyEvent"))
	testNode2.node.AddListener(listener)

	res, err := sendPing(testNode1.node, testNode2.node.Address(), 500*time.Millisecond)
	s.Error(err)
	s.Nil(res)

	// Assert the event was fired, but after a brief sleep, because the events
	// are fired in goroutines
	time.Sleep(10 * time.Millisecond)
	listener.AssertExpectations(s.T())
}

func TestPingTestSuite(t *testing.T) {
	suite.Run(t, new(PingTestSuite))
}
