// Copyright (c) 2015 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package main

import (
	"flag"
	"regexp"
	"time"

	"github.com/cactus/go-statsd-client/statsd"
	"github.com/uber-common/bark"
	"github.com/uber/ringpop-go"
	"github.com/uber/ringpop-go/discovery/jsonfile"
	"github.com/uber/ringpop-go/swim"

	log "github.com/Sirupsen/logrus"
	"github.com/uber/tchannel-go"
)

var (
	hostport        = flag.String("listen", "127.0.0.1:3000", "hostport to start ringpop on")
	hostfile        = flag.String("hosts", "./hosts.json", "path to hosts file")
	stats           = flag.String("stats", "", "enable stats emitting, destination can be a host-port (e.g. localhost:8125) or a file-name (e.g. ./stats.log); if unset, no stats are emitted")
	hostportPattern = regexp.MustCompile(`^(\d+.\d+.\d+.\d+):\d+$`)
	statsPattern    = regexp.MustCompile(`^(.+):(\d+)$`)
)

func main() {
	verbose := flag.Bool("verbose", false, "enable debug level logging")
	flag.Parse()

	if !hostportPattern.MatchString(*hostport) {
		log.Fatalf("bad hostport: %s", *hostport)
	}

	ch, err := tchannel.NewChannel("ringpop", nil)
	if err != nil {
		log.Fatalf("could not create channel: %v", err)
	}

	logger := log.StandardLogger()
	if *verbose {
		logger.Level = log.DebugLevel
	}

	options := []ringpop.Option{ringpop.Channel(ch),
		ringpop.Identity(*hostport),
		ringpop.Logger(bark.NewLoggerFromLogrus(logger)),
		ringpop.SuspectPeriod(5 * time.Second),
		ringpop.FaultyPeriod(5 * time.Second),
		ringpop.TombstonePeriod(5 * time.Second)}

	if *stats != "" {
		var statsdClient statsd.Statter
		var err error
		if statsPattern.MatchString(*stats) {
			statsdClient, err = statsd.New(*stats, "")
			if err != nil {
				log.Fatalf("colud not open stats connection: %v", err)
			}
		} else {
			statsdClient, err = NewFileStatsd(*stats)
			if err != nil {
				log.Fatalf("colud not open stats file: %v", err)
			}
		}
		statter := bark.NewStatsReporterFromCactus(statsdClient)
		options = append(options, ringpop.Statter(statter))
	}

	rp, _ := ringpop.New("ringpop", options...)

	if err := ch.ListenAndServe(*hostport); err != nil {
		log.Fatalf("could not listen on %s: %v", *hostport, err)
	}

	opts := &swim.BootstrapOptions{}
	opts.DiscoverProvider = jsonfile.New(*hostfile)

	_, err = rp.Bootstrap(opts)
	if err != nil {
		log.Fatalf("bootstrap failed: %v", err)
	}

	// block
	select {}
}
