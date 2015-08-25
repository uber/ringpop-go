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

package forward

import (
	"io/ioutil"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/uber/ringpop-go/swim/util"
	"github.com/uber/tchannel/golang"
)

// A Sender is used to route the request to the proper destination,
// the server returned by Lookup(key)
type Sender interface {
	// WhoAmI should return the address of the local sender
	WhoAmI() string

	// Lookup should return the server the request belongs to
	Lookup(string) string
}

// Options for the creation of a forwarder
type Options struct {
	MaxRetries     int
	RerouteRetries bool
	RetrySchedule  []time.Duration
	Timeout        time.Duration
	Logger         *log.Logger
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

	opts.MaxRetries = util.SelectInt(opts.MaxRetries, def.MaxRetries)
	opts.Timeout = util.SelectDuration(opts.Timeout, def.Timeout)

	if opts.RetrySchedule == nil {
		opts.RetrySchedule = def.RetrySchedule
	}

	if opts.Logger == nil {
		opts.Logger = def.Logger
	}

	return opts
}

// A Forwarder is used to forward requests to their destinations
type Forwarder struct {
	sender  Sender
	channel *tchannel.SubChannel
	logger  *log.Logger
}

// NewForwarder returns a new forwarder
func NewForwarder(s Sender, ch *tchannel.SubChannel, logger *log.Logger) *Forwarder {
	if logger == nil {
		logger = &log.Logger{Out: ioutil.Discard}
	}

	f := &Forwarder{
		sender:  s,
		channel: ch,
		logger:  logger,
	}

	return f
}

// ForwardRequest forwards a request to the given service and endpoint and writes the response
// to Response
func (f *Forwarder) ForwardRequest(request, response interface{}, destination, service, endpoint string,
	keys []string, opts *Options) error {

	opts = f.mergeDefaultOptions(opts)

	rs := newRequestSender(f.sender, f.channel, request, response, keys,
		destination, service, endpoint, opts)
	return rs.Send()
}
