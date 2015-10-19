## Synopsis

Defines an interface for loggers and stats reporters that can be passed to Uber Go libraries.  
Provides implementations which wrap a common logging module, [logrus](https://github.com/Sirupsen/logrus), 
and a common stats reporting module [go-statsd-client](https://github.com/cactus/go-statsd-client).  
Clients may also choose to implement these interfaces themselves.

## Key Interfaces

### Logging

```go
// Logger is an interface for loggers accepted by Uber's libraries.
type Logger interface {
	// Log at debug level
	Debug(args ...interface{})

	// Log at debug level with fmt.Printf-like formatting
	Debugf(format string, args ...interface{})

	// Log at info level
	Info(args ...interface{})

	// Log at info level with fmt.Printf-like formatting
	Infof(format string, args ...interface{})

	// Log at warning level
	Warn(args ...interface{})

	// Log at warning level with fmt.Printf-like formatting
	Warnf(format string, args ...interface{})

	// Log at error level
	Error(args ...interface{})

	// Log at error level with fmt.Printf-like formatting
	Errorf(format string, args ...interface{})

	// Log at fatal level, then terminate process (irrecoverable)
	Fatal(args ...interface{})

	// Log at fatal level with fmt.Printf-like formatting, then terminate process (irrecoverable)
	Fatalf(format string, args ...interface{})

	// Log at panic level, then panic (recoverable)
	Panic(args ...interface{})

	// Log at panic level with fmt.Printf-like formatting, then panic (recoverable)
	Panicf(format string, args ...interface{})

	// Return a logger with the specified key-value pair set, to be logged in a subsequent normal logging call
	WithField(key string, value interface{}) Logger

	// Return a logger with the specified key-value pairs set, to be  included in a subsequent normal logging call
	WithFields(keyValues LogFields) Logger

	// Return map fields associated with this logger, if any (i.e. if this logger was returned from WithField[s])
	// If no fields are set, returns nil
	Fields() Fields
}
```

### Stats Reporting

```go
// StatsReporter is an interface for statsd-like stats reporters accepted by Uber's libraries.
// Its methods take optional tag dictionaries which may be ignored by concrete implementations.
type StatsReporter interface {
	// Increment a statsd-like counter with optional tags
	IncCounter(name string, tags Tags, value int64)

	// Increment a statsd-like gauge ("set" of the value) with optional tags
	UpdateGauge(name string, tags Tags, value int64)

	// Record a statsd-like timer with optional tags
	RecordTimer(name string, tags Tags, d time.Duration)
}
```

## Basic Usage

```go
logger := logrus.New()
barkLogger := bark.NewLoggerFromLogrus(logger)
barkLogger.WithFields(bark.Fields{"someField":"someValue"}).Info("Message")

statsd, err := statsd.New("127.0.0.1:8125", "barktest")
if err != nil {
    logger.Fatal("Example code failed")
}

barkStatsReporter := bark.NewStatsReporterFromCactus(statsd)  
barkStatsReporter.IncCounter("foo", map[string]string{"tag":"val"}, 1)
 
ubermodule.New(ubermodule.Config{
    logger: barkLogger
    statsd: barkStatsReporter
})
```

## Contributors

dh

## License

bark is available under the MIT license. See the LICENSE file for more info.
