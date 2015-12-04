package mocks

import "github.com/stretchr/testify/mock"

import "time"

import "github.com/uber/ringpop-go/events"
import "github.com/uber/ringpop-go/forward"

import "github.com/uber/ringpop-go/swim"
import "github.com/uber/tchannel-go"

type Ringpop struct {
	mock.Mock
}

func (_m *Ringpop) Destroy() {
	_m.Called()
}
func (_m *Ringpop) Destroyed() bool {
	ret := _m.Called()

	var r0 bool
	if rf, ok := ret.Get(0).(func() bool); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}
func (_m *Ringpop) App() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}
func (_m *Ringpop) WhoAmI() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}
func (_m *Ringpop) Uptime() time.Duration {
	ret := _m.Called()

	var r0 time.Duration
	if rf, ok := ret.Get(0).(func() time.Duration); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(time.Duration)
	}

	return r0
}
func (_m *Ringpop) RegisterListener(l events.EventListener) {
	_m.Called(l)
}
func (_m *Ringpop) Bootstrap(opts *swim.BootstrapOptions) ([]string, error) {
	ret := _m.Called(opts)

	var r0 []string
	if rf, ok := ret.Get(0).(func(*swim.BootstrapOptions) []string); ok {
		r0 = rf(opts)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]string)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(*swim.BootstrapOptions) error); ok {
		r1 = rf(opts)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
func (_m *Ringpop) HandleEvent(event interface{}) {
	_m.Called(event)
}
func (_m *Ringpop) Checksum() uint32 {
	ret := _m.Called()

	var r0 uint32
	if rf, ok := ret.Get(0).(func() uint32); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(uint32)
	}

	return r0
}
func (_m *Ringpop) Lookup(key string) string {
	ret := _m.Called(key)

	var r0 string
	if rf, ok := ret.Get(0).(func(string) string); ok {
		r0 = rf(key)
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}
func (_m *Ringpop) LookupN(key string, n int) []string {
	ret := _m.Called(key, n)

	var r0 []string
	if rf, ok := ret.Get(0).(func(string, int) []string); ok {
		r0 = rf(key, n)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]string)
		}
	}

	return r0
}
func (_m *Ringpop) HandleOrForward(key string, request []byte, response *[]byte, service string, endpoint string, format tchannel.Format, opts *forward.Options) (bool, error) {
	ret := _m.Called(key, request, response, service, endpoint, format, opts)

	var r0 bool
	if rf, ok := ret.Get(0).(func(string, []byte, *[]byte, string, string, tchannel.Format, *forward.Options) bool); ok {
		r0 = rf(key, request, response, service, endpoint, format, opts)
	} else {
		r0 = ret.Get(0).(bool)
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, []byte, *[]byte, string, string, tchannel.Format, *forward.Options) error); ok {
		r1 = rf(key, request, response, service, endpoint, format, opts)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
func (_m *Ringpop) Forward(dest string, keys []string, request []byte, service string, endpoint string, format tchannel.Format, opts *forward.Options) ([]byte, error) {
	ret := _m.Called(dest, keys, request, service, endpoint, format, opts)

	var r0 []byte
	if rf, ok := ret.Get(0).(func(string, []string, []byte, string, string, tchannel.Format, *forward.Options) []byte); ok {
		r0 = rf(dest, keys, request, service, endpoint, format, opts)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]byte)
		}
	}

	var r1 error
	if rf, ok := ret.Get(1).(func(string, []string, []byte, string, string, tchannel.Format, *forward.Options) error); ok {
		r1 = rf(dest, keys, request, service, endpoint, format, opts)
	} else {
		r1 = ret.Error(1)
	}

	return r0, r1
}
