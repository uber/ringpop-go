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

// CountMembers provides a mock function with given fields: predicates
func (_m *SwimNode) CountMembers(predicates ...swim.MemberPredicate) int {
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

// GetMembers provides a mock function with given fields: predicates
func (_m *SwimNode) GetMembers(predicates ...swim.MemberPredicate) []swim.Member {
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

// RegisterListener provides a mock function with given fields: l
func (_m *SwimNode) RegisterListener(l events.EventListener) {
	_m.Called(l)
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
