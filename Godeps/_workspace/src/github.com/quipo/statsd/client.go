package statsd

import (
	"fmt"
	"log"
	"net"
	"os"
	"strings"
	"time"

	"github.com/quipo/statsd/event"
)

// Logger interface compatible with log.Logger
type Logger interface {
	Println(v ...interface{})
}

// note Hostname is exported so clients can set it to something different than the default
var Hostname string

func init() {
	host, err := os.Hostname()
	if nil == err {
		Hostname = host
	}
}

// StatsdClient is a client library to send events to StatsD
type StatsdClient struct {
	conn   net.Conn
	addr   string
	prefix string
	Logger Logger
}

// NewStatsdClient - Factory
func NewStatsdClient(addr string, prefix string) *StatsdClient {
	// allow %HOST% in the prefix string
	prefix = strings.Replace(prefix, "%HOST%", Hostname, 1)
	return &StatsdClient{
		addr:   addr,
		prefix: prefix,
		Logger: log.New(os.Stdout, "[StatsdClient] ", log.Ldate|log.Ltime),
	}
}

// String returns the StatsD server address
func (c *StatsdClient) String() string {
	return c.addr
}

// CreateSocket creates a UDP connection to a StatsD server
func (c *StatsdClient) CreateSocket() error {
	conn, err := net.DialTimeout("udp", c.addr, 5*time.Second)
	if err != nil {
		return err
	}
	c.conn = conn
	return nil
}

// Close the UDP connection
func (c *StatsdClient) Close() error {
	if nil == c.conn {
		return nil
	}
	return c.conn.Close()
}

// See statsd data types here: http://statsd.readthedocs.org/en/latest/types.html
// or also https://github.com/b/statsd_spec

// Incr - Increment a counter metric. Often used to note a particular event
func (c *StatsdClient) Incr(stat string, count int64) error {
	if 0 != count {
		return c.send(stat, "%d|c", count)
	}
	return nil
}

// Decr - Decrement a counter metric. Often used to note a particular event
func (c *StatsdClient) Decr(stat string, count int64) error {
	if 0 != count {
		return c.send(stat, "%d|c", -count)
	}
	return nil
}

// Timing - Track a duration event
// the time delta must be given in milliseconds
func (c *StatsdClient) Timing(stat string, delta int64) error {
	return c.send(stat, "%d|ms", delta)
}

// PrecisionTiming - Track a duration event
// the time delta has to be a duration
func (c *StatsdClient) PrecisionTiming(stat string, delta time.Duration) error {
	return c.send(stat, fmt.Sprintf("%.6f%s|ms", float64(delta)/float64(time.Millisecond), "%d"), 0)
}

// Gauge - Gauges are a constant data type. They are not subject to averaging,
// and they don’t change unless you change them. That is, once you set a gauge value,
// it will be a flat line on the graph until you change it again. If you specify
// delta to be true, that specifies that the gauge should be updated, not set. Due to the
// underlying protocol, you can't explicitly set a gauge to a negative number without
// first setting it to zero.
func (c *StatsdClient) Gauge(stat string, value int64) error {
	if value < 0 {
		c.send(stat, "%d|g", 0)
		return c.send(stat, "%d|g", value)
	}
	return c.send(stat, "%d|g", value)
}

// GaugeDelta -- Send a change for a gauge
func (c *StatsdClient) GaugeDelta(stat string, value int64) error {
	// Gauge Deltas are always sent with a leading '+' or '-'. The '-' takes care of itself but the '+' must added by hand
	if value < 0 {
		return c.send(stat, "%d|g", value)
	}
	return c.send(stat, "+%d|g", value)
}

// FGauge -- Send a floating point value for a gauge
func (c *StatsdClient) FGauge(stat string, value float64) error {
	if value < 0 {
		c.send(stat, "%d|g", 0)
		return c.send(stat, "%g|g", value)
	}
	return c.send(stat, "%g|g", value)
}

// FGaugeDelta -- Send a floating point change for a gauge
func (c *StatsdClient) FGaugeDelta(stat string, value float64) error {
	if value < 0 {
		return c.send(stat, "%g|g", value)
	}
	return c.send(stat, "+%g|g", value)
}

// Absolute - Send absolute-valued metric (not averaged/aggregated)
func (c *StatsdClient) Absolute(stat string, value int64) error {
	return c.send(stat, "%d|a", value)
}

// FAbsolute - Send absolute-valued floating point metric (not averaged/aggregated)
func (c *StatsdClient) FAbsolute(stat string, value float64) error {
	return c.send(stat, "%g|a", value)
}

// Total - Send a metric that is continously increasing, e.g. read operations since boot
func (c *StatsdClient) Total(stat string, value int64) error {
	return c.send(stat, "%d|t", value)
}

// write a UDP packet with the statsd event
func (c *StatsdClient) send(stat string, format string, value interface{}) error {
	if c.conn == nil {
		return fmt.Errorf("not connected")
	}
	stat = strings.Replace(stat, "%HOST%", Hostname, 1)
	format = fmt.Sprintf("%s%s:%s", c.prefix, stat, format)
	_, err := fmt.Fprintf(c.conn, format, value)
	return err
}

// SendEvent - Sends stats from an event object
func (c *StatsdClient) SendEvent(e event.Event) error {
	if c.conn == nil {
		return fmt.Errorf("cannot send stats, not connected to StatsD server")
	}
	for _, stat := range e.Stats() {
		//fmt.Printf("SENDING EVENT %s%s\n", c.prefix, stat)
		_, err := fmt.Fprintf(c.conn, "%s%s", c.prefix, stat)
		if nil != err {
			return err
		}
	}
	return nil
}
