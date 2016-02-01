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
	"fmt"
	"strconv"
)

// Level is the severity level of a log message.
type Level uint8

// Additional levels can be created from integers between Debug and maxLevel.
const (
	// Panic log level
	Panic Level = iota
	// Fatal log level
	Fatal
	// Error log level
	Error
	// Warn log level
	Warn
	// Info log level
	Info
	// Debug log level
	Debug

	// maxLevel is the maximum valid log level.
	// This is used internally for boundary check.
	maxLevel = int(255)
)

// String converts a log level to its string representation.
func (lvl Level) String() string {
	switch lvl {
	case Panic:
		return "panic"
	case Fatal:
		return "fatal"
	case Error:
		return "error"
	case Warn:
		return "warn"
	case Info:
		return "info"
	case Debug:
		return "debug"
	}
	return strconv.Itoa(int(lvl))
}

// Parse converts a string to a log level.
func Parse(lvl string) (Level, error) {
	switch lvl {
	case "fatal":
		return Fatal, nil
	case "panic":
		return Panic, nil
	case "error":
		return Error, nil
	case "warn":
		return Warn, nil
	case "info":
		return Info, nil
	case "debug":
		return Debug, nil
	}

	level, err := strconv.Atoi(lvl)
	if level > maxLevel {
		err = fmt.Errorf("invalid level value: %s", lvl)
	}
	return Level(level), err
}
