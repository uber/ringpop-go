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
	"bytes"
	json2 "encoding/json"
	"errors"
	"sync"
	"testing"
	"time"

	events "github.com/uber/ringpop-go/events/test/mocks"
	"github.com/uber/ringpop-go/test/thrift/pingpong"

	athrift "github.com/apache/thrift/lib/go/thrift"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/uber/tchannel-go"
	"github.com/uber/tchannel-go/json"
	"github.com/uber/tchannel-go/thrift"
	"golang.org/x/net/context"
)

type ContextKey string

type ForwarderTestSuite struct {
	suite.Suite
	sender    *MockSender
	forwarder *Forwarder
	channel   *tchannel.Channel
	peer      *tchannel.Channel
}

type Ping struct {
	Message string `json:"message"`
}

func (p Ping) Bytes() []byte {
	data, _ := json2.Marshal(p)
	return data
}

type Pong struct {
	Message string `json:"message"`
	From    string `json:"from"`
	Headers map[string]string
}

func (s *ForwarderTestSuite) registerPong(address string, channel *tchannel.Channel) {
	hmap := map[string]interface{}{
		"/ping": func(ctx json.Context, ping *Ping) (*Pong, error) {
			return &Pong{"Hello, world!", address, ctx.Headers()}, nil
		},
		"/error": func(ctx json.Context, ping *Ping) (*Pong, error) {
			return nil, errors.New("remote error")
		},
	}
	s.Require().NoError(json.Register(channel, hmap, func(ctx context.Context, err error) {}))

	thriftHandler := &pingpong.MockTChanPingPong{}

	// successful request with context
	thriftHandler.On("Ping", mock.MatchedBy(
		func(c thrift.Context) bool {
			return true
		}), &pingpong.Ping{
		Key: "ctxTest",
	}).Return(&pingpong.Pong{
		Source: address,
	}, nil)

	// successful request
	thriftHandler.On("Ping", mock.Anything, &pingpong.Ping{
		Key: "success",
	}).Return(&pingpong.Pong{
		Source: address,
	}, nil)

	// error request
	thriftHandler.On("Ping", mock.Anything, &pingpong.Ping{
		Key: "error",
	}).Return(nil, &pingpong.PingError{})

	server := thrift.NewServer(channel)
	server.Register(pingpong.NewTChanPingPongServer(thriftHandler))
}

func (s *ForwarderTestSuite) SetupSuite() {

	channel, err := tchannel.NewChannel("test", nil)
	s.Require().NoError(err, "channel must be created successfully")
	s.channel = channel

	peer, err := tchannel.NewChannel("test", nil)
	s.Require().NoError(err, "channel must be created successfully")
	s.registerPong("correct pinging host", peer)
	s.Require().NoError(peer.ListenAndServe("127.0.0.1:0"), "channel must listen")

	sender := &MockSender{}
	sender.On("Lookup", "me").Return("192.0.2.1:1", nil)
	sender.On("WhoAmI").Return("192.0.2.1:1", nil)

	// processes can not listen on port 0 so it is safe to assume that this address is failing immediatly, preventing the timeout path to kick in.
	sender.On("Lookup", "immediate fail").Return("127.0.0.1:0", nil)
	sender.On("Lookup", "reachable").Return(peer.PeerInfo().HostPort, nil)
	sender.On("Lookup", "unreachable").Return("192.0.2.128:1", nil)
	sender.On("Lookup", "error").Return("", errors.New("lookup error"))
	s.sender = sender
	s.peer = peer

	s.forwarder = NewForwarder(s.sender, s.channel.GetSubChannel("forwarder"))
}

func (s *ForwarderTestSuite) TearDownSuite() {
	s.channel.Close()
	s.peer.Close()
}

func (s *ForwarderTestSuite) TestForwardJSON() {
	var ping Ping
	var pong Pong

	dest, err := s.sender.Lookup("reachable")
	s.NoError(err)

	headerBytes := []byte(`{"hdr1": "val1"}`)
	res, err := s.forwarder.ForwardRequest(ping.Bytes(), dest, "test", "/ping", []string{"reachable"},
		tchannel.JSON, &Options{Headers: headerBytes})
	s.NoError(err, "expected request to be forwarded")

	s.NoError(json2.Unmarshal(res, &pong))
	s.Equal("correct pinging host", pong.From)
	s.Equal("Hello, world!", pong.Message)
	s.Equal(map[string]string{"hdr1": "val1"}, pong.Headers)
}

func (s *ForwarderTestSuite) TestForwardJSONErrorResponse() {
	var ping Ping

	dest, err := s.sender.Lookup("reachable")
	s.NoError(err)

	_, err = s.forwarder.ForwardRequest(ping.Bytes(), dest, "test", "/error", []string{"reachable"},
		tchannel.JSON, nil)
	s.EqualError(err, "remote error")
}

func (s *ForwarderTestSuite) TestForwardJSONInvalidEndpoint() {
	var ping Ping

	dest, err := s.sender.Lookup("reachable")
	s.NoError(err)

	_, err = s.forwarder.ForwardRequest(ping.Bytes(), dest, "test", "/invalid", []string{"reachable"},
		tchannel.JSON, &Options{
			MaxRetries: 1,
			RetrySchedule: []time.Duration{
				100 * time.Millisecond,
			},
		})
	s.EqualError(err, "max retries exceeded")
}

func (s *ForwarderTestSuite) TestForwardThrift() {
	dest, err := s.sender.Lookup("reachable")
	s.NoError(err)

	request := &pingpong.PingPongPingArgs{
		Request: &pingpong.Ping{
			Key: "success",
		},
	}

	bytes, err := SerializeThrift(request)
	s.NoError(err, "expected ping to be serialized")

	res, err := s.forwarder.ForwardRequest(bytes, dest, "test", "PingPong::Ping", []string{"reachable"},
		tchannel.Thrift, nil)
	s.NoError(err, "expected request to be forwarded")

	var response pingpong.PingPongPingResult

	err = DeserializeThrift(res, &response)
	s.NoError(err)

	s.Equal("correct pinging host", response.Success.Source)
}

func (s *ForwarderTestSuite) TestForwardThriftWithCtxOption() {
	dest, err := s.sender.Lookup("reachable")
	s.NoError(err)

	request := &pingpong.PingPongPingArgs{
		Request: &pingpong.Ping{
			Key: "ctxTest",
		},
	}

	bytes1, err := SerializeThrift(request)
	s.NoError(err, "expected ping to be serialized")


	k := ContextKey("key")
	ctx := thrift.Wrap(context.WithValue(context.Background(), k, "val"))

	res, err := s.forwarder.ForwardRequest(bytes1, dest, "test", "PingPong::Ping", []string{"reachable"},
		tchannel.Thrift, &Options{
			Ctx: ctx,
		})
	s.NoError(err, "expected request to be forwarded")

	var response pingpong.PingPongPingResult

	err = DeserializeThrift(res, &response)
	s.NoError(err)

	s.Equal("correct pinging host", response.Success.Source)
}

func (s *ForwarderTestSuite) TestForwardThriftErrorResponse() {
	dest, err := s.sender.Lookup("reachable")
	s.NoError(err)

	request := &pingpong.PingPongPingArgs{
		Request: &pingpong.Ping{
			Key: "error",
		},
	}

	bytes, err := SerializeThrift(request)
	s.NoError(err, "expected ping to be serialized")

	res, err := s.forwarder.ForwardRequest(bytes, dest, "test", "PingPong::Ping", []string{"reachable"},
		tchannel.Thrift, nil)
	s.NoError(err, "expected request to be forwarded")

	var response pingpong.PingPongPingResult

	err = DeserializeThrift(res, &response)
	s.NoError(err)

	s.NotNil(response.PingError, "expected a pingerror")
}

func (s *ForwarderTestSuite) TestMaxRetries() {
	var ping Ping

	dest, err := s.sender.Lookup("immediate fail")
	s.NoError(err)

	_, err = s.forwarder.ForwardRequest(ping.Bytes(), dest, "test", "/ping", []string{"immediate fail"},
		tchannel.JSON, &Options{
			MaxRetries:    2,
			RetrySchedule: []time.Duration{time.Millisecond, time.Millisecond},
		})

	s.EqualError(err, "max retries exceeded")
}

func (s *ForwarderTestSuite) TestLookupErrorInRetry() {
	var ping Ping

	dest, err := s.sender.Lookup("immediate fail")
	s.NoError(err)

	_, err = s.forwarder.ForwardRequest(ping.Bytes(), dest, "test", "/ping", []string{"error"},
		tchannel.JSON, &Options{
			MaxRetries:    2,
			RetrySchedule: []time.Duration{time.Millisecond, time.Millisecond},
		})

	// lookup errors are swallowed and result in the key missing in the dests list, so a diverged error is expected
	s.EqualError(err, "key destinations have diverged")
}

func (s *ForwarderTestSuite) TestKeysDiverged() {
	var ping Ping

	dest, err := s.sender.Lookup("immediate fail")
	s.NoError(err)

	// no keys should result in destinations length of 0 during retry, causing abortion of request
	_, err = s.forwarder.ForwardRequest(ping.Bytes(), dest, "test", "/ping", nil, tchannel.JSON, &Options{
		MaxRetries:    2,
		RetrySchedule: []time.Duration{time.Millisecond, time.Millisecond},
	})

	s.EqualError(err, "key destinations have diverged")
}

func (s *ForwarderTestSuite) TestRequestTimesOut() {
	var ping Ping

	dest, err := s.sender.Lookup("unreachable")
	s.NoError(err)

	_, err = s.forwarder.ForwardRequest(ping.Bytes(), dest, "test", "/ping", []string{"unreachable"}, tchannel.JSON, &Options{
		// By providing a negative timeout the context will directly return with
		// a DeadlineExceeded error
		Timeout: -1,
	})

	s.EqualError(err, "request timed out")
}

func (s *ForwarderTestSuite) TestRequestRerouted() {
	var ping Ping
	var pong Pong

	dest, err := s.sender.Lookup("immediate fail")
	s.NoError(err)

	res, err := s.forwarder.ForwardRequest(ping.Bytes(), dest, "test", "/ping", []string{"reachable"},
		tchannel.JSON, &Options{
			MaxRetries:     1,
			RerouteRetries: true,
			RetrySchedule:  []time.Duration{time.Millisecond},
		})
	s.NoError(err, "expected request to be rerouted")

	s.NoError(json2.Unmarshal(res, &pong))
	s.Equal("correct pinging host", pong.From)
	s.Equal("Hello, world!", pong.Message)
}

func (s *ForwarderTestSuite) TestRequestNoReroutes() {
	var ping Ping

	dest, err := s.sender.Lookup("immediate fail")
	s.NoError(err)

	_, err = s.forwarder.ForwardRequest(ping.Bytes(), dest, "test", "/ping", []string{"reachable"},
		tchannel.JSON, &Options{
			MaxRetries:    1,
			RetrySchedule: []time.Duration{time.Millisecond},
		})

	s.EqualError(err, "max retries exceeded")
}

func (s *ForwarderTestSuite) TestInvalidInflightDecrement() {
	var wg sync.WaitGroup
	wg.Add(1)

	listener := &events.EventListener{}
	listener.On("HandleEvent", mock.AnythingOfTypeArgument("forward.InflightRequestsMiscountEvent")).Run(func(args mock.Arguments) {
		wg.Done()
	}).Return()

	s.forwarder.inflight = 0
	s.forwarder.AddListener(listener)
	defer s.forwarder.RemoveListener(listener)
	s.forwarder.decrementInflight()

	s.Assertions.Equal(int64(0), s.forwarder.inflight, "Expected inflight to stay at 0 when decremented at 0")

	// wait for HandleEvent with forward.InflightRequestsMiscountEvent being called
	wg.Wait()
}

func TestForwarderTestSuite(t *testing.T) {
	suite.Run(t, new(ForwarderTestSuite))
}

func TestSetForwardedHeader(t *testing.T) {
	// empty keys array test
	ctx, _ := thrift.NewContext(0 * time.Second)
	ctx = SetForwardedHeader(ctx, nil)
	assert.Equal(t, "[]", ctx.Headers()[ForwardedHeaderName], "expected the forwarding header to be set and be an empty array instead of null for the nil pointer")

	// preserve existing headers
	ctx, _ = thrift.NewContext(0 * time.Second)
	ctx = thrift.WithHeaders(ctx, map[string]string{
		"keep": "this key",
	})
	ctx = SetForwardedHeader(ctx, []string{"foo"})
	assert.Equal(t, "[\"foo\"]", ctx.Headers()[ForwardedHeaderName], "expected the forwarding header to be set to a serialized array of keys used in forwarding")
	assert.Equal(t, "this key", ctx.Headers()["keep"], "expected the header set before the forwarding header to still exist")

	// multiple keys encoded in the header
	ctx, _ = thrift.NewContext(0 * time.Second)
	ctx = SetForwardedHeader(ctx, []string{"key1", "key2"})
	assert.Equal(t, "[\"key1\",\"key2\"]", ctx.Headers()[ForwardedHeaderName], "expected the forwarding header to be set with both keys encoded")
}

func TestDeleteForwardedHeader(t *testing.T) {
	ctx, _ := thrift.NewContext(0 * time.Second)
	if DeleteForwardedHeader(ctx) {
		t.Errorf("ringpop claimed that the forwarded header was set before it was set")
	}
	ctx = SetForwardedHeader(ctx, nil)
	if !DeleteForwardedHeader(ctx) {
		t.Errorf("ringpop was not able to identify that the forwarded header was set")
	}

	ctx, _ = thrift.NewContext(0 * time.Second)
	ctx = thrift.WithHeaders(ctx, map[string]string{
		"keep": "this key",
	})
	if DeleteForwardedHeader(ctx) {
		t.Errorf("ringpop claimed that the forwarded header was set before it was set in the case of alread present headers")
	}
	ctx = SetForwardedHeader(ctx, nil)
	if !DeleteForwardedHeader(ctx) {
		t.Errorf("ringpop was not able to identify that the forwarded header was set in the case of alread present headers")
	}
}

// SerializeThrift takes a thrift struct and returns the serialized bytes
// of that struct using the thrift binary protocol. This is a temporary
// measure before frames can be forwarded directly past the endpoint to the proper
// destinaiton.
func SerializeThrift(s athrift.TStruct) ([]byte, error) {
	var b []byte
	var buffer = bytes.NewBuffer(b)

	transport := athrift.NewStreamTransportW(buffer)
	if err := s.Write(athrift.NewTBinaryProtocolTransport(transport)); err != nil {
		return nil, err
	}

	if err := transport.Flush(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// DeserializeThrift takes a byte slice and attempts to write it into the
// given thrift struct using the thrift binary protocol. This is a temporary
// measure before frames can be forwarded directly past the endpoint to the proper
// destinaiton.
func DeserializeThrift(b []byte, s athrift.TStruct) error {
	reader := bytes.NewReader(b)
	transport := athrift.NewStreamTransportR(reader)
	return s.Read(athrift.NewTBinaryProtocolTransport(transport))
}
