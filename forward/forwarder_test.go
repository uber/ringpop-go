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
	json2 "encoding/json"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/uber/tchannel-go"
	"github.com/uber/tchannel-go/json"
	"github.com/uber/tchannel-go/thrift"
	"golang.org/x/net/context"
)

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
}

func (s *ForwarderTestSuite) registerPong(address string, channel *tchannel.Channel) {
	handler := func(ctx json.Context, ping *Ping) (*Pong, error) {
		return &Pong{"Hello, world!", address}, nil
	}
	hmap := map[string]interface{}{"/ping": handler}

	s.Require().NoError(json.Register(channel, hmap,
		func(ctx context.Context, err error) {}))
}

func (s *ForwarderTestSuite) SetupSuite() {
	sender := &MockSender{}
	sender.On("Lookup", "me").Return("127.0.0.1:3001", nil)
	sender.On("Lookup", "other 1").Return("127.0.0.1:3002", nil)
	sender.On("Lookup", "other 2").Return("127.0.0.1:3003", nil)
	sender.On("Lookup", "unreachable").Return("127.0.0.2:3001", nil)
	sender.On("WhoAmI").Return("127.0.0.1:3001", nil)
	s.sender = sender

	channel, err := tchannel.NewChannel("test", nil)
	s.Require().NoError(err, "channel must be created successfully")
	s.channel = channel

	peer, err := tchannel.NewChannel("test", nil)
	s.Require().NoError(err, "channel must be created successfully")
	s.registerPong("127.0.0.1:3002", peer)
	s.Require().NoError(peer.ListenAndServe("127.0.0.1:3002"), "channel must listen")
	s.peer = peer

	s.forwarder = NewForwarder(s.sender, s.channel.GetSubChannel("forwarder"), nil)
}

func (s *ForwarderTestSuite) TearDownSuite() {
	s.channel.Close()
	s.peer.Close()
}

func (s *ForwarderTestSuite) TestForward() {
	var ping Ping
	var pong Pong

	dest, err := s.sender.Lookup("other 1")
	s.NoError(err)

	res, err := s.forwarder.ForwardRequest(ping.Bytes(), dest, "test", "/ping", []string{"other 1"},
		tchannel.JSON, nil)
	s.NoError(err, "expected request to be forwarded")

	s.NoError(json2.Unmarshal(res, &pong))
	s.Equal("127.0.0.1:3002", pong.From)
	s.Equal("Hello, world!", pong.Message)
}

func (s *ForwarderTestSuite) TestMaxRetries() {
	var ping Ping

	dest, err := s.sender.Lookup("other 2")
	s.NoError(err)

	_, err = s.forwarder.ForwardRequest(ping.Bytes(), dest, "test", "/ping", []string{"other 2"},
		tchannel.JSON, &Options{
			MaxRetries:    2,
			RetrySchedule: []time.Duration{time.Millisecond, time.Millisecond},
		})

	s.EqualError(err, "max retries exceeded")
}

func (s *ForwarderTestSuite) TestKeysDiverged() {
	var ping Ping

	dest, err := s.sender.Lookup("other 2")
	s.NoError(err)

	// no keys should result in destinations length of 0 during retry, causing abortion of request
	_, err = s.forwarder.ForwardRequest(ping.Bytes(), dest, "test", "/ping", nil, tchannel.JSON,
		&Options{MaxRetries: 2, RetrySchedule: []time.Duration{time.Millisecond, time.Millisecond}})

	s.EqualError(err, "key destinations have diverged")
}

func (s *ForwarderTestSuite) TestRequestTimesOut() {
	var ping Ping

	dest, err := s.sender.Lookup("unreachable")
	s.NoError(err)

	_, err = s.forwarder.ForwardRequest(ping.Bytes(), dest, "test", "/ping", nil, tchannel.JSON,
		&Options{Timeout: time.Millisecond})

	s.EqualError(err, "request timed out")
}

func (s *ForwarderTestSuite) TestRequestRerouted() {
	var ping Ping
	var pong Pong

	res, err := s.forwarder.ForwardRequest(ping.Bytes(), "127.0.0.1:3003", "test", "/ping", []string{"other 1"},
		tchannel.JSON, &Options{
			MaxRetries:     1,
			RerouteRetries: true,
			RetrySchedule:  []time.Duration{time.Millisecond},
		})
	s.NoError(err, "expected request to be rerouted")

	s.NoError(json2.Unmarshal(res, &pong))
	s.Equal("127.0.0.1:3002", pong.From)
	s.Equal("Hello, world!", pong.Message)
}

func (s *ForwarderTestSuite) TestRequestNoReroutes() {
	var ping Ping

	_, err := s.forwarder.ForwardRequest(ping.Bytes(), "127.0.0.1:3003", "test", "/ping", []string{"other 1"},
		tchannel.JSON, &Options{
			MaxRetries:    1,
			RetrySchedule: []time.Duration{time.Millisecond},
		})

	s.Error(err)
}

func (s *ForwarderTestSuite) TestRegisterListener() {
	listener := &EventListener{}
	listener.On("HandleEvent").Return()

	s.forwarder.RegisterListener(listener)
	s.Assertions.Equal(1, len(s.forwarder.listeners), "Expected 1 listener to be registered")

	// remove all listeners
	s.forwarder.listeners = nil
}

func (s *ForwarderTestSuite) TestEmit() {
	// wait for HandleEvent being called
	var wg sync.WaitGroup
	wg.Add(1) // expect 1 call to HandleEvent

	listener := &EventListener{}
	listener.On("HandleEvent", mock.Anything).Run(func(args mock.Arguments) {
		wg.Done()
	}).Return()

	s.forwarder.RegisterListener(listener)

	// emit an empty struct
	s.forwarder.emit(struct{}{})

	wg.Wait()

	// remove all listeners
	s.forwarder.listeners = nil
}

func (s *ForwarderTestSuite) TestEmit2() {
	// wait for HandleEvent being called
	var wg sync.WaitGroup
	wg.Add(2) // expect 2 calls to HandleEvent

	listener1 := &EventListener{}
	listener1.On("HandleEvent", mock.Anything).Run(func(args mock.Arguments) {
		wg.Done()
	}).Return()

	listener2 := &EventListener{}
	listener2.On("HandleEvent", mock.Anything).Run(func(args mock.Arguments) {
		wg.Done()
	}).Return()

	s.forwarder.RegisterListener(listener1)
	s.forwarder.RegisterListener(listener2)

	// emit an empty struct
	s.forwarder.emit(struct{}{})

	wg.Wait()

	// remove all listeners
	s.forwarder.listeners = nil
}

func (s *ForwarderTestSuite) TestInvalidInflightDecrement() {
	var wg sync.WaitGroup
	wg.Add(1)

	listener := &EventListener{}
	listener.On("HandleEvent", mock.AnythingOfTypeArgument("forward.InflightRequestsMiscountEvent")).Run(func(args mock.Arguments) {
		wg.Done()
	}).Return()

	s.forwarder.inflight = 0
	s.forwarder.RegisterListener(listener)
	s.forwarder.decrementInflight()

	s.Assertions.Equal(int64(0), s.forwarder.inflight, "Expected inflight to stay at 0 when decremented at 0")

	// wait for HandleEvent with forward.InflightRequestsMiscountEvent being called
	wg.Wait()
	s.forwarder.listeners = nil
}

func TestForwarderTestSuite(t *testing.T) {
	suite.Run(t, new(ForwarderTestSuite))
}

func TestSetForwardedHeader(t *testing.T) {
	ctx, _ := thrift.NewContext(0 * time.Second)
	ctx = SetForwardedHeader(ctx)
	if ctx.Headers()["ringpop-forwarded"] != "true" {
		t.Errorf("ringpop forwarding header is not set")
	}

	ctx, _ = thrift.NewContext(0 * time.Second)
	ctx = thrift.WithHeaders(ctx, map[string]string{
		"keep": "this key",
	})
	ctx = SetForwardedHeader(ctx)

	if ctx.Headers()["ringpop-forwarded"] != "true" {
		t.Errorf("ringpop forwarding header is not set if there were headers set already")
	}
	if ctx.Headers()["keep"] != "this key" {
		t.Errorf("ringpop forwarding header removed a header that was already present")
	}
}

func TestHasForwardedHeader(t *testing.T) {
	ctx, _ := thrift.NewContext(0 * time.Second)
	if HasForwardedHeader(ctx) {
		t.Errorf("ringpop claimed that the forwarded header was set before it was set")
	}
	ctx = SetForwardedHeader(ctx)
	if !HasForwardedHeader(ctx) {
		t.Errorf("ringpop was not able to identify that the forwarded header was set")
	}

	ctx, _ = thrift.NewContext(0 * time.Second)
	ctx = thrift.WithHeaders(ctx, map[string]string{
		"keep": "this key",
	})
	if HasForwardedHeader(ctx) {
		t.Errorf("ringpop claimed that the forwarded header was set before it was set in the case of alread present headers")
	}
	ctx = SetForwardedHeader(ctx)
	if !HasForwardedHeader(ctx) {
		t.Errorf("ringpop was not able to identify that the forwarded header was set in the case of alread present headers")
	}
}
