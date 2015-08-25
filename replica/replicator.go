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

package replica

import (
	"errors"
	"io/ioutil"
	"reflect"
	"sync"

	"github.com/Sirupsen/logrus"
	log "github.com/uber/bark"
	"github.com/uber/ringpop-go/forward"
	"github.com/uber/ringpop-go/replica/util"
	"github.com/uber/tchannel/golang"
)

const (
	// Parallel fanout mode for replicator read write requests. Sends out requests
	// in parallel.
	Parallel = "parallel"

	// SerialSequential fanout mode for replicator read write requests. Sends out
	// requests one at a time going through the preference list sequentially
	SerialSequential = "serial"

	// SerialBalanced fanout mode for replicator read write requests. Sends out
	// requests one at a time, going through the preference list in a random order
	SerialBalanced = "balanced"

	read  = "read"
	write = "write"
)

// A Sender is used to lookup the destinations for requests given a key.
type Sender interface {
	// Lookup should return a server address
	Lookup(string) string

	// LookupN should return n server addresses
	LookupN(string, int) []string

	// WhoAmI should return the local address of the sender
	WhoAmI() string
}

// A Response is a response from a replicator read/write request.
type Response struct {
	Destination string
	Keys        []string
	Body        interface{}
}

// Options for sending a read/write replicator request
type Options struct {
	NValue, RValue, WValue int
	FanoutMode             string
}

type callOptions struct {
	Keys       []string
	Dests      []string
	Request    interface{}
	Restype    interface{}
	KeysByDest map[string][]string
	Operation  string
}

// A Replicator is used to replicate a request across nodes such that they share
// ownership of some data.
type Replicator struct {
	sender    Sender
	channel   *tchannel.SubChannel
	forwarder *forward.Forwarder
	logger    log.Logger
	defaults  *Options
}

func selectFanoutMode(mode string) string {
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

	opts.NValue = util.SelectInt(opts.NValue, def.NValue)
	opts.RValue = util.SelectInt(opts.RValue, def.RValue)
	opts.WValue = util.SelectInt(opts.WValue, def.WValue)
	opts.FanoutMode = selectFanoutMode(opts.FanoutMode)

	return opts
}

// NewReplicator returns a new Replicator instance that makes calls with the given
// SubChannel to the service defined by SubChannel.GetServiceName(). The given n/w/r
// values will be used as defaults for the replicator when none are provided
func NewReplicator(s Sender, channel *tchannel.SubChannel, logger log.Logger,
	opts *Options) *Replicator {

	if logger == nil {
		logger = log.NewLoggerFromLogrus(&logrus.Logger{
			Out: ioutil.Discard,
		})
	}

	f := forward.NewForwarder(s, channel, logger)

	opts = mergeDefaultOptions(opts, &Options{3, 1, 3, Parallel})
	return &Replicator{s, channel, f, logger, opts}
}

// Read replicates a read request. It takes key(s) to be used for lookup of the requests
// destination, a request to send, the operation to perform at the destination, options
// for forwarding the request as well as options for ffanning out the request. It also
// takes a response type, which is the type of struct that will be returned in each
// responses.Body in response. Response type must be a concrete struct. The body field
// will contain a pointer to that type of struct.
func (r *Replicator) Read(keys []string, request, restype interface{}, operation string,
	fopts *forward.Options, opts *Options) (responses []Response, err error) {

	opts = mergeDefaultOptions(opts, r.defaults)
	return r.readWrite(read, keys, request, restype, operation, fopts, opts)
}

// Write replicates a write request. It takes key(s) to be used for lookup of the requests
// destination, a request to send, the operation to perform at the destination, options
// for forwarding the request as well as options for ffanning out the request. It also
// takes a response type, which is the type of struct that will be returned in each
// responses.Body in response. Response type must be a concrete struct. The body field
// will contain a pointer to that type of struct.
func (r *Replicator) Write(keys []string, request, response interface{}, operation string,
	fopts *forward.Options, opts *Options) (responses []Response, err error) {

	opts = mergeDefaultOptions(opts, r.defaults)
	return r.readWrite(write, keys, request, response, operation, fopts, opts)
}

func (r *Replicator) groupReplicas(keys []string, n int) (map[string][]string,
	map[string][]string) {

	destsByKey := make(map[string][]string)
	keysByDest := make(map[string][]string)

	for _, key := range keys {
		dests := r.sender.LookupN(key, n)
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

func validateResponseType(restype interface{}) error {
	t := reflect.TypeOf(restype)

	if t.Kind() != reflect.Struct && (t.Kind() != reflect.Map || t.Key().Kind() != reflect.String) {
		return errors.New("invalid type for response")
	}

	return nil
}

func (r *Replicator) readWrite(rw string, keys []string, request, restype interface{},
	operation string, fopts *forward.Options, opts *Options) ([]Response, error) {

	if err := validateResponseType(restype); err != nil {
		return []Response{}, err
	}

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
		Restype:    restype,
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

	var responses []Response
	var errors []error
	var wg sync.WaitGroup
	var l sync.Mutex

	for _, dest := range copts.Dests {
		wg.Add(1)
		go func(dest string) {
			defer wg.Done()

			res, err := r.forwardRequest(dest, copts, fopts)

			l.Lock()
			defer l.Unlock()

			if err != nil {
				errors = append(errors, err)
				return
			}
			responses = append(responses, res)
		}(dest)
	}

	wg.Wait()

	return responses, nil
}

func (r *Replicator) serial(rwValue int, copts *callOptions,
	fopts *forward.Options, opts *Options) ([]Response, []error) {

	var responses []Response
	var errors []error

	if opts.FanoutMode == SerialBalanced {
		copts.Dests = util.Shuffle(copts.Dests)
	}

	for _, dest := range copts.Dests {
		res, err := r.forwardRequest(dest, copts, fopts)
		if err != nil {
			errors = append(errors, err)
			continue
		}
		responses = append(responses, res)
	}

	return responses, nil
}

func (r *Replicator) forwardRequest(dest string, copts *callOptions,
	fopts *forward.Options) (Response, error) {

	var response Response
	var res = reflect.New(reflect.TypeOf(copts.Restype))
	var keys = copts.KeysByDest[dest]

	err := r.forwarder.ForwardRequest(copts.Request, res.Interface(), dest,
		r.channel.ServiceName(), copts.Operation, keys, fopts)

	if err != nil {
		r.logger.WithFields(log.Fields{
			"error": err,
		}).Warn("replicator read/write error")

		return response, err
	}

	response.Destination = dest
	response.Keys = keys
	response.Body = res.Interface()

	return response, nil
}
