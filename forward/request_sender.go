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
	"github.com/uber/ringpop-go/events"
	"github.com/uber/ringpop-go/logging"
	"github.com/uber/ringpop-go/shared"
	"github.com/uber/tchannel-go"
	"github.com/uber/tchannel-go/raw"
	"github.com/uber/tchannel-go/thrift"
)

// errDestinationsDiverged is an error that is returned from AttemptRetry
// if keys that previously hashed to the same destination diverge.
var errDestinationsDiverged = errors.New("key destinations have diverged")

// A requestSender is used to send a request to its destination, as defined by the sender's
// lookup method
type requestSender struct {
	sender  Sender
	emitter events.EventEmitter
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

	headers []byte
	ctx     thrift.Context

	startTime, retryStartTime time.Time

	logger log.Logger
}

// NewRequestSender returns a new request sender that can be used to forward a request to its destination
func newRequestSender(sender Sender, emitter events.EventEmitter, channel shared.SubChannel, request []byte, keys []string,
	destination, service, endpoint string, format tchannel.Format, opts *Options) *requestSender {

	logger := logging.Logger("sender")
	if address, err := sender.WhoAmI(); err != nil {
		logger = logger.WithField("local", address)
	}

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
		headers:        opts.Headers,
		ctx:            opts.Ctx,
		logger:         logger,
	}
}

func (s *requestSender) Send() (res []byte, err error) {
	var cancel context.CancelFunc
	var ctx thrift.Context
	if s.ctx == nil {
		ctx, cancel = shared.NewTChannelContext(s.timeout)
	} else {
		var c context.Context
		c, cancel = context.WithTimeout(s.ctx, s.timeout)
		ctx = tchannel.Wrap(c)
	}
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
				s.emitter.EmitEvent(RetrySuccessEvent{s.retries})
			}
			return res, nil
		}

		if s.retries < s.maxRetries {
			return s.ScheduleRetry()
		}

		address, _ := s.sender.WhoAmI()

		s.logger.WithFields(log.Fields{
			"local":       address,
			"destination": s.destination,
			"service":     s.service,
			"endpoint":    s.endpoint,
		}).Warn("max retries exceeded for request")

		s.emitter.EmitEvent(MaxRetriesEvent{s.maxRetries})

		return nil, errors.New("max retries exceeded")
	case <-ctx.Done(): // request timed out

		address, _ := s.sender.WhoAmI()

		s.logger.WithFields(log.Fields{
			"local":       address,
			"destination": s.destination,
			"service":     s.service,
			"endpoint":    s.endpoint,
		}).Warn("request timed out")

		return nil, errors.New("request timed out")
	}
}

// calls remote service and writes response to s.response
func (s *requestSender) MakeCall(ctx context.Context, res *[]byte, fwdError *error, appError *error) <-chan bool {
	done := make(chan bool, 1)
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
		headers := s.headers
		if s.format == tchannel.Thrift {
			if headers == nil {
				headers = []byte{0, 0}
			}
			_, arg3, _, err = raw.WriteArgs(call, headers, s.request)
		} else {
			var resp *tchannel.OutboundCallResponse
			_, arg3, resp, err = raw.WriteArgs(call, headers, s.request)

			// check if the response is an application level error
			if err == nil && resp.ApplicationError() {
				// parse the json from the application level error
				errResp := struct {
					Type    string `json:"type"`
					Message string `json:"message"`
				}{}

				err = json.Unmarshal(arg3, &errResp)

				// if parsing succeeded return the error as an application error
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

// AttemptRetry attempts to resend a request. Before resending it will
// lookup the keys provided to the requestSender upon construction. If
// keys that previously hashed to the same destination diverge, an
// errDestinationsDiverged error will be returned. If keys do not diverge,
// the will be rerouted to their new destination. Rerouting can be disabled
// by toggling the rerouteRetries flag.
func (s *requestSender) AttemptRetry() ([]byte, error) {
	s.retries++

	s.emitter.EmitEvent(RetryAttemptEvent{})

	dests := s.LookupKeys(s.keys)
	if len(dests) != 1 {
		s.emitter.EmitEvent(RetryAbortEvent{errDestinationsDiverged.Error()})
		return nil, errDestinationsDiverged
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
	s.emitter.EmitEvent(RerouteEvent{
		s.destination,
		destination,
	})

	s.destination = destination // update request destination

	return s.Send()
}

// LookupKeys looks up the destinations of the keys provided. Returns a slice
// of destinations. If multiple keys hash to the same destination, they will
// be deduped.
func (s *requestSender) LookupKeys(keys []string) []string {
	// Lookup and dedupe the destinations of the keys.
	destSet := make(map[string]struct{})
	for _, key := range keys {
		dest, err := s.sender.Lookup(key)
		if err != nil {
			// TODO Do something better than swallowing these errors.
			continue
		}

		destSet[dest] = struct{}{}
	}

	// Return the unique destinations as a slice.
	dests := make([]string, 0, len(destSet))
	for dest := range destSet {
		dests = append(dests, dest)
	}
	return dests
}
