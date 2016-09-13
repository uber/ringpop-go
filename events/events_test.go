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

package events

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
)

type testEvent struct{}

type EventsTestSuite struct {
	suite.Suite

	e EventEmitter
}

func (s *EventsTestSuite) TestEventListenerRegistration() {
	var wg1 sync.WaitGroup
	var wg2 sync.WaitGroup

	// expect listener to be invoked only once
	wg1.Add(1)
	l1 := &MockEventListener{}
	l1.On("HandleEvent", testEvent{}).Run(func(args mock.Arguments) {
		wg1.Done()
	}).Return()
	s.e.AddListener(l1)
	// make sure listener does stick around for next tests
	defer s.e.RemoveListener(l1)

	// expect listener to be invoked twice
	wg2.Add(2)
	l2 := &MockEventListener{}
	l2.On("HandleEvent", testEvent{}).Run(func(args mock.Arguments) {
		wg2.Done()
	}).Return()
	s.e.AddListener(l2)
	// make sure listener does stick around for next tests
	defer s.e.RemoveListener(l2)

	// emit event by value
	s.e.EmitEvent(testEvent{})

	wg1.Wait()

	l1.AssertCalled(s.T(), "HandleEvent", testEvent{})
	l1.AssertNumberOfCalls(s.T(), "HandleEvent", 1)

	// remove listener
	s.e.RemoveListener(l1)

	// emit new event that should not be emitted to the listener
	s.e.EmitEvent(testEvent{})

	wg2.Wait()

	l1.AssertCalled(s.T(), "HandleEvent", testEvent{})
	l1.AssertNumberOfCalls(s.T(), "HandleEvent", 1)

	l2.AssertNumberOfCalls(s.T(), "HandleEvent", 2)
}

func (s *EventsTestSuite) TestRegisterNilListener() {
	added := s.e.AddListener(nil)
	s.Assert().False(added, "expected a nil listener to not be added")

	// this is to test that there is no nil pointer dereference when emitting a
	// value after a nil listener has been added
	s.e.EmitEvent(testEvent{})
}

func (s *EventsTestSuite) TestEmitEventWithNoListener() {
	// test and make sure that emitting to an empty event listener works.
	s.e.EmitEvent(testEvent{})
}

func (s *EventsTestSuite) TestDeregisterUnregisteredListener() {
	unregistered := &MockEventListener{}
	removed := s.e.RemoveListener(unregistered)
	s.Assert().False(removed, "expected to not remove a listener that never has been added")
}

func (s *EventsTestSuite) TestDeregisterNilListener() {
	removed := s.e.RemoveListener(nil)
	s.Assert().False(removed, "expect a nil listener to not be removed since it has not been added")

	s.e.AddListener(nil)

	removed = s.e.RemoveListener(nil)
	s.Assert().False(removed, "expect a nil listener to not be removed since it can't be added")
}

func (s *EventsTestSuite) TestRemoveListenerDuringHandleEvent() {
	var wg1 sync.WaitGroup

	// expect listener to be invoked only once
	wg1.Add(1)
	l1 := &MockEventListener{}
	l1.On("HandleEvent", testEvent{}).Run(func(args mock.Arguments) {
		s.e.RemoveListener(l1)
		wg1.Done()
	}).Return()
	s.e.AddListener(l1)
	// make sure listener does stick around for next tests
	defer s.e.RemoveListener(l1)

	s.e.EmitEvent(testEvent{})

	wg1.Wait()

	l1.AssertCalled(s.T(), "HandleEvent", testEvent{})
	l1.AssertNumberOfCalls(s.T(), "HandleEvent", 1)

	s.e.EmitEvent(testEvent{})

	l1.AssertCalled(s.T(), "HandleEvent", testEvent{})
	l1.AssertNumberOfCalls(s.T(), "HandleEvent", 1)
}

func (s *EventsTestSuite) TestRegisterTwice() {
	var wg1 sync.WaitGroup

	// expect listener to be invoked only once
	wg1.Add(1)
	l1 := &MockEventListener{}
	l1.On("HandleEvent", testEvent{}).Run(func(args mock.Arguments) {
		wg1.Done()
	}).Return()
	defer s.e.RemoveListener(l1)

	added := s.e.AddListener(l1)
	s.Assert().True(added, "expected a listener to be added on the first occurance")
	added = s.e.AddListener(l1)
	s.Assert().False(added, "expected a listener to not be added on the second occurance")

	s.e.EmitEvent(testEvent{})

	wg1.Wait()

	l1.AssertCalled(s.T(), "HandleEvent", testEvent{})
	l1.AssertNumberOfCalls(s.T(), "HandleEvent", 1)
}

func TestSyncEventEmitter(t *testing.T) {
	suite.Run(t, &EventsTestSuite{
		e: &SyncEventEmitter{},
	})
}

func TestAsyncEventEmitter(t *testing.T) {
	suite.Run(t, &EventsTestSuite{
		e: &AsyncEventEmitter{},
	})
}
