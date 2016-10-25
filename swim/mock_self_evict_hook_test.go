package swim

import "github.com/stretchr/testify/mock"

type MockSelfEvictHook struct {
	mock.Mock
}

// PreEvict provides a mock function with given fields:
func (_m *MockSelfEvictHook) PreEvict() {
	_m.Called()
}

// PostEvict provides a mock function with given fields:
func (_m *MockSelfEvictHook) PostEvict() {
	_m.Called()
}
