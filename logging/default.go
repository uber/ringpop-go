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

var defaultFacility = NewFacility(nil)

// SetLogger sets the underlying logger for the default facility.
func SetLogger(log bark.Logger) { defaultFacility.SetLogger(log) }

// SetLevel sets the severity level for a named logger on the default facility.
func SetLevel(logName string, level Level) error { return defaultFacility.SetLevel(logName, level) }

// SetLevels same as SetLevels but for multiple named loggers.
func SetLevels(levels map[string]Level) error { return defaultFacility.SetLevels(levels) }

// Logger returns a named logger from the default facility.
func Logger(logName string) bark.Logger { return defaultFacility.Logger(logName) }
