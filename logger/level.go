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

import "fmt"

// Don't expose level to outside world.
type level uint8

// Level represents a log level used to filter log messages.
type Level interface {
	// Level returns the underlying value of a log level.
	Level() level
}

func (lvl level) Level() level {
	return lvl
}

// String converts a log level to its string representation.
func (lvl level) String() string {
	switch lvl {
	case Trace:
		return "trace"
	case Debug:
		return "debug"
	case Info:
		return "info"
	case Warn:
		return "warn"
	case Error:
		return "error"
	case Fatal:
		return "fatal"
	case Panic:
		return "panic"
	case Off:
		return "off"
	}

	return "unknown"
}

// Parse converts a string to a log level.
func Parse(lvl string) (Level, error) {
	switch lvl {
	case "off":
		return Off, nil
	case "panic":
		return Panic, nil
	case "fatal":
		return Fatal, nil
	case "error":
		return Error, nil
	case "warn":
		return Warn, nil
	case "info":
		return Info, nil
	case "debug":
		return Debug, nil
	case "trace":
		return Trace, nil
	}

	var l Level
	return l, fmt.Errorf("not a valid Level: %q", lvl)
}

const (
	// Trace log level
	Trace level = iota
	// Debug log level
	Debug
	// Info log level
	Info
	// Warn log level
	Warn
	// Error log level
	Error
	// Fatal log level
	Fatal
	// Panic log level
	Panic
	// Off log level, turns off all logging.
	// Use this carefuly as is it skips Panic calls.
	Off
)

const defaultNotConfiguredNamedLogger = Warn
