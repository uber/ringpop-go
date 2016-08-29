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

type emittingEventRegistrar interface {
	EventEmitter
	EventRegistrar
}

type testEvent struct{}

type EventsTestSuite struct {
	suite.Suite

	e emittingEventRegistrar
}

func (s *EventsTestSuite) SetupTest() {
}

func (s *EventsTestSuite) TearDownTest() {
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
	s.e.RegisterListener(l1)
	// make sure listener does stick around for next tests
	defer s.e.DeregisterListener(l1)

	// expect listener to be invoked twice
	wg2.Add(2)
	l2 := &MockEventListener{}
	l2.On("HandleEvent", testEvent{}).Run(func(args mock.Arguments) {
		wg2.Done()
	}).Return()
	s.e.RegisterListener(l2)
	// make sure listener does stick around for next tests
	defer s.e.DeregisterListener(l2)

	// emit event by value
	s.e.EmitEvent(testEvent{})

	wg1.Wait()

	l1.AssertCalled(s.T(), "HandleEvent", testEvent{})
	l1.AssertNumberOfCalls(s.T(), "HandleEvent", 1)

	// remove listener
	s.e.DeregisterListener(l1)

	// emit new event that should not be emitted to the listener
	s.e.EmitEvent(testEvent{})

	wg2.Wait()

	l1.AssertCalled(s.T(), "HandleEvent", testEvent{})
	l1.AssertNumberOfCalls(s.T(), "HandleEvent", 1)

	l2.AssertNumberOfCalls(s.T(), "HandleEvent", 2)
}

func (s *EventsTestSuite) TestRegisterNilListener() {
	s.e.RegisterListener(nil)
	s.e.EmitEvent(testEvent{})
}

func (s *EventsTestSuite) TestDeregisterUnregisteredListener() {
	unregistered := &MockEventListener{}
	s.e.DeregisterListener(unregistered)
}

func (s *EventsTestSuite) TestDeregisterNilListener() {
	s.e.DeregisterListener(nil)
}

func (s *EventsTestSuite) TestDeregisterDuringHandleEvent() {
	var wg1 sync.WaitGroup

	// expect listener to be invoked only once
	wg1.Add(1)
	l1 := &MockEventListener{}
	l1.On("HandleEvent", testEvent{}).Run(func(args mock.Arguments) {
		s.e.DeregisterListener(l1)
		wg1.Done()
	}).Return()
	s.e.RegisterListener(l1)
	// make sure listener does stick around for next tests
	defer s.e.DeregisterListener(l1)

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

	s.e.RegisterListener(l1)
	s.e.RegisterListener(l1)

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
