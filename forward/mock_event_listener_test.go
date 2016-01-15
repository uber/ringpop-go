package forward

import "github.com/uber/ringpop-go/events"
import "github.com/stretchr/testify/mock"

type EventListener struct {
	mock.Mock
}

func (_m *EventListener) HandleEvent(event events.Event) {
	_m.Called(event)
}
