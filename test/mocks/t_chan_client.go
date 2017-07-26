package mocks

import "github.com/uber/tchannel-go/thrift"
import "github.com/stretchr/testify/mock"

import athrift "github.com/apache/thrift/lib/go/thrift"

type TChanClient struct {
	mock.Mock
}

// Call provides a mock function with given fields: ctx, serviceName, methodName, req, resp
func (_m *TChanClient) Call(ctx thrift.Context, serviceName string, methodName string, req athrift.TStruct, resp athrift.TStruct) (bool, error) {
	ret := _m.Called(ctx, serviceName, methodName, req, resp)

	var r0 bool
	if rf, ok := ret.Get(0).(func(thrift.Context, string, string, athrift.TStruct, athrift.TStruct) bool); ok {
		r0 = rf(ctx, serviceName, methodName, req, resp)
	} else {
		r0 = ret.Get(0).(bool)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(thrift.Context, string, string, athrift.TStruct, athrift.TStruct) error); ok {
		r1 = rf(ctx, serviceName, methodName, req, resp)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
