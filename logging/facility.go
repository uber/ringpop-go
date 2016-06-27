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
	"sync"

	"github.com/uber-common/bark"
)

// Facility is a collection of named loggers that can be configured
// individually.
type Facility struct {
	logger bark.Logger
	levels map[string]Level

	mu sync.RWMutex
}

// NewFacility creates a new log facility with the specified logger as the
// underlying logger. If no logger is passed, a no-op implementation is used.
func NewFacility(log bark.Logger) *Facility {
	if log == nil {
		log = NoLogger
	}
	return &Facility{
		logger: log,
		levels: make(map[string]Level),
	}
}

// SetLevels is like SetLevel but for multiple named loggers.
func (f *Facility) SetLevels(levels map[string]Level) error {
	for logName, level := range levels {
		if level < Fatal {
			return fmt.Errorf("cannot set a level above %s for %s", Fatal, logName)
		}
	}
	// Prevent changing levels while a message is logged.
	f.mu.Lock()
	defer f.mu.Unlock()
	for logName, level := range levels {
		f.levels[logName] = level
	}
	return nil
}

// SetLevel sets the minimum severity level for a named logger. All messages
// produced by a named logger with a severity lower than the one set here are
// silenced.
// In most logger implementations, both Fatal and Panic stop the execution.
// This means that those levels only produce a single, and often important,
// message. Because of that, it's an error to pass a level above Fatal to this
// method.
func (f *Facility) SetLevel(logName string, level Level) error {
	if level < Fatal {
		return fmt.Errorf("cannot set a level above %s for %s", Fatal, logName)
	}
	// Prevent changing levels while a message is logged.
	f.mu.Lock()
	defer f.mu.Unlock()
	f.levels[logName] = level
	return nil
}

// SetLogger sets the underlying logger. All log messages produced, that are
// not silenced, are propagated to this logger.
func (f *Facility) SetLogger(log bark.Logger) {
	// Prevent changing the logger while a message is logged.
	f.mu.Lock()
	defer f.mu.Unlock()
	f.logger = log
}

// Logger returns a new named logger.
func (f *Facility) Logger(logName string) bark.Logger {
	return &namedLogger{
		name:      logName,
		forwardTo: f,
	}

}

// Log logs messages with a severity level equal to or higher than the one set
// with SetLevel. If that's not the case, the message is silenced.
// If the logName was not previously configured with SetLevel, the messages are
// never silenced.
// Instead on using this method directly, one can call Logger method to get a
// bark.Logger instance bound to a specific name.
func (f *Facility) Log(logName string, wantLevel Level, fields bark.Fields, msg []interface{}) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if setLevel, ok := f.levels[logName]; ok {
		// For me this is confusing, so here's a clarification.
		// The levels are consecutive integers starting from 0 defined in this order:
		// Panic=0, Fatal=1, Error=2, Warning=3, etc...
		// For the condition to make more sense, consider a named
		// logger set to Fatal and an incoming Error message.
		if setLevel < wantLevel {
			return
		}
	}
	logger := f.logger
	// If there are any fields, apply them.
	if len(fields) > 0 {
		logger = logger.WithFields(fields)
	}
	switch wantLevel {
	case Debug:
		logger.Debug(msg...)
	case Info:
		logger.Info(msg...)
	case Warn:
		logger.Warn(msg...)
	case Error:
		logger.Error(msg...)
	case Fatal:
		logger.Fatal(msg...)
	case Panic:
		logger.Panic(msg...)
	}
}

// Logf is the same as Log but with fmt.Printf-like formatting
func (f *Facility) Logf(logName string, wantLevel Level, fields bark.Fields, format string, msg []interface{}) {
	f.mu.RLock()
	defer f.mu.RUnlock()
	if setLevel, ok := f.levels[logName]; ok {
		if setLevel < wantLevel {
			return
		}
	}
	logger := f.logger
	if len(fields) > 0 {
		logger = logger.WithFields(fields)
	}
	switch wantLevel {
	case Debug:
		logger.Debugf(format, msg...)
	case Info:
		logger.Infof(format, msg...)
	case Warn:
		logger.Warnf(format, msg...)
	case Error:
		logger.Errorf(format, msg...)
	case Fatal:
		logger.Fatalf(format, msg...)
	case Panic:
		logger.Panicf(format, msg...)
	}
}
