package logger

import (
	"github.com/uber-common/bark"
)

type Config struct {
	Logger     bark.Logger
	Ringpop    Level
	Gossip     Level
	Suspicion  Level
	Ring       Level
	Membership Level
	Damping    Level
}

type RingpopLogger struct {
	factory *LoggerFactory
}

func New(conf Config) *RingpopLogger {
	logger := &RingpopLogger{
		factory: NewLoggerFactory(conf.Logger),
	}
	logger.Config(conf)
	return logger
}

const (
	ringpopLogger    = "ringpop"
	gossipLogger     = "gossip"
	suspicionLogger  = "suspicion"
	ringLogger       = "ring"
	membershipLogger = "membership"
	dampingLogger    = "damping"
)

func (rl *RingpopLogger) Config(conf Config) {
	rl.factory.SetLogger(conf.Logger)
	rl.factory.SetLevel(ringpopLogger, conf.Ringpop)
	rl.factory.SetLevel(gossipLogger, conf.Gossip)
	rl.factory.SetLevel(suspicionLogger, conf.Suspicion)
	rl.factory.SetLevel(ringLogger, conf.Ring)
	rl.factory.SetLevel(membershipLogger, conf.Membership)
	rl.factory.SetLevel(dampingLogger, conf.Damping)
}

func (rl *RingpopLogger) Ringpop() Logger {
	return rl.factory.Logger(ringpopLogger)
}

func (rl *RingpopLogger) Gossip() Logger {
	return rl.factory.Logger(gossipLogger)
}

func (rl *RingpopLogger) Suspicion() Logger {
	return rl.factory.Logger(suspicionLogger)
}

func (rl *RingpopLogger) Ring() Logger {
	return rl.factory.Logger(ringLogger)
}

func (rl *RingpopLogger) Membership() Logger {
	return rl.factory.Logger(membershipLogger)
}

func (rl *RingpopLogger) Damping() Logger {
	return rl.factory.Logger(dampingLogger)
}
