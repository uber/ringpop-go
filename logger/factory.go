// Copyright (c) 2015 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

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
	if named, ok := lf.cache[name]; ok {
		named.setLevel(level)
	} else {
		lf.cache[name] = &namedLogger{
			logger: lf.logger,
			min:    level,
		}
	}
	return nil
}

// Logger returns a named logger. If no level was previously set for this
// named logger it defaults to Warn.
func (lf *LoggerFactory) Logger(name string) Logger {
	// XXX: fix races
	if named, ok := lf.cache[name]; ok {
		return named
	} else {
		named := &namedLogger{
			logger: lf.logger,
			min:    defaultNotConfiguredNamedLogger,
		}
		lf.cache[name] = named
		return named
	}
}

// Override the bark logger implementation with another one.
func (lf *LoggerFactory) SetLogger(logger bark.Logger) {
	lf.logger = logger
	for _, named := range lf.cache {
		named.setLogger(logger)
	}
}
