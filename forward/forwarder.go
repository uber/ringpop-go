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

// Package forward provides a mechanism to forward TChannel requests.
package forward

import (
	"encoding/json"
	"sync"
	"time"

	log "github.com/uber-common/bark"
	"github.com/uber/ringpop-go/events"
	"github.com/uber/ringpop-go/logging"
	"github.com/uber/ringpop-go/shared"
	"github.com/uber/ringpop-go/util"
	"github.com/uber/tchannel-go"
	"github.com/uber/tchannel-go/thrift"
)

// A Sender is used to route the request to the proper destination,
// the server returned by Lookup(key)
type Sender interface {
	// WhoAmI should return the address of the local sender
	WhoAmI() (string, error)

	// Lookup should return the server the request belongs to
	Lookup(string) (string, error)
}

// Options for the creation of a forwarder
type Options struct {
	Ctx            thrift.Context
	MaxRetries     int
	RerouteRetries bool
	RetrySchedule  []time.Duration
	Timeout        time.Duration
	Headers        []byte
}

func (f *Forwarder) defaultOptions() *Options {
	return &Options{
		MaxRetries:    3,
		RetrySchedule: []time.Duration{3 * time.Second, 6 * time.Second, 12 * time.Second},
		Timeout:       3 * time.Second,
	}
}

func (f *Forwarder) mergeDefaultOptions(opts *Options) *Options {
	def := f.defaultOptions()

	if opts == nil {
		return def
	}

	var merged Options

	merged.MaxRetries = util.SelectInt(opts.MaxRetries, def.MaxRetries)
	merged.Timeout = util.SelectDuration(opts.Timeout, def.Timeout)
	merged.RerouteRetries = opts.RerouteRetries

	merged.RetrySchedule = opts.RetrySchedule
	if opts.RetrySchedule == nil {
		merged.RetrySchedule = def.RetrySchedule
	}
	merged.Headers = opts.Headers
	merged.Ctx = opts.Ctx

	return &merged
}

// A Forwarder is used to forward requests to their destinations
type Forwarder struct {
	events.AsyncEventEmitter

	sender  Sender
	channel shared.SubChannel
	logger  log.Logger

	inflightLock sync.Mutex
	inflight     int64
}

// NewForwarder returns a new forwarder
func NewForwarder(s Sender, ch shared.SubChannel) *Forwarder {

	logger := logging.Logger("forwarder")
	if address, err := s.WhoAmI(); err == nil {
		logger = logger.WithField("local", address)
	}

	return &Forwarder{
		sender:  s,
		channel: ch,
		logger:  logger,
	}
}

func (f *Forwarder) incrementInflight() {
	f.inflightLock.Lock()
	f.inflight++
	inflight := f.inflight
	f.inflightLock.Unlock()

	f.EmitEvent(InflightRequestsChangedEvent{inflight})
}

func (f *Forwarder) decrementInflight() {
	f.inflightLock.Lock()
	pre := f.inflight
	f.inflight--

	// make sure that we do not decrement below 0
	if f.inflight < 0 {
		f.inflight = 0
		f.EmitEvent(InflightRequestsMiscountEvent{InflightDecrement})
	}

	inflight := f.inflight
	f.inflightLock.Unlock()

	if pre != inflight {
		f.EmitEvent(InflightRequestsChangedEvent{inflight})
	}
}

// ForwardRequest forwards a request to the given service and endpoint returns the response.
// Keys are used by the sender to lookup the destination on retry. If you have multiple keys
// and their destinations diverge on a retry then the call is aborted.
func (f *Forwarder) ForwardRequest(request []byte, destination, service, endpoint string,
	keys []string, format tchannel.Format, opts *Options) ([]byte, error) {

	f.EmitEvent(RequestForwardedEvent{})

	f.incrementInflight()
	opts = f.mergeDefaultOptions(opts)
	rs := newRequestSender(f.sender, f, f.channel, request, keys, destination, service, endpoint, format, opts)
	b, err := rs.Send()
	f.decrementInflight()

	if err != nil {
		f.EmitEvent(FailedEvent{})
	} else {
		f.EmitEvent(SuccessEvent{})
	}

	return b, err
}

var (
	// ForwardedHeaderName is the name used by the ringpop adapter to indicate
	// it is a forwarded request.
	ForwardedHeaderName = "ringpop-forward-keys"
)

// SetForwardedHeader adds a header to the current thrift context indicating
// that the call has been forwarded by another node in the ringpop ring. This
// header is used when a remote call is received to determine if forwarding
// checks needs to be applied. By not forwarding already forwarded calls we
// prevent unbound forwarding in the ring in case of memebership disagreement.
// The keys provided will be serialized as the value of the key and can be used
// in the future to check if key inconsistencies are found while forwarding.
// Currently this is not checked
func SetForwardedHeader(ctx thrift.Context, keys []string) thrift.Context {
	// copy headers to make sure two calls do not leak headers to each other
	headers := make(map[string]string, len(ctx.Headers())+1)
	for key, value := range ctx.Headers() {
		headers[key] = value
	}

	if keys == nil {
		// prevent null serialization, but use an empty array instead
		keys = []string{}
	}
	keysBytes, _ := json.Marshal(keys)
	keysString := string(keysBytes)

	// set the header indicating the call is forwarded for the provided keys
	headers[ForwardedHeaderName] = keysString

	// return the ctx with new headrs
	return thrift.WithHeaders(ctx, headers)
}

// DeleteForwardedHeader takes the headers that came in via TChannel and looks
// for the precense of a specific ringpop header to see if ringpop already
// forwarded the message. If the header is present it will delete the header
// from the context. The return value indicates if the header was present and
// deleted
func DeleteForwardedHeader(ctx thrift.Context) bool {
	_, ok := ctx.Headers()[ForwardedHeaderName]
	if ok {
		delete(ctx.Headers(), ForwardedHeaderName)
	}
	return ok
}
