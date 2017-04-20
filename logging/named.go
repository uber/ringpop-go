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
	"github.com/uber-common/bark"
)

// namedLogger is a bark.Logger implementation that has a name. It forwards all
// log requests to a logReceiver, adding its own name in the process.
type namedLogger struct {
	name      string
	forwardTo logReceiver
	err       error
	fields    bark.Fields
}

type logReceiver interface {
	Log(logName string, wantLevel Level, fields bark.Fields, msg []interface{})
	Logf(logName string, wantLevel Level, fields bark.Fields, format string, msg []interface{})
}

// Forward all method calls to the logReceiver. The logReceiver can decide,
// based on the name of the logger and the desired severity level if the
// message should be silenced or not.

func (l *namedLogger) Debug(args ...interface{}) { l.forwardTo.Log(l.name, Debug, l.fields, args) }
func (l *namedLogger) Info(args ...interface{})  { l.forwardTo.Log(l.name, Info, l.fields, args) }
func (l *namedLogger) Warn(args ...interface{})  { l.forwardTo.Log(l.name, Warn, l.fields, args) }
func (l *namedLogger) Error(args ...interface{}) { l.forwardTo.Log(l.name, Error, l.fields, args) }
func (l *namedLogger) Fatal(args ...interface{}) { l.forwardTo.Log(l.name, Fatal, l.fields, args) }
func (l *namedLogger) Panic(args ...interface{}) { l.forwardTo.Log(l.name, Panic, l.fields, args) }
func (l *namedLogger) Debugf(fmt string, args ...interface{}) {
	l.forwardTo.Logf(l.name, Debug, l.fields, fmt, args)
}
func (l *namedLogger) Infof(fmt string, args ...interface{}) {
	l.forwardTo.Logf(l.name, Info, l.fields, fmt, args)
}
func (l *namedLogger) Warnf(fmt string, args ...interface{}) {
	l.forwardTo.Logf(l.name, Warn, l.fields, fmt, args)
}
func (l *namedLogger) Errorf(fmt string, args ...interface{}) {
	l.forwardTo.Logf(l.name, Error, l.fields, fmt, args)
}
func (l *namedLogger) Fatalf(fmt string, args ...interface{}) {
	l.forwardTo.Logf(l.name, Fatal, l.fields, fmt, args)
}
func (l *namedLogger) Panicf(fmt string, args ...interface{}) {
	l.forwardTo.Logf(l.name, Panic, l.fields, fmt, args)
}

// WithField creates a new namedLogger that retains the name but has an updated
// copy of the fields.
func (l *namedLogger) WithField(key string, value interface{}) bark.Logger {
	newSize := len(l.fields) + 1
	newFields := make(map[string]interface{}, newSize) // Hold the updated copy
	for k, v := range l.fields {
		newFields[k] = v
	}
	newFields[key] = value // Set the new key.

	return &namedLogger{
		name:      l.name,
		forwardTo: l.forwardTo,
		fields:    newFields,
	}
}

func (l *namedLogger) WithFields(fields bark.LogFields) bark.Logger {
	other := fields.Fields()
	newSize := len(l.fields) + len(other)
	newFields := make(map[string]interface{}, newSize) // Hold the updated copy
	for k, v := range l.fields {
		newFields[k] = v
	}
	// The fields passed to the method call override any previously defined
	// fields with the same name.
	for k, v := range other {
		newFields[k] = v
	}

	return &namedLogger{
		name:      l.name,
		forwardTo: l.forwardTo,
		fields:    newFields,
	}
}

// Return a new named logger with the error set to be included in a subsequent
// normal logging call
func (l *namedLogger) WithError(err error) bark.Logger {
	return &namedLogger{
		name:      l.name,
		forwardTo: l.forwardTo,
		err:       err,
		fields:    l.Fields(),
	}
}

// This is needed to fully implement the bark.Logger interface.
func (l *namedLogger) Fields() bark.Fields {
	return l.fields
}
