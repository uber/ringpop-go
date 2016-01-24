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
	"github.com/Sirupsen/logrus"
	"github.com/uber-common/bark"
	"io/ioutil"
)

// Options is used to set the ringpop logger implementation and the minimum
// output level for each named logger.
type Options struct {
	// The underlying logger implementation to use.
	Logger bark.Logger

	// Minimum level to which the Ringpop logger is restricted.
	Ringpop Level
	// Minimum level to which the Gossip logger is restricted.
	Gossip Level
	// Minimum level to which the Suspicion logger is restricted.
	Suspicion Level
	// Minimum level to which the Ring logger is restricted.
	Ring Level
	// Minimum level to which the Membership logger is restricted.
	Membership Level
	// Minimum level to which the Damping logger is restricted.
	Damping Level
}

// LogFactory provides a convenient way to produce named loggers and pass
// metadata as fields between separate parts of the codebase.
type LogFactory interface {
	// Ringpop retutrns the logger used to log messages outside of any module.
	Ringpop() Logger
	// Gossip returns the logger used to log messages in the "gossip" module.
	Gossip() Logger
	// Suspicion returns the logger used to log messages in the "suspicion" module.
	Suspicion() Logger
	// Ring returns the logger used to log messages in the "ring" module.
	Ring() Logger
	// Membership returns the logger used to log messages in the "membership" module.
	Membership() Logger
	// Damping returns the logger used to log messages in the "damping" module.
	Damping() Logger
	WithField(key string, value interface{}) LogFactory
	WithFields(fields Fields) LogFactory
}

// LogFacility is a LogFactory with some extra configuration methods.
type LogFacility interface {
	LogFactory
	// Update changes the underlying logger and the levels for each named
	// logger. If no logger is provided, the existing one is preserved;
	// the same is true for unset level values.
	Update(opts Options)
}

type ringpopFacility struct {
	facility Facility
}

// New returns a static wrapper around Facility used by ringpop to produce
// named loggers. Each named logger can be configured independently to output
// messages only above a minimum level. If no logger is passed, one is created
// by default.
func New(opts Options) LogFacility {
	defaultLogger := bark.NewLoggerFromLogrus(&logrus.Logger{
		Out: ioutil.Discard,
	})
	rf := &ringpopFacility{
		facility: NewFacility(defaultLogger),
	}
	rf.Update(opts)
	return rf
}

const (
	ringpopLogger    = "ringpop"
	gossipLogger     = "gossip"
	suspicionLogger  = "suspicion"
	ringLogger       = "ring"
	membershipLogger = "membership"
	dampingLogger    = "damping"
)

func (rf *ringpopFacility) Update(opts Options) {
	if opts.Logger != nil {
		rf.facility.SetLogger(opts.Logger)
	}
	if opts.Ringpop != nil {
		rf.facility.SetLevel(ringpopLogger, opts.Ringpop)
	}
	if opts.Gossip != nil {
		rf.facility.SetLevel(gossipLogger, opts.Gossip)
	}
	if opts.Suspicion != nil {
		rf.facility.SetLevel(suspicionLogger, opts.Suspicion)
	}
	if opts.Ring != nil {
		rf.facility.SetLevel(ringLogger, opts.Ring)
	}
	if opts.Membership != nil {
		rf.facility.SetLevel(membershipLogger, opts.Membership)
	}
	if opts.Damping != nil {
		rf.facility.SetLevel(dampingLogger, opts.Damping)
	}
}

func (rf *ringpopFacility) Ringpop() Logger    { return rf.facility.Logger(ringpopLogger) }
func (rf *ringpopFacility) Gossip() Logger     { return rf.facility.Logger(gossipLogger) }
func (rf *ringpopFacility) Suspicion() Logger  { return rf.facility.Logger(suspicionLogger) }
func (rf *ringpopFacility) Ring() Logger       { return rf.facility.Logger(ringLogger) }
func (rf *ringpopFacility) Membership() Logger { return rf.facility.Logger(membershipLogger) }
func (rf *ringpopFacility) Damping() Logger    { return rf.facility.Logger(dampingLogger) }

func (rf *ringpopFacility) WithField(key string, value interface{}) LogFactory {
	return &ringpopFactory{factory: rf.facility.WithField(key, value)}
}
func (rf *ringpopFacility) WithFields(fields Fields) LogFactory {
	return &ringpopFactory{factory: rf.facility.WithFields(fields)}
}

type ringpopFactory struct {
	factory Factory
}

func (rf *ringpopFactory) Ringpop() Logger    { return rf.factory.Logger(ringpopLogger) }
func (rf *ringpopFactory) Gossip() Logger     { return rf.factory.Logger(gossipLogger) }
func (rf *ringpopFactory) Suspicion() Logger  { return rf.factory.Logger(suspicionLogger) }
func (rf *ringpopFactory) Ring() Logger       { return rf.factory.Logger(ringLogger) }
func (rf *ringpopFactory) Membership() Logger { return rf.factory.Logger(membershipLogger) }
func (rf *ringpopFactory) Damping() Logger    { return rf.factory.Logger(dampingLogger) }

func (rf *ringpopFactory) WithField(key string, value interface{}) LogFactory {
	return &ringpopFactory{factory: rf.factory.WithField(key, value)}
}
func (rf *ringpopFactory) WithFields(fields Fields) LogFactory {
	return &ringpopFactory{factory: rf.factory.WithFields(fields)}
}
