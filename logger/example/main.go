package main

import (
	"github.com/Sirupsen/logrus"
	"github.com/uber-common/bark"
	"github.com/uber/ringpop-go/logger"
)

func main() {
	logrusLogger := logrus.New()
	// Let all messages pass
	logrusLogger.Level = logrus.DebugLevel
	barkLogger := bark.NewLoggerFromLogrus(logrusLogger)

	// Create the logger facility
	facility := logger.NewFacility(barkLogger)

	// "mainfunc" is unconfigured, defaults to Warn
	log := facility.Logger("mainfunc")
	log.Warn("before first x call")

	facility.SetLevel("x", logger.Trace)
	x(facility) // x expects a Factory

	// change the configuration at runtime
	facility.SetLevel("x", logger.Off)
	facility.SetLevel("z", logger.Off)

	log.Warn("before second x call, disable x and z")
	x(facility.WithField("secondrun", true))

	// config changes apply to existing loggers too
	facility.SetLevel("mainfunc", logger.Off)
	log.Warn("silenced")
}

func x(f logger.Factory) {
	// f.SetLevel() and f.SetLogger() are unavailable here
	f = f.WithField("x", true)
	log := f.Logger("x").WithField("field1", 1)
	log.Trace("in x")
	log.Trace("still in x")
	// pass the augumented factory, with the x field set
	y(f)
	// call again, with different x value
	y(f.WithField("x", false))
}

func y(f logger.Factory) {
	log := f.Logger("y")
	log.WithField("field2", 2).Warn("in y")
	z(f)
}

func z(f logger.Factory) {
	f.Logger("z").Warn("in z")
}
