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
	"errors"
	"time"

	log "github.com/uber/bark"
	"github.com/uber/tchannel/golang"
	"github.com/uber/tchannel/golang/json"
)

// A requestSender is used to send a request to its destination, as defined by the sender's
// lookup method
type requestSender struct {
	sender  Sender
	channel *tchannel.SubChannel

	request, response interface{}
	destination       string
	service, endpoint string
	keys              []string

	destinations []string // destinations the request has been routed to ?

	timeout             time.Duration
	retries, maxRetries int
	retrySchedule       []time.Duration
	rerouteReties       bool

	startTime, retryStartTime time.Time

	logger log.Logger
}

// NewRequestSender returns a new request sender that can be used to forward a request to its destination
func newRequestSender(sender Sender, channel *tchannel.SubChannel, request, response interface{},
	keys []string, destination, service, endpoint string, opts *Options) *requestSender {

	return &requestSender{
		sender:        sender,
		channel:       channel,
		request:       request,
		response:      response,
		keys:          keys,
		destination:   destination,
		service:       service,
		endpoint:      endpoint,
		timeout:       opts.Timeout,
		maxRetries:    opts.MaxRetries,
		retrySchedule: opts.RetrySchedule,
		rerouteReties: opts.RerouteRetries,
		logger:        opts.Logger,
	}
}

func (s *requestSender) Send() (err error) {
	ctx, cancel := json.NewContext(s.timeout)
	defer cancel()

	select {
	case err := <-s.MakeCall(ctx):
		if err == nil {
			return nil // response written to s.response
		}

		if s.retries < s.maxRetries {
			return s.ScheduleRetry()
		}

		s.logger.WithFields(log.Fields{
			"local":       s.sender.WhoAmI(),
			"destination": s.destination,
			"service":     s.service,
			"endpoint":    s.endpoint,
		}).Warn("max retries exceeded for request")

		return errors.New("max retries exceeded")

	case <-ctx.Done(): // request timed out
		s.logger.WithFields(log.Fields{
			"local":       s.sender.WhoAmI(),
			"destination": s.destination,
			"service":     s.service,
			"endpoint":    s.endpoint,
		}).Warn("request timed out")

		return errors.New("request timed out")
	}
}

// calls remote service and writes response to s.response
func (s *requestSender) MakeCall(ctx json.Context) <-chan error {
	errC := make(chan error)
	go func() {
		defer close(errC)

		peer := s.channel.Peers().GetOrAdd(s.destination)

		if err := json.CallPeer(ctx, peer, s.service, s.endpoint, s.request, s.response); err != nil {
			errC <- err
			return
		}

		errC <- nil
	}()

	return errC
}

func (s *requestSender) ScheduleRetry() error {
	if s.retries == 0 {
		s.retryStartTime = time.Now()
	}

	time.Sleep(s.retrySchedule[s.retries])

	return s.AttemptRetry()
}

func (s *requestSender) AttemptRetry() error {
	s.retries++

	dests := s.LookupKeys(s.keys)
	if len(dests) > 1 || len(dests) == 0 {
		return errors.New("key destinations have diverged")
	}

	if s.rerouteReties {
		newDest := dests[0]
		// nothing rebalanced so send again
		if newDest != s.destination {
			return s.RerouteRetry(newDest)
		}
	}

	// else just send
	return s.Send()
}

func (s *requestSender) RerouteRetry(destination string) error {
	s.destination = destination // update request destination
	return s.Send()
}

func (s *requestSender) LookupKeys(keys []string) (dests []string) {
	for _, key := range keys {
		dests = append(dests, s.sender.Lookup(key))
	}

	return dests
}
