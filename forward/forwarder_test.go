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
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/uber/tchannel/golang"
	"github.com/uber/tchannel/golang/json"
	"golang.org/x/net/context"
)

type DummySender struct{ local, lookup string }

func (d DummySender) Lookup(key string) string {
	return d.lookup
}

func (d DummySender) WhoAmI() string {
	return d.local
}

type ForwarderTestSuite struct {
	suite.Suite
	sender    *DummySender
	forwarder *Forwarder
	channel   *tchannel.Channel
	peers     map[string]*tchannel.Channel
}

type Ping struct {
	Message string `json:"message"`
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

func (s *ForwarderTestSuite) AddPeer(address string) {
	channel, err := tchannel.NewChannel("test", nil)
	s.Require().NoError(err, "channel must be created successfully")

	s.registerPong(address, channel)

	s.Require().NoError(channel.ListenAndServe(address), "channel must listen")

	s.peers[address] = channel
}

func (s *ForwarderTestSuite) SetupTest() {
	s.peers = make(map[string]*tchannel.Channel)

	s.sender = &DummySender{"127.0.0.1:3001", "127.0.0.1:3001"}

	channel, err := tchannel.NewChannel("test", nil)
	s.Require().NoError(err, "channel must be created successfully")
	s.channel = channel

	s.forwarder = NewForwarder(s.sender, s.channel.GetSubChannel("forwarder"), nil)
}

func (s *ForwarderTestSuite) TearDownTest() {
	s.channel.Close()
	for _, peer := range s.peers {
		peer.Close()
	}
}

func (s *ForwarderTestSuite) TestForward() {
	var ping Ping
	var pong Pong

	s.AddPeer("127.0.0.1:3002")

	s.sender.lookup = "127.0.0.1:3002"
	dest := s.sender.Lookup("some key")

	err := s.forwarder.ForwardRequest(ping, &pong, dest, "test", "/ping", []string{"some key"}, nil)

	s.NoError(err, "expected request to be forwarded")
	s.Equal("127.0.0.1:3002", pong.From)
	s.Equal("Hello, world!", pong.Message)
}

func (s *ForwarderTestSuite) TestMaxRetries() {
	var ping Ping
	var pong Pong

	s.sender.lookup = "127.0.0.1:3002"
	dest := s.sender.Lookup("some key")

	err := s.forwarder.ForwardRequest(ping, &pong, dest, "test", "/ping", []string{"some key"},
		&Options{MaxRetries: 2, RetrySchedule: []time.Duration{time.Millisecond, time.Millisecond}})

	s.EqualError(err, "max retries exceeded")
}

func (s *ForwarderTestSuite) TestKeysDiverged() {
	var ping Ping
	var pong Pong

	s.sender.lookup = "127.0.0.1:3002"
	dest := s.sender.Lookup("some key")

	// no keys should result in destinations length of 0 during retry, causing abortion of request
	err := s.forwarder.ForwardRequest(ping, &pong, dest, "test", "/ping", []string{},
		&Options{MaxRetries: 2, RetrySchedule: []time.Duration{time.Millisecond, time.Millisecond}})

	s.EqualError(err, "key destinations have diverged")
}

func (s *ForwarderTestSuite) TestRequestTimesOut() {
	var ping Ping
	var pong Pong

	s.sender.lookup = "127.0.0.2:3001"
	dest := s.sender.Lookup("some key")

	err := s.forwarder.ForwardRequest(ping, &pong, dest, "test", "/ping", []string{},
		&Options{Timeout: time.Millisecond})

	s.EqualError(err, "request timed out")
}

func (s *ForwarderTestSuite) TestRequestRerouted() {
	var ping Ping
	var pong Pong

	s.AddPeer("127.0.0.1:3002")
	s.sender.lookup = "127.0.0.1:3002"

	err := s.forwarder.ForwardRequest(ping, &pong, "127.0.0.1:3003", "test", "/ping", []string{"some key"},
		&Options{MaxRetries: 1, RetrySchedule: []time.Duration{time.Millisecond}, RerouteRetries: true})

	s.NoError(err, "expected request to be rerouted")
	s.Equal("127.0.0.1:3002", pong.From)
	s.Equal("Hello, world!", pong.Message)
}

func (s *ForwarderTestSuite) TestRequestNoReroutes() {
	var ping Ping
	var pong Pong

	s.AddPeer("127.0.0.1:3002")
	s.sender.lookup = "127.0.0.1:3002"

	err := s.forwarder.ForwardRequest(ping, &pong, "127.0.0.1:3003", "test", "/ping", []string{"some key"},
		&Options{MaxRetries: 1, RetrySchedule: []time.Duration{time.Millisecond}})

	s.Error(err)
}

func TestForwarderTestSuite(t *testing.T) {
	suite.Run(t, new(ForwarderTestSuite))
}
