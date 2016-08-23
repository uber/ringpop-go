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

	"github.com/stretchr/testify/mock"

	"github.com/stretchr/testify/suite"
)

type SelfEvictTestSuite struct {
	suite.Suite
	tnode, tpeer *testNode
	node, peer   *Node
}

func (s *SelfEvictTestSuite) SetupTest() {
	s.tnode = newChannelNode(s.T())
	s.node = s.tnode.node
	s.tpeer = newChannelNode(s.T())
	s.peer = s.tpeer.node

	bootstrapNodes(s.T(), s.tnode, s.tpeer)
}

func (s *SelfEvictTestSuite) TearDownTest() {
	destroyNodes(s.tnode, s.tpeer)
}

func (s *SelfEvictTestSuite) TestSelfEvict_RegisterSelfEvictHook() {
	// have some hooks
	hook1 := new(MockSelfEvictHook)
	hook1.On("Name").Return("hook1")

	hookDuplicate := new(MockSelfEvictHook)
	hookDuplicate.On("Name").Return("hook1")

	hook2 := new(MockSelfEvictHook)
	hook2.On("Name").Return("hook2")

	err := s.node.RegisterSelfEvictHook(hook1)
	s.Assert().NoError(err, "expected no error when a hook gets attached for the first time")

	err = s.node.RegisterSelfEvictHook(hook2)
	s.Assert().NoError(err, "expected no error when a second hook gets attached for the first time")

	err = s.node.RegisterSelfEvictHook(hookDuplicate)
	s.Assert().Error(err, "expected an error when a duplicate hook gets attached to ringpop")
}

func (s *SelfEvictTestSuite) TestSelfEvict_SelfEvict() {
	hooks := &MockSelfEvictHook{}
	hooks.On("Name").Return("hooks")

	hooks.On("PreEvict").Run(func(args mock.Arguments) {
		phase := s.node.selfEvict.currentPhase()
		s.Require().NotNil(phase, "expected a phase when running pre evict")
		s.Assert().Equal(preEvict, phase.phase, "expected PreEvict to be called during the pre eviction phase")
	}).Return()

	hooks.On("PostEvict").Run(func(args mock.Arguments) {
		phase := s.node.selfEvict.currentPhase()
		s.Require().NotNil(phase, "expected a phase when running post evict")
		s.Assert().Equal(postEvict, phase.phase, "expected PostEvict to be called during the post eviction phase")
	}).Return()

	s.node.RegisterSelfEvictHook(hooks)
	err := s.node.SelfEvict()

	s.Assert().NoError(err, "expected no error during self eviction")

	var phases []evictionPhase
	for _, phase := range s.node.selfEvict.phases {
		phases = append(phases, phase.phase)
	}
	s.Assert().Equal([]evictionPhase{
		preEvict,
		evicting,
		postEvict,
		done,
	}, phases, "expected all phases to be present in the execution struct")

	s.Assert().Equal(Faulty, s.node.memberlist.local.Status, "expected the status of the local node to be Faulty")

	hooks.AssertNumberOfCalls(s.T(), "PreEvict", 1)
	hooks.AssertNumberOfCalls(s.T(), "PostEvict", 1)
}

func TestSelfEvictTestSuite(t *testing.T) {
	suite.Run(t, new(SelfEvictTestSuite))
}
