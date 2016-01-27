package logging

import "github.com/uber-common/bark"
import "github.com/stretchr/testify/mock"

type MockLogger struct {
	mock.Mock
}

func (_m *MockLogger) Debug(args ...interface{}) {
	_m.Called(args)
}
func (_m *MockLogger) Debugf(format string, args ...interface{}) {
	_m.Called(format, args)
}
func (_m *MockLogger) Info(args ...interface{}) {
	_m.Called(args)
}
func (_m *MockLogger) Infof(format string, args ...interface{}) {
	_m.Called(format, args)
}
func (_m *MockLogger) Warn(args ...interface{}) {
	_m.Called(args)
}
func (_m *MockLogger) Warnf(format string, args ...interface{}) {
	_m.Called(format, args)
}
func (_m *MockLogger) Error(args ...interface{}) {
	_m.Called(args)
}
func (_m *MockLogger) Errorf(format string, args ...interface{}) {
	_m.Called(format, args)
}
func (_m *MockLogger) Fatal(args ...interface{}) {
	_m.Called(args)
}
func (_m *MockLogger) Fatalf(format string, args ...interface{}) {
	_m.Called(format, args)
}
func (_m *MockLogger) Panic(args ...interface{}) {
	_m.Called(args)
}
func (_m *MockLogger) Panicf(format string, args ...interface{}) {
	_m.Called(format, args)
}
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
