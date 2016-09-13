package events

import "github.com/stretchr/testify/mock"

type MockEventListener struct {
	mock.Mock
}

// HandleEvent provides a mock function with given fields: event
func (_m *MockEventListener) HandleEvent(event Event) {
	_m.Called(event)
}
