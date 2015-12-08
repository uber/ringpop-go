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
	json2 "encoding/json"
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
	"github.com/uber/ringpop-go/forward"
	"github.com/uber/ringpop-go/shared"
	"github.com/uber/tchannel-go"
	"github.com/uber/tchannel-go/json"
	"golang.org/x/net/context"
)

var foptsTimeout = &forward.Options{
	MaxRetries:    2,
	RetrySchedule: []time.Duration{time.Millisecond, time.Millisecond},
}

type dummySender struct {
	local   string
	lookup  string
	lookupN []string
}

func (d dummySender) WhoAmI() (string, error) {
	return d.local, nil
}

func (d dummySender) Lookup(key string) (string, error) {
	return d.lookup, nil
}

func (d dummySender) LookupN(key string, n int) ([]string, error) {
	return d.lookupN, nil
}

type ReplicatorTestSuite struct {
	suite.Suite
	sender     *dummySender
	channel    *tchannel.Channel
	replicator *Replicator
	peers      map[string]*tchannel.Channel
}

type Ping struct {
	From string `json:"from"`
}

func (p Ping) Bytes() []byte {
	data, _ := json2.Marshal(p)
	return data
}

type Pong struct {
	Message string `json:"message"`
	From    string `json:"from"`
}

func (s *ReplicatorTestSuite) SetupSuite() {
	ch, err := tchannel.NewChannel("service", nil)
	s.Require().NoError(err, "channel must create successfully")
	s.channel = ch

	peerAddresses := []string{"127.0.0.1:3002", "127.0.0.1:3003", "127.0.0.1:3004"}
	for _, address := range peerAddresses {
		s.AddPeer(address)
	}

	s.sender = &dummySender{"127.0.0.1:3001", "127.0.0.1:3001", peerAddresses}

	s.replicator = NewReplicator(s.sender, ch.GetSubChannel("ping"), nil, nil)
}

func (s *ReplicatorTestSuite) AddPeer(address string) {
	if s.peers == nil {
		s.peers = make(map[string]*tchannel.Channel)
	}

	ch, err := tchannel.NewChannel("ping", nil)
	s.Require().NoError(err, "channel must create successfully")
	s.RegisterHandler(ch, address)
	s.Require().NoError(ch.ListenAndServe(address))
	s.peers[address] = ch
}

func (s *ReplicatorTestSuite) ResetLookupN() {
	var lookupN []string
	for peer := range s.peers {
		lookupN = append(lookupN, peer)
	}

	s.sender.lookupN = lookupN
}

func (s *ReplicatorTestSuite) RegisterHandler(ch shared.SubChannel, address string) {
	handler := map[string]interface{}{
		"/ping": func(ctx json.Context, ping *Ping) (*Pong, error) {
			s.Equal(ping.From, "127.0.0.1:3001")
			return &Pong{"Hello, world!", address}, nil
		},
	}

	s.Require().NoError(json.Register(ch, handler, func(ctx context.Context, err error) {
		s.Fail("calls shouldn't fail")
	}))
}

func (s *ReplicatorTestSuite) TearDownSuite() {
	s.channel.Close()
	for _, peer := range s.peers {
		peer.Close()
	}
}

func (s *ReplicatorTestSuite) TestRead() {
	s.ResetLookupN()

	var ping = Ping{From: "127.0.0.1:3001"}
	var dests, err = s.sender.LookupN("key", 3)
	s.NoError(err)

	// parallel
	responses, err := s.replicator.Read([]string{"key"}, ping.Bytes(), "/ping", foptsTimeout, nil)
	s.NoError(err, "calls should be replicated")
	s.Len(responses, 3, "expected response from each peer")
	for _, response := range responses {
		var pong Pong
		s.Require().NoError(json2.Unmarshal(response.Body, &pong))
		s.Contains(dests, pong.From)
	}

	// serial sequential
	responses, err = s.replicator.Read([]string{"key"}, ping.Bytes(), "/ping", foptsTimeout, &Options{
		FanoutMode: SerialSequential,
	})
	s.NoError(err, "calls should be replicated")
	s.Len(responses, 3, "expected response from each peer")
	for _, response := range responses {
		var pong Pong
		s.Require().NoError(json2.Unmarshal(response.Body, &pong))
		s.Contains(dests, pong.From)
	}
	// serial balanced
	responses, err = s.replicator.Read([]string{"key"}, ping.Bytes(), "/ping", foptsTimeout, &Options{
		FanoutMode: SerialBalanced,
	})
	s.NoError(err, "calls should be replicated")
	s.Len(responses, 3, "expected response from each peer")
	for _, response := range responses {
		var pong Pong
		s.Require().NoError(json2.Unmarshal(response.Body, &pong))
		s.Contains(dests, pong.From)
	}
}

func (s *ReplicatorTestSuite) TestMultipleKeys() {
	s.ResetLookupN()

	var ping = Ping{From: "127.0.0.1:3001"}
	var dests, err = s.sender.LookupN("key", 3)
	s.NoError(err)

	// parallel
	responses, err := s.replicator.Read([]string{"key1", "key2"}, ping.Bytes(), "/ping", foptsTimeout, nil)
	s.NoError(err, "calls should be replicated")
	s.Len(responses, 3, "expected response from each peer")
	for _, response := range responses {
		var pong Pong
		s.Require().NoError(json2.Unmarshal(response.Body, &pong))
		s.Contains(dests, pong.From)
	}

}

// TODO: some of these tests are kind of redudant because read and write call the same code...
// ...oh well.
func (s *ReplicatorTestSuite) TestWrite() {
	s.ResetLookupN()

	var ping = Ping{From: "127.0.0.1:3001"}
	var dests, err = s.sender.LookupN("key", 3)
	s.NoError(err)

	// parallel
	responses, err := s.replicator.Write([]string{"key"}, ping.Bytes(), "/ping", foptsTimeout, nil)
	s.NoError(err, "calls should be replicated")
	s.Len(responses, 3, "expected response from each peer")
	for _, response := range responses {
		var pong Pong
		s.Require().NoError(json2.Unmarshal(response.Body, &pong))
		s.Contains(dests, pong.From)
	}

	// serial sequential
	responses, err = s.replicator.Write([]string{"key"}, ping.Bytes(), "/ping", foptsTimeout, &Options{
		FanoutMode: SerialSequential,
	})
	s.NoError(err, "calls should be replicated")
	s.Len(responses, 3, "expected response from each peer")
	for _, response := range responses {
		var pong Pong
		s.Require().NoError(json2.Unmarshal(response.Body, &pong))
		s.Contains(dests, pong.From)
	}
	// serial balanced
	responses, err = s.replicator.Write([]string{"key"}, ping.Bytes(), "/ping", foptsTimeout, &Options{
		FanoutMode: SerialBalanced,
	})
	s.NoError(err, "calls should be replicated")
	s.Len(responses, 3, "expected response from each peer")
	for _, response := range responses {
		var pong Pong
		s.Require().NoError(json2.Unmarshal(response.Body, &pong))
		s.Contains(dests, pong.From)
	}
}

func (s *ReplicatorTestSuite) TestRWValueNotSatisfied() {
	s.sender.lookupN = []string{
		"127.0.0.1:3002",
		"127.0.0.1:3003",
		"127.0.0.1:3012",
		"127.0.0.1:3013",
	}

	var ping = Ping{From: "127.0.0.1:3001"}

	_, err := s.replicator.Read([]string{"key"}, ping.Bytes(), "/ping", foptsTimeout, &Options{
		RValue: 3,
		NValue: 3,
	})

	s.EqualError(err, "rw value not satisfied")

	_, err = s.replicator.Read([]string{"key"}, ping.Bytes(), "/ping", foptsTimeout, &Options{
		RValue:     3,
		NValue:     3,
		FanoutMode: SerialSequential,
	})

	s.EqualError(err, "rw value not satisfied")
}

func (s *ReplicatorTestSuite) TestInvalidRWValue() {
	s.ResetLookupN()

	_, err := s.replicator.Read([]string{}, Ping{}.Bytes(), "/ping", nil, &Options{
		RValue: 3,
		NValue: 1,
	})

	s.EqualError(err, "rw value cannot exceed n value")

	_, err = s.replicator.Write([]string{}, Ping{}.Bytes(), "/ping", nil, &Options{
		WValue: 3,
		NValue: 1,
	})

	s.EqualError(err, "rw value cannot exceed n value")
}

func (s *ReplicatorTestSuite) TestNotEnoughDests() {
	s.sender.lookupN = []string{}

	_, err := s.replicator.Read([]string{}, Ping{}.Bytes(), "/ping", nil, &Options{
		RValue: 3,
	})

	s.EqualError(err, "rw value not satisfied by destination")
}

func TestReplicatorTestSuite(t *testing.T) {
	suite.Run(t, new(ReplicatorTestSuite))
}
