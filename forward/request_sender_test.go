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

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/uber/ringpop-go/events"
	"github.com/uber/ringpop-go/shared"
	"github.com/uber/ringpop-go/test/mocks/logger"
	"github.com/uber/tchannel-go"
)

type requestSenderTestSuite struct {
	suite.Suite
	requestSender *requestSender
	mockSender    *MockSender
}

type dummies struct {
	channel  shared.SubChannel
	dest     string
	emitter  events.EventEmitter
	endpoint string
	format   tchannel.Format
	keys     []string
	options  *Options
	request  []byte
	service  string
}

func (s *requestSenderTestSuite) SetupTest() {
	mockSender := &MockSender{}
	mockSender.On("WhoAmI").Return("", nil)
	dummies := s.newDummies(mockSender)
	s.requestSender = newRequestSender(mockSender, dummies.emitter,
		dummies.channel, dummies.request, dummies.keys, dummies.dest,
		dummies.service, dummies.endpoint, dummies.format, dummies.options)
	s.mockSender = mockSender
}

func (s *requestSenderTestSuite) newDummies(mockSender *MockSender) *dummies {
	channel, err := tchannel.NewChannel("dummychannel", nil)
	s.NoError(err, "no error creating TChannel")

	return &dummies{
		channel:  channel,
		dest:     "dummydest",
		endpoint: "/dummyendpoint",
		emitter:  NewForwarder(mockSender, channel),
		format:   tchannel.Thrift,
		keys:     []string{},
		options:  &Options{},
		request:  []byte{},
		service:  "dummyservice",
	}
}

func newDummyLogger() *mocklogger.Logger {
	logger := &mocklogger.Logger{}
	logger.On("WithFields", mock.Anything).Return(logger)
	logger.On("Warn", mock.Anything).Return(nil)
	return logger
}

func stubLookupWithKeys(mockSender *MockSender, dest string, keys ...string) {
	for _, key := range keys {
		mockSender.On("Lookup", key).Return(dest, nil)
	}
}

func (s *requestSenderTestSuite) TestAttemptRetrySendsMultipleKeys() {
	s.mockSender.On("WhoAmI").Return("192.0.2.1:0", nil)
	stubLookupWithKeys(s.mockSender, "192.0.2.1:1", "key1", "key2")

	// Mutate keys that are looked up prior to attempted retry.
	s.requestSender.keys = []string{"key1", "key2"}

	_, err := s.requestSender.AttemptRetry()
	s.NotEqual(errDestinationsDiverged, err, "not a diverged error")
}

func (s *requestSenderTestSuite) TestLookupKeysDedupes() {
	// 2 groups of 2 keys. Each group hashes to a different destination.
	stubLookupWithKeys(s.mockSender, "192.0.2.1:1", "key1", "key2")
	stubLookupWithKeys(s.mockSender, "192.0.2.1:2", "key3", "key4")

	dests := s.requestSender.LookupKeys([]string{"key1", "key2"})
	s.Len(dests, 1, "dedupes single destination for multiple keys")

	dests = s.requestSender.LookupKeys([]string{"key1", "key2", "key3", "key4"})
	s.Len(dests, 2, "dedupes multiple destinations for multiple keys")
}

func TestRequestSenderTestSuite(t *testing.T) {
	suite.Run(t, new(requestSenderTestSuite))
}
