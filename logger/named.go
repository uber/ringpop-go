package logger

import (
	"github.com/uber-common/bark"
)

// This wrapper limits a bark.Logger to emit messages only above a level.
type namedLogger struct {
	logger bark.Logger
	min    Level
}

func (rl *namedLogger) setLevel(level Level) {
	rl.min = level
}

func (rl *namedLogger) shouldOutput(level Level) bool {
	return level >= rl.min
}

// Trace is forwarded to Debug
func (rl *namedLogger) Trace(args ...interface{}) {
	if rl.shouldOutput(Trace) {
		rl.logger.Debug(args...)
	}
}

func (rl *namedLogger) Debug(args ...interface{}) {
	if rl.shouldOutput(Debug) {
		rl.logger.Debug(args...)
	}
}
func (rl *namedLogger) Info(args ...interface{}) {
	if rl.shouldOutput(Info) {
		rl.logger.Info(args...)
	}
}
func (rl *namedLogger) Warn(args ...interface{}) {
	if rl.shouldOutput(Warn) {
		rl.logger.Warn(args...)
	}
}
func (rl *namedLogger) Error(args ...interface{}) {
	if rl.shouldOutput(Error) {
		rl.logger.Error(args...)
	}
}
func (rl *namedLogger) Fatal(args ...interface{}) {
	if rl.shouldOutput(Fatal) {
		rl.logger.Fatal(args...)
	}
}
func (rl *namedLogger) Panic(args ...interface{}) {
	if rl.shouldOutput(Panic) {
		rl.logger.Panic(args...)
	}
}

// Tracef is forwarded to Debugf
func (rl *namedLogger) Tracef(format string, args ...interface{}) {
	if rl.shouldOutput(Trace) {
		rl.logger.Debugf(format, args...)
	}
}
func (rl *namedLogger) Debugf(format string, args ...interface{}) {
	if rl.shouldOutput(Debug) {
		rl.logger.Debugf(format, args...)
	}
}
func (rl *namedLogger) Infof(format string, args ...interface{}) {
	if rl.shouldOutput(Info) {
		rl.logger.Infof(format, args...)
	}
}
func (rl *namedLogger) Warnf(format string, args ...interface{}) {
	if rl.shouldOutput(Warn) {
		rl.logger.Warnf(format, args...)
	}
}
func (rl *namedLogger) Errorf(format string, args ...interface{}) {
	if rl.shouldOutput(Error) {
		rl.logger.Errorf(format, args...)
	}
}
func (rl *namedLogger) Fatalf(format string, args ...interface{}) {
	if rl.shouldOutput(Fatal) {
		rl.logger.Fatalf(format, args...)
	}
}
func (rl *namedLogger) Panicf(format string, args ...interface{}) {
	if rl.shouldOutput(Panic) {
		rl.logger.Panicf(format, args...)
	}
}
func (rl *namedLogger) WithField(key string, value interface{}) bark.Logger {
	return &namedLogger{
		logger: rl.logger.WithField(key, value),
		min:    rl.min}
}
func (rl *namedLogger) WithFields(fields bark.LogFields) bark.Logger {
	return &namedLogger{
		logger: rl.logger.WithFields(fields),
		min:    rl.min}
}
func (rl *namedLogger) Fields() bark.Fields {
	return rl.logger.Fields()
}
