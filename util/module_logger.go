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

type ModuleLogger struct {
	levels map[string]Level
	logs   map[string]log.Logger
	*loggerProxy
}

func NewModuleLogger(logger log.Logger) *ModuleLogger {
	l := &ModuleLogger{
		levels: make(map[string]Level),
		logs:   make(map[string]log.Logger)}
	l.loggerProxy = newNamedLogger(RootModule, l)
	l.SetLogger(RootModule, logger)
	return l
}

func (ml *ModuleLogger) getLogger(moduleName string) *RestrictedLogger {
	l := ml.logs[RootModule]
	if logger, ok := ml.logs[moduleName]; ok {
		l = logger
	}
	level := lowestLevel
	if l, ok := ml.levels[moduleName]; ok {
		level = l
	}
	return &RestrictedLogger{
		logger: l,
		min:    level}
}

func (ml *ModuleLogger) SetLogger(moduleName string, logger log.Logger) {
	ml.logs[moduleName] = logger
}

const lowestLevel = DebugLevel
const highestLevel = OffLevel

// Checking only for greater values is fine as long as Level is unsigned.
var tooHighErr = fmt.Errorf("minLevel must be less than or equal to %s", highestLevel)

func (ml *ModuleLogger) SetLevel(moduleName string, level Level) error {
	if level > highestLevel {
		return tooHighErr
	}
	ml.levels[moduleName] = level
	return nil
}

func (ml *ModuleLogger) Logger(moduleName string) *loggerProxy {
	return newNamedLogger(moduleName, ml)
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

func newFieldLogger(key string, value interface{}, proxy *loggerProxy) *loggerProxy {
	return &loggerProxy{
		l: func() *RestrictedLogger {
			return proxy.getParentLogger().WithField(key, value)
		}}
}

func newFieldsLogger(fields log.LogFields, proxy *loggerProxy) *loggerProxy {
	return &loggerProxy{
		l: func() *RestrictedLogger {
			return proxy.getParentLogger().WithFields(fields)
		}}
}

func newNamedLogger(moduleName string, ml *ModuleLogger) *loggerProxy {
	return &loggerProxy{
		l: func() *RestrictedLogger {
			return ml.getLogger(moduleName)
		}}
}

type loggerProxy struct{ l func() *RestrictedLogger }

func (p *loggerProxy) getParentLogger() *RestrictedLogger        { return p.l() }
func (p *loggerProxy) Debug(args ...interface{})                 { p.l().Debug(args...) }
func (p *loggerProxy) Info(args ...interface{})                  { p.l().Info(args...) }
func (p *loggerProxy) Warn(args ...interface{})                  { p.l().Warn(args...) }
func (p *loggerProxy) Error(args ...interface{})                 { p.l().Error(args...) }
func (p *loggerProxy) Fatal(args ...interface{})                 { p.l().Fatal(args...) }
func (p *loggerProxy) Panic(args ...interface{})                 { p.l().Panic(args...) }
func (p *loggerProxy) Debugf(format string, args ...interface{}) { p.l().Debugf(format, args...) }
func (p *loggerProxy) Infof(format string, args ...interface{})  { p.l().Infof(format, args...) }
func (p *loggerProxy) Warnf(format string, args ...interface{})  { p.l().Warnf(format, args...) }
func (p *loggerProxy) Errorf(format string, args ...interface{}) { p.l().Errorf(format, args...) }
func (p *loggerProxy) Fatalf(format string, args ...interface{}) { p.l().Fatalf(format, args...) }
func (p *loggerProxy) Panicf(format string, args ...interface{}) { p.l().Panicf(format, args...) }
func (p *loggerProxy) Fields() log.Fields                        { return p.l().Fields() }
func (p *loggerProxy) WithField(key string, value interface{}) log.Logger {
	return newFieldLogger(key, value, p)
}
func (p *loggerProxy) WithFields(fields log.LogFields) log.Logger {
	return newFieldsLogger(fields, p)
}
