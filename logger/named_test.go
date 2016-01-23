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

package logger

import (
	"github.com/stretchr/testify/suite"
	"github.com/uber-common/bark"
	"github.com/uber/ringpop-go/test/mocks"
	"strings"
	"testing"
)

type NamedLoggerSuite struct {
	mock   *mocks.Logger
	logger *namedLogger
	suite.Suite
}

func levelToMethod(level Level) string {
	// Trace is just internal, it calls into Debug
	if level == Trace {
		level = Debug
	}
	return strings.Title(level.String())
}

func (s *NamedLoggerSuite) SetupTest() {
	s.mock = &mocks.Logger{}
	s.logger = &namedLogger{
		logger: s.mock,
	}

	levels := []Level{Trace, Debug, Info, Warn, Error, Fatal, Panic}

	// Set all expected calls with the form:
	// Debug("debug") and Debugf("debug", "debug")
	for _, level := range levels {
		lString := level.String()
		s.mock.On(levelToMethod(level), []interface{}{lString})
		s.mock.On(levelToMethod(level)+"f", lString, []interface{}{lString})
	}
}

// Make sure .Debug("debug") and .Debugf("debug", "debug") was called
func (s *NamedLoggerSuite) assertMessageLoggedAt(level Level) {
	lString := level.String()
	s.mock.AssertCalled(s.T(), levelToMethod(level), []interface{}{lString})
	s.mock.AssertCalled(s.T(), levelToMethod(level)+"f", lString, []interface{}{lString})
}

// Make sure .Debug("debug") and .Debugf("debug", "debug") was not called
func (s *NamedLoggerSuite) assertMessageNotLoggedAt(level Level) {
	lString := level.String()
	s.mock.AssertNotCalled(s.T(), levelToMethod(level), []interface{}{lString})
	s.mock.AssertNotCalled(s.T(), levelToMethod(level)+"f", lString, []interface{}{lString})
}

// Call both .Debug("debug") and Debugf("debug", "debug") on the logger
func (s *NamedLoggerSuite) logMessageAt(level Level) {
	lString := level.String()
	switch level {
	case Trace:
		s.logger.Trace(lString)
		s.logger.Tracef(lString, lString)
	case Debug:
		s.logger.Debug(lString)
		s.logger.Debugf(lString, lString)
	case Info:
		s.logger.Info(lString)
		s.logger.Infof(lString, lString)
	case Warn:
		s.logger.Warn(lString)
		s.logger.Warnf(lString, lString)
	case Error:
		s.logger.Error(lString)
		s.logger.Errorf(lString, lString)
	case Fatal:
		s.logger.Fatal(lString)
		s.logger.Fatalf(lString, lString)
	case Panic:
		s.logger.Panic(lString)
		s.logger.Panicf(lString, lString)
	}
}

func (s *NamedLoggerSuite) TestDefaultLevel() {
	s.logMessageAt(Trace)
	s.assertMessageLoggedAt(Trace)
}

func (s *NamedLoggerSuite) TestOffLevel() {
	s.logger.setLevel(Off)
	s.logMessageAt(Panic)
	s.assertMessageNotLoggedAt(Panic)
}

func (s *NamedLoggerSuite) TestDebug() {
	s.logger.setLevel(Debug)
	s.logMessageAt(Trace)
	s.assertMessageNotLoggedAt(Trace)
	s.logMessageAt(Debug)
	s.assertMessageLoggedAt(Debug)
}

func (s *NamedLoggerSuite) TestInfo() {
	s.logger.setLevel(Info)
	s.logMessageAt(Debug)
	s.assertMessageNotLoggedAt(Debug)
	s.logMessageAt(Info)
	s.assertMessageLoggedAt(Info)
}

func (s *NamedLoggerSuite) TestWarn() {
	s.logger.setLevel(Warn)
	s.logMessageAt(Info)
	s.assertMessageNotLoggedAt(Info)
	s.logMessageAt(Warn)
	s.assertMessageLoggedAt(Warn)
}

func (s *NamedLoggerSuite) TestError() {
	s.logger.setLevel(Error)
	s.logMessageAt(Warn)
	s.assertMessageNotLoggedAt(Warn)
	s.logMessageAt(Error)
	s.assertMessageLoggedAt(Error)
}

func (s *NamedLoggerSuite) TestFatal() {
	s.logger.setLevel(Fatal)
	s.logMessageAt(Error)
	s.assertMessageNotLoggedAt(Error)
	s.logMessageAt(Fatal)
	s.assertMessageLoggedAt(Fatal)
}

func (s *NamedLoggerSuite) TestPanic() {
	s.logger.setLevel(Panic)
	s.logMessageAt(Fatal)
	s.assertMessageNotLoggedAt(Fatal)
	s.logMessageAt(Panic)
	s.assertMessageLoggedAt(Panic)
}

func (s *NamedLoggerSuite) TestWithField() {
	otherLogger := &mocks.Logger{}
	s.mock.On("WithField", "a", 1).Return(otherLogger)
	withFieldLogger := s.logger.WithField("a", 1).(*namedLogger)
	s.Equal(otherLogger, withFieldLogger.logger)
}

func (s *NamedLoggerSuite) TestWithFields() {
	otherLogger := &mocks.Logger{}
	s.mock.On("WithFields", nil).Return(otherLogger)
	withFieldLogger := s.logger.WithFields(nil).(*namedLogger)
	s.Equal(otherLogger, withFieldLogger.logger)
}

func (s *NamedLoggerSuite) TestFields() {
	fields := bark.Fields{}
	s.mock.On("Fields").Return(fields)
	s.Equal(s.logger.Fields(), fields)
}

func TestNamedLoggerSuite(t *testing.T) {
	suite.Run(t, new(NamedLoggerSuite))
}
