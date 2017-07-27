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

package logging

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/uber/ringpop-go/test/mocks/logger"
)

type DefaultLoggingTestSuite struct {
	suite.Suite
	mockLogger *mocklogger.Logger
}

func (s *DefaultLoggingTestSuite) SetupTest() {
	s.mockLogger = &mocklogger.Logger{}
	SetLogger(s.mockLogger)
	// Set expected calls
	s.mockLogger.On("Warn", mock.Anything)
}

func (s *DefaultLoggingTestSuite) TestMessagePropagation() {
	Logger("named").Warn("msg")
	s.mockLogger.AssertCalled(s.T(), "Warn", []interface{}{"msg"})
}

func (s *DefaultLoggingTestSuite) TestSetLevel() {
	SetLevel("name", Fatal)
	Logger("named").Warn("msg")
	s.mockLogger.AssertCalled(s.T(), "Warn", []interface{}{"msg"})
}

func (s *DefaultLoggingTestSuite) TestSetLevels() {
	SetLevels(map[string]Level{"named": Fatal})
	Logger("named").Warn("msg")
	s.mockLogger.AssertNotCalled(s.T(), "Warn", []interface{}{"msg"})
}

func TestDefaultLoggingTestSuite(t *testing.T) {
	suite.Run(t, new(DefaultLoggingTestSuite))
}
