// Copyright (c) 2015 Uber Technologies, Inc.

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

package mockhyperbahn

import (
	"errors"
	"sync"

	"github.com/uber/tchannel-go"
	"github.com/uber/tchannel-go/hyperbahn"
	"github.com/uber/tchannel-go/json"
)

// Mock is up a mock Hyperbahn server for tests.
type Mock struct {
	sync.RWMutex

	ch         *tchannel.Channel
	respCh     chan int
	advertised []string
}

// New returns a mock Hyperbahn server that can be used for testing.
func New() (*Mock, error) {
	ch, err := tchannel.NewChannel("hyperbahn", nil)
	if err != nil {
		return nil, err
	}

	mh := &Mock{
		ch:     ch,
		respCh: make(chan int),
	}
	if err := json.Register(ch, json.Handlers{"ad": mh.adHandler}, nil); err != nil {
		return nil, err
	}

	return mh, ch.ListenAndServe("127.0.0.1:0")
}

// Configuration returns a hyperbahn.Configuration object used to configure a
// hyperbahn.Client to talk to this mock server.
func (h *Mock) Configuration() hyperbahn.Configuration {
	return hyperbahn.Configuration{
		InitialNodes: []string{h.ch.PeerInfo().HostPort},
	}
}

func (h *Mock) adHandler(ctx json.Context, req *hyperbahn.AdRequest) (*hyperbahn.AdResponse, error) {
	h.Lock()
	for _, s := range req.Services {
		h.advertised = append(h.advertised, s.Name)
	}
	h.Unlock()

	select {
	case n := <-h.respCh:
		if n == 0 {
			return nil, errors.New("error")
		}
		return &hyperbahn.AdResponse{ConnectionCount: n}, nil
	default:
		// Return a default response
		return &hyperbahn.AdResponse{ConnectionCount: 3}, nil
	}
}

// GetAdvertised returns the list of services registered.
func (h *Mock) GetAdvertised() []string {
	h.RLock()
	defer h.RUnlock()

	return h.advertised
}

// Close stops the mock Hyperbahn server.
func (h *Mock) Close() {
	h.ch.Close()
}

// QueueError queues an error to be returned on the next advertise call.
func (h *Mock) QueueError() {
	h.respCh <- 0
}

// QueueResponse queues a response from Hyperbahn.
// numConnections must be greater than 0.
func (h *Mock) QueueResponse(numConnections int) {
	if numConnections <= 0 {
		panic("QueueResponse must have numConnections > 0")
	}

	h.respCh <- numConnections
}
