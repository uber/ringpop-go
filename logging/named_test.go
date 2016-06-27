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

	"github.com/stretchr/testify/suite"
	"github.com/uber-common/bark"
)

type NamedLoggerTestSuite struct {
	suite.Suite
	recv   *recv
	logger *namedLogger
}

type recv struct {
	LogName   string
	WantLevel Level
	Fields    bark.Fields
	Format    string
	Msg       []interface{}
}

func (r *recv) Log(logName string, wantLevel Level, fields bark.Fields, msg []interface{}) {
	r.LogName, r.WantLevel, r.Fields, r.Format, r.Msg = logName, wantLevel, fields, "", msg
}

func (r *recv) Logf(logName string, wantLevel Level, fields bark.Fields, format string, msg []interface{}) {
	r.LogName, r.WantLevel, r.Fields, r.Format, r.Msg = logName, wantLevel, fields, format, msg
}

func (s *NamedLoggerTestSuite) SetupTest() {
	s.recv = &recv{}
	s.logger = &namedLogger{
		name:      "name",
		forwardTo: s.recv,
		fields:    bark.Fields{"key": 1},
	}
}

func (s *NamedLoggerTestSuite) TestForwarding() {
	cases := []struct {
		level    Level
		logMeth  func(l *namedLogger, args ...interface{})
		logMethf func(l *namedLogger, format string, args ...interface{})
	}{
		{Debug, (*namedLogger).Debug, (*namedLogger).Debugf},
		{Info, (*namedLogger).Info, (*namedLogger).Infof},
		{Warn, (*namedLogger).Warn, (*namedLogger).Warnf},
		{Error, (*namedLogger).Error, (*namedLogger).Errorf},
		{Fatal, (*namedLogger).Fatal, (*namedLogger).Fatalf},
		{Panic, (*namedLogger).Panic, (*namedLogger).Panicf},
	}
	for _, c := range cases {
		// logger.Debug("msg")
		c.logMeth(s.logger, "msg")

		expected := &recv{
			LogName:   "name",
			WantLevel: c.level,
			Fields:    bark.Fields{"key": 1},
			Format:    "",
			Msg:       []interface{}{"msg"},
		}

		s.Equal(s.recv, expected, "Log messages should be forwarded unmodified.")

		*s.recv = recv{} // reset

		// logger.Debugf("format", "msg")
		c.logMethf(s.logger, "format", "msg")

		expected.Format = "format"
		s.Equal(s.recv, expected, "Log messages should be forwarded unmodified.")
	}
}

func (s *NamedLoggerTestSuite) TestWithFields() {
	l := s.logger.WithField("a", 1).WithFields(bark.Fields{"b": 2, "a": 3})
	l.Debug("msg")
	expected := &recv{
		LogName:   "name",
		WantLevel: Debug,
		Fields:    bark.Fields{"key": 1, "a": 3, "b": 2},
		Msg:       []interface{}{"msg"},
	}
	s.Equal(s.recv, expected, "Multiple fields should be merged.")
}

func (s *NamedLoggerTestSuite) TestFields() {
	s.Equal(s.logger.Fields(), bark.Fields{"key": 1}, "A logger should return its fields.")
}

func TestNamedLoggerTestSuite(t *testing.T) {
	suite.Run(t, new(NamedLoggerTestSuite))
}
