package pingpong

import "github.com/stretchr/testify/mock"

import "github.com/uber/tchannel-go/thrift"

type MockTChanPingPong struct {
	mock.Mock
}

// Ping provides a mock function with given fields: ctx, request
func (_m *MockTChanPingPong) Ping(ctx thrift.Context, request *Ping) (*Pong, error) {
	ret := _m.Called(ctx, request)

	var r0 *Pong
	if rf, ok := ret.Get(0).(func(thrift.Context, *Ping) *Pong); ok {
		r0 = rf(ctx, request)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*Pong)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(thrift.Context, *Ping) error); ok {
		r1 = rf(ctx, request)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
