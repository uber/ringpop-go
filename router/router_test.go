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
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/uber/ringpop-go/test/mocks"
	"github.com/uber/tchannel-go"
)

// ringpop.Interface compatible struct with stubbed methods
func TestRingpopRouterGetLocalClient(t *testing.T) {
	// setup test
	cf := &mocks.ClientFactory{}
	cf.On("GetLocalClient").Return(nil)

	rp := &mocks.Ringpop{}
	rp.On("RegisterListener", mock.Anything).Return()
	rp.On("Lookup", "hello").Return("127.0.0.1:3000")
	rp.On("WhoAmI").Return("127.0.0.1:3000")

	router := NewRouter(rp, cf, nil)

	router.GetClient("hello")
	cf.AssertCalled(t, "GetLocalClient")
}

func TestRingpopRouterGetLocalClientCached(t *testing.T) {
	// setup test
	cf := &mocks.ClientFactory{}
	cf.On("GetLocalClient").Return(nil)

	rp := &mocks.Ringpop{}
	rp.On("RegisterListener", mock.Anything).Return()
	rp.On("Lookup", "hello").Return("127.0.0.1:3000")
	rp.On("WhoAmI").Return("127.0.0.1:3000")

	router := NewRouter(rp, cf, nil)

	router.GetClient("hello")
	cf.AssertCalled(t, "GetLocalClient")

	router.GetClient("hello")
	cf.AssertNumberOfCalls(t, "GetLocalClient", 1)
}

func TestRingpopRouterMakeRemoteClient(t *testing.T) {
	// setup test
	ch, err := tchannel.NewChannel("remote", nil)
	if err != nil {
		panic(err)
	}

	// setup test
	cf := &mocks.ClientFactory{}
	cf.On("MakeRemoteClient", mock.Anything).Return(nil)

	rp := &mocks.Ringpop{}
	rp.On("RegisterListener", mock.Anything).Return()
	rp.On("Lookup", "hello").Return("127.0.0.1:3001")
	rp.On("WhoAmI").Return("127.0.0.1:3000")

	router := NewRouter(rp, cf, ch)

	router.GetClient("hello")
	cf.AssertCalled(t, "MakeRemoteClient", mock.Anything)
}

func TestRingpopRouterMakeRemoteClientCached(t *testing.T) {
	// setup test
	ch, err := tchannel.NewChannel("remote", nil)
	if err != nil {
		panic(err)
	}

	// setup test
	cf := &mocks.ClientFactory{}
	cf.On("MakeRemoteClient", mock.Anything).Return(nil)

	rp := &mocks.Ringpop{}
	rp.On("RegisterListener", mock.Anything).Return()
	rp.On("Lookup", "hello").Return("127.0.0.1:3001")
	rp.On("WhoAmI").Return("127.0.0.1:3000")

	router := NewRouter(rp, cf, ch)

	router.GetClient("hello")
	cf.AssertCalled(t, "MakeRemoteClient", mock.Anything)

	router.GetClient("hello")
	cf.AssertNumberOfCalls(t, "MakeRemoteClient", 1)
}
