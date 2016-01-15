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
}

func (suite *ModuleTestSuite) assertMsg(msgLevel Level, args ...interface{}) {
	assert.Equal(suite.T(), suite.dl.msg.level, msgLevel, "Message emitted with wrong level.")
	assert.Equal(suite.T(), suite.dl.msg.args, args, "Message args don't match.")
}

func (suite *ModuleTestSuite) assertMsgf(msgLevel Level, format string, args ...interface{}) {
	suite.assertMsg(msgLevel, args...)
	assert.Equal(suite.T(), suite.dl.msg.format, format, "Message emitted with wrong format.")
}

func (suite *ModuleTestSuite) assertNoMsg() {
	assert.Nil(suite.T(), suite.dl.msg, "Unexpected message emitted.")
}

func (suite *ModuleTestSuite) TestDefaultLevel() {
	l := suite.ml.GetLogger("unnamed")
	l.Info("Info Msg")
	suite.assertNoMsg()
	l.Infof("Info Format", 1, 2)
	suite.assertNoMsg()
	l.Warn("Warn Msg", 1, 2)
	suite.assertMsg(WarnLevel, "Warn Msg", 1, 2)
	l.Warnf("Warn Format", 1, 2)
	suite.assertMsgf(WarnLevel, "Warn Format", 1, 2)
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
	err := suite.ml.SetModuleLevel("test", highestLevel)
	assert.Nil(suite.T(), err)
	err = suite.ml.SetModuleLevel("test", highestLevel+1)
	assert.NotNil(suite.T(), err)
	err = suite.ml.SetModuleLevel("test", lowestLevel)
	assert.Nil(suite.T(), err)
}

func TestModuleTestSuite(t *testing.T) {
	suite.Run(t, new(ModuleTestSuite))
}
