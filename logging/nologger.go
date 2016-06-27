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

// NoLogger is a bark-compatible logger that does nothing with the log
// messages.
type noLogger struct{}

func (l noLogger) Debug(args ...interface{})                           {}
func (l noLogger) Debugf(format string, args ...interface{})           {}
func (l noLogger) Info(args ...interface{})                            {}
func (l noLogger) Infof(format string, args ...interface{})            {}
func (l noLogger) Warn(args ...interface{})                            {}
func (l noLogger) Warnf(format string, args ...interface{})            {}
func (l noLogger) Error(args ...interface{})                           {}
func (l noLogger) Errorf(format string, args ...interface{})           {}
func (l noLogger) Fatal(args ...interface{})                           {}
func (l noLogger) Fatalf(format string, args ...interface{})           {}
func (l noLogger) Panic(args ...interface{})                           {}
func (l noLogger) Panicf(format string, args ...interface{})           {}
func (l noLogger) WithField(key string, value interface{}) bark.Logger { return l }
func (l noLogger) WithFields(keyValues bark.LogFields) bark.Logger     { return l }
func (l noLogger) Fields() bark.Fields                                 { return nil }

// NoLogger is the default logger used by logging facilities when a logger
// is not passed.
var NoLogger = noLogger{}
