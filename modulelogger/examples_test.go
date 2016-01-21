package modulelogger

import (
	"github.com/Sirupsen/logrus"
	"github.com/uber-common/bark"
	"os"
)

func getLogrusLogger() bark.Logger {
	logger := logrus.New()
	logger.Out = os.Stdout
	logger.Level = logrus.DebugLevel
	// TextFormatter is broken because it adds a trailing space to messages
	// which is impossible to add in the Output section.
	// JsonFormatter is also not ideal because the time can't be disabled
	logger.Formatter = &logrus.JSONFormatter{TimestampFormat: "notime"}
	return bark.NewLoggerFromLogrus(logger)
}

func ExampleUsingTheRootLogger() {
	logger := getLogrusLogger()
	ml := New(logger)

	// ml is itself a logger that passes all messages to the wrapped logger
	ml.Debug("hello ml")

	// The root logger can be configured using the "root" name
	ml.SetLevel("root", WarnLevel)

	ml.Debug("filtered, level too low")
	ml.Warn("hello again")

	// This is equivalent with using ml directly
	ml.Logger("root").Debug("filtered, level too low")

	// Output:
	// {"level":"debug","msg":"hello ml","time":"notime"}
	// {"level":"warning","msg":"hello again","time":"notime"}
}

func ExampleSetLevelsPerModule() {
	logger := getLogrusLogger()
	ml := New(logger)

	ml.SetLevel("x", ErrorLevel)
	ml.SetLevel("y", WarnLevel)

	// module loggers only emit messages above the configured level
	x := ml.Logger("x")
	x.Warn("filtered, level too low")
	x.Error("from x")
	y := ml.Logger("y")
	y.Info("filtered, level too low")
	y.Warn("from y")

	// any change on a module level is seen by existing loggers
	ml.SetLevel("y", DebugLevel)
	y.Debug("y again")

	// Output:
	// {"level":"error","msg":"from x","time":"notime"}
	// {"level":"warning","msg":"from y","time":"notime"}
	// {"level":"debug","msg":"y again","time":"notime"}
}

func ExampleUncofiguredModuleLoggers() {
	logger := getLogrusLogger()
	ml := New(logger)

	ml.SetLevel("root", PanicLevel)

	// unconfigured module loggers emit all messages, regardless of root
	// logger configuration
	x := ml.Logger("x")
	x.Debug("hello x")

	// Output:
	// {"level":"debug","msg":"hello x","time":"notime"}
}

func ExampleWithFields() {
	logger := getLogrusLogger()
	ml := New(logger)
	ml.SetLevel("x", ErrorLevel)

	x := ml.Logger("x")

	// Any fields set on the logger are forwarded to the wrapped logger
	otherLogger := x.WithField("a", 1).WithField("b", 2)
	otherLogger.Info("filtered, level too low")
	otherLogger.Error("from x")

	// Output:
	// {"a":1,"b":2,"level":"error","msg":"from x","time":"notime"}
}

func ExampleChainig() {
	logger := getLogrusLogger()
	ml := New(logger)
	ml.SetLevel("x", InfoLevel)
	ml.SetLevel("y", WarnLevel)

	// Add some metadata to the root logger
	mlWithField := ml.WithField("app", "test")

	// Sometimes a logger accumulates metadata and is passed around in the
	// application from one module to another.
	// It's possible to change the module on an existing logger while
	// preserving previous metadata
	module1 := GetModuleLogger(mlWithField, "x").WithField("field_x", 1)

	// When the logger is passed to another module, the underlying type is
	// still a ModuleLogger. This makes it so that another module logger
	// can be retrieved, preserving the previous fields. In other words, if
	// the interface would allow it, this would be similar to:
	// ml.WithField("app", "test").Logger("x").WithField("field_x", 1).Logger("y").WithField("field_y", 2)
	module2 := GetModuleLogger(module1, "y").WithField("field_y", 2)

	module1.Debug("filtered, level too low")
	module1.Info("m1")
	module2.Info("filtered, level too low")
	module2.Warn("m2")

	// Output:
	// {"app":"test","field_x":1,"level":"info","msg":"m1","time":"notime"}
	// {"app":"test","field_x":1,"field_y":2,"level":"warning","msg":"m2","time":"notime"}
}
