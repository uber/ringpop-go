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
	"io/ioutil"

	"github.com/Sirupsen/logrus"
	log "github.com/uber-common/bark"
	"github.com/uber/ringpop-go/hashring"
	"github.com/uber/ringpop-go/shared"
)

type configuration struct {
	// App is the name used to uniquely identify members of the same ring.
	// Members will only talk to other members with the same app name. Note
	// that App is taken as an argument of the Ringpop constructor and not a
	// configuration option. This is to prevent accidental misconfiguration.
	App string
}

// HashRingConfiguration is a configuration struct that can be passed to the
// Ringpop constructor to customize hash ring options.
type HashRingConfiguration struct {
	// ReplicaPoints is the number of positions a node will be assigned on the
	// ring. A bigger number will provide better key distribution, but require
	// more computation when building or traversing the ring (typically on
	// lookups or membership changes).
	ReplicaPoints int
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
		errs = append(errs, errors.New("Channel is required"))
	}
	if rp.identityResolver == nil {
		errs = append(errs, errors.New("Identity resolver is nil"))
	}
	return errs
}

// Runtime options

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
func HashRingConfig(c *hashring.HashRingConfiguration) Option {
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
		r.logger = l
		r.log = l
		return nil
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

// Default options

// defaultIdentityResolver sets the default identityResolver
func defaultIdentityResolver(r *Ringpop) error {
	r.identityResolver = r.channelIdentityResolver
	return nil
}

// defaultLogger is the default logger that is used for Ringpop if one is not
// provided by the user.
func defaultLogger(r *Ringpop) error {
	return Logger(log.NewLoggerFromLogrus(&logrus.Logger{
		Out: ioutil.Discard,
	}))(r)
}

func defaultStatter(r *Ringpop) error {
	return Statter(noopStatsReporter{})(r)
}

func defaultHashRingOptions(r *Ringpop) error {
	return HashRingConfig(defaultHashRingConfiguration)(r)
}

// defaultOptions are the default options/values when Ringpop is created. They
// can be overridden at runtime.
var defaultOptions = []Option{
	defaultIdentityResolver,
	defaultLogger,
	defaultStatter,
	defaultHashRingOptions,
}

var defaultHashRingConfiguration = &hashring.HashRingConfiguration{
	ReplicaPoints: 100,
}
