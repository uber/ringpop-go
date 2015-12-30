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
	"encoding/json"
	"errors"
	"time"

	"golang.org/x/net/context"

	log "github.com/uber-common/bark"
	"github.com/uber/ringpop-go/shared"
	"github.com/uber/tchannel-go"
	"github.com/uber/tchannel-go/raw"
)

// A requestSender is used to send a request to its destination, as defined by the sender's
// lookup method
type requestSender struct {
	sender  Sender
	emitter eventEmitter
	channel shared.SubChannel

	request           []byte
	destination       string
	service, endpoint string
	keys              []string
	format            tchannel.Format

	destinations []string // destinations the request has been routed to ?

	timeout             time.Duration
	retries, maxRetries int
	retrySchedule       []time.Duration
	rerouteRetries      bool

	startTime, retryStartTime time.Time

	logger log.Logger
}

// NewRequestSender returns a new request sender that can be used to forward a request to its destination
func newRequestSender(sender Sender, emitter eventEmitter, channel shared.SubChannel, request []byte, keys []string,
	destination, service, endpoint string, format tchannel.Format, opts *Options) *requestSender {

	return &requestSender{
		sender:         sender,
		emitter:        emitter,
		channel:        channel,
		request:        request,
		keys:           keys,
		destination:    destination,
		service:        service,
		endpoint:       endpoint,
		format:         format,
		timeout:        opts.Timeout,
		maxRetries:     opts.MaxRetries,
		retrySchedule:  opts.RetrySchedule,
		rerouteRetries: opts.RerouteRetries,
		logger:         opts.Logger,
	}
}

func (s *requestSender) Send() (res []byte, err error) {
	ctx, cancel := shared.NewTChannelContext(s.timeout)
	defer cancel()

	var forwardError, applicationError error

	select {
	case <-s.MakeCall(ctx, &res, &forwardError, &applicationError):
		if applicationError != nil {
			return nil, applicationError
		}

		if forwardError == nil {
			if s.retries > 0 {
				// forwarding succeeded after retries
				s.emitter.emit(RetrySuccessEvent{s.retries})
			}
			return res, nil
		}

		if s.retries < s.maxRetries {
			return s.ScheduleRetry()
		}

		identity, _ := s.sender.WhoAmI()

		s.logger.WithFields(log.Fields{
			"local":       identity,
			"destination": s.destination,
			"service":     s.service,
			"endpoint":    s.endpoint,
		}).Warn("max retries exceeded for request")

		s.emitter.emit(MaxRetriesEvent{s.maxRetries})

		return nil, errors.New("max retries exceeded")
	case <-ctx.Done(): // request timed out

		identity, _ := s.sender.WhoAmI()

		s.logger.WithFields(log.Fields{
			"local":       identity,
			"destination": s.destination,
			"service":     s.service,
			"endpoint":    s.endpoint,
		}).Warn("request timed out")

		return nil, errors.New("request timed out")
	}
}

// calls remote service and writes response to s.response
func (s *requestSender) MakeCall(ctx context.Context, res *[]byte, fwdError *error, appError *error) <-chan bool {
	done := make(chan bool)
	go func() {
		defer close(done)

		peer := s.channel.Peers().GetOrAdd(s.destination)

		call, err := peer.BeginCall(ctx, s.service, s.endpoint, &tchannel.CallOptions{
			Format: s.format,
		})
		if err != nil {
			*fwdError = err
			done <- true
			return
		}

		var arg3 []byte
		if s.format == tchannel.Thrift {
			_, arg3, _, err = raw.WriteArgs(call, []byte{0, 0}, s.request)
		} else {
			var resp *tchannel.OutboundCallResponse
			_, arg3, resp, err = raw.WriteArgs(call, nil, s.request)

			// check if the response is an application level error
			if err == nil && resp.ApplicationError() {
				// parse the json from the application level error
				errResp := struct {
					Type    string `json:"type"`
					Message string `json:"message"`
				}{}

				err = json.Unmarshal(arg3, &errResp)

				// if parsing worked return the application level error over the application error channel
				if err == nil {
					*appError = errors.New(errResp.Message)
					done <- true
					return
				}
			}
		}
		if err != nil {
			*fwdError = err
			done <- true
			return
		}

		*res = arg3
		done <- true
	}()

	return done
}

func (s *requestSender) ScheduleRetry() ([]byte, error) {
	if s.retries == 0 {
		s.retryStartTime = time.Now()
	}

	time.Sleep(s.retrySchedule[s.retries])

	return s.AttemptRetry()
}

func (s *requestSender) AttemptRetry() ([]byte, error) {
	s.retries++

	s.emitter.emit(RetryAttemptEvent{})

	dests := s.LookupKeys(s.keys)
	if len(dests) > 1 || len(dests) == 0 {
		s.emitter.emit(RetryAbortEvent{"key destinations have diverged"})
		return nil, errors.New("key destinations have diverged")
	}

	if s.rerouteRetries {
		newDest := dests[0]
		// nothing rebalanced so send again
		if newDest != s.destination {
			return s.RerouteRetry(newDest)
		}
	}

	// else just send
	return s.Send()
}

func (s *requestSender) RerouteRetry(destination string) ([]byte, error) {
	s.emitter.emit(RerouteEvent{
		s.destination,
		destination,
	})

	s.destination = destination // update request destination

	return s.Send()
}

func (s *requestSender) LookupKeys(keys []string) (dests []string) {
	for _, key := range keys {
		dest, err := s.sender.Lookup(key)
		if err != nil {
			continue
		}
		dests = append(dests, dest)
	}

	return dests
}
