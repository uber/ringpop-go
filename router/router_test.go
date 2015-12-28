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
	"github.com/uber/ringpop-go/swim"
	"github.com/uber/ringpop-go/test/mocks"
	"github.com/uber/tchannel-go"
)

func TestRingpopRouterGetLocalClient(t *testing.T) {
	cf := &mocks.ClientFactory{}
	cf.On("GetLocalClient").Return(nil)

	rp := &mocks.Ringpop{}
	rp.On("RegisterListener", mock.Anything).Return()
	rp.On("Lookup", "hello").Return("127.0.0.1:3000", nil)
	rp.On("WhoAmI").Return("127.0.0.1:3000", nil)

	router := New(rp, cf, nil)

	router.GetClient("hello")
	cf.AssertCalled(t, "GetLocalClient")
}

func TestRingpopRouterGetLocalClientCached(t *testing.T) {
	cf := &mocks.ClientFactory{}
	cf.On("GetLocalClient").Return(nil)

	rp := &mocks.Ringpop{}
	rp.On("RegisterListener", mock.Anything).Return()
	rp.On("Lookup", "hello").Return("127.0.0.1:3000", nil)
	rp.On("WhoAmI").Return("127.0.0.1:3000", nil)

	router := New(rp, cf, nil)

	router.GetClient("hello")
	cf.AssertCalled(t, "GetLocalClient")

	router.GetClient("hello")
	cf.AssertNumberOfCalls(t, "GetLocalClient", 1)
}

func TestRingpopRouterMakeRemoteClient(t *testing.T) {
	ch, err := tchannel.NewChannel("remote", nil)
	if err != nil {
		panic(err)
	}

	cf := &mocks.ClientFactory{}
	cf.On("MakeRemoteClient", mock.Anything).Return(nil)

	rp := &mocks.Ringpop{}
	rp.On("RegisterListener", mock.Anything).Return()
	rp.On("Lookup", "hello").Return("127.0.0.1:3001", nil)
	rp.On("WhoAmI").Return("127.0.0.1:3000", nil)

	router := New(rp, cf, ch)

	router.GetClient("hello")
	cf.AssertCalled(t, "MakeRemoteClient", mock.Anything)
}

func TestRingpopRouterMakeRemoteClientCached(t *testing.T) {
	ch, err := tchannel.NewChannel("remote", nil)
	if err != nil {
		panic(err)
	}

	cf := &mocks.ClientFactory{}
	cf.On("MakeRemoteClient", mock.Anything).Return(nil)

	rp := &mocks.Ringpop{}
	rp.On("RegisterListener", mock.Anything).Return()
	rp.On("Lookup", "hello").Return("127.0.0.1:3001", nil)
	rp.On("WhoAmI").Return("127.0.0.1:3000", nil)

	router := New(rp, cf, ch)

	router.GetClient("hello")
	cf.AssertCalled(t, "MakeRemoteClient", mock.Anything)

	router.GetClient("hello")
	cf.AssertNumberOfCalls(t, "MakeRemoteClient", 1)
}

func TestRingpopRouterRemoveExistingClient(t *testing.T) {
	rp := &mocks.Ringpop{}
	rp.On("RegisterListener", mock.Anything).Return()

	router := New(rp, nil, nil).(*router)

	hostport := "127.0.0.1:3000"
	router.clientCache[hostport] = "Some client"
	assert.Equal(t, "Some client", router.clientCache[hostport])

	router.removeClient(hostport)
	assert.Equal(t, nil, router.clientCache[hostport])
}

func TestRingpopRouterRemoveNonExistingClient(t *testing.T) {
	rp := &mocks.Ringpop{}
	rp.On("RegisterListener", mock.Anything).Return()

	router := New(rp, nil, nil).(*router)

	hostport := "127.0.0.1:3000"
	assert.Equal(t, nil, router.clientCache[hostport])
	router.removeClient(hostport)
	assert.Equal(t, nil, router.clientCache[hostport])
}

func TestRingpopRouterRemoveClientOnSwimFaultyEvent(t *testing.T) {
	rp := &mocks.Ringpop{}
	rp.On("RegisterListener", mock.Anything).Return()

	router := New(rp, nil, nil).(*router)

	hostport := "127.0.0.1:3000"
	router.clientCache[hostport] = "Some client"
	assert.Equal(t, "Some client", router.clientCache[hostport])

	router.HandleEvent(swim.MemberlistChangesReceivedEvent{
		Changes: []swim.Change{
			swim.Change{
				Address: hostport,
				Status:  swim.Faulty,
			},
		},
	})

	assert.Equal(t, nil, router.clientCache[hostport])
}

func TestRingpopRouterRemoveClientOnSwimLeaveEvent(t *testing.T) {
	rp := &mocks.Ringpop{}
	rp.On("RegisterListener", mock.Anything).Return()

	router := New(rp, nil, nil).(*router)

	hostport := "127.0.0.1:3000"
	router.clientCache[hostport] = "Some client"
	assert.Equal(t, "Some client", router.clientCache[hostport])

	router.HandleEvent(swim.MemberlistChangesReceivedEvent{
		Changes: []swim.Change{
			swim.Change{
				Address: hostport,
				Status:  swim.Leave,
			},
		},
	})

	assert.Equal(t, nil, router.clientCache[hostport])
}

func TestRingpopRouterNotRemoveClientOnSwimSuspectEvent(t *testing.T) {
	rp := &mocks.Ringpop{}
	rp.On("RegisterListener", mock.Anything).Return()

	router := New(rp, nil, nil).(*router)

	hostport := "127.0.0.1:3000"
	router.clientCache[hostport] = "Some client"
	assert.Equal(t, "Some client", router.clientCache[hostport])

	router.HandleEvent(swim.MemberlistChangesReceivedEvent{
		Changes: []swim.Change{
			swim.Change{
				Address: hostport,
				Status:  swim.Suspect,
			},
		},
	})

	assert.Equal(t, "Some client", router.clientCache[hostport])
}

func TestRingpopRouterGetClientForwardLookupError(t *testing.T) {
	cf := &mocks.ClientFactory{}
	cf.On("GetLocalClient").Return(nil)

	stubError := errors.New("Ringpop not started")

	rp := &mocks.Ringpop{}
	rp.On("RegisterListener", mock.Anything).Return()
	rp.On("Lookup", "hello").Return("", stubError)
	rp.On("WhoAmI").Return("127.0.0.1:3000", nil)

	router := New(rp, cf, nil)

	_, err := router.GetClient("hello")
	assert.Equal(t, errors.New("Ringpop not started"), err)
}

func TestRingpopRouterGetClientForwardWhoAmIError(t *testing.T) {
	cf := &mocks.ClientFactory{}
	cf.On("GetLocalClient").Return(nil)

	stubError := errors.New("Ringpop not started")

	rp := &mocks.Ringpop{}
	rp.On("RegisterListener", mock.Anything).Return()
	rp.On("Lookup", "hello").Return("127.0.0.1:3000", nil)
	rp.On("WhoAmI").Return("", stubError)

	router := New(rp, cf, nil)

	_, err := router.GetClient("hello")
	assert.Equal(t, errors.New("Ringpop not started"), err)
}
