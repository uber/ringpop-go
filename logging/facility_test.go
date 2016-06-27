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
	"strings"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/uber-common/bark"
)

type LogFacilityTestSuite struct {
	suite.Suite
	mockLogger *MockLogger
	facility   *Facility
}

func (s *LogFacilityTestSuite) SetupTest() {
	s.mockLogger = &MockLogger{}
	s.facility = NewFacility(s.mockLogger)
	// Set expected calls
	for _, meth := range []string{"Debug", "Info", "Warn", "Error", "Fatal", "Panic"} {
		s.mockLogger.On(meth, mock.Anything)
		s.mockLogger.On(meth+"f", mock.Anything, mock.Anything)
	}
	s.mockLogger.On("WithFields", mock.Anything).Return(s.mockLogger)
	s.mockLogger.On("WithField", mock.Anything, mock.Anything).Return(s.mockLogger)
}

func (s *LogFacilityTestSuite) TestForwarding() {
	levels := []Level{Debug, Info, Warn, Error, Fatal, Panic}
	for _, level := range levels {
		methName := strings.Title(level.String())
		msg := []interface{}{"msg"}

		fields := bark.Fields{"a": 1}
		s.facility.Log("name", level, fields, msg)
		s.mockLogger.AssertCalled(s.T(), methName, msg)
		s.mockLogger.AssertCalled(s.T(), "WithFields", fields)

		fields = bark.Fields{"b": 2}
		s.facility.Logf("name", level, fields, "format %s", msg)
		s.mockLogger.AssertCalled(s.T(), methName+"f", "format %s", msg)
		s.mockLogger.AssertCalled(s.T(), "WithFields", fields)
	}
}

func (s *LogFacilityTestSuite) TestSetLogger() {
	newLogger := &MockLogger{}
	s.facility.SetLogger(newLogger)
	newLogger.On("Debug", mock.Anything)
	msg := []interface{}{"msg"}
	s.facility.Log("name", Debug, nil, msg)
	newLogger.AssertCalled(s.T(), "Debug", msg)
}

func (s *LogFacilityTestSuite) TestSetLevel() {
	s.facility.SetLevel("name", Warn)
	s.afterLevelIsSet()
}

func (s *LogFacilityTestSuite) TestSetLevels() {
	s.facility.SetLevels(map[string]Level{"name": Warn})
	s.afterLevelIsSet()
}

func (s *LogFacilityTestSuite) afterLevelIsSet() {
	msg := []interface{}{"msg"}
	s.facility.Log("name", Debug, nil, msg)
	s.facility.Logf("name", Debug, nil, "format %s", msg)
	s.mockLogger.AssertNotCalled(s.T(), "Debug", msg)
	s.mockLogger.AssertNotCalled(s.T(), "Debugf", "format %s", msg)
}

func (s *LogFacilityTestSuite) TestSetLevelError() {
	s.Error(s.facility.SetLevel("name", Panic), "Setting a severity level above Fatal should fail.")
}

func (s *LogFacilityTestSuite) TestSetLevelsError() {
	s.Error(s.facility.SetLevels(map[string]Level{"name": Panic}), "Setting a severity level above Fatal should fail.")
}

func (s *LogFacilityTestSuite) TestNamedLogger() {
	logger := s.facility.Logger("name")
	logger.Debug("msg")
	s.mockLogger.AssertCalled(s.T(), "Debug", []interface{}{"msg"})
}

func TestLogFacilityTestSuite(t *testing.T) {
	suite.Run(t, new(LogFacilityTestSuite))
}
