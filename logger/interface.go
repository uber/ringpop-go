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
	"github.com/uber-common/bark"
)

// Logger is similar to bark.Logger but with an extra Trace method.
type Logger interface {
	// Log at trace level
	Trace(args ...interface{})
	// Log at trace level with fmt.Printf-like formatting
	Tracef(format string, args ...interface{})

	// Log at debug level
	Debug(args ...interface{})
	// Log at debug level with fmt.Printf-like formatting
	Debugf(format string, args ...interface{})

	// Log at info level
	Info(args ...interface{})
	// Log at info level with fmt.Printf-like formatting
	Infof(format string, args ...interface{})

	// Log at warning level
	Warn(args ...interface{})
	// Log at warning level with fmt.Printf-like formatting
	Warnf(format string, args ...interface{})

	// Log at error level
	Error(args ...interface{})
	// Log at error level with fmt.Printf-like formatting
	Errorf(format string, args ...interface{})

	// Log at fatal level, then terminate process (irrecoverable)
	Fatal(args ...interface{})
	// Log at fatal level with fmt.Printf-like formatting, then terminate process (irrecoverable)
	Fatalf(format string, args ...interface{})

	// Log at panic level, then panic (recoverable)
	Panic(args ...interface{})
	// Log at panic level with fmt.Printf-like formatting, then panic (recoverable)
	Panicf(format string, args ...interface{})

	// Return a logger with the specified key-value pair set, to be logged in a subsequent normal logging call
	WithField(key string, value interface{}) Logger
	// Return a logger with the specified key-value pairs set, to be  included in a subsequent normal logging call
	WithFields(keyValues Fields) Logger

	// We don't use Fields()
}

// Factory knows how to produce named loggers.
type Factory interface {
	// Logger returns a named logger.
	// If a minimum level was not set for this named logger, it defaults
	// to Warn.
	Logger(name string) Logger

	// WithField returns a new factory. All named loggers produced with
	// this factory preserve this field.
	// The method can be chained multiple times.
	WithField(key string, value interface{}) Factory

	// WithFields is similar with WithField but is useful to set multiple
	// fields at once.
	WithFields(fields Fields) Factory
}

// Facility can be used to configure and produce named loggers.
type Facility interface {
	Factory

	// SetLevel sets the minimum level for all loggers sharing the same
	// name. A named logger only emits messages with a level equal to or
	// greater than this level. This also updates existing loggers.
	SetLevel(name string, level Level) error

	// SetLogger sets the underlying logger implementation.
	SetLogger(logger bark.Logger)
}
