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
	"sync/atomic"
	"testing"

	"github.com/uber/tchannel-go"
)

type errorLoggerState struct {
	matchCount []uint32
}

type errorLogger struct {
	tchannel.Logger
	t testing.TB
	v *LogVerification
	s *errorLoggerState
}

// checkFilters returns whether the message can be ignored by the filters.
func (l errorLogger) checkFilters(msg string) bool {
	match := -1
	for i, filter := range l.v.Filters {
		if strings.Contains(msg, filter.Filter) {
			match = i
		}
	}

	if match == -1 {
		return false
	}

	matchCount := atomic.AddUint32(&l.s.matchCount[match], 1)
	return uint(matchCount) <= l.v.Filters[match].Count
}
func (l errorLogger) checkErr(prefix, msg string) {
	if l.checkFilters(msg) {
		return
	}

	l.t.Errorf("%v: %s %v", prefix, msg, l.Fields())
}

func (l errorLogger) Fatal(msg string) {
	l.checkErr("[Fatal]", msg)
	l.Logger.Fatal(msg)
}

func (l errorLogger) Error(msg string) {
	l.checkErr("[Error]", msg)
	l.Logger.Error(msg)
}

func (l errorLogger) Warn(msg string) {
	l.checkErr("[Warn]", msg)
	l.Logger.Warn(msg)
}

func (l errorLogger) WithFields(fields ...tchannel.LogField) tchannel.Logger {
	return errorLogger{l.Logger.WithFields(fields...), l.t, l.v, l.s}
}
