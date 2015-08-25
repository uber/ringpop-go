package replica

import (
	"testing"
	"time"

	"golang.org/x/net/context"

	"github.com/stretchr/testify/suite"
	"github.com/uber/ringpop-go/forward"
	"github.com/uber/tchannel/golang"
	"github.com/uber/tchannel/golang/json"
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

func (d dummySender) WhoAmI() string {
	return d.local
}

func (d dummySender) Lookup(key string) string {
	return d.lookup
}

func (d dummySender) LookupN(key string, n int) []string {
	return d.lookupN
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

func (s *ReplicatorTestSuite) RegisterHandler(ch tchannel.Registrar, address string) {
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
	var pong Pong

	var dests = s.sender.LookupN("key", 3)

	// parallel
	responses, err := s.replicator.Read([]string{"key"}, ping, pong, "/ping", foptsTimeout, nil)
	s.NoError(err, "calls should be replicated")
	s.Len(responses, 3, "expected response from each peer")
	for _, response := range responses {
		body, ok := response.Body.(*Pong)
		s.Require().True(ok, "expected pong response")
		s.Contains(dests, body.From)
	}

	// serial sequential
	responses, err = s.replicator.Read([]string{"key"}, ping, pong, "/ping", foptsTimeout, &Options{
		FanoutMode: SerialSequential,
	})
	s.NoError(err, "calls should be replicated")
	s.Len(responses, 3, "expected response from each peer")
	for _, response := range responses {
		body, ok := response.Body.(*Pong)
		s.Require().True(ok, "expected pong response")
		s.Contains(dests, body.From)
	}

	// serial balanced
	responses, err = s.replicator.Read([]string{"key"}, ping, pong, "/ping", foptsTimeout, &Options{
		FanoutMode: SerialBalanced,
	})
	s.NoError(err, "calls should be replicated")
	s.Len(responses, 3, "expected response from each peer")
	for _, response := range responses {
		body, ok := response.Body.(*Pong)
		s.Require().True(ok, "expected pong response")
		s.Contains(dests, body.From)
	}
}

func (s *ReplicatorTestSuite) TestMultipleKeys() {
	s.ResetLookupN()

	var ping = Ping{From: "127.0.0.1:3001"}
	var pong Pong

	var dests = s.sender.LookupN("key", 3)

	// parallel
	responses, err := s.replicator.Read([]string{"key1", "key2"}, ping, pong, "/ping", foptsTimeout, nil)
	s.NoError(err, "calls should be replicated")
	s.Len(responses, 3, "expected response from each peer")
	for _, response := range responses {
		body, ok := response.Body.(*Pong)
		s.Require().True(ok, "expected pong response")
		s.Contains(dests, body.From)
	}

}

// TODO: some of these tests are kind of redudant because read and write call the same code...
// ...oh well.
func (s *ReplicatorTestSuite) TestWrite() {
	s.ResetLookupN()

	var ping = Ping{From: "127.0.0.1:3001"}
	var pong Pong

	var dests = s.sender.LookupN("key", 3)

	// parallel
	responses, err := s.replicator.Write([]string{"key"}, ping, pong, "/ping", foptsTimeout, nil)
	s.NoError(err, "calls should be replicated")
	s.Len(responses, 3, "expected response from each peer")
	for _, response := range responses {
		body, ok := response.Body.(*Pong)
		s.Require().True(ok, "expected pong response")
		s.Contains(dests, body.From)
	}

	// serial sequential
	responses, err = s.replicator.Write([]string{"key"}, ping, pong, "/ping", foptsTimeout, &Options{
		FanoutMode: SerialSequential,
	})
	s.NoError(err, "calls should be replicated")
	s.Len(responses, 3, "expected response from each peer")
	for _, response := range responses {
		body, ok := response.Body.(*Pong)
		s.Require().True(ok, "expected pong response")
		s.Contains(dests, body.From)
	}

	// serial balanced
	responses, err = s.replicator.Write([]string{"key"}, ping, pong, "/ping", foptsTimeout, &Options{
		FanoutMode: SerialBalanced,
	})
	s.NoError(err, "calls should be replicated")
	s.Len(responses, 3, "expected response from each peer")
	for _, response := range responses {
		body, ok := response.Body.(*Pong)
		s.Require().True(ok, "expected pong response")
		s.Contains(dests, body.From)
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
	var pong Pong

	_, err := s.replicator.Read([]string{"key"}, ping, pong, "/ping", foptsTimeout, &Options{
		RValue: 3,
		NValue: 3,
	})

	s.EqualError(err, "rw value not satisfied")

	_, err = s.replicator.Read([]string{"key"}, ping, pong, "/ping", foptsTimeout, &Options{
		RValue:     3,
		NValue:     3,
		FanoutMode: SerialSequential,
	})

	s.EqualError(err, "rw value not satisfied")
}

func (s *ReplicatorTestSuite) TestInvalidRWValue() {
	s.ResetLookupN()

	_, err := s.replicator.Read([]string{}, Ping{}, Pong{}, "/ping", nil, &Options{
		RValue: 3,
		NValue: 1,
	})

	s.EqualError(err, "rw value cannot exceed n value")

	_, err = s.replicator.Write([]string{}, Ping{}, Pong{}, "/ping", nil, &Options{
		WValue: 3,
		NValue: 1,
	})

	s.EqualError(err, "rw value cannot exceed n value")
}

func (s *ReplicatorTestSuite) TestNotEnoughDests() {
	s.sender.lookupN = []string{}

	_, err := s.replicator.Read([]string{}, Ping{}, Pong{}, "/ping", nil, &Options{
		RValue: 3,
	})

	s.EqualError(err, "rw value not satisfied by destination")
}

func (s *ReplicatorTestSuite) TestInvalidResponseTypes() {
	s.ResetLookupN()

	var someStruct = struct{}{}

	_, err := s.replicator.Read([]string{"key"}, 0, &someStruct, "nah", nil, nil)
	s.EqualError(err, "invalid type for response", "pointer to struct is bad")

	var badMap map[int]int

	_, err = s.replicator.Read([]string{"key"}, 0, badMap, "nah", nil, nil)
	s.EqualError(err, "invalid type for response", "non map[string]... is bad")

	var n int

	_, err = s.replicator.Read([]string{"key"}, 0, n, "nah", nil, nil)
	s.EqualError(err, "invalid type for response", "numbers are bad")

	var str string

	_, err = s.replicator.Read([]string{"key"}, 0, str, "nah", nil, nil)
	s.EqualError(err, "invalid type for response", "strings are bad")

	var validMap1 map[string]int
	var validMap2 map[string]interface{}
	var validMap3 map[string]struct{ x, y, z int }

	err = validateResponseType(validMap1)
	s.NoError(err)

	err = validateResponseType(validMap2)
	s.NoError(err)

	err = validateResponseType(validMap3)
	s.NoError(err)

	err = validateResponseType(struct{ x, y, z int }{})
	s.NoError(err)
}

func TestReplicatorTestSuite(t *testing.T) {
	suite.Run(t, new(ReplicatorTestSuite))
}
