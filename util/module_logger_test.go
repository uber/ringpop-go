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
	ml *moduleLogger
}

func (suite *ModuleTestSuite) SetupTest() {
	suite.dl = &dummyLogger{}
	suite.ml = NewModuleLogger(suite.dl)
	suite.ml.SetModuleLevel("testdebug", DebugLevel)
	suite.ml.SetModuleLevel("testpanic", PanicLevel)
	suite.ml.SetModuleLevel("testoff", OffLevel)
}

func (suite *ModuleTestSuite) assertMsg(msgLevel Level, args ...interface{}) {
	suite.dl.assertMsg(suite.T(), msgLevel, args...)
}

func (suite *ModuleTestSuite) assertMsgf(msgLevel Level, format string, args ...interface{}) {
	suite.dl.assertMsgf(suite.T(), msgLevel, format, args...)
}

func (suite *ModuleTestSuite) assertNoMsg() {
	suite.dl.assertNoMsg(suite.T())
}

func (suite *ModuleTestSuite) TestOrigLogger() {
	l := suite.ml.GetLogger("unnamed")
	l.Debug("Debug Msg", 1, 2)
	suite.assertMsg(DebugLevel, "Debug Msg", 1, 2)
	l.Debugf("Debug Format", 1, 2)
	suite.assertMsgf(DebugLevel, "Debug Format", 1, 2)
}

func (suite *ModuleTestSuite) TestDebug() {
	l := suite.ml.GetLogger("testdebug")
	l.Debug("Debug Msg", 1, 2)
	suite.assertMsg(DebugLevel, "Debug Msg", 1, 2)
	l.Debugf("Debug Format", 1, 2)
	suite.assertMsgf(DebugLevel, "Debug Format", 1, 2)
	l.Info("Info Msg", 1, 2)
	suite.assertMsg(InfoLevel, "Info Msg", 1, 2)
	l.Infof("Info Format", 1, 2)
	suite.assertMsgf(InfoLevel, "Info Format", 1, 2)
	l.Warn("Warn Msg", 1, 2)
	suite.assertMsg(WarnLevel, "Warn Msg", 1, 2)
	l.Warnf("Warn Format", 1, 2)
	suite.assertMsgf(WarnLevel, "Warn Format", 1, 2)
	l.Error("Error Msg", 1, 2)
	suite.assertMsg(ErrorLevel, "Error Msg", 1, 2)
	l.Errorf("Error Format", 1, 2)
	suite.assertMsgf(ErrorLevel, "Error Format", 1, 2)
	l.Fatal("Fatal Msg", 1, 2)
	suite.assertMsg(FatalLevel, "Fatal Msg", 1, 2)
	l.Fatalf("Fatal Format", 1, 2)
	suite.assertMsgf(FatalLevel, "Fatal Format", 1, 2)
	l.Panic("Panic Msg", 1, 2)
	suite.assertMsg(PanicLevel, "Panic Msg", 1, 2)
	l.Panicf("Panic Format", 1, 2)
	suite.assertMsgf(PanicLevel, "Panic Format", 1, 2)
}

func (suite *ModuleTestSuite) TestPanic() {
	l := suite.ml.GetLogger("testpanic")
	l.Debug("Debug Msg", 1, 2)
	suite.assertNoMsg()
	l.Debugf("Debug Format", 1, 2)
	suite.assertNoMsg()
	l.Info("Info Msg", 1, 2)
	suite.assertNoMsg()
	l.Infof("Info Format", 1, 2)
	suite.assertNoMsg()
	l.Warn("Warn Msg", 1, 2)
	suite.assertNoMsg()
	l.Warnf("Warn Format", 1, 2)
	suite.assertNoMsg()
	l.Error("Error Msg", 1, 2)
	suite.assertNoMsg()
	l.Errorf("Error Format", 1, 2)
	suite.assertNoMsg()
	l.Fatal("Fatal Msg", 1, 2)
	suite.assertNoMsg()
	l.Fatalf("Fatal Format", 1, 2)
	suite.assertNoMsg()
	l.Panic("Panic Msg", 1, 2)
	suite.assertMsg(PanicLevel, "Panic Msg", 1, 2)
	l.Panicf("Panic Format", 1, 2)
	suite.assertMsgf(PanicLevel, "Panic Format", 1, 2)
}

func (suite *ModuleTestSuite) TestOff() {
	l := suite.ml.GetLogger("testoff")
	l.Panic("Panic Msg", 1, 2)
	suite.assertNoMsg()
}

func (suite *ModuleTestSuite) TestChangeLogger() {
	newLogger := &dummyLogger{}
	l := suite.ml.SetLogger(newLogger).GetLogger("testpanic")
	l.Fatal("Fatal Msg", 1, 2)
	newLogger.assertNoMsg(suite.T())
	l.Panic("Panic Msg", 1, 2)
	newLogger.assertMsg(suite.T(), PanicLevel, "Panic Msg", 1, 2)
}

func (suite *ModuleTestSuite) TestCache() {
	suite.ml.SetModuleLevel("m1", InfoLevel)
	suite.ml.SetModuleLevel("m2", InfoLevel)
	_ = suite.ml.GetLogger("m1")
	m2 := suite.ml.GetLogger("m2")
	m2.Debug("m2")
	suite.assertNoMsg()
	m2.Info("m2")
	suite.assertMsg(InfoLevel, "m2")
}

// Make sure the logger returned by WithField is still wrapped
func (suite *ModuleTestSuite) TestWithField() {
	l := suite.ml.GetLogger("testpanic")
	new_l := l.WithField("field", 1)
	assert.Equal(suite.T(), suite.dl.fieldKey, "field")
	assert.Equal(suite.T(), suite.dl.fieldValue, 1)
	new_l.Fatal("Fatal Msg", 1, 2)
	suite.assertNoMsg()
	new_l.Panic("Panic Msg", 1, 2)
	suite.assertMsg(PanicLevel, "Panic Msg", 1, 2)
}

type dummyLogFields struct{}

func (dummyLogFields) Fields() map[string]interface{} {
	return nil
}

// Make sure the logger returned by WithFields is still wrapped
func (suite *ModuleTestSuite) TestWithFields() {
	l := suite.ml.GetLogger("testpanic")
	dlf := new(dummyLogFields)
	new_l := l.WithFields(dlf)
	assert.Equal(suite.T(), suite.dl.fields, dlf)
	new_l.Fatal("Fatal Msg", 1, 2)
	suite.assertNoMsg()
	new_l.Panic("Panic Msg", 1, 2)
	suite.assertMsg(PanicLevel, "Panic Msg", 1, 2)
}

func (suite *ModuleTestSuite) TestPreconditions() {
	assert := assert.New(suite.T())
	err := suite.ml.SetModuleLevel("test", highestLevel)
	assert.NoError(err)
	err = suite.ml.SetModuleLevel("test", highestLevel+1)
	assert.Error(err)
	err = suite.ml.SetModuleLevel("test", lowestLevel)
	assert.NoError(err)
}

func TestModuleTestSuite(t *testing.T) {
	suite.Run(t, new(ModuleTestSuite))
}
