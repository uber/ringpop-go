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
	"github.com/uber/ringpop-go/membership"
	"github.com/uber/ringpop-go/shared"
	"github.com/uber/ringpop-go/swim"
	"github.com/uber/ringpop-go/util"
)

type configuration struct {
	// App is the name used to uniquely identify members of the same ring.
	// Members will only talk to other members with the same app name. Note
	// that App is taken as an argument of the Ringpop constructor and not a
	// configuration option. This is to prevent accidental misconfiguration.
	App string

	// Configure the period by which ringpop emits the stats
	// "membership.checksum-periodic" and "ring.checksum-periodic".
	// See funcs {Membership,Ring}ChecksumStatPeriod for specifics.
	MembershipChecksumStatPeriod time.Duration
	RingChecksumStatPeriod       time.Duration

	// StateTimeouts keeps the state transition timeouts for swim to use
	StateTimeouts swim.StateTimeouts

	// LabelLimits keeps track of configured limits on labels. Among things the
	// number of labels and the size of key and value can be configured.
	LabelLimits swim.LabelOptions

	// InitialLabels configures the initial labels.
	InitialLabels swim.LabelMap

	// SelfEvict holds the settings with regards to self eviction
	SelfEvict swim.SelfEvictOptions

	// RequiresAppInPing configures if ringpop node should reject pings
	// that don't contain app name
	RequiresAppInPing bool
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
	if rp.addressResolver == nil {
		errs = append(errs, errors.New("address resolver is nil"))
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
		}
		r.clock = c
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

// Identity can be used to specify a custom string as the unique identifier for
// this node. The identity should be unique amongst other Ringpop instances; it
// is used in the hashring.
//
// By default, the hostport/address of the node is used as the identity in the
// hashring. An error is thrown if a hostport is manually specified using this
// option, as this would lead to unexpected behaviour. If you want to override
// the node's listening address, use the `Address` option.
func Identity(identity string) Option {
	return func(r *Ringpop) error {
		if util.HostportPattern.MatchString(identity) {
			return ErrInvalidIdentity
		}
		r.config.InitialLabels[membership.IdentityLabelKey] = identity
		return nil
	}
}

// Address is used to specify a static hostport string as this Ringpop
// instance's address.
//
// Example:
//
//     ringpop.New("my-app",
//         ringpop.Channel(myChannel),
//         ringpop.Address("10.32.12.2:21130"),
//     )
//
// You should make sure the address matches the listening address of the
// TChannel object.
//
// By default, you do not need to provide an address. If you do not provide
// one, the address will be resolved automatically by the default resolver.
func Address(address string) Option {
	return AddressResolverFunc(func() (string, error) {
		return address, nil
	})
}

// AddressResolver is a function that returns the listen interface/port
// that Ringpop should use as its address.
type AddressResolver func() (string, error)

// AddressResolverFunc is used to specify a function that will be called when
// the Ringpop instance needs to resolve its address (typically, on
// bootstrap).
func AddressResolverFunc(resolver AddressResolver) Option {
	return func(r *Ringpop) error {
		r.addressResolver = resolver
		return nil
	}
}

// StatPeriodNever defines a "period" which disables a periodic stat emission.
const StatPeriodNever = time.Duration(-1)

// StatPeriodDefault defines the default emission period for a periodic stat.
const StatPeriodDefault = time.Duration(5 * time.Second)

// MembershipChecksumStatPeriod configures the period between emissions of the
// stat 'membership.checksum-periodic'. Using a value <=0 (or StatPeriodNever)
// will disable emission of this stat. Using a value in (0, 10ms) will return
// an error, as that value is unrealistically small. Normal values must
// therefore be >=10ms.  StatPeriodDefault defines the default.
func MembershipChecksumStatPeriod(period time.Duration) Option {
	return func(r *Ringpop) error {
		if period <= 0 {
			period = StatPeriodNever
		} else if period < 10*time.Millisecond {
			return errors.New("membership checksum stat period invalid below 10 ms")
		}
		r.config.MembershipChecksumStatPeriod = period
		return nil
	}
}

// RingChecksumStatPeriod configures the period between emissions of the stat
// 'ring.checksum-periodic'. Using a value <=0 (or StatPeriodNever) will
// disable emission of this stat. Using a value in (0, 10ms) will return an
// error, as that value is unrealistically small. Normal values must therefore
// be >=10ms. StatPeriodDefault defines the default.
func RingChecksumStatPeriod(period time.Duration) Option {
	return func(r *Ringpop) error {
		if period <= 0 {
			period = StatPeriodNever
		} else if period < 10*time.Millisecond {
			return errors.New("ring checksum stat period invalid below 10 ms")
		}
		r.config.RingChecksumStatPeriod = period
		return nil
	}
}

// SuspectPeriod configures the period it takes ringpop to declare a node faulty
// after ringpop has first detected the node to be unresponsive to a healthcheck.
// When a node is declared faulty it is removed from the consistent hashring and
// stops forwarding traffic to that node. All keys previously routed to that node
// will then be routed to the new owner of the key
func SuspectPeriod(period time.Duration) Option {
	return func(r *Ringpop) error {
		r.config.StateTimeouts.Suspect = period
		return nil
	}
}

// FaultyPeriod configures the period Ringpop keeps a faulty node in its memberlist.
// Even though the node will not receive any traffic it is still present in the
// list in case it will come back online later. After this timeout ringpop will
// remove the node from its membership list permanently. If a node happens to come
// back after it has been removed from the membership Ringpop still allows it to
// join and take its old position in the hashring. To remove the node from the
// distributed membership it will mark it as a tombstone which can be removed from
// every members membership list independently.
func FaultyPeriod(period time.Duration) Option {
	return func(r *Ringpop) error {
		r.config.StateTimeouts.Faulty = period
		return nil
	}
}

// TombstonePeriod configures the period of the last time of the lifecycle in of
// a node in the membership list. This period should give the gossip protocol the
// time it needs to disseminate this change. If configured too short the node in
// question might show up again in faulty state in the distributed memberlist of
// Ringpop.
func TombstonePeriod(period time.Duration) Option {
	return func(r *Ringpop) error {
		r.config.StateTimeouts.Tombstone = period
		return nil
	}
}

// LabelLimitCount limits the number of labels an application can set on this
// node.
func LabelLimitCount(count int) Option {
	return func(r *Ringpop) error {
		r.config.LabelLimits.Count = count
		return nil
	}
}

// LabelLimitKeySize limits the size that a key of a label can be.
func LabelLimitKeySize(size int) Option {
	return func(r *Ringpop) error {
		r.config.LabelLimits.KeySize = size
		return nil
	}
}

// LabelLimitValueSize limits the size that a value of a label can be.
func LabelLimitValueSize(size int) Option {
	return func(r *Ringpop) error {
		r.config.LabelLimits.ValueSize = size
		return nil
	}
}

// SelfEvictPingRatio configures the maximum percentage/ratio of the members to
// actively ping while self evicting. A bigger ratio would allow for bigger
// batch sizes during restarts without the self eviction being lost due to all
// nodes having the knowledge being shutdown at the same time.
//
// A smaller ratio will cause less network traffic and therefore slightly faster
// shutdown times. A ratio that exceeds 1 will be capped to one when the self
// eviction is executed as it does not make sense to send the gossip to the same
// node twice. A negative value will cause no pings to be sent out during self
// eviction.
//
// In no case will there be more pings sent then makes sense by the limit of the
// current piggyback count
func SelfEvictPingRatio(ratio float64) Option {
	return func(r *Ringpop) error {
		r.config.SelfEvict.PingRatio = ratio
		return nil
	}
}

// RequiresAppInPing configures if ringpop node should reject pings
// that don't contain app name
func RequiresAppInPing(requiresAppInPing bool) Option {
	return func(r *Ringpop) error {
		r.config.RequiresAppInPing = requiresAppInPing
		return nil
	}
}

// Default options

// defaultClock sets the ringpop clock interface to use the system clock
func defaultClock(r *Ringpop) error {
	return Clock(clock.New())(r)
}

// defaultAddressResolver sets the default addressResolver
func defaultAddressResolver(r *Ringpop) error {
	r.addressResolver = r.channelAddressResolver
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

func defaultMembershipChecksumStatPeriod(r *Ringpop) error {
	return MembershipChecksumStatPeriod(StatPeriodDefault)(r)
}

func defaultRingChecksumStatPeriod(r *Ringpop) error {
	return RingChecksumStatPeriod(StatPeriodDefault)(r)
}

// defaultOptions are the default options/values when Ringpop is created. They
// can be overridden at runtime.
var defaultOptions = []Option{
	defaultClock,
	defaultAddressResolver,
	defaultLogLevels,
	defaultStatter,
	defaultMembershipChecksumStatPeriod,
	defaultRingChecksumStatPeriod,
	defaultHashRingOptions,
}

var defaultHashRingConfiguration = &hashring.Configuration{
	ReplicaPoints: 100,
}
