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

// namedLogger is a Logger implementation that adds the logger name to all
// incomming messages and forwards them to receiver.
type namedLogger struct {
	name   string
	fields Fields
	recv   logReceiver
}

func (rl *namedLogger) Trace(args ...interface{}) { rl.recv.Log(rl.newLogMessage(Trace, args)) }
func (rl *namedLogger) Debug(args ...interface{}) { rl.recv.Log(rl.newLogMessage(Debug, args)) }
func (rl *namedLogger) Info(args ...interface{})  { rl.recv.Log(rl.newLogMessage(Info, args)) }
func (rl *namedLogger) Warn(args ...interface{})  { rl.recv.Log(rl.newLogMessage(Warn, args)) }
func (rl *namedLogger) Error(args ...interface{}) { rl.recv.Log(rl.newLogMessage(Error, args)) }
func (rl *namedLogger) Fatal(args ...interface{}) { rl.recv.Log(rl.newLogMessage(Fatal, args)) }
func (rl *namedLogger) Panic(args ...interface{}) { rl.recv.Log(rl.newLogMessage(Panic, args)) }

func (rl *namedLogger) Tracef(format string, args ...interface{}) {
	rl.recv.Logf(rl.newLogfMessage(Trace, format, args))
}
func (rl *namedLogger) Debugf(format string, args ...interface{}) {
	rl.recv.Logf(rl.newLogfMessage(Debug, format, args))
}
func (rl *namedLogger) Infof(format string, args ...interface{}) {
	rl.recv.Logf(rl.newLogfMessage(Info, format, args))
}
func (rl *namedLogger) Warnf(format string, args ...interface{}) {
	rl.recv.Logf(rl.newLogfMessage(Warn, format, args))
}
func (rl *namedLogger) Errorf(format string, args ...interface{}) {
	rl.recv.Logf(rl.newLogfMessage(Error, format, args))
}
func (rl *namedLogger) Fatalf(format string, args ...interface{}) {
	rl.recv.Logf(rl.newLogfMessage(Fatal, format, args))
}
func (rl *namedLogger) Panicf(format string, args ...interface{}) {
	rl.recv.Logf(rl.newLogfMessage(Panic, format, args))
}

// WithField creates a new namedLogger with a copy of current fields plus the
// new one. On key overlap, the value is overwritten.
func (rl *namedLogger) WithField(key string, value interface{}) Logger {
	return &namedLogger{
		name: rl.name,
		// This creates an updated copy of fields
		fields: rl.fields.withField(key, value),
		recv:   rl.recv,
	}
}

// WithFields is similar to WithField but for multiple fields.
func (rl *namedLogger) WithFields(fields Fields) Logger {
	return &namedLogger{
		name: rl.name,
		// This creates and updated copy of fields
		fields: rl.fields.withFields(fields),
		recv:   rl.recv,
	}
}

func (rl *namedLogger) newLogMessage(level level, args []interface{}) *namedMsg {
	return &namedMsg{
		name:   rl.name,
		level:  level,
		fields: rl.fields,
		args:   args,
	}
}
func (rl *namedLogger) newLogfMessage(level level, format string, args []interface{}) *namedfMsg {
	return &namedfMsg{
		namedMsg: rl.newLogMessage(level, args),
		format:   format,
	}
}
