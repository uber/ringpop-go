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

package swim

import (
	"math"
	"math/rand"
	"time"

	"github.com/uber-common/bark"
	"github.com/uber/ringpop-go/logging"
	"github.com/uber/ringpop-go/util"
)

const (
	defaultInitial = 100 * time.Millisecond
	defaultMax     = 60 * time.Second
)

var delayerRand = rand.New(rand.NewSource(time.Now().UnixNano()))
var defaultRandomizer = delayerRand.Intn
var defaultSleeper = time.Sleep
var noDelay = time.Duration(0)

// joinDelayer defines the API of a delayer implementation that
// applies a delay in between repeated join attempts.
type joinDelayer interface {
	delay() time.Duration
}

// delayRandomizer is a function that returns a random number between
// 0 and an upper-bound, provided in its first argument.
type delayRandomizer func(int) int

// delaySleeper is a function that pauses execution for time.Duration.
type delaySleeper func(time.Duration)

// delayOpts is a struct that houses substitutable parameters of a joinDelayer
// in order to alter its behavior.
type delayOpts struct {
	initial    time.Duration
	max        time.Duration
	randomizer delayRandomizer
	sleeper    delaySleeper
}

// newDelayOpts creates a delayOpts struct with default values.
func newDelayOpts() *delayOpts {
	return &delayOpts{
		initial:    defaultInitial,
		max:        defaultMax,
		randomizer: defaultRandomizer,
		sleeper:    defaultSleeper,
	}
}

// exponentialDelayer applies a delay in between repeated join attempts.
// The delay increases exponentially and is capped at maxDelay.
type exponentialDelayer struct {
	// logger logs when a maximum delay is reached.
	logger bark.Logger

	// initialDelay is the fixed portion of the delay by which
	// the exponential backoff is multiplied.
	initialDelay time.Duration

	// maxDelay is the maximum delay applied to a join attempt.
	maxDelay time.Duration

	// maxDelayReached is a flag that is toggled once the delay
	// applied to a join attempt reaches or exceeds the max delay.
	maxDelayReached bool

	// nextDelayMin tracks the last used upper-bound for a join attempt delay.
	// Upon the next join attempt, nextDelayMin will be used as the lower-bound
	// for that delay. This acts as a shifting window for the bounds of the
	// exponential backoff.
	nextDelayMin float64

	// randomizer generates a random delay in between a nextDelayMin
	// and maxDelay.
	randomizer delayRandomizer

	// sleeper pauses execution of a join attempt.
	sleeper delaySleeper

	// numDelays tracks the number of times delay has been called on the
	// delayer. It's also used as the backoff exponent.
	numDelays uint
}

// newExponentialDelayer creates a new exponential delayer. joiner is required.
// opts is optional.
func newExponentialDelayer(joiner string, opts *delayOpts) (*exponentialDelayer, error) {
	if opts == nil {
		opts = newDelayOpts()
	}

	randomizer := opts.randomizer
	if randomizer == nil {
		randomizer = defaultRandomizer
	}

	sleeper := opts.sleeper
	if sleeper == nil {
		sleeper = defaultSleeper
	}

	return &exponentialDelayer{
		logger:          logging.Logger("join").WithField("local", joiner),
		initialDelay:    opts.initial,
		nextDelayMin:    0,
		maxDelayReached: false,
		maxDelay:        opts.max,
		randomizer:      randomizer,
		sleeper:         sleeper,
		numDelays:       0,
	}, nil
}

// delay delays a join attempt by sleeping for an amount of time. The
// amount of time is computed as an exponential backoff based on the number
// of join attempts that have been made at the time of the function call;
// the number of attempts is 0-based. It returns a time.Duration equal to the
// amount of delay applied.
func (d *exponentialDelayer) delay() time.Duration {
	// Convert durations to time in millis
	initialDelayMs := float64(util.MS(d.initialDelay))
	maxDelayMs := float64(util.MS(d.maxDelay))

	// Compute uncapped exponential delay (exponent is the number of join
	// attempts so far). Then, make sure the computed delay is capped at its
	// max. Apply a random jitter to the actual sleep duration and finally,
	// sleep.
	uncappedDelay := initialDelayMs * math.Pow(2, float64(d.numDelays))
	cappedDelay := math.Min(maxDelayMs, uncappedDelay)

	// If cappedDelay and nextDelayMin are equal, we have reached the point
	// at which the exponential backoff has reached its max; apply no more
	// jitter.
	var jitteredDelay int
	if cappedDelay == d.nextDelayMin {
		jitteredDelay = int(cappedDelay)
	} else {
		jitteredDelay = d.randomizer(int(cappedDelay-d.nextDelayMin)) + int(d.nextDelayMin)
	}

	// If this is the first time an uncapped delay reached or exceeded the
	// maximum allowable delay, log a message.
	if uncappedDelay >= maxDelayMs && d.maxDelayReached == false {
		d.logger.WithFields(bark.Fields{
			"numDelays":     d.numDelays,
			"initialDelay":  d.initialDelay,
			"minDelay":      d.nextDelayMin,
			"maxDelay":      d.maxDelay,
			"uncappedDelay": uncappedDelay,
			"cappedDelay":   cappedDelay,
			"jitteredDelay": jitteredDelay,
		}).Warn("ringpop join attempt delay reached max")
		d.maxDelayReached = true
	}

	// Set lower-bound for next attempt to maximum of current attempt.
	d.nextDelayMin = cappedDelay

	sleepDuration := time.Duration(jitteredDelay) * time.Millisecond
	d.sleeper(sleepDuration)

	// Increment the exponent used for backoff calculation.
	d.numDelays++

	return sleepDuration
}

// nullDelayer is an empty implementation of joinDelayer.
type nullDelayer struct{}

// delay applies no delay.
func (d *nullDelayer) delay() time.Duration {
	return time.Duration(0)
}
