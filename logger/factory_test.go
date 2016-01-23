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
	"github.com/uber/ringpop-go/test/mocks"
	"testing"
)

type LoggerFactorySuite struct {
	suite.Suite
	mock    *mocks.Logger
	factory *LoggerFactory
}

func (s *LoggerFactorySuite) SetupTest() {
	s.mock = &mocks.Logger{}
	s.factory = NewLoggerFactory(s.mock)
}

func (s *LoggerFactorySuite) TestLoggerIdentity() {
	s.Exactly(s.factory.Logger("x"), s.factory.Logger("x"))
}

func (s *LoggerFactorySuite) TestDefaultLevel() {
	logger := s.factory.Logger("x").(*namedLogger)
	s.Equal(logger.min, Warn)
}

func (s *LoggerFactorySuite) TestSetLevel() {
	s.factory.SetLevel("x", Warn)
	logger := s.factory.Logger("x").(*namedLogger)
	s.Equal(logger.min, Warn)
	s.factory.SetLevel("x", Panic)
	s.Equal(logger.min, Panic)
}

func (s *LoggerFactorySuite) TestSetLevelLow() {
	s.Error(s.factory.SetLevel("x", unset))
}

func (s *LoggerFactorySuite) TestSetLevelHigh() {
	s.Error(s.factory.SetLevel("x", highestLevel+1))
}

func TestLoggerFactorySuite(t *testing.T) {
	suite.Run(t, new(LoggerFactorySuite))
}
