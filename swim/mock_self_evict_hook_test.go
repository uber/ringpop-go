package swim

import "github.com/stretchr/testify/mock"

type MockSelfEvictHook struct {
	mock.Mock
}

// Name provides a mock function with given fields:
func (_m *MockSelfEvictHook) Name() string {
	ret := _m.Called()

	var r0 string
	if rf, ok := ret.Get(0).(func() string); ok {
		r0 = rf()
	} else {
		r0 = ret.Get(0).(string)
	}

	return r0
}

// PreEvict provides a mock function with given fields:
func (_m *MockSelfEvictHook) PreEvict() {
	_m.Called()
}

// PostEvict provides a mock function with given fields:
func (_m *MockSelfEvictHook) PostEvict() {
	_m.Called()
}
