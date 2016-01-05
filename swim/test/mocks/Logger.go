package mocks

import "github.com/uber-common/bark"
import "github.com/stretchr/testify/mock"

type Logger struct {
	mock.Mock
}

func (_m *Logger) Debug(args ...interface{}) {
	_m.Called(args)
}
func (_m *Logger) Debugf(format string, args ...interface{}) {
	_m.Called(format, args)
}
func (_m *Logger) Info(args ...interface{}) {
	_m.Called(args)
}
func (_m *Logger) Infof(format string, args ...interface{}) {
	_m.Called(format, args)
}
func (_m *Logger) Warn(args ...interface{}) {
	_m.Called(args)
}
func (_m *Logger) Warnf(format string, args ...interface{}) {
	_m.Called(format, args)
}
func (_m *Logger) Error(args ...interface{}) {
	_m.Called(args)
}
func (_m *Logger) Errorf(format string, args ...interface{}) {
	_m.Called(format, args)
}
func (_m *Logger) Fatal(args ...interface{}) {
	_m.Called(args)
}
func (_m *Logger) Fatalf(format string, args ...interface{}) {
	_m.Called(format, args)
}
func (_m *Logger) Panic(args ...interface{}) {
	_m.Called(args)
}
func (_m *Logger) Panicf(format string, args ...interface{}) {
	_m.Called(format, args)
}
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
