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
	"testing"
)

type dummyReceiver struct {
	msg  *namedMsg
	msgf *namedfMsg
}

func (recv *dummyReceiver) Log(m *namedMsg) {
	recv.msg = m
}

func (recv *dummyReceiver) Logf(m *namedfMsg) {
	recv.msgf = m
}

type NamedLoggerSuite struct {
	suite.Suite
	recv   *dummyReceiver
	logger *namedLogger
}

func (s *NamedLoggerSuite) SetupTest() {
	s.recv = &dummyReceiver{}
	s.logger = &namedLogger{
		name:   "name",
		fields: Fields{"key": 1},
		recv:   s.recv,
	}
}

// Test the messages are forwarded untouched with the right level.
func (s *NamedLoggerSuite) TestForwarding() {
	cases := []struct {
		meth  func(logger *namedLogger, args ...interface{})
		methf func(logger *namedLogger, format string, args ...interface{})
		level level
	}{
		{meth: (*namedLogger).Trace, methf: (*namedLogger).Tracef, level: Trace},
		{meth: (*namedLogger).Debug, methf: (*namedLogger).Debugf, level: Debug},
		{meth: (*namedLogger).Info, methf: (*namedLogger).Infof, level: Info},
		{meth: (*namedLogger).Warn, methf: (*namedLogger).Warnf, level: Warn},
		{meth: (*namedLogger).Error, methf: (*namedLogger).Errorf, level: Error},
		{meth: (*namedLogger).Fatal, methf: (*namedLogger).Fatalf, level: Fatal},
		{meth: (*namedLogger).Panic, methf: (*namedLogger).Panicf, level: Panic},
	}
	for _, c := range cases {
		c.meth(s.logger, "msg")
		expected := &namedMsg{
			name:   "name",
			level:  c.level,
			fields: Fields{"key": 1},
			args:   []interface{}{"msg"},
		}
		s.Equal(s.recv.msg, expected)
		c.methf(s.logger, "format", "msg")
		s.Equal(s.recv.msgf, &namedfMsg{
			namedMsg: expected,
			format:   "format",
		})
	}
}

func (s *NamedLoggerSuite) TestFieldChain() {
	logger := s.logger.WithField("test", 1).WithField("key", 0)
	logger.Debug("msg")
	expected := &namedMsg{
		name:   "name",
		level:  Debug,
		fields: Fields{"key": 0, "test": 1},
		args:   []interface{}{"msg"},
	}
	s.Equal(s.recv.msg, expected)
	logger.Debugf("format", "msg")
	s.Equal(s.recv.msgf, &namedfMsg{
		namedMsg: expected,
		format:   "format",
	})
}

func (s *NamedLoggerSuite) TestFieldsChain() {
	logger := s.logger.WithFields(Fields{"a": 1, "b": 2})
	logger = logger.WithFields(Fields{"b": 3, "key": 0})
	logger.Debug("msg")
	expected := &namedMsg{
		name:   "name",
		level:  Debug,
		fields: Fields{"key": 0, "a": 1, "b": 3},
		args:   []interface{}{"msg"},
	}
	s.Equal(s.recv.msg, expected)
	logger.Debugf("format", "msg")
	s.Equal(s.recv.msgf, &namedfMsg{
		namedMsg: expected,
		format:   "format",
	})
}

func TestNamedLoggerSuite(t *testing.T) {
	suite.Run(t, new(NamedLoggerSuite))
}
