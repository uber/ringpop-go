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
	"github.com/Sirupsen/logrus"
	"github.com/uber-common/bark"
	"io/ioutil"
)

// Options is used to set the ringpop logger implementation and the minimum
// output level for each named logger.
type Options struct {
	// The underlying logger implementation to use.
	Logger bark.Logger

	Ringpop    Level
	Gossip     Level
	Suspicion  Level
	Ring       Level
	Membership Level
	Damping    Level
}

// RingpopLogger is a factory for ringpop named loggers. Each named logger can
// be configured independently to output messages only above a minimum level.
type RingpopLogger struct {
	factory *LoggerFactory
}

// New creates a logger factory. If no logger is provided, a default one is
// used. Unset module levels default to Warn.
func New(conf Options) (*RingpopLogger, error) {
	defaultLogger := bark.NewLoggerFromLogrus(&logrus.Logger{
		Out: ioutil.Discard,
	})
	logger := &RingpopLogger{
		factory: NewLoggerFactory(defaultLogger),
	}
	if err := logger.Update(conf); err != nil {
		return nil, err
	}
	return logger, nil
}

const (
	ringpopLogger    = "ringpop"
	gossipLogger     = "gossip"
	suspicionLogger  = "suspicion"
	ringLogger       = "ring"
	membershipLogger = "membership"
	dampingLogger    = "damping"
)

// Update changes the underlying logger and the levels for each named logger.
// If no logger is provided, the existing one is preserved; the same is true
// for unset level values.
func (rl *RingpopLogger) Update(conf Options) error {
	if conf.Logger != nil {
		rl.factory.SetLogger(conf.Logger)
	}
	if conf.Ringpop != unset {
		if err := rl.factory.SetLevel(ringpopLogger, conf.Ringpop); err != nil {
			return fmt.Errorf("Config.Ringpop: %v", err)
		}
	}
	if conf.Gossip != unset {
		if err := rl.factory.SetLevel(gossipLogger, conf.Gossip); err != nil {
			return fmt.Errorf("Config.Gossip: %v", err)
		}
	}
	if conf.Suspicion != unset {
		if err := rl.factory.SetLevel(suspicionLogger, conf.Suspicion); err != nil {
			return fmt.Errorf("Config.Suspicion: %v", err)
		}
	}
	if conf.Ring != unset {
		if err := rl.factory.SetLevel(ringLogger, conf.Ring); err != nil {
			return fmt.Errorf("Config.Ring: %v", err)
		}
	}
	if conf.Membership != unset {
		if err := rl.factory.SetLevel(membershipLogger, conf.Membership); err != nil {
			return fmt.Errorf("Config.Membership: %v", err)
		}
	}
	if conf.Damping != unset {
		if err := rl.factory.SetLevel(dampingLogger, conf.Damping); err != nil {
			return fmt.Errorf("Config.Damping: %v", err)
		}
	}
	return nil
}

func (rl *RingpopLogger) Ringpop() Logger {
	return rl.factory.Logger(ringpopLogger)
}

func (rl *RingpopLogger) Gossip() Logger {
	return rl.factory.Logger(gossipLogger)
}

func (rl *RingpopLogger) Suspicion() Logger {
	return rl.factory.Logger(suspicionLogger)
}

func (rl *RingpopLogger) Ring() Logger {
	return rl.factory.Logger(ringLogger)
}

func (rl *RingpopLogger) Membership() Logger {
	return rl.factory.Logger(membershipLogger)
}

func (rl *RingpopLogger) Damping() Logger {
	return rl.factory.Logger(dampingLogger)
}
