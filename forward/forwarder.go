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
	"io/ioutil"
	"time"

	"github.com/Sirupsen/logrus"
	log "github.com/uber-common/bark"
	"github.com/uber/ringpop-go/shared"
	"github.com/uber/ringpop-go/swim/util"
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
	MaxRetries     int
	RerouteRetries bool
	RetrySchedule  []time.Duration
	Timeout        time.Duration
	Logger         log.Logger
}

func (f *Forwarder) defaultOptions() *Options {
	return &Options{
		MaxRetries:    3,
		RetrySchedule: []time.Duration{3 * time.Second, 6 * time.Second, 12 * time.Second},
		Timeout:       3 * time.Second,
		Logger:        f.logger,
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

	merged.Logger = opts.Logger
	if opts.Logger == nil {
		merged.Logger = def.Logger
	}

	return &merged
}

// A Forwarder is used to forward requests to their destinations
type Forwarder struct {
	sender  Sender
	channel shared.SubChannel
	logger  log.Logger
}

// NewForwarder returns a new forwarder
func NewForwarder(s Sender, ch shared.SubChannel, logger log.Logger) *Forwarder {
	if logger == nil {
		logger = log.NewLoggerFromLogrus(&logrus.Logger{
			Out: ioutil.Discard,
		})
	}

	return &Forwarder{
		sender:  s,
		channel: ch,
		logger:  logger,
	}
}

// ForwardRequest forwards a request to the given service and endpoint returns the response.
// Keys are used by the sender to lookup the destination on retry. If you have multiple keys
// and their destinations diverge on a retry then the call is aborted.
func (f *Forwarder) ForwardRequest(request []byte, destination, service, endpoint string,
	keys []string, format tchannel.Format, opts *Options) ([]byte, error) {

	opts = f.mergeDefaultOptions(opts)
	rs := newRequestSender(f.sender, f.channel, request, keys, destination, service, endpoint, format, opts)
	return rs.Send()
}

var (
	forwardedHeaderName  = "ringpop-forwarded"
	staticForwardHeaders = map[string]string{forwardedHeaderName: "true"}
)

// SetForwardedHeader adds a header to the current thrift context indicating
// that the call has been forwarded by another node in the ringpop ring.
// This header is used when a remote call is received to determine if forwarding
// checks needs to be applied. By not forwarding already forwarded calls we
// prevent unbound forwarding in the ring in case of memebership disagreement.
func SetForwardedHeader(ctx thrift.Context) thrift.Context {
	headers := ctx.Headers()
	if len(headers) == 0 {
		return thrift.WithHeaders(ctx, staticForwardHeaders)
	}

	headers[forwardedHeaderName] = "true"
	return thrift.WithHeaders(ctx, headers)
}

// HasForwardedHeader takes the headers that came in via TChannel and looks for
// the precense of a specific ringpop header to see if ringpop already forwarded
// the message. When a message has already been forwarded by ringpop the
// forwarding logic should not be used to prevent unbound forwarding.
func HasForwardedHeader(ctx thrift.Context) bool {
	_, ok := ctx.Headers()[forwardedHeaderName]
	return ok
}
