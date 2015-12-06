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

package ringpop

import (
	"testing"

	"github.com/stretchr/testify/suite"
	"github.com/uber/ringpop-go/internal/native"
	"github.com/uber/tchannel-go"
)

type RingTestSuite struct {
	suite.Suite
	ringpop *Ringpop
	ring    native.HashRing
}

func (s *RingTestSuite) SetupTest() {
	ch, err := tchannel.NewChannel("test", nil)
	s.Require().NoError(err, "channel must create successfully")

	s.ringpop, err = New("test", Identity("127.0.0.1:3001"), Channel(ch))

	s.NoError(err)
	s.NotNil(s.ringpop)

	s.ring = s.ringpop.ring
}

func (s *RingTestSuite) TearDownTest() {
	s.ringpop.Destroy()
}

func (s *RingTestSuite) TestAddServer() {
	s.ring.AddServer("server1")
	s.ring.AddServer("server2")

	s.True(s.ring.HasServer("server1"), "expected server to be in ring")
	s.True(s.ring.HasServer("server2"), "expected server to be in ring")
	s.False(s.ring.HasServer("server3"), "expected server to not be in ring")

	s.ring.AddServer("server1")
	s.True(s.ring.HasServer("server1"), "expected server to be in ring")
}

func (s *RingTestSuite) TestRemoveServer() {
	s.ring.AddServer("server1")
	s.ring.AddServer("server2")

	s.True(s.ring.HasServer("server1"), "expected server to be in ring")
	s.True(s.ring.HasServer("server2"), "expected server to be in ring")

	s.ring.RemoveServer("server1")

	s.False(s.ring.HasServer("server1"), "expected server to not be in ring")
	s.True(s.ring.HasServer("server2"), "expected server to be in ring")

	s.ring.RemoveServer("server3")

	s.False(s.ring.HasServer("server3"), "expected server to not be in ring")
}

func (s *RingTestSuite) TestChecksumChanges() {
	checksum := s.ring.Checksum()

	s.ring.AddServer("server1")
	s.ring.AddServer("server2")

	s.NotEqual(checksum, s.ring.Checksum(), "expected checksum to have changed on server add")

	checksum = s.ring.Checksum()

	s.ring.RemoveServer("server1")

	s.NotEqual(checksum, s.ring.Checksum(), "expected checksum to have changed on server remove")
}

func (s *RingTestSuite) TestServerCount() {
	s.Equal(0, s.ring.ServerCount(), "expected one server to be in ring")

	s.ring.AddServer("server1")
	s.ring.AddServer("server2")
	s.ring.AddServer("server3")

	s.Equal(3, s.ring.ServerCount(), "expected three servers to be in ring")

	s.ring.RemoveServer("server1")

	s.Equal(2, s.ring.ServerCount(), "expected two servers to be in ring")
}

func (s *RingTestSuite) TestAddRemoveServers() {
	add := []string{"server1", "server2"}
	remove := []string{"server3", "server4"}

	s.ring.AddRemoveServers(remove, nil)

	s.Equal(2, s.ring.ServerCount(), "expected two servers to be in ring")

	oldChecksum := s.ring.Checksum()

	s.ring.AddRemoveServers(add, remove)

	s.Equal(2, s.ring.ServerCount(), "expected two servers to be in ring")

	s.NotEqual(oldChecksum, s.ring.Checksum(), "expected checksum to change")
}

func (s *RingTestSuite) TestLookup() {
	s.ring.AddServer("server1")
	s.ring.AddServer("server2")

	_, ok := s.ring.Lookup("key")

	s.True(ok, "expected Lookup to hash key to a server")

	s.ring.RemoveServer("server1")
	s.ring.RemoveServer("server2")
	s.ring.RemoveServer(s.ringpop.WhoAmI())

	_, ok = s.ring.Lookup("key")

	s.False(ok, "expected Lookup to find no server for key to hash to")
}

func (s *RingTestSuite) TestLookupN() {
	servers := s.ring.LookupN("nil", 5)
	s.Len(servers, 0, "expected no servers")

	unique := make(map[string]bool)

	addresses := genAddresses(1, 1, 10)
	s.ring.AddRemoveServers(addresses, nil)

	servers = s.ring.LookupN("key", 5)
	s.Len(servers, 5, "expected five servers to be returned by lookup")
	for _, server := range servers {
		unique[server] = true
	}
	s.Len(unique, 5, "expected to get five unique servers")

	unique = make(map[string]bool)

	servers = s.ring.LookupN("another key", 100)
	s.Len(servers, 10, "expected to get max number of servers")
	for _, server := range servers {
		unique[server] = true
	}
	s.Len(unique, 10, "expected to get 10 unique servers")

	unique = make(map[string]bool)

	s.ring.RemoveServer(addresses[0])
	servers = s.ring.LookupN("yet another key", 10)
	s.Len(servers, 9, "expected to get nine servers")
	for _, server := range servers {
		unique[server] = true
	}
	s.Len(unique, 9, "expected to get nine unique servers")
}

func TestRingTestSuite(t *testing.T) {
	suite.Run(t, new(RingTestSuite))
}
