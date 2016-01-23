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

type LevelSuite struct {
	suite.Suite
}

// Make sure that Level -> string -> Level conversion returns the same thing.
func (s *LevelSuite) assertParseLevel(level Level) {
	levelAsString := level.String()
	newLevel, err := Parse(levelAsString)
	s.NoError(err)
	s.Exactly(newLevel, level)
}

func (s *LevelSuite) TestParseAllLevels() {
	allLevels := []Level{Trace, Info, Debug, Warn, Error, Fatal, Panic, Off}
	for _, level := range allLevels {
		s.assertParseLevel(level)
	}
}

func (s *LevelSuite) TestParseBadLevel() {
	_, err := Parse("badlevel")
	s.Error(err)
}

func (s *LevelSuite) TestUnknownLevel() {
	s.Equal(Level(100).String(), "unknown")
}

func TestLevelSuite(t *testing.T) {
	suite.Run(t, new(LevelSuite))
}
