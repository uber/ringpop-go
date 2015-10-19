// Copyright (c) 2015 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

// Package bark provides an abstraction for loggers and stats reporters used in Uber's
// Go libraries.  It decouples these libraries slightly from specific
// logger implementations; for example, the popular open source library
// logrus, which offers no interfaces (and thus cannot be, for instance, easily mocked).
//
// Users may choose to implement the interfaces themselves or to use the provided wrappers
// for logrus loggers and cactus/go-statsd-client stats reporters.
package bark

import (
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/cactus/go-statsd-client/statsd"
)

// Logger is an interface for loggers accepted by Uber's libraries.
type Logger interface {
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
	WithFields(keyValues LogFields) Logger

	// Return map fields associated with this logger, if any (i.e. if this logger was returned from WithField[s])
	// If no fields are set, returns nil
	Fields() Fields
}

// Logfields is an interface for dictionaries passed to Logger's WithFields logging method.
// It exists to provide a layer of indirection so code already using other
// "Fields" types can be changed to use bark.Logger instances without
// any refactoring (sidestepping the fact that if we used a concrete
// type, then yourloggingmodule.Fields would not assignable to bark.Fields).
type LogFields interface {
	Fields() map[string]interface{}
}

// Fields is the concrete LogFields type, as in myLogger.WithFields(bark.Fields{"foo": "bar"}).Info("Fields!")
type Fields map[string]interface{}

// Fields provides indirection for broader compatibility for the LogFields interface type.
func (f Fields) Fields() map[string]interface{} {
	return f
}

// NewLoggerFromLogrus creates a bark-compliant wrapper for a logrus-brand logger.
func NewLoggerFromLogrus(logger *logrus.Logger) Logger {
	return newBarkLogrusLogger(logger)
}

// Tags is an alias of map[string]string, a type for tags associated with a statistic
type Tags map[string]string

// StatsReporter is an interface for statsd-like stats reporters accepted by Uber's libraries.
// Its methods take optional tag dictionaries which may be ignored by concrete implementations.
type StatsReporter interface {
	// Increment a statsd-like counter with optional tags
	IncCounter(name string, tags Tags, value int64)

	// Increment a statsd-like gauge ("set" of the value) with optional tags
	UpdateGauge(name string, tags Tags, value int64)

	// Record a statsd-like timer with optional tags
	RecordTimer(name string, tags Tags, d time.Duration)
}

// NewStatsReporterFromCactus creates a bark-compliant wrapper for a cactus-brand statsd Statter.
func NewStatsReporterFromCactus(statter statsd.Statter) StatsReporter {
	return newBarkCactusStatsReporter(statter)
}
