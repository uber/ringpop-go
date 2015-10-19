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

package bark

import "github.com/Sirupsen/logrus"

// Interface provides indirection so Entry and Logger implementations can use exact same methods
type logrusLoggerOrEntry interface {
	Debug(args ...interface{})
	Debugf(format string, args ...interface{})
	Info(args ...interface{})
	Infof(format string, args ...interface{})
	Warn(args ...interface{})
	Warnf(format string, args ...interface{})
	Error(args ...interface{})
	Errorf(format string, args ...interface{})
	Fatal(args ...interface{})
	Fatalf(format string, args ...interface{})
	Panic(args ...interface{})
	Panicf(format string, args ...interface{})
	WithField(key string, value interface{}) *logrus.Entry
	WithFields(keyValues logrus.Fields) *logrus.Entry
}

// The bark-compliant Logger implementation.  Dispatches directly to wrapped logrus
// instance for all methods except WithField and WithFields
type barkLogrusLogger struct {
	logrusLoggerOrEntry
}

// Note: logger is immutable, safe to use non-pointer receivers
func newBarkLogrusLogger(loggerOrEntry logrusLoggerOrEntry) Logger {
	return barkLogrusLogger{logrusLoggerOrEntry: loggerOrEntry}
}

func (l barkLogrusLogger) WithField(key string, value interface{}) Logger {
	return newBarkLogrusLogger(l.logrusLoggerOrEntry.WithField(key, value))
}

func (l barkLogrusLogger) WithFields(logFields LogFields) Logger {
	if (logFields == nil) {
		return l
	}

	return newBarkLogrusLogger(l.logrusLoggerOrEntry.WithFields(logrus.Fields(logFields.Fields())))
}

func (l barkLogrusLogger) Fields() Fields {
	if entry, ok := l.logrusLoggerOrEntry.(*logrus.Entry); ok && entry.Data != nil {
		return Fields(entry.Data)
	}

	return nil
}
