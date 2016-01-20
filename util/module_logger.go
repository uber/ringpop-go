package util

import (
	"fmt"
	log "github.com/uber-common/bark"
)

// The level definition is stolen from logrus.

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
	return l, fmt.Errorf("not a valid Level: %q", lvl)
}

// The order is reversed compared to logrus for a safer default.
const (
	DebugLevel Level = iota
	InfoLevel
	WarnLevel
	ErrorLevel
	FatalLevel
	PanicLevel
	OffLevel
)

const RootModule = "root"

// NewModuleLogger returns a ModuleLogger instance with a consistent internal
// state. The logger passed serves as the root logger, and all sub-loggers
// default to using it unless configured otherwise.
func NewModuleLogger(logger log.Logger) *ModuleLogger {
	ml := &ModuleLogger{
		levels: make(map[string]Level),
		logs:   make(map[string]log.Logger),
		l: func(rl *RestrictedLogger) *RestrictedLogger {
			return rl
		}}
	ml.SetLogger(RootModule, logger)
	return ml.Logger(RootModule).(*ModuleLogger)
}

type RestrictedLogger struct {
	logger log.Logger
	min    Level
}

func (rl *RestrictedLogger) shouldOutput(level Level) bool { return level >= rl.min }
func (rl *RestrictedLogger) Debug(args ...interface{}) {
	if rl.shouldOutput(DebugLevel) {
		rl.logger.Debug(args...)
	}
}
func (rl *RestrictedLogger) Info(args ...interface{}) {
	if rl.shouldOutput(InfoLevel) {
		rl.logger.Info(args...)
	}
}
func (rl *RestrictedLogger) Warn(args ...interface{}) {
	if rl.shouldOutput(WarnLevel) {
		rl.logger.Warn(args...)
	}
}
func (rl *RestrictedLogger) Error(args ...interface{}) {
	if rl.shouldOutput(ErrorLevel) {
		rl.logger.Error(args...)
	}
}
func (rl *RestrictedLogger) Fatal(args ...interface{}) {
	if rl.shouldOutput(FatalLevel) {
		rl.logger.Fatal(args...)
	}
}
func (rl *RestrictedLogger) Panic(args ...interface{}) {
	if rl.shouldOutput(PanicLevel) {
		rl.logger.Panic(args...)
	}
}
func (rl *RestrictedLogger) Debugf(format string, args ...interface{}) {
	if rl.shouldOutput(DebugLevel) {
		rl.logger.Debugf(format, args...)
	}
}
func (rl *RestrictedLogger) Infof(format string, args ...interface{}) {
	if rl.shouldOutput(InfoLevel) {
		rl.logger.Infof(format, args...)
	}
}
func (rl *RestrictedLogger) Warnf(format string, args ...interface{}) {
	if rl.shouldOutput(WarnLevel) {
		rl.logger.Warnf(format, args...)
	}
}
func (rl *RestrictedLogger) Errorf(format string, args ...interface{}) {
	if rl.shouldOutput(ErrorLevel) {
		rl.logger.Errorf(format, args...)
	}
}
func (rl *RestrictedLogger) Fatalf(format string, args ...interface{}) {
	if rl.shouldOutput(FatalLevel) {
		rl.logger.Fatalf(format, args...)
	}
}
func (rl *RestrictedLogger) Panicf(format string, args ...interface{}) {
	if rl.shouldOutput(PanicLevel) {
		rl.logger.Panicf(format, args...)
	}
}
func (rl *RestrictedLogger) WithField(key string, value interface{}) *RestrictedLogger {
	return &RestrictedLogger{
		logger: rl.logger.WithField(key, value),
		min:    rl.min}
}
func (rl *RestrictedLogger) WithFields(fields log.LogFields) *RestrictedLogger {
	return &RestrictedLogger{
		logger: rl.logger.WithFields(fields),
		min:    rl.min}
}
func (rl *RestrictedLogger) Fields() log.Fields {
	return rl.logger.Fields()
}

// ModuleLogger is a flat sub-logger configuration and factory. Each sub-logger
// can be restricted to emit messages above a minimum level or use a different
// underlying logger. The configuration can be changed on the fly even for
// existing sub-loggers.
// Method calls that return other loggers return instead a ModuleLogger
// instance. This simplifies porting existing code unaware of module logging.
type ModuleLogger struct {
	l      func(*RestrictedLogger) *RestrictedLogger
	levels map[string]Level
	logs   map[string]log.Logger
}

func (ml *ModuleLogger) getParentLogger(*RestrictedLogger) *RestrictedLogger { return ml.l(nil) }
func (ml *ModuleLogger) Debug(args ...interface{})                           { ml.l(nil).Debug(args...) }
func (ml *ModuleLogger) Info(args ...interface{})                            { ml.l(nil).Info(args...) }
func (ml *ModuleLogger) Warn(args ...interface{})                            { ml.l(nil).Warn(args...) }
func (ml *ModuleLogger) Error(args ...interface{})                           { ml.l(nil).Error(args...) }
func (ml *ModuleLogger) Fatal(args ...interface{})                           { ml.l(nil).Fatal(args...) }
func (ml *ModuleLogger) Panic(args ...interface{})                           { ml.l(nil).Panic(args...) }
func (ml *ModuleLogger) Debugf(format string, args ...interface{})           { ml.l(nil).Debugf(format, args...) }
func (ml *ModuleLogger) Infof(format string, args ...interface{})            { ml.l(nil).Infof(format, args...) }
func (ml *ModuleLogger) Warnf(format string, args ...interface{})            { ml.l(nil).Warnf(format, args...) }
func (ml *ModuleLogger) Errorf(format string, args ...interface{})           { ml.l(nil).Errorf(format, args...) }
func (ml *ModuleLogger) Fatalf(format string, args ...interface{})           { ml.l(nil).Fatalf(format, args...) }
func (ml *ModuleLogger) Panicf(format string, args ...interface{})           { ml.l(nil).Panicf(format, args...) }
func (ml *ModuleLogger) Fields() log.Fields                                  { return ml.l(nil).Fields() }

// SetLogger overrides the logger used for a sub-logger.
// The root logger, can be referenced using the "root" name.
func (ml *ModuleLogger) SetLogger(moduleName string, logger log.Logger) {
	ml.logs[moduleName] = logger
}

const lowestLevel = DebugLevel
const highestLevel = OffLevel

// Checking only for greater values is fine as long as Level is unsigned.
var tooHighErr = fmt.Errorf("minLevel must be less than or equal to %s", highestLevel)

// SetLevel sets the minimum level for a sub-logger. A logger only emits
// messages that have a level equal to or greater than the one set.
// Unless configured, a sub-logger outputs all messages.
func (ml *ModuleLogger) SetLevel(moduleName string, level Level) error {
	if level > highestLevel {
		return tooHighErr
	}
	ml.levels[moduleName] = level
	return nil
}

// Logger returns a sub-logger.
func (ml *ModuleLogger) Logger(moduleName string) log.Logger {
	return &ModuleLogger{
		levels: ml.levels,
		logs:   ml.logs,
		l: func(rl *RestrictedLogger) *RestrictedLogger {
			if rl == nil {
				l := ml.logs[RootModule]
				if logger, ok := ml.logs[moduleName]; ok {
					l = logger
				}
				level := lowestLevel
				if l, ok := ml.levels[moduleName]; ok {
					level = l
				}
				rl = &RestrictedLogger{
					logger: l,
					min:    level}
			}
			return ml.l(rl)
		}}
}

func (ml *ModuleLogger) WithField(key string, value interface{}) log.Logger {
	return &ModuleLogger{
		levels: ml.levels,
		logs:   ml.logs,
		l: func(rl *RestrictedLogger) *RestrictedLogger {
			return ml.l(rl).WithField(key, value)
		}}
}

func (ml *ModuleLogger) WithFields(fields log.LogFields) log.Logger {
	return &ModuleLogger{
		levels: ml.levels,
		logs:   ml.logs,
		l: func(rl *RestrictedLogger) *RestrictedLogger {
			return ml.l(rl).WithFields(fields)
		}}
}

// If passed a module logger it returns a sub-logger, otherwise return the
// original logger untouched.
func GetModuleLogger(logger log.Logger, moduleName string) log.Logger {
	if moduleLogger, ok := logger.(*ModuleLogger); ok {
		return moduleLogger.Logger(moduleName)
	}
	return logger
}
