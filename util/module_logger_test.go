package util

import (
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	log "github.com/uber-common/bark"
	"testing"
)

type logMsg struct {
	format string
	args   []interface{}
	level  Level
}

type dummyLogger struct {
	msg        *logMsg
	fieldKey   string
	fieldValue interface{}
	fields     log.LogFields
}

func (dl *dummyLogger) Debug(args ...interface{}) {
	dl.msg = &logMsg{args: args, level: DebugLevel}
}

func (dl *dummyLogger) Debugf(format string, args ...interface{}) {
	dl.msg = &logMsg{format: format, args: args, level: DebugLevel}
}

func (dl *dummyLogger) Info(args ...interface{}) {
	dl.msg = &logMsg{args: args, level: InfoLevel}
}

func (dl *dummyLogger) Infof(format string, args ...interface{}) {
	dl.msg = &logMsg{format: format, args: args, level: InfoLevel}
}

func (dl *dummyLogger) Warn(args ...interface{}) {
	dl.msg = &logMsg{args: args, level: WarnLevel}
}

func (dl *dummyLogger) Warnf(format string, args ...interface{}) {
	dl.msg = &logMsg{format: format, args: args, level: WarnLevel}
}

func (dl *dummyLogger) Error(args ...interface{}) {
	dl.msg = &logMsg{args: args, level: ErrorLevel}
}

func (dl *dummyLogger) Errorf(format string, args ...interface{}) {
	dl.msg = &logMsg{format: format, args: args, level: ErrorLevel}
}

func (dl *dummyLogger) Fatal(args ...interface{}) {
	dl.msg = &logMsg{args: args, level: FatalLevel}
}

func (dl *dummyLogger) Fatalf(format string, args ...interface{}) {
	dl.msg = &logMsg{format: format, args: args, level: FatalLevel}
}

func (dl *dummyLogger) Panic(args ...interface{}) {
	dl.msg = &logMsg{args: args, level: PanicLevel}
}

func (dl *dummyLogger) Panicf(format string, args ...interface{}) {
	dl.msg = &logMsg{format: format, args: args, level: PanicLevel}
}

func (dl *dummyLogger) WithField(key string, value interface{}) log.Logger {
	dl.fieldKey = key
	dl.fieldValue = value
	return dl
}

func (dl *dummyLogger) WithFields(keyValues log.LogFields) log.Logger {
	dl.fields = keyValues
	return dl
}

func (dl *dummyLogger) Fields() log.Fields {
	return nil
}

func (dl *dummyLogger) assertMsg(t assert.TestingT, msgLevel Level, args ...interface{}) {
	assert.Equal(t, dl.msg.level, msgLevel, "Message emitted with wrong level.")
	assert.Equal(t, dl.msg.args, args, "Message emitted with wrong args.")
}

func (dl *dummyLogger) assertMsgf(t assert.TestingT, msgLevel Level, format string, args ...interface{}) {
	dl.assertMsg(t, msgLevel, args...)
	assert.Equal(t, dl.msg.format, format, "Message emitted with wrong format.")
}

func (dl *dummyLogger) assertNoMsg(t assert.TestingT) {
	assert.Nil(t, dl.msg, "Unexpected message emitted.")
}

type ModuleTestSuite struct {
	suite.Suite
	dl *dummyLogger
	ml *ModuleLogger
}

func (s *ModuleTestSuite) SetupTest() {
	s.dl = &dummyLogger{}
	s.ml = NewModuleLogger(s.dl)
	s.ml.SetLevel("testdebug", DebugLevel)
	s.ml.SetLevel("testpanic", PanicLevel)
	s.ml.SetLevel("testoff", OffLevel)
}

func (s *ModuleTestSuite) assertMsg(msgLevel Level, args ...interface{}) {
	s.dl.assertMsg(s.T(), msgLevel, args...)
}

func (s *ModuleTestSuite) assertMsgf(msgLevel Level, format string, args ...interface{}) {
	s.dl.assertMsgf(s.T(), msgLevel, format, args...)
}

func (s *ModuleTestSuite) assertNoMsg() {
	s.dl.assertNoMsg(s.T())
}

func (s *ModuleTestSuite) TestRootModule() {
	l := s.ml
	l.Debug("D", 1, 2)
	s.assertMsg(DebugLevel, "D", 1, 2)
	l.Panic("P", 1, 2)
	s.assertMsg(PanicLevel, "P", 1, 2)
}

func (s *ModuleTestSuite) TestSubLogger() {
	l := s.ml.Logger("testpanic")
	l.Debug("D", 1, 2)
	s.assertNoMsg()
	l.Panic("P", 1, 2)
	s.assertMsg(PanicLevel, "P", 1, 2)
}

func (s *ModuleTestSuite) TestRuntimeUpdate() {
	l := s.ml.Logger("testpanic")
	s.ml.SetLevel("testpanic", OffLevel)
	l.Panic("P", 1, 2)
	s.assertNoMsg()
}

func (s *ModuleTestSuite) TestOrigLogger() {
	l := s.ml.Logger("unnamed")
	l.Debug("Debug Msg", 1, 2)
	s.assertMsg(DebugLevel, "Debug Msg", 1, 2)
	l.Debugf("Debug Format", 1, 2)
	s.assertMsgf(DebugLevel, "Debug Format", 1, 2)
}

func (s *ModuleTestSuite) TestDebug() {
	l := s.ml.Logger("testdebug")
	l.Debug("Debug Msg", 1, 2)
	s.assertMsg(DebugLevel, "Debug Msg", 1, 2)
	l.Debugf("Debug Format", 1, 2)
	s.assertMsgf(DebugLevel, "Debug Format", 1, 2)
	l.Info("Info Msg", 1, 2)
	s.assertMsg(InfoLevel, "Info Msg", 1, 2)
	l.Infof("Info Format", 1, 2)
	s.assertMsgf(InfoLevel, "Info Format", 1, 2)
	l.Warn("Warn Msg", 1, 2)
	s.assertMsg(WarnLevel, "Warn Msg", 1, 2)
	l.Warnf("Warn Format", 1, 2)
	s.assertMsgf(WarnLevel, "Warn Format", 1, 2)
	l.Error("Error Msg", 1, 2)
	s.assertMsg(ErrorLevel, "Error Msg", 1, 2)
	l.Errorf("Error Format", 1, 2)
	s.assertMsgf(ErrorLevel, "Error Format", 1, 2)
	l.Fatal("Fatal Msg", 1, 2)
	s.assertMsg(FatalLevel, "Fatal Msg", 1, 2)
	l.Fatalf("Fatal Format", 1, 2)
	s.assertMsgf(FatalLevel, "Fatal Format", 1, 2)
	l.Panic("Panic Msg", 1, 2)
	s.assertMsg(PanicLevel, "Panic Msg", 1, 2)
	l.Panicf("Panic Format", 1, 2)
	s.assertMsgf(PanicLevel, "Panic Format", 1, 2)
}

func (s *ModuleTestSuite) TestPanic() {
	l := s.ml.Logger("testpanic")
	l.Debug("Debug Msg", 1, 2)
	s.assertNoMsg()
	l.Debugf("Debug Format", 1, 2)
	s.assertNoMsg()
	l.Info("Info Msg", 1, 2)
	s.assertNoMsg()
	l.Infof("Info Format", 1, 2)
	s.assertNoMsg()
	l.Warn("Warn Msg", 1, 2)
	s.assertNoMsg()
	l.Warnf("Warn Format", 1, 2)
	s.assertNoMsg()
	l.Error("Error Msg", 1, 2)
	s.assertNoMsg()
	l.Errorf("Error Format", 1, 2)
	s.assertNoMsg()
	l.Fatal("Fatal Msg", 1, 2)
	s.assertNoMsg()
	l.Fatalf("Fatal Format", 1, 2)
	s.assertNoMsg()
	l.Panic("Panic Msg", 1, 2)
	s.assertMsg(PanicLevel, "Panic Msg", 1, 2)
	l.Panicf("Panic Format", 1, 2)
	s.assertMsgf(PanicLevel, "Panic Format", 1, 2)
}

func (s *ModuleTestSuite) TestOff() {
	l := s.ml.Logger("testoff")
	l.Panic("Panic Msg", 1, 2)
	s.assertNoMsg()
}

func (s *ModuleTestSuite) TestChangeLogger() {
	newLogger := &dummyLogger{}
	s.ml.SetLogger("testpanic", newLogger)
	l := s.ml.Logger("testpanic")
	l.Fatal("Fatal Msg", 1, 2)
	newLogger.assertNoMsg(s.T())
	l.Panic("Panic Msg", 1, 2)
	newLogger.assertMsg(s.T(), PanicLevel, "Panic Msg", 1, 2)
}

func (s *ModuleTestSuite) TestCache() {
	s.ml.SetLevel("m1", InfoLevel)
	s.ml.SetLevel("m2", InfoLevel)
	_ = s.ml.Logger("m1")
	m2 := s.ml.Logger("m2")
	m2.Debug("m2")
	s.assertNoMsg()
	m2.Info("m2")
	s.assertMsg(InfoLevel, "m2")
}

// Make sure the logger returned by WithField is still wrapped.
func (s *ModuleTestSuite) TestWithField() {
	l := s.ml.Logger("testpanic")
	newLogger := l.WithField("field", 1)
	newLogger.Fatal("Fatal Msg", 1, 2)
	assert.Equal(s.T(), s.dl.fieldKey, "field")
	assert.Equal(s.T(), s.dl.fieldValue, 1)
	s.assertNoMsg()
	newLogger.Panic("Panic Msg", 1, 2)
	s.assertMsg(PanicLevel, "Panic Msg", 1, 2)
}

type dummyLogFields struct{}

func (dummyLogFields) Fields() map[string]interface{} {
	return nil
}

// Make sure the logger returned by WithFields is still wrapped.
func (s *ModuleTestSuite) TestWithFields() {
	l := s.ml.Logger("testpanic")
	dlf := new(dummyLogFields)
	newLogger := l.WithFields(dlf)
	newLogger.Fatal("Fatal Msg", 1, 2)
	assert.Equal(s.T(), s.dl.fields, dlf)
	s.assertNoMsg()
	newLogger.Panic("Panic Msg", 1, 2)
	s.assertMsg(PanicLevel, "Panic Msg", 1, 2)
}

func (s *ModuleTestSuite) TestPreconditions() {
	assert := assert.New(s.T())
	err := s.ml.SetLevel("test", highestLevel)
	assert.NoError(err)
	err = s.ml.SetLevel("test", highestLevel+1)
	assert.Error(err)
	err = s.ml.SetLevel("test", lowestLevel)
	assert.NoError(err)
}

func TestModuleTestSuite(t *testing.T) {
	suite.Run(t, new(ModuleTestSuite))
}
