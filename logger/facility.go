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

// loggerFacility wraps a bark.Logger and implements the Facility interface.
type loggerFacility struct {
	// The underlying logger used by all named loggers
	logger bark.Logger
	// The minimum level for each named logger
	levels map[string]level
}

// NewFacility returns an empty logger facility. All named loggers produced by
// the facility forward their messages to log.
func NewFacility(log bark.Logger) Facility {
	return &loggerFacility{
		logger: log,
		levels: make(map[string]level),
	}
}

func (lf *loggerFacility) SetLevel(name string, level Level) error {
	// No level interval check is needed because level values can't be
	// created from outside of this package.
	l := level.Level()
	lf.levels[name] = l
	return nil
}

func (lf *loggerFacility) SetLogger(logger bark.Logger) {
	lf.logger = logger
}

func (lf *loggerFacility) Logger(name string) Logger {
	return &namedLogger{
		name: name,
		recv: lf,
	}
}

func (lf *loggerFacility) WithField(key string, value interface{}) Factory {
	return &loggerFacilityFields{
		recv:   lf,
		fields: Fields{key: value},
	}
}

func (lf *loggerFacility) WithFields(fields Fields) Factory {
	return &loggerFacilityFields{
		recv:   lf,
		fields: fields,
	}
}

func (lf *loggerFacility) shouldOutput(name string, level level) bool {
	minLevel := defaultNotConfiguredNamedLogger
	if confLevel, ok := lf.levels[name]; ok {
		minLevel = confLevel
	}
	return minLevel <= level
}

func (lf *loggerFacility) getLogger(fields Fields) bark.Logger {
	logger := lf.logger
	if len(fields) > 0 {
		logger = logger.WithFields(fields)
	}
	return logger
}

type namedMsg struct {
	name   string
	level  level
	fields Fields
	args   []interface{}
}

// Having this method makes updating configuration convenient.
// Without it, a call on SetLevel must push an update to all namedLogger
// instances. The bookkeeping of all instances is not trivial because any
// namedLogger can create additional instances when WithField/WithFiels is
// called. All those references must be tracked carefully using indirect
// references to allow the GC to reclaim memory.
// Making all namedLogger instances delegate back to this method sidesteps
// this problem entirely.
func (lf *loggerFacility) Log(msg *namedMsg) {
	if !lf.shouldOutput(msg.name, msg.level) {
		return
	}
	logger := lf.getLogger(msg.fields)
	switch msg.level {
	case Trace:
		logger.Debug(msg.args...)
	case Debug:
		logger.Debug(msg.args...)
	case Info:
		logger.Info(msg.args...)
	case Warn:
		logger.Warn(msg.args...)
	case Error:
		logger.Error(msg.args...)
	case Fatal:
		logger.Fatal(msg.args...)
	case Panic:
		logger.Panic(msg.args...)
	}
}

type namedfMsg struct {
	*namedMsg
	format string
}

func (lf *loggerFacility) Logf(msg *namedfMsg) {
	if !lf.shouldOutput(msg.name, msg.level) {
		return
	}
	logger := lf.getLogger(msg.fields)
	switch msg.level {
	case Trace:
		logger.Debugf(msg.format, msg.args...)
	case Debug:
		logger.Debugf(msg.format, msg.args...)
	case Info:
		logger.Infof(msg.format, msg.args...)
	case Warn:
		logger.Warnf(msg.format, msg.args...)
	case Error:
		logger.Errorf(msg.format, msg.args...)
	case Fatal:
		logger.Fatalf(msg.format, msg.args...)
	case Panic:
		logger.Panicf(msg.format, msg.args...)
	}
}

type logReceiver interface {
	Log(*namedMsg)
	Logf(*namedfMsg)
}

type loggerFacilityFields struct {
	recv   logReceiver
	fields Fields
}

func (lff *loggerFacilityFields) Log(msg *namedMsg) {
	lff.recv.Log(&namedMsg{
		name:  msg.name,
		level: msg.level,
		// Combine message fields with own fields and forward
		fields: lff.fields.withFields(msg.fields),
		args:   msg.args,
	})
}

func (lff *loggerFacilityFields) Logf(msg *namedfMsg) {
	lff.recv.Logf(&namedfMsg{
		namedMsg: &namedMsg{
			name:  msg.name,
			level: msg.level,
			// Combine message fields with own fields and forward
			fields: lff.fields.withFields(msg.fields),
			args:   msg.args,
		},
		format: msg.format,
	})
}

func (lff *loggerFacilityFields) Logger(name string) Logger {
	return &namedLogger{
		name: name,
		recv: lff,
	}
}

func (lff *loggerFacilityFields) WithField(key string, value interface{}) Factory {
	return &loggerFacilityFields{
		recv:   lff.recv,
		fields: lff.fields.withField(key, value),
	}
}

func (lff *loggerFacilityFields) WithFields(fields Fields) Factory {
	return &loggerFacilityFields{
		recv:   lff.recv,
		fields: lff.fields.withFields(fields),
	}
}
