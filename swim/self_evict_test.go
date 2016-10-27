// Copyright (c) 2016 Uber Technologies, Inc.
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
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	eventsmocks "github.com/uber/ringpop-go/events/test/mocks"
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
	waitForConvergence(s.T(), 100, s.tnode, s.tpeer)
}

func (s *SelfEvictTestSuite) TearDownTest() {
	destroyNodes(s.tnode, s.tpeer)
}

func (s *SelfEvictTestSuite) TestSelfEvict_RegisterSelfEvictHook() {
	// have some hooks
	hook1 := new(MockSelfEvictHook)
	hookDuplicate := hook1
	hook2 := new(MockSelfEvictHook)

	err := s.node.RegisterSelfEvictHook(hook1)
	s.Assert().NoError(err, "expected no error when a hook gets attached for the first time")

	err = s.node.RegisterSelfEvictHook(hook2)
	s.Assert().NoError(err, "expected no error when a second hook gets attached for the first time")

	err = s.node.RegisterSelfEvictHook(hookDuplicate)
	s.Assert().Error(err, "expected an error when a duplicate hook gets attached to ringpop")
}

func (s *SelfEvictTestSuite) TestSelfEvict_RegisterSelfEvictHook_AfterEviction() {
	s.node.SelfEvict()

	hook1 := new(MockSelfEvictHook)
	err := s.node.RegisterSelfEvictHook(hook1)
	s.Assert().Error(err, "expected an error when adding a hook after self eviciton is called")
}

func (s *SelfEvictTestSuite) TestSelfEvict_SelfEvict() {
	hooks := &MockSelfEvictHook{}

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

func (s *SelfEvictTestSuite) TestSelfEvict_SelfEvict_InvokedTwice_AfterCompleted() {
	err := s.node.SelfEvict()
	s.Assert().NoError(err, "expected no error during self eviction")
	err = s.node.SelfEvict()
	s.Assert().Error(err, "expected an error when self evict is called for the second time.")
}

func (s *SelfEvictTestSuite) TestSelfEvict_SelfEvict_InvokedTwice_Concurrent() {
	// hooks are used to validate the sequence has only be run once
	hooks := &MockSelfEvictHook{}
	hooks.On("PreEvict").Return()
	hooks.On("PostEvict").Return()
	s.node.RegisterSelfEvictHook(hooks)

	var wg sync.WaitGroup
	var errorCount int32
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			err := s.node.SelfEvict()
			if err != nil {
				// if there is an error it should be ErrSelfEvictionInProgress
				s.Assert().Equal(ErrSelfEvictionInProgress, err, "expected an error that indicates self eviction is in progress")
				// keep track of the amount of errors, we only expect 1 in the end
				atomic.AddInt32(&errorCount, 1)
			}
			wg.Done()
		}()
	}

	wg.Wait()

	// because SelfEvict returns an error on the second invocation we expect the hooks to only be called once
	hooks.AssertNumberOfCalls(s.T(), "PreEvict", 1)
	hooks.AssertNumberOfCalls(s.T(), "PostEvict", 1)

	s.Assert().EqualValues(1, errorCount, "expected number of errors when SelfEvict is invoked twice")
}

func (s *SelfEvictTestSuite) TestSelfEvict_SelfEvict_SelfEvictedEvent() {
	listener := &eventsmocks.EventListener{}
	listener.On("HandleEvent", mock.AnythingOfType("SelfEvictedEvent")).Run(func(args mock.Arguments) {
		event := args[0].(SelfEvictedEvent)
		s.Assert().Equal(4, event.PhasesCount, "expected phases count to contain the number of phases executed")
	})

	// catch all
	listener.On("HandleEvent", mock.Anything).Return()

	s.node.AddListener(listener)
	err := s.node.SelfEvict()
	s.Assert().NoError(err, "expected no error during self eviction")

	listener.AssertCalled(s.T(), "HandleEvent", mock.AnythingOfType("SelfEvictedEvent"))
}

func (s *SelfEvictTestSuite) TestSelfEvict_SelfEvict_GossipFaulty() {
	address := s.node.Address()

	peerView, _ := s.peer.memberlist.Member(address)
	s.Assert().Equal(Alive, peerView.Status, "expected to be seen as alive by a peer before self eviction")

	err := s.node.SelfEvict()
	s.Assert().NoError(err, "expected no error during self eviction")

	peerView, _ = s.peer.memberlist.Member(address)
	s.Assert().Equal(Faulty, peerView.Status, "expected to be seen as faulty by a peer after self eviction")

	phase := s.node.selfEvict.phases[evicting]
	s.Require().Equal(evicting, phase.phase, "expected the evicting phase at this position in the phases phases")

	s.Assert().Equal(1, phase.numberOfPings, "expected 1 ping")
	s.Assert().Equal(int32(1), phase.numberOfSuccessfulPings, "expected 1 successful ping")
}

func (s *SelfEvictTestSuite) TestSelfEvict_SelfEvict_NegativeRatio() {
	s.node.selfEvict.options.PingRatio = -1
	address := s.node.Address()

	peerView, _ := s.peer.memberlist.Member(address)
	s.Assert().Equal(Alive, peerView.Status, "expected to be seen as alive by a peer before self eviction")

	err := s.node.SelfEvict()
	s.Assert().NoError(err, "expected no error during self eviction")

	// we want to make sure the node has been marked as faulty even when we do
	// not send pings to its peers
	s.Assert().Equal(Faulty, s.node.memberlist.local.Status, "expected the status of the local node to be Faulty")

	phase := s.node.selfEvict.phases[evicting]
	s.Require().Equal(evicting, phase.phase, "expected the evicting phase at this position in the phases phases")

	s.Assert().Equal(0, phase.numberOfPings, "expected no pings when PingRatio is a negative number")
	s.Assert().Equal(int32(0), phase.numberOfSuccessfulPings, "expected no successful ping")
}

func TestSelfEvictTestSuite(t *testing.T) {
	suite.Run(t, new(SelfEvictTestSuite))
}
