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

	"github.com/uber-common/bark"
	"github.com/uber/ringpop-go"
	"github.com/uber/ringpop-go/discovery/jsonfile"
	"github.com/uber/ringpop-go/swim"

	log "github.com/Sirupsen/logrus"
	"github.com/uber/tchannel-go"
)

var (
	hostport = flag.String("listen", "127.0.0.1:3000", "hostport to start ringpop on")
	hostfile = flag.String("hosts", "./hosts.json", "path to hosts file")

	suspectPeriod   = flag.Int("suspect-period", 5000, "suspect period in ms")
	faultyPeriod    = flag.Int("faulty-period", 24*60*60*1000, "faulty period in ms")
	tombstonePeriod = flag.Int("tombstone-period", 5000, "tombstone period in ms")

	hostportPattern = regexp.MustCompile(`^(\d+.\d+.\d+.\d+):\d+$`)
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
	rp, _ := ringpop.New("ringpop",
		ringpop.Channel(ch),
		ringpop.Identity(*hostport),
		ringpop.Logger(bark.NewLoggerFromLogrus(logger)),

		ringpop.SuspectPeriod(time.Duration(*suspectPeriod)*time.Millisecond),
		ringpop.FaultyPeriod(time.Duration(*faultyPeriod)*time.Millisecond),
		ringpop.TombstonePeriod(time.Duration(*tombstonePeriod)*time.Millisecond),
	)

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
