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
package router

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/uber/ringpop-go/swim"
	"github.com/uber/ringpop-go/test/mocks"
	"github.com/uber/tchannel-go"
)

type RouterTestSuite struct {
	suite.Suite
	router        Router
	internal      *router
	ringpop       *mocks.Ringpop
	clientFactory *mocks.ClientFactory
}

func (s *RouterTestSuite) SetupTest() {
	s.clientFactory = &mocks.ClientFactory{}
	s.clientFactory.On("GetLocalClient").Return("local client")
	s.clientFactory.On("MakeRemoteClient", mock.Anything).Return("remote client")

	s.ringpop = &mocks.Ringpop{}
	s.ringpop.On("AddListener", mock.Anything).Return(false)
	s.ringpop.On("WhoAmI").Return("127.0.0.1:3000", nil)
	s.ringpop.On("Lookup", "local").Return("127.0.0.1:3000", nil)
	s.ringpop.On("Lookup", "local2").Return("127.0.0.1:3000", nil)
	s.ringpop.On("Lookup", "remote").Return("127.0.0.1:3001", nil)
	s.ringpop.On("Lookup", "remote2").Return("127.0.0.1:3001", nil)
	s.ringpop.On("Lookup", "error").Return("", errors.New("ringpop not ready"))

	s.ringpop.On("LookupN", "localfirst", 2).Return([]string{"127.0.0.1:3000", "127.0.0.1:3001"}, nil)
	s.ringpop.On("LookupN", "remotefirst", 2).Return([]string{"127.0.0.1:3001", "127.0.0.1:3000"}, nil)

	ch, err := tchannel.NewChannel("remote", nil)
	s.NoError(err)

	s.router = New(s.ringpop, s.clientFactory, ch)
	s.internal = s.router.(*router)
}

func (s *RouterTestSuite) TestRingpopRouterGetLocalClient() {
	client, isRemote, err := s.router.GetClient("local")
	s.NoError(err)
	s.Equal("local client", client)
	s.False(isRemote, "local key should not be a remote client")
	s.clientFactory.AssertCalled(s.T(), "GetLocalClient")
}

func (s *RouterTestSuite) TestRingpopRouterGetNClientsLocalFirst() {
	clients, err := s.router.GetNClients("localfirst", 2)
	s.NoError(err)

	s.Equal("local client", clients[0].Client)
	s.Equal("remote client", clients[1].Client)

	s.False(clients[0].IsRemote, "first client for localfirst key should not be a remote client")
	s.True(clients[1].IsRemote, "second client for localfirst key should not be a local client")
}

func (s *RouterTestSuite) TestRingpopRouterGetNClientsRemoteFirst() {
	clients, err := s.router.GetNClients("remotefirst", 2)
	s.NoError(err)

	s.Equal("remote client", clients[0].Client)
	s.Equal("local client", clients[1].Client)

	s.True(clients[0].IsRemote, "first client for localfirst key should not be a local client")
	s.False(clients[1].IsRemote, "second client for localfirst key should not be a remote client")
}

func (s *RouterTestSuite) TestRingpopRouterGetLocalClientCached() {
	client, isRemote, err := s.router.GetClient("local")
	s.NoError(err)
	s.Equal("local client", client)
	s.False(isRemote, "local key should not be a remote client")
	s.clientFactory.AssertCalled(s.T(), "GetLocalClient")

	client, isRemote, err = s.router.GetClient("local")
	s.NoError(err)
	s.Equal("local client", client)
	s.False(isRemote, "local key should not be a remote client (cached)")
	s.clientFactory.AssertNumberOfCalls(s.T(), "GetLocalClient", 1)
}

func (s *RouterTestSuite) TestRingpopRouterGetLocalClientCachedWithDifferentKey() {
	client, isRemote, err := s.router.GetClient("local")
	s.NoError(err)
	s.Equal("local client", client)
	s.False(isRemote, "local key should not be a remote client")
	s.clientFactory.AssertCalled(s.T(), "GetLocalClient")

	client, isRemote, err = s.router.GetClient("local2")
	s.NoError(err)
	s.Equal("local client", client)
	s.False(isRemote, "local key should not be a remote client (cached)")
	s.clientFactory.AssertNumberOfCalls(s.T(), "GetLocalClient", 1)
}

func (s *RouterTestSuite) TestRingpopRouterMakeRemoteClient() {
	client, isRemote, err := s.router.GetClient("remote")
	s.NoError(err)
	s.Equal("remote client", client, "expected the remote client to be returned")
	s.True(isRemote, "remote key should be a remote client")
	s.clientFactory.AssertCalled(s.T(), "MakeRemoteClient", mock.Anything)
}

func (s *RouterTestSuite) TestRingpopRouterMakeRemoteClientCached() {
	client, isRemote, err := s.router.GetClient("remote")
	s.NoError(err)
	s.Equal("remote client", client)
	s.True(isRemote, "remote key should be a remote client")
	s.clientFactory.AssertCalled(s.T(), "MakeRemoteClient", mock.Anything)

	client, isRemote, err = s.router.GetClient("remote")
	s.NoError(err)
	s.Equal("remote client", client)
	s.True(isRemote, "remote key should be a remote client (cached)")
	s.clientFactory.AssertNumberOfCalls(s.T(), "MakeRemoteClient", 1)
}

func (s *RouterTestSuite) TestRingpopRouterMakeRemoteClientCachedWithDifferentKey() {
	client, isRemote, err := s.router.GetClient("remote")
	s.NoError(err)
	s.Equal("remote client", client)
	s.True(isRemote, "remote key should be a remote client")
	s.clientFactory.AssertCalled(s.T(), "MakeRemoteClient", mock.Anything)

	client, isRemote, err = s.router.GetClient("remote2")
	s.NoError(err)
	s.Equal("remote client", client)
	s.True(isRemote, "remote key should be a remote client (cached)")
	s.clientFactory.AssertNumberOfCalls(s.T(), "MakeRemoteClient", 1)
}

func (s *RouterTestSuite) TestRingpopRouterRemoveExistingClient() {
	// populate cache
	_, _, err := s.router.GetClient("local")
	s.NoError(err)
	s.clientFactory.AssertNumberOfCalls(s.T(), "GetLocalClient", 1)

	// find the node that the client has been cached for so we can remove the entry
	dest, err := s.ringpop.Lookup("local")
	s.NoError(err)
	s.internal.removeClient(dest)

	// repopulate and test if a new client has been created or that the cache stayed alive
	_, _, err = s.router.GetClient("local")
	s.NoError(err)
	s.clientFactory.AssertNumberOfCalls(s.T(), "GetLocalClient", 2)
}

func (s *RouterTestSuite) TestRingpopRouterRemoveNonExistingClient() {
	dest, err := s.ringpop.Lookup("local")
	s.NoError(err)
	s.internal.removeClient(dest)

	// make sure no calls are made to the client factory during cache eviction
	s.clientFactory.AssertNotCalled(s.T(), "GetLocalClient")
	s.clientFactory.AssertNotCalled(s.T(), "MakeRemoteClient")
}

func (s *RouterTestSuite) TestRingpopRouterRemoveClientOnSwimFaultyEvent() {
	// populate cache
	_, _, err := s.router.GetClient("local")
	s.NoError(err)
	s.clientFactory.AssertNumberOfCalls(s.T(), "GetLocalClient", 1)

	dest, err := s.ringpop.Lookup("local")
	s.NoError(err)
	s.internal.HandleEvent(swim.MemberlistChangesReceivedEvent{
		Changes: []swim.Change{
			{
				Address: dest,
				Status:  swim.Faulty,
			},
		},
	})

	// repopulate and test if a new client has been created or that the cache stayed alive
	_, _, err = s.router.GetClient("local")
	s.NoError(err)
	s.clientFactory.AssertNumberOfCalls(s.T(), "GetLocalClient", 2)
}

func (s *RouterTestSuite) TestRingpopRouterRemoveClientOnSwimLeaveEvent() {
	// populate cache
	_, _, err := s.router.GetClient("local")
	s.NoError(err)
	s.clientFactory.AssertNumberOfCalls(s.T(), "GetLocalClient", 1)

	dest, err := s.ringpop.Lookup("local")
	s.NoError(err)
	s.internal.HandleEvent(swim.MemberlistChangesReceivedEvent{
		Changes: []swim.Change{
			{
				Address: dest,
				Status:  swim.Leave,
			},
		},
	})

	// repopulate and test if a new client has been created or that the cache stayed alive
	_, _, err = s.router.GetClient("local")
	s.NoError(err)
	s.clientFactory.AssertNumberOfCalls(s.T(), "GetLocalClient", 2)
}

func (s *RouterTestSuite) TestRingpopRouterNotRemoveClientOnSwimSuspectEvent() {
	// populate cache
	_, _, err := s.router.GetClient("local")
	s.NoError(err)
	s.clientFactory.AssertNumberOfCalls(s.T(), "GetLocalClient", 1)

	dest, err := s.ringpop.Lookup("local")
	s.NoError(err)
	s.internal.HandleEvent(swim.MemberlistChangesReceivedEvent{
		Changes: []swim.Change{
			{
				Address: dest,
				Status:  swim.Suspect,
			},
		},
	})

	// repopulate and test if a new client has been created or that the cache stayed alive
	_, _, err = s.router.GetClient("local")
	s.NoError(err)
	s.clientFactory.AssertNumberOfCalls(s.T(), "GetLocalClient", 1)
}

func (s *RouterTestSuite) TestRingpopRouterGetClientForwardLookupError() {
	_, _, err := s.router.GetClient("error")
	s.EqualError(err, "ringpop not ready")
}

func TestRingpopRouterGetClientForwardWhoAmIError(t *testing.T) {
	cf := &mocks.ClientFactory{}
	cf.On("GetLocalClient").Return(nil)

	stubError := errors.New("ringpop not ready")

	rp := &mocks.Ringpop{}
	rp.On("AddListener", mock.Anything).Return(false)
	rp.On("Lookup", "hello").Return("127.0.0.1:3000", nil)
	rp.On("WhoAmI").Return("", stubError)

	router := New(rp, cf, nil)

	_, _, err := router.GetClient("hello")
	assert.EqualError(t, err, "ringpop not ready")
}

func TestRouterTestSuite(t *testing.T) {
	suite.Run(t, new(RouterTestSuite))
}
