package logger

import (
	"github.com/stretchr/testify/suite"
	"github.com/uber/ringpop-go/test/mocks"
	"testing"
)

type LoggerFactorySuite struct {
	suite.Suite
	mock    *mocks.Logger
	factory *LoggerFactory
}

func (s *LoggerFactorySuite) SetupTest() {
	s.mock = &mocks.Logger{}
	s.factory = NewLoggerFactory(s.mock)
}

func (s *LoggerFactorySuite) TestLoggerIdentity() {
	s.Exactly(s.factory.Logger("x"), s.factory.Logger("x"))
}

func (s *LoggerFactorySuite) TestDefaultLevel() {
	logger := s.factory.Logger("x").(*namedLogger)
	s.Equal(logger.min, Warn)
}

func (s *LoggerFactorySuite) TestSetLevel() {
	s.factory.SetLevel("x", Warn)
	logger := s.factory.Logger("x").(*namedLogger)
	s.Equal(logger.min, Warn)
	s.factory.SetLevel("x", Panic)
	s.Equal(logger.min, Panic)
}

func (s *LoggerFactorySuite) TestSetLevelLow() {
	s.Error(s.factory.SetLevel("x", unset))
}

func (s *LoggerFactorySuite) TestSetLevelHigh() {
	s.Error(s.factory.SetLevel("x", highestLevel+1))
}

func TestLoggerFactorySuite(t *testing.T) {
	suite.Run(t, new(LoggerFactorySuite))
}
