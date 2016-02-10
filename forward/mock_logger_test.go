package forward

import "github.com/uber-common/bark"
import "github.com/stretchr/testify/mock"

type Logger struct {
	mock.Mock
}

// Debug provides a mock function with given fields: args
func (_m *Logger) Debug(args ...interface{}) {
	_m.Called(args)
}

// Debugf provides a mock function with given fields: format, args
func (_m *Logger) Debugf(format string, args ...interface{}) {
	_m.Called(format, args)
}

// Info provides a mock function with given fields: args
func (_m *Logger) Info(args ...interface{}) {
	_m.Called(args)
}

// Infof provides a mock function with given fields: format, args
func (_m *Logger) Infof(format string, args ...interface{}) {
	_m.Called(format, args)
}

// Warn provides a mock function with given fields: args
func (_m *Logger) Warn(args ...interface{}) {
	_m.Called(args)
}

// Warnf provides a mock function with given fields: format, args
func (_m *Logger) Warnf(format string, args ...interface{}) {
	_m.Called(format, args)
}

// Error provides a mock function with given fields: args
func (_m *Logger) Error(args ...interface{}) {
	_m.Called(args)
}

// Errorf provides a mock function with given fields: format, args
func (_m *Logger) Errorf(format string, args ...interface{}) {
	_m.Called(format, args)
}

// Fatal provides a mock function with given fields: args
func (_m *Logger) Fatal(args ...interface{}) {
	_m.Called(args)
}

// Fatalf provides a mock function with given fields: format, args
func (_m *Logger) Fatalf(format string, args ...interface{}) {
	_m.Called(format, args)
}

// Panic provides a mock function with given fields: args
func (_m *Logger) Panic(args ...interface{}) {
	_m.Called(args)
}

// Panicf provides a mock function with given fields: format, args
func (_m *Logger) Panicf(format string, args ...interface{}) {
	_m.Called(format, args)
}

// WithField provides a mock function with given fields: key, value
func (_m *Logger) WithField(key string, value interface{}) bark.Logger {
	ret := _m.Called(key, value)

	var r0 bark.Logger
	if rf, ok := ret.Get(0).(func(string, interface{}) bark.Logger); ok {
		r0 = rf(key, value)
	} else {
		r0 = ret.Get(0).(bark.Logger)
	}

	return r0
}

// WithFields provides a mock function with given fields: keyValues
func (_m *Logger) WithFields(keyValues bark.LogFields) bark.Logger {
	ret := _m.Called(keyValues)

	var r0 bark.Logger
	if rf, ok := ret.Get(0).(func(bark.LogFields) bark.Logger); ok {
		r0 = rf(keyValues)
	} else {
		r0 = ret.Get(0).(bark.Logger)
	}

	return r0
}

// Fields provides a mock function with given fields:
func (_m *Logger) Fields() bark.Fields {
	ret := _m.Called()

	var r0 bark.Fields
	if rf, ok := ret.Get(0).(func() bark.Fields); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(bark.Fields)
	}

	return r0
}
