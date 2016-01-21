package modulelogger

import (
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/suite"
	"github.com/uber-common/bark"
	"github.com/uber/ringpop-go/test/mocks"
	"testing"
)

// Just like mocks.Logger but with convenient On and Onf methods.
type mockLogger struct {
	*mocks.Logger
}

// mock.On("Debug", "msg") instead of mock.On("Debug", []interface{}{"msg"})
func (mock *mockLogger) On(methodName string, arguments ...interface{}) *mock.Call {
	return mock.Logger.On(methodName, arguments)
}

func (mock *mockLogger) Onf(methodName string, format string, arguments ...interface{}) *mock.Call {
	return mock.Logger.On(methodName, format, arguments)
}

func newMockLogger() *mockLogger {
	return &mockLogger{Logger: &mocks.Logger{}}
}

type ModuleLoggerSuite struct {
	suite.Suite
	// unit under test
	uut *ModuleLogger
	log *mockLogger
}

func (s *ModuleLoggerSuite) assertParseLevel(level Level) {
	l, err := ParseLevel(level.String())
	s.Require().NoError(err)
	s.Exactly(level, l)
}

func (s *ModuleLoggerSuite) SetupTest() {
	s.log = newMockLogger()
	s.uut = New(s.log)
	err := s.uut.SetLevel("testFatal", FatalLevel)
	s.Require().NoError(err)
}

func (s *ModuleLoggerSuite) TestPassThrough() {
	s.log.On("Debug", "d msg", 1)
	s.log.On("Info", "i msg", 2)
	s.log.On("Warn", "w msg", 3)
	s.log.On("Error", "e msg", 4)
	s.log.On("Fatal", "f msg", 5)
	s.log.On("Panic", "p msg", 6)
	s.log.Onf("Debugf", "d format", "d msg", 1)
	s.log.Onf("Infof", "i format", "i msg", 2)
	s.log.Onf("Warnf", "w format", "w msg", 3)
	s.log.Onf("Errorf", "e format", "e msg", 4)
	s.log.Onf("Fatalf", "f format", "f msg", 5)
	s.log.Onf("Panicf", "p format", "p msg", 6)
	s.uut.Debug("d msg", 1)
	s.uut.Info("i msg", 2)
	s.uut.Warn("w msg", 3)
	s.uut.Error("e msg", 4)
	s.uut.Fatal("f msg", 5)
	s.uut.Panic("p msg", 6)
	s.uut.Debugf("d format", "d msg", 1)
	s.uut.Infof("i format", "i msg", 2)
	s.uut.Warnf("w format", "w msg", 3)
	s.uut.Errorf("e format", "e msg", 4)
	s.uut.Fatalf("f format", "f msg", 5)
	s.uut.Panicf("p format", "p msg", 6)
	s.log.AssertExpectations(s.T())
}

func (s *ModuleLoggerSuite) TestRootFields() {
	withFieldLogger := newMockLogger()
	withFieldsLogger := newMockLogger()
	fields := bark.Fields{"a": 1}
	s.log.Logger.On("WithField", "key", "value").Return(withFieldLogger)
	s.log.Logger.On("WithFields", fields).Return(withFieldsLogger)
	s.log.Logger.On("Fields").Return(fields)
	withFieldLogger.On("Debug", "d msg")
	withFieldsLogger.On("Warn", "w msg")
	s.uut.WithField("key", "value").Debug("d msg")
	s.uut.WithFields(fields).Warn("w msg")
	s.Exactly(s.uut.Fields(), fields)
	s.log.AssertExpectations(s.T())
}

func (s *ModuleLoggerSuite) TestLazyFields() {
	l := s.uut.Logger("test")
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

	s.uut.SetLogger("test", logger1)
	fieldLogger.Debug("d msg")
	s.uut.SetLevel("test", OffLevel)
	fieldLogger.Panic("d msg")
	s.Exactly(fieldLogger.Fields(), fields3)

	logger1.AssertExpectations(s.T())
	logger2.AssertExpectations(s.T())
	logger3.AssertExpectations(s.T())
	logger4.AssertExpectations(s.T())
	logger5.AssertExpectations(s.T())
}

func (s *ModuleLoggerSuite) TestLimitLevels() {
	s.log.On("Debug", "d msg", 1)
	s.log.On("Info", "i msg", 2)
	s.log.On("Warn", "w msg", 3)
	s.log.On("Error", "e msg", 4)
	s.log.On("Fatal", "f msg", 5)
	s.log.On("Panic", "p msg", 6)
	s.uut.SetLevel(RootModule, DebugLevel)
	s.uut.Debug("d msg", 1)
	s.uut.SetLevel(RootModule, InfoLevel)
	s.uut.Info("i msg", 2)
	s.uut.SetLevel(RootModule, WarnLevel)
	s.uut.Warn("w msg", 3)
	s.uut.SetLevel(RootModule, ErrorLevel)
	s.uut.Error("e msg", 4)
	s.uut.SetLevel(RootModule, FatalLevel)
	s.uut.Fatal("f msg", 5)
	s.uut.SetLevel(RootModule, PanicLevel)
	s.uut.Panic("p msg", 6)
	s.log.AssertExpectations(s.T())
}

func (s *ModuleLoggerSuite) TestFatalLevel() {
	l := s.uut.Logger("testFatal")
	s.log.On("Fatal", "f msg", 5)
	s.log.On("Panic", "p msg", 6)
	l.Debug("d msg", 1)
	l.Info("i msg", 2)
	l.Warn("w msg", 3)
	l.Error("e msg", 4)
	l.Fatal("f msg", 5)
	l.Panic("p msg", 6)
	s.log.AssertExpectations(s.T())
}

func (s *ModuleLoggerSuite) TestChangeRootLogger() {
	mock := newMockLogger()
	mock.On("Fatal", "f msg")
	mock.On("Panic", "p msg")
	s.uut.SetLogger(RootModule, mock)
	s.uut.Fatal("f msg")
	s.uut.Logger(RootModule).Panic("p msg")
	mock.AssertExpectations(s.T())
}

func (s *ModuleLoggerSuite) TestChangeRootLevel() {
	s.uut.SetLevel(RootModule, OffLevel)
	s.uut.Panic("p msg")
	s.uut.Logger(RootModule).Panic("p msg")
	s.log.AssertExpectations(s.T())
}

func (s *ModuleLoggerSuite) TestChangeModuleLogger() {
	l := s.uut.Logger("testFatal")
	mock := newMockLogger()
	mock.On("Fatal", "f msg")
	s.uut.SetLogger("testFatal", mock)
	l.Fatal("f msg")
	mock.AssertExpectations(s.T())
}

func (s *ModuleLoggerSuite) TestChangeModuleLevel() {
	l := s.uut.Logger("testFatal")
	s.log.On("Debug", "d msg")
	s.uut.SetLevel("testFatal", DebugLevel)
	l.Debug("d msg")
	s.log.AssertExpectations(s.T())
}

func (s *ModuleLoggerSuite) TestSetLevelError() {
	err := s.uut.SetLevel("testFatal", OffLevel+100)
	s.Error(err)
}

func (s *ModuleLoggerSuite) TestParseLevels() {
	s.assertParseLevel(DebugLevel)
	s.assertParseLevel(InfoLevel)
	s.assertParseLevel(WarnLevel)
	s.assertParseLevel(ErrorLevel)
	s.assertParseLevel(FatalLevel)
	s.assertParseLevel(PanicLevel)
	s.assertParseLevel(OffLevel)
	_, err := ParseLevel((OffLevel + 100).String())
	s.Error(err)
}

func (s *ModuleLoggerSuite) TestChangeModuleOnTheFly() {
	testLogger := newMockLogger()
	otherLogger1 := newMockLogger()
	otherLogger2 := newMockLogger()
	otherLogger3 := newMockLogger()
	s.uut.SetLogger("test", testLogger)
	s.uut.SetLogger("other", otherLogger1)

	fields := bark.Fields{"a": 1}
	otherLogger1.Logger.On("WithField", "key", "value").Return(otherLogger2)
	otherLogger2.Logger.On("WithFields", fields).Return(otherLogger3)
	otherLogger3.On("Debug", "d")

	var l bark.Logger
	l = s.uut.Logger("test")
	l = l.WithField("key", "value")
	l = l.WithFields(fields)
	l = GetModuleLogger(l, "other")
	l.Debug("d")

	testLogger.AssertExpectations(s.T())
	otherLogger1.AssertExpectations(s.T())
	otherLogger2.AssertExpectations(s.T())
	otherLogger3.AssertExpectations(s.T())
}

func (s *ModuleLoggerSuite) TestParseInvalidLevel() {
	_, err := ParseLevel("nolevel")
	s.Error(err)
}

func (s *ModuleLoggerSuite) TestGetModuleLogger() {
	s.log.On("Fatal", "f")
	s.log.On("Panic", "p")
	l := GetModuleLogger(s.uut, "testFatal")
	l.Fatal("f")
	l = GetModuleLogger(l, "testFatal")
	l.Panic("p")
	s.log.AssertExpectations(s.T())
}

func TestModuleLoggerSuite(t *testing.T) {
	suite.Run(t, new(ModuleLoggerSuite))
}
