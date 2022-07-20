package remoteservice

import (
	"github.com/stretchr/testify/mock"
	"github.com/uber/ringpop-go/test/remoteservice/gen/go/shared"
	"github.com/uber/tchannel-go/thrift"
)

type TChanRemoteService struct {
	mock.Mock
}

// RemoteCall provides a mock function with given fields: ctx, name
func (_m *TChanRemoteService) RemoteCall(ctx thrift.Context, name shared.Name) error {
	ret := _m.Called(ctx, name)

	var r0 error
	if rf, ok := ret.Get(0).(func(thrift.Context, shared.Name) error); ok {
		r0 = rf(ctx, name)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
