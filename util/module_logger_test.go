package util_test

import (
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/uber-common/bark"
	"github.com/uber/ringpop-go/test/mocks"
	"github.com/uber/ringpop-go/util"
	"testing"
)

type MockLogger struct {
	*mocks.Logger
}

func (ml *MockLogger) On(methodName string, arguments ...interface{}) *mock.Call {
	return ml.Logger.On(methodName, arguments)
}

func (ml *MockLogger) Onf(methodName string, format string, arguments ...interface{}) *mock.Call {
	return ml.Logger.On(methodName, format, arguments)
}

func newMockLogger() *MockLogger {
	return &MockLogger{Logger: &mocks.Logger{}}
}

type ModuleLoggerSuite struct {
	suite.Suite
	ml *util.ModuleLogger
	l  *MockLogger
}

func (s *ModuleLoggerSuite) assertParseLevel(level util.Level) {
	l, err := util.ParseLevel(level.String())
	s.Require().NoError(err)
	s.Exactly(level, l)
}

func (s *ModuleLoggerSuite) SetupTest() {
	s.l = newMockLogger()
	s.ml = util.NewModuleLogger(s.l)
	err := s.ml.SetLevel("testFatal", util.FatalLevel)
	s.Require().NoError(err)
}

func (s *ModuleLoggerSuite) TestPassThrough() {
	s.l.On("Debug", "d msg", 1)
	s.l.On("Info", "i msg", 2)
	s.l.On("Warn", "w msg", 3)
	s.l.On("Error", "e msg", 4)
	s.l.On("Fatal", "f msg", 5)
	s.l.On("Panic", "p msg", 6)
	s.l.Onf("Debugf", "d format", "d msg", 1)
	s.l.Onf("Infof", "i format", "i msg", 2)
	s.l.Onf("Warnf", "w format", "w msg", 3)
	s.l.Onf("Errorf", "e format", "e msg", 4)
	s.l.Onf("Fatalf", "f format", "f msg", 5)
	s.l.Onf("Panicf", "p format", "p msg", 6)
	s.ml.Debug("d msg", 1)
	s.ml.Info("i msg", 2)
	s.ml.Warn("w msg", 3)
	s.ml.Error("e msg", 4)
	s.ml.Fatal("f msg", 5)
	s.ml.Panic("p msg", 6)
	s.ml.Debugf("d format", "d msg", 1)
	s.ml.Infof("i format", "i msg", 2)
	s.ml.Warnf("w format", "w msg", 3)
	s.ml.Errorf("e format", "e msg", 4)
	s.ml.Fatalf("f format", "f msg", 5)
	s.ml.Panicf("p format", "p msg", 6)
	s.l.AssertExpectations(s.T())
}

func (s *ModuleLoggerSuite) TestRootFields() {
	withFieldLogger := newMockLogger()
	withFieldsLogger := newMockLogger()
	fields := bark.Fields{"a": 1}
	s.l.Logger.On("WithField", "key", "value").Return(withFieldLogger)
	s.l.Logger.On("WithFields", fields).Return(withFieldsLogger)
	s.l.Logger.On("Fields").Return(fields)
	withFieldLogger.On("Debug", "d msg")
	withFieldsLogger.On("Warn", "w msg")
	s.ml.WithField("key", "value").Debug("d msg")
	s.ml.WithFields(fields).Warn("w msg")
	s.Exactly(s.ml.Fields(), fields)
	s.l.AssertExpectations(s.T())
}

func (s *ModuleLoggerSuite) TestLazyFields() {
	l := s.ml.Logger("test")
	fields1 := bark.Fields{"a": 1}
	fields2 := bark.Fields{"b": 2}
	fields3 := bark.Fields{"c": 3}
	fieldLogger := l.WithField("key1", "value1")
	fieldLogger = fieldLogger.WithFields(fields1)
	fieldLogger = fieldLogger.WithField("key2", "value2")
	fieldLogger = fieldLogger.WithFields(fields2)

	logger1 := newMockLogger()
	logger2 := newMockLogger()
	logger3 := newMockLogger()
	logger4 := newMockLogger()
	logger5 := newMockLogger()
	logger1.Logger.On("WithField", "key1", "value1").Return(logger2)
	logger2.Logger.On("WithFields", fields1).Return(logger3)
	logger3.Logger.On("WithField", "key2", "value2").Return(logger4)
	logger4.Logger.On("WithFields", fields2).Return(logger5)
	logger5.On("Debug", "d msg")
	logger5.Logger.On("Fields").Return(fields3)

	s.ml.SetLogger("test", logger1)
	fieldLogger.Debug("d msg")
	s.ml.SetLevel("test", util.OffLevel)
	fieldLogger.Panic("d msg")
	s.Exactly(fieldLogger.Fields(), fields3)

	logger1.AssertExpectations(s.T())
	logger2.AssertExpectations(s.T())
	logger3.AssertExpectations(s.T())
	logger4.AssertExpectations(s.T())
	logger5.AssertExpectations(s.T())
}

func (s *ModuleLoggerSuite) TestLimitLevels() {
	s.l.On("Debug", "d msg", 1)
	s.l.On("Info", "i msg", 2)
	s.l.On("Warn", "w msg", 3)
	s.l.On("Error", "e msg", 4)
	s.l.On("Fatal", "f msg", 5)
	s.l.On("Panic", "p msg", 6)
	s.ml.SetLevel(util.RootModule, util.DebugLevel)
	s.ml.Debug("d msg", 1)
	s.ml.SetLevel(util.RootModule, util.InfoLevel)
	s.ml.Info("i msg", 2)
	s.ml.SetLevel(util.RootModule, util.WarnLevel)
	s.ml.Warn("w msg", 3)
	s.ml.SetLevel(util.RootModule, util.ErrorLevel)
	s.ml.Error("e msg", 4)
	s.ml.SetLevel(util.RootModule, util.FatalLevel)
	s.ml.Fatal("f msg", 5)
	s.ml.SetLevel(util.RootModule, util.PanicLevel)
	s.ml.Panic("p msg", 6)
	s.l.AssertExpectations(s.T())
}

func (s *ModuleLoggerSuite) TestFatalLevel() {
	l := s.ml.Logger("testFatal")
	s.l.On("Fatal", "f msg", 5)
	s.l.On("Panic", "p msg", 6)
	l.Debug("d msg", 1)
	l.Info("i msg", 2)
	l.Warn("w msg", 3)
	l.Error("e msg", 4)
	l.Fatal("f msg", 5)
	l.Panic("p msg", 6)
	s.l.AssertExpectations(s.T())
}

func (s *ModuleLoggerSuite) TestChangeRootLogger() {
	mockLogger := newMockLogger()
	mockLogger.On("Fatal", "f msg")
	mockLogger.On("Panic", "p msg")
	s.ml.SetLogger(util.RootModule, mockLogger)
	s.ml.Fatal("f msg")
	s.ml.Logger(util.RootModule).Panic("p msg")
	mockLogger.AssertExpectations(s.T())
}

func (s *ModuleLoggerSuite) TestChangeRootLevel() {
	s.ml.SetLevel(util.RootModule, util.OffLevel)
	s.ml.Panic("p msg")
	s.ml.Logger(util.RootModule).Panic("p msg")
	s.l.AssertExpectations(s.T())
}

func (s *ModuleLoggerSuite) TestChangeModuleLogger() {
	l := s.ml.Logger("testFatal")
	mockLogger := newMockLogger()
	mockLogger.On("Fatal", "f msg")
	s.ml.SetLogger("testFatal", mockLogger)
	l.Fatal("f msg")
	mockLogger.AssertExpectations(s.T())
}

func (s *ModuleLoggerSuite) TestChangeModuleLevel() {
	l := s.ml.Logger("testFatal")
	s.l.On("Debug", "d msg")
	s.ml.SetLevel("testFatal", util.DebugLevel)
	l.Debug("d msg")
	s.l.AssertExpectations(s.T())
}

func (s *ModuleLoggerSuite) TestSetLevelError() {
	err := s.ml.SetLevel("testFatal", util.OffLevel+100)
	s.Error(err)
}

func (s *ModuleLoggerSuite) TestParseLevels() {
	s.assertParseLevel(util.DebugLevel)
	s.assertParseLevel(util.InfoLevel)
	s.assertParseLevel(util.WarnLevel)
	s.assertParseLevel(util.ErrorLevel)
	s.assertParseLevel(util.FatalLevel)
	s.assertParseLevel(util.PanicLevel)
	s.assertParseLevel(util.OffLevel)
	_, err := util.ParseLevel((util.OffLevel + 100).String())
	s.Error(err)
}

func (s *ModuleLoggerSuite) TestChangeModuleOnTheFly() {
	testLogger := newMockLogger()
	otherLogger1 := newMockLogger()
	otherLogger2 := newMockLogger()
	otherLogger3 := newMockLogger()
	s.ml.SetLogger("test", testLogger)
	s.ml.SetLogger("other", otherLogger1)

	fields := bark.Fields{"a": 1}
	otherLogger1.Logger.On("WithField", "key", "value").Return(otherLogger2)
	otherLogger2.Logger.On("WithFields", fields).Return(otherLogger3)
	otherLogger3.On("Debug", "d")

	l := s.ml.Logger("test")
	l = l.WithField("key", "value")
	l = l.WithFields(fields)
	l = util.GetModuleLogger(l, "other")
	l.Debug("d")

	testLogger.AssertExpectations(s.T())
	otherLogger1.AssertExpectations(s.T())
	otherLogger2.AssertExpectations(s.T())
	otherLogger3.AssertExpectations(s.T())
}

func (s *ModuleLoggerSuite) TestParseInvalidLevel() {
	_, err := util.ParseLevel("nolevel")
	s.Error(err)
}

func (s *ModuleLoggerSuite) TestGetModuleLogger() {
	s.l.On("Fatal", "f")
	s.l.On("Panic", "p")
	l := util.GetModuleLogger(s.ml, "testFatal")
	l.Fatal("f")
	l = util.GetModuleLogger(l, "testFatal")
	l.Panic("p")
	s.l.AssertExpectations(s.T())
}

func TestModuleLoggerSuite(t *testing.T) {
	suite.Run(t, new(ModuleLoggerSuite))
}
