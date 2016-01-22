package logger

import (
	"github.com/uber-common/bark"
)

// This wrapper limits a bark.Logger to emit messages only above a level.
type restrictedLogger struct {
	logger bark.Logger
	min    Level
}

func (rl *restrictedLogger) setLevel(level Level) {
	rl.min = level
}

func (rl *restrictedLogger) shouldOutput(level Level) bool {
	return level >= rl.min
}

func (rl *restrictedLogger) Debug(args ...interface{}) {
	if rl.shouldOutput(Debug) {
		rl.logger.Debug(args...)
	}
}
func (rl *restrictedLogger) Info(args ...interface{}) {
	if rl.shouldOutput(Info) {
		rl.logger.Info(args...)
	}
}
func (rl *restrictedLogger) Warn(args ...interface{}) {
	if rl.shouldOutput(Warn) {
		rl.logger.Warn(args...)
	}
}
func (rl *restrictedLogger) Error(args ...interface{}) {
	if rl.shouldOutput(Error) {
		rl.logger.Error(args...)
	}
}
func (rl *restrictedLogger) Fatal(args ...interface{}) {
	if rl.shouldOutput(Fatal) {
		rl.logger.Fatal(args...)
	}
}
func (rl *restrictedLogger) Panic(args ...interface{}) {
	if rl.shouldOutput(Panic) {
		rl.logger.Panic(args...)
	}
}
func (rl *restrictedLogger) Debugf(format string, args ...interface{}) {
	if rl.shouldOutput(Debug) {
		rl.logger.Debugf(format, args...)
	}
}
func (rl *restrictedLogger) Infof(format string, args ...interface{}) {
	if rl.shouldOutput(Info) {
		rl.logger.Infof(format, args...)
	}
}
func (rl *restrictedLogger) Warnf(format string, args ...interface{}) {
	if rl.shouldOutput(Warn) {
		rl.logger.Warnf(format, args...)
	}
}
func (rl *restrictedLogger) Errorf(format string, args ...interface{}) {
	if rl.shouldOutput(Error) {
		rl.logger.Errorf(format, args...)
	}
}
func (rl *restrictedLogger) Fatalf(format string, args ...interface{}) {
	if rl.shouldOutput(Fatal) {
		rl.logger.Fatalf(format, args...)
	}
}
func (rl *restrictedLogger) Panicf(format string, args ...interface{}) {
	if rl.shouldOutput(Panic) {
		rl.logger.Panicf(format, args...)
	}
}
func (rl *restrictedLogger) WithField(key string, value interface{}) *restrictedLogger {
	return &restrictedLogger{
		logger: rl.logger.WithField(key, value),
		min:    rl.min}
}
func (rl *restrictedLogger) WithFields(fields bark.LogFields) *restrictedLogger {
	return &restrictedLogger{
		logger: rl.logger.WithFields(fields),
		min:    rl.min}
}
func (rl *restrictedLogger) Fields() bark.Fields {
	return rl.logger.Fields()
}
