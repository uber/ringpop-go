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

package router

import (
	"sync"

	"github.com/uber/ringpop-go"
	"github.com/uber/ringpop-go/events"
	"github.com/uber/ringpop-go/swim"
	"github.com/uber/tchannel-go"
	"github.com/uber/tchannel-go/thrift"
)

type router struct {
	ringpop ringpop.Interface
	factory ClientFactory
	channel *tchannel.Channel

	rw          sync.RWMutex
	clientCache map[string]cacheEntry
}

// A Router creates instances of TChannel Thrift Clients via the help of the
// ClientFactory
type Router interface {
	// GetClient provides the caller with a client for a given key. At the same
	// time it will inform the caller if the client is a remote client or a
	// local client via the isRemote return value.
	GetClient(key string) (client interface{}, isRemote bool, err error)
	// GetNClients provides the caller with an ordered slice of clients for a
	// given key. Each result is a struct with a reference to the actual client
	// and a bool indicating whether or not that client is a remote client or a
	// local client.
	GetNClients(key string, n int) (clients []ClientResult, err error)
}

// A ClientFactory is able to provide an implementation of a TChan[Service]
// interface that can dispatch calls to the actual implementation. This could be
// both a local or a remote implementation of the interface based on the dest
// provided
type ClientFactory interface {
	GetLocalClient() interface{}
	MakeRemoteClient(client thrift.TChanClient) interface{}
}

type cacheEntry struct {
	client   interface{}
	isRemote bool
}

// New creates an instance that validates the Router interface. A Router
// will be used to get implementations of service interfaces that implement a
// distributed microservice.
func New(rp ringpop.Interface, f ClientFactory, ch *tchannel.Channel) Router {
	r := &router{
		ringpop: rp,
		factory: f,
		channel: ch,

		clientCache: make(map[string]cacheEntry),
	}
	rp.AddListener(r)
	return r
}

func (r *router) HandleEvent(event events.Event) {
	switch event := event.(type) {
	case swim.MemberlistChangesReceivedEvent:
		for _, change := range event.Changes {
			r.handleChange(change)
		}
	}
}

func (r *router) handleChange(change swim.Change) {
	switch change.Status {
	case swim.Faulty, swim.Leave:
		r.removeClient(change.Address)
	}
}

// Get the client for a certain destination from our internal cache, or
// delegates the creation to the ClientFactory.
func (r *router) GetClient(key string) (client interface{}, isRemote bool, err error) {
	dest, err := r.ringpop.Lookup(key)
	if err != nil {
		return nil, false, err
	}

	return r.getClientByHost(dest)
}

// ClientResult is a struct that contains a reference to the actual callable
// client and a bool indicating whether or not that client is local or remote.
type ClientResult struct {
	HostPort string
	Client   interface{}
	IsRemote bool
}

func (r *router) GetNClients(key string, n int) ([]ClientResult, error) {
	dests, err := r.ringpop.LookupN(key, n)
	if err != nil {
		return nil, err
	}

	clients := make([]ClientResult, n, n)

	for i, dest := range dests {
		client, isRemote, err := r.getClientByHost(dest)
		if err != nil {
			return nil, err
		}
		clients[i] = ClientResult{dest, client, isRemote}
	}
	return clients, nil
}

func (r *router) getClientByHost(dest string) (client interface{}, isRemote bool, err error) {
	r.rw.RLock()
	cachedEntry, ok := r.clientCache[dest]
	r.rw.RUnlock()
	if ok {
		client = cachedEntry.client
		isRemote = cachedEntry.isRemote
		return client, isRemote, nil
	}

	// no match so far, get a complete lock for creation
	r.rw.Lock()
	defer r.rw.Unlock()

	// double check it is not created between read and complete lock
	cachedEntry, ok = r.clientCache[dest]
	if ok {
		client = cachedEntry.client
		isRemote = cachedEntry.isRemote
		return client, isRemote, nil
	}

	me, err := r.ringpop.WhoAmI()
	if err != nil {
		return nil, false, err
	}

	// use the ClientFactory to get the client
	if dest == me {
		isRemote = false
		client = r.factory.GetLocalClient()
	} else {
		isRemote = true
		thriftClient := thrift.NewClient(
			r.channel,
			r.channel.ServiceName(),
			&thrift.ClientOptions{
				HostPort: dest,
			},
		)
		client = r.factory.MakeRemoteClient(thriftClient)
	}

	// cache the client
	r.clientCache[dest] = cacheEntry{
		client:   client,
		isRemote: isRemote,
	}
	return client, isRemote, nil
}

func (r *router) removeClient(hostport string) {
	r.rw.Lock()
	delete(r.clientCache, hostport)
	r.rw.Unlock()
}
