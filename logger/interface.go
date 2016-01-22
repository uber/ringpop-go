package logger

import (
	"github.com/uber-common/bark"
)

// Similar to bark.Logger but with an extra Trace method.
type Logger interface {
	bark.Logger
	// Forwards to Debug
	Trace(args ...interface{})
	Tracef(format string, args ...interface{})
}
