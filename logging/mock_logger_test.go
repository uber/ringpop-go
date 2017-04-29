package logging

import "github.com/uber-common/bark"
import "github.com/stretchr/testify/mock"

type MockLogger struct {
	mock.Mock
}

// Debug provides a mock function with given fields: args
func (_m *MockLogger) Debug(args ...interface{}) {
	_m.Called(args)
}

// Debugf provides a mock function with given fields: format, args
func (_m *MockLogger) Debugf(format string, args ...interface{}) {
	_m.Called(format, args)
}

// Info provides a mock function with given fields: args
func (_m *MockLogger) Info(args ...interface{}) {
	_m.Called(args)
}

// Infof provides a mock function with given fields: format, args
func (_m *MockLogger) Infof(format string, args ...interface{}) {
	_m.Called(format, args)
}

// Warn provides a mock function with given fields: args
func (_m *MockLogger) Warn(args ...interface{}) {
	_m.Called(args)
}

// Warnf provides a mock function with given fields: format, args
func (_m *MockLogger) Warnf(format string, args ...interface{}) {
	_m.Called(format, args)
}

// Error provides a mock function with given fields: args
func (_m *MockLogger) Error(args ...interface{}) {
	_m.Called(args)
}

// Errorf provides a mock function with given fields: format, args
func (_m *MockLogger) Errorf(format string, args ...interface{}) {
	_m.Called(format, args)
}

// Fatal provides a mock function with given fields: args
func (_m *MockLogger) Fatal(args ...interface{}) {
	_m.Called(args)
}

// Fatalf provides a mock function with given fields: format, args
func (_m *MockLogger) Fatalf(format string, args ...interface{}) {
	_m.Called(format, args)
}

// Panic provides a mock function with given fields: args
func (_m *MockLogger) Panic(args ...interface{}) {
	_m.Called(args)
}

// Panicf provides a mock function with given fields: format, args
func (_m *MockLogger) Panicf(format string, args ...interface{}) {
	_m.Called(format, args)
}

// WithField provides a mock function with given fields: key, value
func (_m *MockLogger) WithField(key string, value interface{}) bark.Logger {
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
func (_m *MockLogger) WithFields(keyValues bark.LogFields) bark.Logger {
	ret := _m.Called(keyValues)

	var r0 bark.Logger
	if rf, ok := ret.Get(0).(func(bark.LogFields) bark.Logger); ok {
		r0 = rf(keyValues)
	} else {
		r0 = ret.Get(0).(bark.Logger)
	}

	return r0
}

// WithError provides a mock function with given fields: err
func (_m *MockLogger) WithError(err error) bark.Logger {
	ret := _m.Called(err)

	var r0 bark.Logger
	if rf, ok := ret.Get(0).(func(error) bark.Logger); ok {
		r0 = rf(err)
	} else {
		r0 = ret.Get(0).(bark.Logger)
	}

	return r0
}

// Fields provides a mock function with given fields:
func (_m *MockLogger) Fields() bark.Fields {
	ret := _m.Called()

	var r0 bark.Fields
	if rf, ok := ret.Get(0).(func() bark.Fields); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(bark.Fields)
	}

	return r0
}
