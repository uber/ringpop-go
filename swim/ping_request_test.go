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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"github.com/uber/ringpop-go/events"
	"github.com/uber/ringpop-go/util"
	"github.com/uber/tchannel-go"
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
	waitForConvergence(s.T(), 100, s.tnode, s.peers[0])

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
	waitForConvergence(s.T(), 100, s.tnode, s.peers[0])

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

// When testing the indirectPing, there are a lot of different ping-req
// scenarios. There are four identifiable scenarios where the target is
// reachable:
// 1. Target is reached by one node and the other node blocks;
// 2. Target is not-ready for first ping-req, reachable for the second;
// 3. Target is unreachable for first ping-req, reachable for the second;
// 4. Connection error to helper node, the second helper reaches the target.

// There are two cases in which the target is definitely not reachable:
// 5. all ping-reqs report that the target is not-ready;
// 6. all ping-reqs report that the target in unreachable.

// The indirectPing is said to be inconclusive when all the helper nodes
// are unreachable.
// 7. Connection errors from all nodes that perform the pings.

// When the first ping-req with an ok status, the indirectPing
// immediately returns that it has reached the target. Even though the
// status of the second ping is unknown.
func TestIndirectPing1(t *testing.T) {
	block := make(chan bool)

	tnodes := genChannelNodes(t, 4)
	defer destroyNodes(tnodes...)

	sender, helper1, helper2, target := tnodes[0], tnodes[1], tnodes[2], tnodes[3]
	bootstrapNodes(t, sender, helper1, helper2, target)
	waitForConvergence(t, 100, sender, helper1, helper2, target)

	targetHostPort := target.node.Address()

	// block one ping-req indefinitely
	helper1.node.AddListener(onPingRequestReceive(func() {
		<-block
	}))

	reached, errs := indirectPing(sender.node, targetHostPort, 2, time.Second)
	assert.True(t, reached, "expected that target is reached")
	assert.Len(t, errs, 0, "expected no errors from the helper nodes")
	close(block)
}

// First ping the target while it is not-ready, on second ping the
// target is bootstrapped, and the ping-req reports with an ok status.
// The indirectPing returns that it has reached the target.
func TestIndirectPing2(t *testing.T) {
	cont := make(chan bool, 1)

	tnodes := genChannelNodes(t, 4)
	defer func() {
		destroyNodes(tnodes...)
	}()

	// don't bootstrap the target
	sender, helper1, helper2, target := tnodes[0], tnodes[1], tnodes[2], tnodes[3]
	bootstrapNodes(t, sender, helper1, helper2)
	waitForConvergence(t, 100, sender, helper1, helper2)

	tnodes = append(tnodes, newChannelNode(t))
	targetHostPort := target.node.Address()

	// block ping-req until first ping-req is done
	helper1.node.AddListener(onPingRequestReceive(func() {
		<-cont
	}))

	// bootstrap node and then unblock second ping-req
	sender.node.AddListener(onPingRequestComplete(func() {
		// bootstrap target node
		bootstrapNodes(t, sender, helper1, helper2, target)
		cont <- true
	}))

	reached, errs := indirectPing(sender.node, targetHostPort, 2, time.Second)
	assert.True(t, reached, "expected that target is reached")
	assert.Len(t, errs, 0, "expected no errors from the helper nodes")
}

// First ping the target while it responds with a tchannel error.
// On the second ping the target is bootstrapped and the ping-req reports with
// an ok status. The indirectPing returns that it has reached the target
func TestIndirectPing3(t *testing.T) {
	cont := make(chan bool, 1)

	tnodes := genChannelNodes(t, 4)
	defer destroyNodes(tnodes...)

	// don't create target yet
	sender, helper1, helper2 := tnodes[0], tnodes[1], tnodes[2]
	bootstrapNodes(t, sender, helper1, helper2)
	waitForConvergence(t, 100, sender, helper1, helper2)

	ch, err := tchannel.NewChannel("test", nil)
	assert.NoError(t, err, "expected to setup tchannel")
	err = ch.ListenAndServe("127.0.0.1:0")
	assert.NoError(t, err, "expected to listen on channel")
	targetHostPort := ch.PeerInfo().HostPort

	// block ping-req until first ping-req is done
	helper1.node.AddListener(onPingRequestReceive(func() {
		<-cont
	}))

	// initialize target and unblock second ping-req
	sender.node.AddListener(onPingRequestComplete(func() {
		// create and add target to cluster
		targetNode := NewNode("test", targetHostPort, ch.GetSubChannel("test"), nil)
		target := &testNode{targetNode, ch, nil}
		tnodes = append(tnodes, target)
		bootstrapNodes(t, sender, helper1, helper2, target)
		cont <- true
	}))

	reached, errs := indirectPing(sender.node, targetHostPort, 2, time.Second)
	assert.True(t, reached, "expected that target is reached")
	assert.Len(t, errs, 0, "expected no errors from the helper nodes")
}

// The first ping-req doesn't reach the helper node. The second ping-req
// does and the ping reached the target. The indirectPing therefore returns
// that it has successfully reached the target.
func TestIndirectPing4(t *testing.T) {
	cont := make(chan bool, 1)

	tnodes := genChannelNodes(t, 4)
	defer destroyNodes(tnodes...)

	sender, helper1, helper2, target := tnodes[0], tnodes[1], tnodes[2], tnodes[3]
	bootstrapNodes(t, sender, helper1, helper2, target)
	waitForConvergence(t, 100, sender, helper1, helper2, target)

	targetHostPort := target.node.Address()

	helper2.closeAndWait(sender.channel)

	// block ping-req of healthy node
	helper1.node.AddListener(onPingRequestReceive(func() {
		<-cont
	}))

	// first ping-req has failed because helper2 is closed, unblock second ping-req
	sender.node.AddListener(onPingRequestComplete(func() {
		cont <- true
	}))

	reached, errs := indirectPing(sender.node, targetHostPort, 2, time.Second)
	assert.True(t, reached, "expected that target is reached")
	assert.Len(t, errs, 1, "expected one connection error from the helper nodes")
}

// Test where target is not bootstrapped and responds with not-readies,
// indirectPing cannot reach the target and returns false.
func TestIndirectPing5(t *testing.T) {
	tnodes := genChannelNodes(t, 4)
	defer destroyNodes(tnodes...)

	// don't bootstrap the target
	sender, helper1, helper2, target := tnodes[0], tnodes[1], tnodes[2], tnodes[3]
	bootstrapNodes(t, sender, helper1, helper2)
	waitForConvergence(t, 100, sender, helper1, helper2)

	// Add an bootstrapped node.
	targetHostPort := target.node.Address()

	reached, errs := indirectPing(sender.node, targetHostPort, 2, time.Second)
	assert.False(t, reached, "expected that target is unreachable")
	assert.Len(t, errs, 0, "expected no errors from the helper nodes")
}

// Test where target node is unreachable, indirectPing is negative and
// returns false.
func TestIndirectPing6(t *testing.T) {
	tnodes := genChannelNodes(t, 4)
	defer destroyNodes(tnodes...)

	sender, helper1, helper2, target := tnodes[0], tnodes[1], tnodes[2], tnodes[3]
	bootstrapNodes(t, sender, helper1, helper2, target)
	waitForConvergence(t, 100, sender, helper1, helper2, target)

	targetHostPort := target.node.Address()
	target.closeAndWait(sender.channel)

	reached, errs := indirectPing(sender.node, targetHostPort, 2, time.Second)
	assert.False(t, reached, "expected that target is not reached")
	assert.Len(t, errs, 0, "expected no errors from the helper nodes")
}

// Test where helper nodes are unreachable, indirectPing is inconclusive
// and returns false to indicate that the target hasn't been reached.
func TestIndirectPing7(t *testing.T) {
	tnodes := genChannelNodes(t, 4)
	defer destroyNodes(tnodes...)

	bootstrapNodes(t, tnodes...)
	waitForConvergence(t, 100, tnodes...)
	sender, helper1, helper2, target := tnodes[0], tnodes[1], tnodes[2], tnodes[3]

	targetHostPort := target.node.Address()
	helper1.closeAndWait(sender.channel)
	helper2.closeAndWait(sender.channel)

	reached, errs := indirectPing(sender.node, targetHostPort, 2, time.Second)
	assert.False(t, reached, "expected that target is unreachable")
	assert.Len(t, errs, 2, "expected only errors from the helper nodes")
}

// onPingRequestComplete can be registered on an EventListener and fires the
// specified function only when the PingRequestSender receives a response or
// error.
func onPingRequestComplete(f func()) *ListenerFunc {
	return &ListenerFunc{
		fn: func(e events.Event) {
			switch e.(type) {
			case PingRequestsSendCompleteEvent, PingRequestSendErrorEvent:
				f()
			}
		},
	}
}

// onPingRequestReceive can be registered on an EventListener and fires f only
// when the PingRequest handler receives a request.
func onPingRequestReceive(f func()) *ListenerFunc {
	return &ListenerFunc{
		fn: func(e events.Event) {
			switch e.(type) {
			case PingRequestReceiveEvent:
				f()
			}
		},
	}
}
