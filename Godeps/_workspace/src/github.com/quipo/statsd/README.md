# StatsD client (Golang)

[![GoDoc](https://godoc.org/github.com/quipo/statsd?status.png)](http://godoc.org/github.com/quipo/statsd)

## Introduction

Go Client library for [StatsD](https://github.com/etsy/statsd/). Contains a direct and a buffered client.
The buffered version will hold and aggregate values for the same key in memory before flushing them at the defined frequency.

This client library was inspired by the one embedded in the [Bit.ly NSQ](https://github.com/bitly/nsq/blob/master/util/statsd_client.go) project, and extended to support some extra custom events used at DataSift.

## Installation

    go get github.com/quipo/statsd

## Supported event types

* Increment - Count occurrences per second/minute of a specific event
* Decrement - Count occurrences per second/minute of a specific event
* Timing - To track a duration event
* Gauge - Gauges are a constant data type. They are not subject to averaging, and they don’t change unless you change them. That is, once you set a gauge value, it will be a flat line on the graph until you change it again
* Absolute - Absolute-valued metric (not averaged/aggregated)
* Total - Continously increasing value, e.g. read operations since boot


## Sample usage

```go
package main

import (
    "time"

	"github.com/quipo/statsd"
)

func main() {
	// init
	prefix := "myproject."
	statsdclient := statsd.NewStatsdClient("localhost:8125", prefix)
	statsdclient.CreateSocket()
	interval := time.Second * 2 // aggregate stats and flush every 2 seconds
	stats := statsd.NewStatsdBuffer(interval, statsdclient)
	defer stats.Close()

	// not buffered: send immediately
	statsdclient.Incr("mymetric", 4)

	// buffered: aggregate in memory before flushing
	stats.Incr("mymetric", 1)
	stats.Incr("mymetric", 3)
	stats.Incr("mymetric", 1)
	stats.Incr("mymetric", 1)
}
```

The string "%HOST%" in the metric name will automatically be replaced with the hostname of the server the event is sent from.


## Author

Lorenzo Alberton

* Web: [http://alberton.info](http://alberton.info)
* Twitter: [@lorenzoalberton](https://twitter.com/lorenzoalberton)
* Linkedin: [/in/lorenzoalberton](https://www.linkedin.com/in/lorenzoalberton)


## Copyright

See [LICENSE](LICENSE) document
