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

package ringpop

import (
	"errors"
	"time"

	"github.com/benbjohnson/clock"
	log "github.com/uber-common/bark"
	"github.com/uber/ringpop-go/hashring"
	"github.com/uber/ringpop-go/logging"
	"github.com/uber/ringpop-go/shared"
)

type configuration struct {
	// App is the name used to uniquely identify members of the same ring.
	// Members will only talk to other members with the same app name. Note
	// that App is taken as an argument of the Ringpop constructor and not a
	// configuration option. This is to prevent accidental misconfiguration.
	App string

	// Configure the period by which ringpop emits the stat
	// "ring.checksum-periodic". See func RingChecksumStatPeriod for
	// specifics.
	RingChecksumStatPeriod time.Duration
}

// An Option is a modifier functions that configure/modify a real Ringpop
// object.
//
// There are typically two types of runtime options you can provide: flags
// (functions that modify the object) and value options (functions the accept
// user-specific arguments and then return a function that modifies the
// object).
//
// For more information, see:
// http://commandcenter.blogspot.com/2014/01/self-referential-functions-and-design.html
//
type Option func(*Ringpop) error

// applyOptions applies runtime configuration options to the specified Ringpop
// instance.
func applyOptions(r *Ringpop, opts []Option) error {
	for _, option := range opts {
		err := option(r)
		if err != nil {
			return err
		}
	}
	return nil
}

// checkOptions checks that the Ringpop instance has been properly configured
// with all the required options.
func checkOptions(rp *Ringpop) []error {
	errs := []error{}
	if rp.channel == nil {
		errs = append(errs, errors.New("channel is required"))
	}
	if rp.identityResolver == nil {
		errs = append(errs, errors.New("identity resolver is nil"))
	}
	return errs
}

// Runtime options

// Clock is used to set the Clock mechanism.  Testing harnesses will typically
// replace this with a mocked clock.
func Clock(c clock.Clock) Option {
	return func(r *Ringpop) error {
		if c == nil {
			return errors.New("clock is required")
		} else {
			r.clock = c
		}
		return nil
	}
}

// Channel is used to provide a TChannel instance that Ringpop should use for
// all communication.
//
// Example:
//
//     rp, err := ringpop.New("my-app", ringpop.Channel(myChannel))
//
// Channel is a required option. The constructor will throw an error if this
// option is not present.
func Channel(ch shared.TChannel) Option {
	return func(r *Ringpop) error {
		r.channel = ch
		return nil
	}
}

// HashRingConfig takes a `HashRingConfiguration` struct that can be used to
// configure the hash ring.
//
// Example:
//
//     rp, err := ringpop.New("my-app",
//         ringpop.Channel(myChannel),
//         ringpop.HashRingConfig(&HashRingConfiguration{
//             ReplicaPoints: 100,
//         }),
//     )
//
// See documentation on the `HashRingConfiguration` struct for more information
// about what options are available.
func HashRingConfig(c *hashring.Configuration) Option {
	return func(r *Ringpop) error {
		r.configHashRing = c
		return nil
	}
}

// Logger is used to specify a bark-compatible logger that will be used for
// all Ringpop logging. If a logger is not provided, one will be created
// automatically.
func Logger(l log.Logger) Option {
	return func(r *Ringpop) error {
		logging.SetLogger(l)
		return nil
	}
}

// LogLevels is used to set the severity log level for all Ringpop named
// loggers.
func LogLevels(levels map[string]logging.Level) Option {
	return func(r *Ringpop) error {
		return logging.SetLevels(levels)
	}
}

// Statter is used to specify a bark-compatible (bark.StatsReporter) stats
// reporter that will be used to record ringpop stats. If a statter is not
// provided, stats will be emitted to a null stats-reporter.
func Statter(s log.StatsReporter) Option {
	return func(r *Ringpop) error {
		r.statter = s
		return nil
	}
}

// Identity is used to specify a static hostport string as this Ringpop
// instance's identity.
//
// Example:
//
//     ringpop.New("my-app",
//         ringpop.Channel(myChannel),
//         ringpop.Identity("10.32.12.2:21130"),
//     )
//
// You should make sure the identity matches the listening address of the
// TChannel object.
//
// By default, you do not need to provide an identity. If you do not provide
// one, the identity will be resolved automatically by the default resolver.
func Identity(hostport string) Option {
	return IdentityResolverFunc(func() (string, error) {
		return hostport, nil
	})
}

// IdentityResolver is a function that returns the listen interface/port
// that Ringpop should identify as.
type IdentityResolver func() (string, error)

// IdentityResolverFunc is used to specify a function that will called when
// the Ringpop instance needs to resolve its identity (typically, on
// bootstrap).
func IdentityResolverFunc(resolver IdentityResolver) Option {
	return func(r *Ringpop) error {
		r.identityResolver = resolver
		return nil
	}
}

// RingChecksumStatPeriodNever defines a "period" which disables
// ring.checksum-periodic stat emission.
const RingChecksumStatPeriodNever = time.Duration(-1)

// RingChecksumStatPeriodDefault defines the default emission period for the
// ring.checksum-periodic stat.
const RingChecksumStatPeriodDefault = time.Duration(5 * time.Second)

// RingChecksumStatPeriod configures the period between emissions of the stat
// 'ring.checksum-periodic'. Using a value <=0 (or RingChecksumStatPeriodNever)
// will disable emission of this stat. Using a value in (0, 10ms) will return
// an error, as that value is unrealistically small. Normal values must
// therefore be >=10ms. RingChecksumStatPeriodDefault defines the default.
func RingChecksumStatPeriod(period time.Duration) Option {
	return func(r *Ringpop) error {
		if period <= 0 {
			period = RingChecksumStatPeriodNever
		} else if period < 10*time.Millisecond {
			return errors.New("ring checksum stat period invalid below 10 ms")
		}
		r.config.RingChecksumStatPeriod = period
		return nil
	}
}

// Default options

// defaultClock sets the ringpop clock interface to use the system clock
func defaultClock(r *Ringpop) error {
	return Clock(clock.New())(r)
}

// defaultIdentityResolver sets the default identityResolver
func defaultIdentityResolver(r *Ringpop) error {
	r.identityResolver = r.channelIdentityResolver
	return nil
}

// defaultLogLevels is the default configuration for all Ringpop named loggers.
func defaultLogLevels(r *Ringpop) error {
	return LogLevels(map[string]logging.Level{
		"damping":       logging.Error,
		"dissemination": logging.Error,
		"gossip":        logging.Error,
		"join":          logging.Warn,
		"membership":    logging.Error,
		"ring":          logging.Error,
		"suspicion":     logging.Error,
	})(r)
}

func defaultStatter(r *Ringpop) error {
	return Statter(noopStatsReporter{})(r)
}

func defaultHashRingOptions(r *Ringpop) error {
	return HashRingConfig(defaultHashRingConfiguration)(r)
}

func defaultRingChecksumStatPeriod(r *Ringpop) error {
	return RingChecksumStatPeriod(RingChecksumStatPeriodDefault)(r)
}

// defaultOptions are the default options/values when Ringpop is created. They
// can be overridden at runtime.
var defaultOptions = []Option{
	defaultClock,
	defaultIdentityResolver,
	defaultLogLevels,
	defaultStatter,
	defaultRingChecksumStatPeriod,
	defaultHashRingOptions,
}

var defaultHashRingConfiguration = &hashring.Configuration{
	ReplicaPoints: 100,
}
