package logger

import (
	"fmt"
	"github.com/uber-common/bark"
)

// LoggerFactory wraps a bark.Logger and provides a way to create named loggers
// restricted to a specific Level.
type LoggerFactory struct {
	logger bark.Logger
	cache  map[string]*namedLogger
}

func NewLoggerFactory(l bark.Logger) *LoggerFactory {
	return &LoggerFactory{
		logger: l,
		cache:  make(map[string]*namedLogger),
	}
}

// SetLevel sets the minimum level for a named logger. A named logger emits
// messages with a level equal to or greater than this level.
func (lf *LoggerFactory) SetLevel(name string, level Level) error {
	if level < lowestLevel {
		return fmt.Errorf("level must be higher than %s", lowestLevel)
	}
	if level > highestLevel {
		return fmt.Errorf("level must be lower than %s", highestLevel)
	}
	if restricted, ok := lf.cache[name]; ok {
		restricted.setLevel(level)
	} else {
		lf.cache[name] = &namedLogger{
			logger: lf.logger,
			min:    level,
		}
	}
	return nil
}

// Logger returns a named logger. If no level was previously set for this
// named logger it defaults to the minimum level.
func (lf *LoggerFactory) Logger(name string) Logger {
	// XXX: fix races
	if restricted, ok := lf.cache[name]; ok {
		return restricted
	} else {
		restricted := &namedLogger{
			logger: lf.logger,
			min:    lowestLevel,
		}
		lf.cache[name] = restricted
		return restricted
	}
}

// Override the bark logger implementation with another one.
func (lf *LoggerFactory) SetLogger(logger bark.Logger) {
	lf.logger = logger
	for _, namedLogger := range lf.cache {
		namedLogger.setLogger(logger)
	}
}
