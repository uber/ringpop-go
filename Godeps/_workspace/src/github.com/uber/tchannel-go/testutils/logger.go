// Copyright (c) 2015 Uber Technologies, Inc.

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

package testutils

import (
	"strings"
	"testing"

	"github.com/uber/tchannel-go"
)

type errorLoggerState struct {
	matchCount int
}

type errorLogger struct {
	tchannel.Logger
	t testing.TB
	v *LogVerification
	s *errorLoggerState
}

func (l errorLogger) checkErr(msg string, args ...interface{}) {
	allowedCount := l.v.AllowedCount

	if filter := l.v.AllowedFilter; filter != "" {
		if !strings.Contains(msg, filter) {
			l.t.Errorf(msg, args...)
		}
	}

	l.s.matchCount++
	if l.s.matchCount <= allowedCount {
		return
	}

	l.t.Errorf(msg, args...)
}

func (l errorLogger) Fatalf(msg string, args ...interface{}) {
	l.checkErr("[Fatal] "+msg, args...)
	l.Logger.Fatalf(msg, args...)
}

func (l errorLogger) Errorf(msg string, args ...interface{}) {
	l.checkErr("[Error] "+msg, args...)
	l.Logger.Errorf(msg, args...)
}

func (l errorLogger) Warnf(msg string, args ...interface{}) {
	l.checkErr("[Warn] "+msg, args...)
	l.Logger.Warnf(msg, args...)
}

func (l errorLogger) WithFields(fields ...tchannel.LogField) tchannel.Logger {
	return errorLogger{l.Logger.WithFields(fields...), l.t, l.v, l.s}
}
