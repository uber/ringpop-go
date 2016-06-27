package mocks

import "github.com/stretchr/testify/mock"

import "github.com/uber/tchannel-go/thrift"

type ClientFactory struct {
	mock.Mock
}

// GetLocalClient provides a mock function with given fields:
func (_m *ClientFactory) GetLocalClient() interface{} {
	ret := _m.Called()

	var r0 interface{}
	if rf, ok := ret.Get(0).(func() interface{}); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(interface{})
		}
	}

	return r0
}

// MakeRemoteClient provides a mock function with given fields: client
func (_m *ClientFactory) MakeRemoteClient(client thrift.TChanClient) interface{} {
	ret := _m.Called(client)

	var r0 interface{}
	if rf, ok := ret.Get(0).(func(thrift.TChanClient) interface{}); ok {
		r0 = rf(client)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(interface{})
		}
	}

	return r0
}
