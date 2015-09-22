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

package swim

import (
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// TODO: add bootstrap tests for seeding host list from file

type BootstrapTestSuite struct {
	suite.Suite
	tnode *testNode
	node  *Node
	peers []*testNode
}

func (s *BootstrapTestSuite) SetupTest() {
	s.tnode = newChannelNode(s.T(), "127.0.0.1:3001")
	s.node = s.tnode.node
}

func (s *BootstrapTestSuite) TearDownTest() {
	destroyNodes(append(s.peers, s.tnode)...)
}

func (s *BootstrapTestSuite) TestBootstrapOk() {
	s.peers = genChannelNodes(s.T(), genAddresses(1, 2, 6))

	bootstrapNodes(s.T(), append(s.peers, s.tnode)...)
}

func (s *BootstrapTestSuite) TestBootstrapTimesOut() {
	_, err := s.node.Bootstrap(&BootstrapOptions{
		Hosts:           genAddresses(1, 1, 10),
		MaxJoinDuration: time.Millisecond,
	})

	s.Error(err, "expected bootstrap to exceed join duration")
}

func (s *BootstrapTestSuite) TestBootstrapJoinsTimeOut() {
	_, err := s.node.Bootstrap(&BootstrapOptions{
		Hosts:           append(genAddresses(2, 1, 5), s.node.Address()),
		MaxJoinDuration: time.Millisecond,
		JoinTimeout:     time.Millisecond / 2,
	})

	s.Error(err, "expected bootstrap to exceed join duration")
}

func (s *BootstrapTestSuite) TestBootstrapDestroy() {
	var lerr lError

	go func() {
		_, err := s.node.Bootstrap(&BootstrapOptions{
			Hosts:       genAddresses(1, 1, 10),
			JoinTimeout: time.Millisecond,
		})
		lerr.Set(err)
	}()

	time.Sleep(2 * time.Millisecond)
	s.node.Destroy()
	time.Sleep(2 * time.Millisecond)

	s.Error(lerr.Err(), "expected bootstrap to fail if node gets destroyed")
}

func TestBootstrapTestSuite(t *testing.T) {
	suite.Run(t, new(BootstrapTestSuite))
}
