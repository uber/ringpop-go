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

	"github.com/stretchr/testify/suite"
	"github.com/uber/ringpop-go/util"
)

type MemberlistIterTestSuite struct {
	suite.Suite
	node        *Node
	m           *memberlist
	i           *memberlistIter
	incarnation int64
}

func (s *MemberlistIterTestSuite) SetupTest() {
	s.incarnation = util.TimeNowMS()
	s.node = NewNode("test", "127.0.0.1:3001", nil, nil)
	s.m = s.node.memberlist
	s.i = s.m.Iter()

	s.m.MakeAlive(s.node.Address(), s.incarnation)
}

func (s *MemberlistIterTestSuite) TearDownTest() {
	s.node.Destroy()
}

func (s *MemberlistIterTestSuite) TestNoneUseable() {
	// populate the membership with two unusable nodes.
	s.m.Update([]Change{
		Change{
			Address:     "127.0.0.1:3002",
			Incarnation: s.incarnation,
			Status:      Faulty,
		},
		Change{
			Address:     "127.0.0.1:3003",
			Incarnation: s.incarnation,
			Status:      Leave,
		},
	})

	member, ok := s.i.Next()
	s.Nil(member, "expected member to be nil")
	s.False(ok, "expected no usable members")
}

func (s *MemberlistIterTestSuite) TestIterOverFive() {
	addresses := fakeHostPorts(1, 1, 2, 6)

	for _, address := range addresses {
		s.m.MakeAlive(address, s.incarnation)
	}

	iterated := make(map[string]int)

	for i := 0; i < 20; i++ {
		member, ok := s.i.Next()
		s.Require().NotNil(member, "expected member to not be nil")
		s.True(ok, "expected a pingable member to be found")

		iterated[member.Address]++
	}

	s.Len(iterated, 5, "expected only 5 members to be iterated over")
	for _, iterations := range iterated {
		s.Equal(4, iterations, "expected each member to be iterated over four times")
	}
}

func (s *MemberlistIterTestSuite) TestIterSkips() {
	s.m.Update([]Change{
		Change{Address: "127.0.0.1:3002", Incarnation: s.incarnation, Status: Alive},
		Change{Address: "127.0.0.1:3003", Incarnation: s.incarnation, Status: Faulty},
		Change{Address: "127.0.0.1:3004", Incarnation: s.incarnation, Status: Alive},
		Change{Address: "127.0.0.1:3005", Incarnation: s.incarnation, Status: Leave},
	})

	iterated := make(map[string]int)

	for i := 0; i < 10; i++ {
		member, ok := s.i.Next()
		s.Require().NotNil(member, "member cannot be nil")
		s.True(ok, "expected a pingable member to be found")

		iterated[member.Address]++
	}

	s.Len(iterated, 2, "expected faulty, leave, local to be skipped")
	for _, iterations := range iterated {
		s.Equal(5, iterations, "expected pingable members to be iterated over twice")
	}
}

func TestMemberlistIterTestSuite(t *testing.T) {
	suite.Run(t, new(MemberlistIterTestSuite))
}
