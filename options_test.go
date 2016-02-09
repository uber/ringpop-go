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
	"testing"
	"time"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/uber/ringpop-go/hashring"
	"github.com/uber/ringpop-go/logging"
	"github.com/uber/ringpop-go/test/mocks"
	"github.com/uber/tchannel-go"
)

type RingpopOptionsTestSuite struct {
	suite.Suite
	ringpop *Ringpop
	channel *tchannel.Channel
}

func (s *RingpopOptionsTestSuite) SetupTest() {
	ch, err := tchannel.NewChannel("test", nil)
	s.Require().NoError(err, "Channel creation failed")

	s.channel = ch
}

func (s *RingpopOptionsTestSuite) TearDownTest() {
	s.channel.Close()
}

// TestDefaults tests that the default options are applied to a Ringpop
// instance during construction, when none are specified by the user.
func (s *RingpopOptionsTestSuite) TestDefaults() {
	rp, err := New("test", Channel(s.channel))
	s.Require().NotNil(rp)
	s.Require().NoError(err)

	// Check that these defaults are not nil
	s.NotNil(rp.logger)
	s.NotNil(rp.statter)
	s.Equal(defaultHashRingConfiguration, rp.configHashRing)

	// Create a ringpop instance to manually apply these options and verify
	// them against the constructed instance. TODO: use a mocked Ringpop
	// instead.
	testRingpop := &Ringpop{}
	defaultStatter(testRingpop)
	defaultHashRingOptions(testRingpop)

	s.Equal(testRingpop.statter, rp.statter)
	s.Equal(testRingpop.configHashRing, rp.configHashRing)
}

// TestDefaultIdentityResolver tests that Ringpop gets the identity from the
// TChannel object by default.
func (s *RingpopOptionsTestSuite) TestDefaultIdentityResolver() {
	// Start listening, to get a hostport assigned
	s.channel.ListenAndServe("127.0.0.1:0")
	hostport := s.channel.PeerInfo().HostPort

	// Create the Ringpop instance with this channel
	rp, err := New("test", Channel(s.channel))
	s.Require().NotNil(rp)
	s.Require().NoError(err)

	identity, err := rp.identity()

	// Check that the identity of Ringpop matches the TChannel hostport
	s.Equal(hostport, identity)
	s.NoError(err)
}

// TestChannelRequired tests that Ringpop creation fails if a Channel is not
// passed.
func (s *RingpopOptionsTestSuite) TestChannelRequired() {
	rp, err := New("test")
	s.Nil(rp)
	s.Error(err)
}

// TestLogger tests that the logger that's passed in gets applied correctly to
// the Ringpop instance.
func (s *RingpopOptionsTestSuite) TestLogger() {
	mockLogger := &mocks.Logger{}
	// Ignore random ringpop messages
	for _, meth := range []string{"Debug", "Info", "Warn", "Error"} {
		mockLogger.On(meth, mock.Anything)
		mockLogger.On(meth+"f", mock.Anything, mock.Anything)
	}
	mockLogger.On("WithField", mock.Anything, mock.Anything).Return(mockLogger)
	mockLogger.On("WithFields", mock.Anything).Return(mockLogger)

	rp, err := New("test", Channel(s.channel), Logger(mockLogger))
	s.Require().NotNil(rp)
	s.Require().NoError(err)

	// The logger is wrapped, test for message propagation
	mockLogger.On("Panic", []interface{}{"hello"})
	rp.logger.Panic("hello")
	mockLogger.AssertCalled(s.T(), "Panic", []interface{}{"hello"})
}

// TestLogLevelsError tests that named loggers can't have a severity level
// above Fatal.
func (s *RingpopOptionsTestSuite) TestLogLevelsError() {
	levels := map[string]logging.Level{"named": logging.Panic}
	_, err := New("test", Channel(s.channel), LogLevels(levels))
	s.Error(err, "Setting log levels above Fatal should fail.")
}

// TestStatter tests that the statter that's passed in gets applied correctly
// to the Ringpop instance.
func (s *RingpopOptionsTestSuite) TestStatter() {
	mockStatter := &mocks.StatsReporter{}

	rp, err := New("test", Channel(s.channel), Statter(mockStatter))
	s.Require().NotNil(rp)
	s.Require().NoError(err)

	s.Exactly(mockStatter, rp.statter)
}

// TestHashRingConfig tests that the HashRing config that's passed in is
// applied and used correctly.
func (s *RingpopOptionsTestSuite) TestHashRingConfig() {
	rp, err := New("test", Channel(s.channel), HashRingConfig(
		&hashring.Configuration{
			ReplicaPoints: 42,
		}),
	)
	s.Require().NotNil(rp)
	s.Require().NoError(err)

	s.Equal(rp.configHashRing.ReplicaPoints, 42)
}

// TestIdentityResolverFunc tests the func that's passed gets applied to the
// Ringpop instance.
func (s *RingpopOptionsTestSuite) TestIdentityResolverFunc() {
	f := func() (string, error) {
		return "127.0.0.1:3001", nil
	}

	rp, err := New("test", Channel(s.channel), IdentityResolverFunc(f))
	s.Require().NotNil(rp)
	s.Require().NoError(err)

	identity, err := rp.identityResolver()

	s.Equal("127.0.0.1:3001", identity)
	s.NoError(err)
}

// TestMissingIdentityResolver tests the Ringpop constructor throws an error
// if the user sets the identity resolver to nil
func (s *RingpopOptionsTestSuite) TestMissingIdentityResolver() {
	rp, err := New("test", Channel(s.channel), IdentityResolverFunc(nil))
	s.Nil(rp)
	s.Error(err)
}

// TestClockNil confirms that nil clock option returns an error.
func (s *RingpopOptionsTestSuite) TestClockNil() {
	rp, err := New("test", Clock(nil))
	s.Nil(rp)
	s.Error(err)
}

// TestDefaultRingChecksumStatPeriod confirms that default gets installed.
func (s *RingpopOptionsTestSuite) TestDefaultRingChecksumStatPeriod() {
	rp, err := New("test", Channel(s.channel))
	s.NoError(err)
	s.Equal(rp.config.RingChecksumStatPeriod, RingChecksumStatPeriodDefault)
}

// TestDisabledRingChecksumStat confirms that disabled switch stays disabled.
func (s *RingpopOptionsTestSuite) TestDisabledRingChecksumStat() {
	tchan := Channel(s.channel)

	rp, err := New("test", tchan, RingChecksumStatPeriod(RingChecksumStatPeriodNever))
	s.NoError(err)
	s.Equal(rp.config.RingChecksumStatPeriod, RingChecksumStatPeriodNever)

	rp, err = New("test", tchan, RingChecksumStatPeriod(0))
	s.NoError(err)
	s.Equal(rp.config.RingChecksumStatPeriod, RingChecksumStatPeriodNever)

	rp, err = New("test", tchan, RingChecksumStatPeriod(-23))
	s.NoError(err)
	s.Equal(rp.config.RingChecksumStatPeriod, RingChecksumStatPeriodNever)
}

// TestSpecifiedRingChecksumStatPeriod confirms that sane periods pass through.
func (s *RingpopOptionsTestSuite) TestSpecifiedRingChecksumStatPeriod() {
	rp, err := New("test", Channel(s.channel), RingChecksumStatPeriod(42*time.Second))
	s.NoError(err)
	s.Equal(rp.config.RingChecksumStatPeriod, time.Duration(42*time.Second))
}

// TestTooSmallRingChecksumStatPeriod confirms that insane periods return error.
func (s *RingpopOptionsTestSuite) TestTooSmallRingChecksumStatPeriod() {
	rp, err := New("test", Channel(s.channel), RingChecksumStatPeriod(1*time.Nanosecond))
	s.Nil(rp)
	s.Error(err)
}

func TestRingpopOptionsTestSuite(t *testing.T) {
	suite.Run(t, new(RingpopOptionsTestSuite))
}
