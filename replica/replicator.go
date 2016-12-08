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

// Package replica extends Ringpop functionality by providing a mechanism to replicate
// a request to multiple nodes in the ring.
package replica

import (
	"errors"
	"sync"

	log "github.com/uber-common/bark"
	"github.com/uber/ringpop-go/forward"
	"github.com/uber/ringpop-go/logging"
	"github.com/uber/ringpop-go/shared"
	"github.com/uber/ringpop-go/util"
	"github.com/uber/tchannel-go"
)

// FanoutMode defines how a replicator should fanout it's requests
type FanoutMode int

const (
	// Parallel fanout mode for replicator read write requests. Sends out requests
	// in parallel.
	Parallel FanoutMode = iota

	// SerialSequential fanout mode for replicator read write requests. Sends out
	// requests one at a time going through the preference list sequentially
	SerialSequential

	// SerialBalanced fanout mode for replicator read write requests. Sends out
	// requests one at a time, going through the preference list in a random order
	SerialBalanced
)

const (
	read int = iota
	write
)

// A Sender is used to lookup the destinations for requests given a key.
type Sender interface {
	// Lookup should return a server address
	Lookup(string) (string, error)

	// LookupN should return n server addresses
	LookupN(string, int) ([]string, error)

	// WhoAmI should return the local address of the sender
	WhoAmI() (string, error)
}

// A Response is a response from a replicator read/write request.
type Response struct {
	Destination string
	Keys        []string
	Body        []byte
}

// Options for sending a read/write replicator request
type Options struct {
	NValue, RValue, WValue int
	FanoutMode             FanoutMode
}

type callOptions struct {
	Keys       []string
	Dests      []string
	Request    []byte
	KeysByDest map[string][]string
	Operation  string
	Format     tchannel.Format
}

// A Replicator is used to replicate a request across nodes such that they share
// ownership of some data.
type Replicator struct {
	sender    Sender
	channel   shared.SubChannel
	forwarder *forward.Forwarder
	logger    log.Logger
	defaults  *Options
}

func selectFanoutMode(mode FanoutMode) FanoutMode {
	switch mode {
	case Parallel, SerialSequential, SerialBalanced:
		return mode
	default:
		return Parallel
	}
}

func mergeDefaultOptions(opts *Options, def *Options) *Options {
	if opts == nil {
		return def
	}

	var merged Options

	merged.NValue = util.SelectInt(opts.NValue, def.NValue)
	merged.RValue = util.SelectInt(opts.RValue, def.RValue)
	merged.WValue = util.SelectInt(opts.WValue, def.WValue)
	merged.FanoutMode = selectFanoutMode(opts.FanoutMode)

	return &merged
}

// NewReplicator returns a new Replicator instance that makes calls with the given
// SubChannel to the service defined by SubChannel.GetServiceName(). The given n/w/r
// values will be used as defaults for the replicator when none are provided
// Deprecation: logger is no longer used.
func NewReplicator(s Sender, channel shared.SubChannel, logger log.Logger,
	opts *Options) *Replicator {

	f := forward.NewForwarder(s, channel)

	opts = mergeDefaultOptions(opts, &Options{3, 1, 3, Parallel})
	logger = logging.Logger("replicator")
	if address, err := s.WhoAmI(); err == nil {
		logger = logger.WithField("local", address)
	}
	return &Replicator{s, channel, f, logger, opts}
}

// Read replicates a read request. It takes key(s) to be used for lookup of the requests
// destination, a request to send, the operation to perform at the destination, options
// for forwarding the request as well as options for ffanning out the request. It also
// takes a response type, which is the type of struct that will be returned in each
// responses.Body in response. Response type must be a concrete struct. The body field
// will contain a pointer to that type of struct.
func (r *Replicator) Read(keys []string, request []byte, operation string, fopts *forward.Options,
	opts *Options) (responses []Response, err error) {

	opts = mergeDefaultOptions(opts, r.defaults)
	return r.readWrite(read, keys, request, operation, fopts, opts)
}

// Write replicates a write request. It takes key(s) to be used for lookup of the requests
// destination, a request to send, the operation to perform at the destination, options
// for forwarding the request as well as options for ffanning out the request. It also
// takes a response type, which is the type of struct that will be returned in each
// responses.Body in response. Response type must be a concrete struct. The body field
// will contain a pointer to that type of struct.
func (r *Replicator) Write(keys []string, request []byte, operation string, fopts *forward.Options,
	opts *Options) (responses []Response, err error) {

	opts = mergeDefaultOptions(opts, r.defaults)
	return r.readWrite(write, keys, request, operation, fopts, opts)
}

func (r *Replicator) groupReplicas(keys []string, n int) (map[string][]string,
	map[string][]string) {

	destsByKey := make(map[string][]string)
	keysByDest := make(map[string][]string)

	for _, key := range keys {
		dests, _ := r.sender.LookupN(key, n)
		destsByKey[key] = dests

		if len(dests) == 0 {
			continue
		}

		for _, dest := range dests {
			keysByDest[dest] = append(keysByDest[dest], key)
		}

	}

	return destsByKey, keysByDest
}

func (r *Replicator) readWrite(rw int, keys []string, request []byte, operation string,
	fopts *forward.Options, opts *Options) ([]Response, error) {

	var rwValue int
	switch rw {
	case read:
		rwValue = opts.RValue
	case write:
		rwValue = opts.WValue
	}

	if rwValue > opts.NValue {
		return nil, errors.New("rw value cannot exceed n value")
	}

	destsByKey, keysByDest := r.groupReplicas(keys, opts.NValue)
	var dests []string
	switch len(keys) {
	case 1:
		// preserve preference list order
		dests = destsByKey[keys[0]]
	default:
		// else arbitary order
		for dest := range keysByDest {
			dests = append(dests, dest)
		}
	}

	if len(dests) < rwValue {
		return nil, errors.New("rw value not satisfied by destination")
	}

	var responses []Response
	var errs []error

	copts := &callOptions{
		Keys:       keys,
		Dests:      dests,
		Request:    request,
		KeysByDest: keysByDest,
		Operation:  operation,
	}

	switch opts.FanoutMode {
	case Parallel:
		responses, errs = r.parallel(rwValue, copts, fopts, opts)
	case SerialSequential, SerialBalanced:
		responses, errs = r.serial(rwValue, copts, fopts, opts)
	}

	if len(responses) < rwValue {
		r.logger.WithFields(log.Fields{
			"nValue":       opts.NValue,
			"rwValue":      rwValue,
			"numResponses": len(responses),
			"numErrors":    len(errs),
			"errors":       errs,
		}).Debug("replicator rw value not satisfied")

		return responses, errors.New("rw value not satisfied")
	}

	return responses, nil
}

// sends read/write requests in parallel
func (r *Replicator) parallel(rwValue int, copts *callOptions, fopts *forward.Options,
	opts *Options) ([]Response, []error) {

	var responses struct {
		successes []Response
		errors    []error
		sync.Mutex
	}

	var wg sync.WaitGroup

	for _, dest := range copts.Dests {
		wg.Add(1)
		go func(dest string) {
			res, err := r.forwardRequest(dest, copts, fopts)

			if err != nil {
				responses.Lock()
				responses.errors = append(responses.errors, err)
				responses.Unlock()
				wg.Done()
				return
			}

			responses.Lock()
			responses.successes = append(responses.successes, res)
			responses.Unlock()
			wg.Done()
		}(dest)
	}

	wg.Wait()

	return responses.successes, responses.errors
}

func (r *Replicator) serial(rwValue int, copts *callOptions,
	fopts *forward.Options, opts *Options) ([]Response, []error) {

	var responses []Response
	var errors []error

	if opts.FanoutMode == SerialBalanced {
		copts.Dests = util.ShuffleStrings(copts.Dests)
	}

	for _, dest := range copts.Dests {
		res, err := r.forwardRequest(dest, copts, fopts)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		responses = append(responses, res)
	}

	return responses, errors
}

func (r *Replicator) forwardRequest(dest string, copts *callOptions, fopts *forward.Options) (Response, error) {
	var response Response
	var keys = copts.KeysByDest[dest]

	res, err := r.forwarder.ForwardRequest(copts.Request, dest, r.channel.ServiceName(),
		copts.Operation, keys, copts.Format, fopts)

	if err != nil {
		r.logger.WithFields(log.Fields{
			"error": err,
		}).Warn("replicator read/write error")

		return response, err
	}

	response.Destination = dest
	response.Keys = keys
	response.Body = res

	return response, nil
}
