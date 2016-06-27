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
)

type LogLevelTestSuite struct {
	suite.Suite
}

// Test that level -> string -> level conversion produces the same level
func (s *LogLevelTestSuite) TestLevels() {
	// This is a custom level, higer than Debug
	customLevel := Debug + 10
	levels := []Level{Panic, Fatal, Error, Warn, Info, Debug, customLevel}
	for _, level := range levels {
		gotLevel, err := Parse(level.String())
		s.NoError(err, "Converting a Level to a string and back should not fail.")
		s.Equal(gotLevel, level, "Converting a Level to a string and back should produce the same Level.")
	}
}

func (s *LogLevelTestSuite) TestInvalidLevel() {
	levels := []string{"", "1234", "abc"}
	for _, level := range levels {
		_, err := Parse(level)
		s.Error(err, "Converting an invalid string to a Level should fail.")
	}
}

func TestLogLevelTestSuite(t *testing.T) {
	suite.Run(t, new(LogLevelTestSuite))
}
