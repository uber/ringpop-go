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
	"errors"
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/benbjohnson/clock"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/uber-common/bark"
	"github.com/uber/ringpop-go/logging"
	"github.com/uber/ringpop-go/shared"
	"github.com/uber/ringpop-go/swim/test/mocks"
	"github.com/uber/tchannel-go"
)

type HandlerTestSuite struct {
	suite.Suite

	mockClock *clock.Mock
	testNode  *testNode
	cluster   *swimCluster

	ctx       tchannel.ContextWithHeaders
	ctxCancel context.CancelFunc
}

func (s *HandlerTestSuite) SetupTest() {
	// Create a test node, on its own. Tests can join this to the cluster for
	// testing, if they want.
	s.testNode = newChannelNode(s.T())
	s.NotNil(s.testNode.node.clock)
	s.mockClock = s.testNode.node.clock.(*clock.Mock)
	s.NotNil(s.mockClock)

	// Create a cluster for testing. Join these guys to each other.
	s.cluster = newSwimCluster(4)
	s.cluster.Bootstrap()

	s.ctx, s.ctxCancel = shared.NewTChannelContext(500 * time.Millisecond)
}

func (s *HandlerTestSuite) TearDownTest() {
	s.ctxCancel()

	if s.cluster != nil {
		s.cluster.Destroy()
	}
}

func (s *HandlerTestSuite) TestGossipStartHandler() {
	s.testNode.node.gossip.Stop()
	s.Require().True(s.testNode.node.gossip.Stopped())

	_, err := s.testNode.node.gossipHandlerStart(s.ctx, &emptyArg{})
	s.NoError(err, "calling handler should not result in error")

	s.False(s.testNode.node.gossip.Stopped())
}

func (s *HandlerTestSuite) TestGossipStopHandler() {
	s.testNode.node.gossip.Start()
	s.Require().False(s.testNode.node.gossip.Stopped())

	_, err := s.testNode.node.gossipHandlerStop(s.ctx, &emptyArg{})
	s.NoError(err, "calling handler should not result in error")

	s.True(s.testNode.node.gossip.Stopped())
}

func (s *HandlerTestSuite) TestToggleGossipHandler() {
	s.Require().True(s.testNode.node.gossip.Stopped())

	var err error

	// Toggle gossip on
	_, err = s.testNode.node.gossipHandler(s.ctx, &emptyArg{})
	s.NoError(err, "calling handler should not result in error")
	s.False(s.testNode.node.gossip.Stopped())

	// Toggle gossip off
	_, err = s.testNode.node.gossipHandler(s.ctx, &emptyArg{})
	s.NoError(err, "calling handler should not result in error")
	s.True(s.testNode.node.gossip.Stopped())
}

func (s *HandlerTestSuite) TestAdminLeaveJoinHandlers() {
	// Join test node to cluster
	s.cluster.Add(s.testNode.node)
	s.Require().Equal(5, s.testNode.node.CountReachableMembers(), "expect a cluster of 5")

	var err error
	var status *Status

	// Test leave handler works correctly
	status, err = s.testNode.node.adminLeaveHandler(s.ctx, &emptyArg{})
	s.NoError(err, "calling handler should not result in error")
	s.Equal(&Status{Status: "ok"}, status)
	s.Equal(4, s.testNode.node.CountReachableMembers())

	// Test join handler brings it back to 5
	s.NotNil(s.mockClock)
	s.mockClock.Add(time.Millisecond)
	status, err = s.testNode.node.adminJoinHandler(s.ctx, &emptyArg{})
	s.NoError(err, "calling handler should not result in error")
	s.Equal(&Status{Status: "rejoined"}, status)
	s.Equal(5, s.testNode.node.CountReachableMembers())
}

// TestRegisterHandlers tests that registerHandler always succeeds.
func (s *HandlerTestSuite) TestRegisterHandlers() {
	s.NoError(s.testNode.node.registerHandlers())
}

func (s *HandlerTestSuite) TestNotImplementedHandler() {
	res, err := notImplementedHandler(s.ctx, &emptyArg{})
	s.EqualError(err, "handler not implemented")
	s.Nil(res)
}

// TestErrorHandler tests that the errorHandler logs the correct error message.
func (s *HandlerTestSuite) TestErrorHandler() {
	logger := &mocks.Logger{}

	// Make sure random log messages can be sent to this logger while it
	// replaces the existing one.
	logger.On("WithField", mock.Anything).Return(logger)
	logger.On("WithFields", mock.Anything).Return(logger)
	for _, meth := range []string{"Debug", "Info", "Warn", "Error"} {
		// Debug(msg)
		logger.On(meth, mock.Anything)
		// Debugf(format, msg)
		logger.On(meth+"f", mock.Anything, mock.Anything)
	}

	errTest := errors.New("test error")
	logging.SetLogger(logger)

	s.testNode.node.errorHandler(s.ctx, errTest)
	logger.AssertCalled(s.T(), "WithFields", bark.Fields{
		"local": s.testNode.node.Address(),
		"error": errTest,
	})
	logger.AssertCalled(s.T(), "Info", []interface{}{"error occurred"})
}

func (s *HandlerTestSuite) TestTickHandler() {
	s.testNode.node.Stop()

	ping, err := s.testNode.node.tickHandler(s.ctx, &emptyArg{})
	s.NoError(err, "calling handler should not result in error")

	s.Equal(s.testNode.node.memberlist.Checksum(), ping.Checksum, "checksum should be returned by tickHandler")
}

// TestJoinHandlerError tests that the join handler returns an error if the
// join request is invalid.
func (s *HandlerTestSuite) TestJoinHandlerError() {
	// Construct an invalid joinRequest
	req := &joinRequest{}

	res, err := s.testNode.node.joinHandler(s.ctx, req)
	s.Nil(res)
	s.Error(err)
}

func (s *HandlerTestSuite) TestPingRequestHandler() {
	// Bootstrap the test node, so it's ready to receive pings
	node := s.testNode.node
	node.Bootstrap(&BootstrapOptions{
		DiscoverProvider: &StaticHostList{[]string{node.Address()}},
	})

	req := &pingRequest{
		Source:            node.Address(),
		SourceIncarnation: node.Incarnation(),
		Checksum:          node.memberlist.Checksum(),
		Changes:           nil,
		Target:            node.Address(),
	}

	res, err := node.pingRequestHandler(s.ctx, req)

	s.NoError(err)
	s.True(res.Ok, "expected ok response from ping")
}

func (s *HandlerTestSuite) TestPingRequestHandlerFail() {
	// Node is not ready to receive pings when it's not bootstrapped
	node := s.testNode.node
	req := &pingRequest{
		Source:            node.Address(),
		SourceIncarnation: node.Incarnation(),
		Checksum:          node.memberlist.Checksum(),
		Changes:           nil,
		Target:            node.Address(),
	}

	res, err := node.pingRequestHandler(s.ctx, req)

	s.Error(err)
	s.Nil(res)
}

func TestHandlerTestSuite(t *testing.T) {
	suite.Run(t, new(HandlerTestSuite))
}
