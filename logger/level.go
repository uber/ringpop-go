package logger

import "fmt"

type Level uint8

func (level Level) String() string {
	switch level {
	case Trace:
		return "trace"
	case Debug:
		return "debug"
	case Info:
		return "info"
	case Warn:
		return "warn"
	case Error:
		return "error"
	case Fatal:
		return "fatal"
	case Panic:
		return "panic"
	case Off:
		return "off"
	}

	return "unknown"
}

// Convert a string to a Level, the reverse of Level.String
func Parse(lvl string) (Level, error) {
	switch lvl {
	case "off":
		return Off, nil
	case "panic":
		return Panic, nil
	case "fatal":
		return Fatal, nil
	case "error":
		return Error, nil
	case "warn":
		return Warn, nil
	case "info":
		return Info, nil
	case "debug":
		return Debug, nil
	case "trace":
		return Trace, nil
	}

	var l Level
	return l, fmt.Errorf("not a valid Level: %q", lvl)
}

const (
	Trace Level = iota
	Debug
	Info
	Warn
	Error
	Fatal
	Panic
	Off
)

const lowestLevel = Trace
const highestLevel = Off
