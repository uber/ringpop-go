package util

import (
	"fmt"
	log "github.com/uber-common/bark"
)

// The level definition is stolen from logrus

type Level uint8

func (level Level) String() string {
	switch level {
	case DebugLevel:
		return "debug"
	case InfoLevel:
		return "info"
	case WarnLevel:
		return "warning"
	case ErrorLevel:
		return "error"
	case FatalLevel:
		return "fatal"
	case PanicLevel:
		return "panic"
	case OffLevel:
		return "off"
	}

	return "unknown"
}

func ParseLevel(lvl string) (Level, error) {
	switch lvl {
	case "off":
		return OffLevel, nil
	case "panic":
		return PanicLevel, nil
	case "fatal":
		return FatalLevel, nil
	case "error":
		return ErrorLevel, nil
	case "warn", "warning":
		return WarnLevel, nil
	case "info":
		return InfoLevel, nil
	case "debug":
		return DebugLevel, nil
	}

	var l Level
	return l, fmt.Errorf("not a valid logrus Level: %q", lvl)
}

// The order is reversed compared to logrus for a safer default
const (
	DebugLevel Level = iota
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
	PanicLevel
	OffLevel
)

// Just like a regular log.Logger, but only emits messages above a certain
// level.
type restrictedLogger struct {
	log.Logger
	minLevel Level // by default set to DebugLevel
}

func newRestrictedLogger(logger log.Logger, minLevel Level) *restrictedLogger {
	return &restrictedLogger{
		Logger:   logger,
		minLevel: minLevel}
}

func shouldOutput(msgLevel Level, minLevel Level) bool {
	return msgLevel >= minLevel
}

func (rl *restrictedLogger) Debug(args ...interface{}) {
	if shouldOutput(DebugLevel, rl.minLevel) {
		rl.Logger.Debug(args...)
	}
}

func (rl *restrictedLogger) Debugf(format string, args ...interface{}) {
	if shouldOutput(DebugLevel, rl.minLevel) {
		rl.Logger.Debugf(format, args...)
	}
}

func (rl *restrictedLogger) Info(args ...interface{}) {
	if shouldOutput(InfoLevel, rl.minLevel) {
		rl.Logger.Info(args...)
	}
}

func (rl *restrictedLogger) Infof(format string, args ...interface{}) {
	if shouldOutput(InfoLevel, rl.minLevel) {
		rl.Logger.Infof(format, args...)
	}
}

func (rl *restrictedLogger) Warn(args ...interface{}) {
	if shouldOutput(WarnLevel, rl.minLevel) {
		rl.Logger.Warn(args...)
	}
}

func (rl *restrictedLogger) Warnf(format string, args ...interface{}) {
	if shouldOutput(WarnLevel, rl.minLevel) {
		rl.Logger.Warnf(format, args...)
	}
}

func (rl *restrictedLogger) Error(args ...interface{}) {
	if shouldOutput(ErrorLevel, rl.minLevel) {
		rl.Logger.Error(args...)
	}
}

func (rl *restrictedLogger) Errorf(format string, args ...interface{}) {
	if shouldOutput(ErrorLevel, rl.minLevel) {
		rl.Logger.Errorf(format, args...)
	}
}

func (rl *restrictedLogger) Fatal(args ...interface{}) {
	if shouldOutput(FatalLevel, rl.minLevel) {
		rl.Logger.Fatal(args...)
	}
}

func (rl *restrictedLogger) Fatalf(format string, args ...interface{}) {
	if shouldOutput(FatalLevel, rl.minLevel) {
		rl.Logger.Fatalf(format, args...)
	}
}

func (rl *restrictedLogger) Panic(args ...interface{}) {
	if shouldOutput(PanicLevel, rl.minLevel) {
		rl.Logger.Panic(args...)
	}
}

func (rl *restrictedLogger) Panicf(format string, args ...interface{}) {
	if shouldOutput(PanicLevel, rl.minLevel) {
		rl.Logger.Panicf(format, args...)
	}
}

func (rl *restrictedLogger) WithField(key string, value interface{}) log.Logger {
	return newRestrictedLogger(rl.Logger.WithField(key, value), rl.minLevel)
}

func (rl *restrictedLogger) WithFields(keyValues log.LogFields) log.Logger {
	return newRestrictedLogger(rl.Logger.WithFields(keyValues), rl.minLevel)
}

type ModuleLogger interface {
	SetModuleLevel(name string, minLevel Level) error
	GetLogger(name string) log.Logger
	SetLogger(logger log.Logger) ModuleLogger
	log.Logger
}

type moduleLogger struct {
	log.Logger
	cache  map[Level]*restrictedLogger
	levels map[string]Level
}

func NewModuleLogger(logger log.Logger) *moduleLogger {
	return &moduleLogger{
		Logger: logger,
		levels: make(map[string]Level),
		cache:  make(map[Level]*restrictedLogger)}
}

const lowestLevel = DebugLevel
const highestLevel = OffLevel

// Checking only for greater values is fine as long as Level is unsigned
var tooHighErr = fmt.Errorf("minLevel must be less than or equal to %s", highestLevel)

func (ml *moduleLogger) SetModuleLevel(name string, minLevel Level) error {
	if minLevel > highestLevel {
		return tooHighErr
	}
	ml.levels[name] = minLevel
	return nil
}

// Return a logger with the specific module configuration or the default logger
// if the module is not configured.
func (ml *moduleLogger) GetLogger(name string) log.Logger {
	if minLevel, ok := ml.levels[name]; ok {
		if logger, ok := ml.cache[minLevel]; ok {
			return logger
		}
		logger := newRestrictedLogger(ml.Logger, minLevel)
		ml.cache[minLevel] = logger
		return logger
	}
	return ml
}

func (ml *moduleLogger) SetLogger(logger log.Logger) ModuleLogger {
	newModuleLogger := NewModuleLogger(logger)
	for moduleName, minLevel := range ml.levels {
		newModuleLogger.SetModuleLevel(moduleName, minLevel)
	}
	return newModuleLogger
}
