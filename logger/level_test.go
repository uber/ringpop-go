package logger

import (
	"github.com/stretchr/testify/suite"
	"testing"
)

type LevelSuite struct {
	suite.Suite
}

// Make sure that Level -> string -> Level conversion returns the same thing.
func (s *LevelSuite) assertParseLevel(level Level) {
	levelAsString := level.String()
	newLevel, err := Parse(levelAsString)
	s.NoError(err)
	s.Exactly(newLevel, level)
}

func (s *LevelSuite) TestParseAllLevels() {
	allLevels := []Level{Trace, Info, Debug, Warn, Error, Fatal, Panic, Off}
	for _, level := range allLevels {
		s.assertParseLevel(level)
	}
}

func (s *LevelSuite) TestParseBadLevel() {
	_, err := Parse("badlevel")
	s.Error(err)
}

func (s *LevelSuite) TestUnknownLevel() {
	s.Equal(Level(100).String(), "unknown")
}

func TestLevelSuite(t *testing.T) {
	suite.Run(t, new(LevelSuite))
}
