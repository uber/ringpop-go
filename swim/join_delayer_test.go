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
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

// Null randomizer and sleeper used for join delay tests.
var noRandom = func(n int) int { return n }
var noSleep = func(d time.Duration) {}

type joinDelayerTestSuite struct {
	suite.Suite
	delayer        *exponentialDelayer
	expectedDelays [6]time.Duration
}

func (s *joinDelayerTestSuite) SetupTest() {
	opts := &delayOpts{
		initial: 100 * time.Millisecond,
		max:     1 * time.Second,
		sleeper: noSleep, // Sleeper used for tests applies no actual delay
	}

	// Represents the steps of a complete exponential backoff
	// as implemented by the exponentialDelayer.
	s.expectedDelays = [...]time.Duration{
		opts.initial,
		200 * time.Millisecond,
		400 * time.Millisecond,
		800 * time.Millisecond,
		opts.max, // Backoff delay is capped at this point.
		opts.max,
	}

	delayer, err := newExponentialDelayer("dummyjoiner", opts)
	s.NoError(err, "expected valid exponential delayer")
	s.delayer = delayer
}

func (s *joinDelayerTestSuite) TestDelayWithRandomness() {
	for i, expectedDelay := range s.expectedDelays {
		delay := s.delayer.delay()
		if i == 0 {
			s.True(delay >= 0 && delay < expectedDelay,
				"first delay should be between 0 and min")
		} else {
			s.True(delay >= s.expectedDelays[i-1] && delay <= expectedDelay,
				"next delays should be within bounds")
		}
	}
}

func (s *joinDelayerTestSuite) TestDelayWithoutRandomness() {
	// Substitute delayer's randomizer with one that produces
	// no randomness whatsoever.
	s.delayer.randomizer = noRandom

	for _, expectedDelay := range s.expectedDelays {
		delay := s.delayer.delay()
		s.EqualValues(expectedDelay, delay, "join attempt delay is correct")
	}
}

func (s *joinDelayerTestSuite) TestMaxDelayReached() {
	for i := range s.expectedDelays {
		s.delayer.delay()
		// This condition assumes that the last two elements
		// of expectedDelay is equal to the max delay.
		if i < len(s.expectedDelays)-2 {
			s.False(s.delayer.maxDelayReached, "max delay not reached")
		} else {
			s.True(s.delayer.maxDelayReached, "max delay not reached")
		}
	}
}

func TestJoinDelayerTestSuite(t *testing.T) {
	suite.Run(t, new(joinDelayerTestSuite))
}
