package mocks

import "github.com/uber/ringpop-go/swim"
import "github.com/stretchr/testify/mock"

import "github.com/uber/ringpop-go/events"

type SwimNode struct {
	mock.Mock
}

// Bootstrap provides a mock function with given fields: opts
func (_m *SwimNode) Bootstrap(opts *swim.BootstrapOptions) ([]string, error) {
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

// CountReachableMembers provides a mock function with given fields: predicates
func (_m *SwimNode) CountReachableMembers(predicates ...swim.MemberPredicate) int {
	ret := _m.Called(predicates)

	var r0 int
	if rf, ok := ret.Get(0).(func(...swim.MemberPredicate) int); ok {
		r0 = rf(predicates...)
	} else {
		r0 = ret.Get(0).(int)
	}

	return r0
}

// Destroy provides a mock function with given fields:
func (_m *SwimNode) Destroy() {
	_m.Called()
}

// GetChecksum provides a mock function with given fields:
func (_m *SwimNode) GetChecksum() uint32 {
	ret := _m.Called()

	var r0 uint32
	if rf, ok := ret.Get(0).(func() uint32); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(uint32)
	}

	return r0
}

// GetReachableMembers provides a mock function with given fields: predicates
func (_m *SwimNode) GetReachableMembers(predicates ...swim.MemberPredicate) []swim.Member {
	ret := _m.Called(predicates)

	var r0 []swim.Member
	if rf, ok := ret.Get(0).(func(...swim.MemberPredicate) []swim.Member); ok {
		r0 = rf(predicates...)
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).([]swim.Member)
		}
	}

	return r0
}

// Labels provides a mock function with given fields:
func (_m *SwimNode) Labels() *swim.NodeLabels {
	ret := _m.Called()

	var r0 *swim.NodeLabels
	if rf, ok := ret.Get(0).(func() *swim.NodeLabels); ok {
		r0 = rf()
	} else {
		if ret.Get(0) != nil {
			r0 = ret.Get(0).(*swim.NodeLabels)
		}
	}

	return r0
}

// MemberStats provides a mock function with given fields:
func (_m *SwimNode) MemberStats() swim.MemberStats {
	ret := _m.Called()

	var r0 swim.MemberStats
	if rf, ok := ret.Get(0).(func() swim.MemberStats); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(swim.MemberStats)
	}

	return r0
}

// ProtocolStats provides a mock function with given fields:
func (_m *SwimNode) ProtocolStats() swim.ProtocolStats {
	ret := _m.Called()

	var r0 swim.ProtocolStats
	if rf, ok := ret.Get(0).(func() swim.ProtocolStats); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(swim.ProtocolStats)
	}

	return r0
}

// Ready provides a mock function with given fields:
func (_m *SwimNode) Ready() bool {
	ret := _m.Called()

	var r0 bool
	if rf, ok := ret.Get(0).(func() bool); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// AddListener provides a mock function with given fields: _a0
func (_m *SwimNode) AddListener(_a0 events.EventListener) bool {
	ret := _m.Called(_a0)

	var r0 bool
	if rf, ok := ret.Get(0).(func(events.EventListener) bool); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// RemoveListener provides a mock function with given fields: _a0
func (_m *SwimNode) RemoveListener(_a0 events.EventListener) bool {
	ret := _m.Called(_a0)

	var r0 bool
	if rf, ok := ret.Get(0).(func(events.EventListener) bool); ok {
		r0 = rf(_a0)
	} else {
		r0 = ret.Get(0).(bool)
	}

	return r0
}

// RegisterSelfEvictHook provides a mock function with given fields: hooks
func (_m *SwimNode) RegisterSelfEvictHook(hooks swim.SelfEvictHook) error {
	ret := _m.Called(hooks)

	var r0 error
	if rf, ok := ret.Get(0).(func(swim.SelfEvictHook) error); ok {
		r0 = rf(hooks)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SelfEvict provides a mock function with given fields:
func (_m *SwimNode) SelfEvict() error {
	ret := _m.Called()

	var r0 error
	if rf, ok := ret.Get(0).(func() error); ok {
		r0 = rf()
	} else {
		r0 = ret.Error(0)
	}

	return r0
}

// SetIdentity provides a mock function with given fields: identity
func (_m *SwimNode) SetIdentity(identity string) error {
	ret := _m.Called(identity)

	var r0 error
	if rf, ok := ret.Get(0).(func(string) error); ok {
		r0 = rf(identity)
	} else {
		r0 = ret.Error(0)
	}

	return r0
}
