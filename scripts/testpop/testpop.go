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
	"os"
	"os/signal"
	"regexp"
	"syscall"
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
	hostport  = flag.String("listen", "127.0.0.1:3000", "hostport to start ringpop on")
	hostfile  = flag.String("hosts", "./hosts.json", "path to hosts file")
	statsFile = flag.String("stats-file", "", "enable stats emitting to a file.")
	statsUDP  = flag.String("stats-udp", "", "enable stats emitting over udp.")

	suspectPeriod   = flag.Int("suspect-period", 5000, "The lifetime of a suspect member in ms. After that the member becomes faulty.")
	faultyPeriod    = flag.Int("faulty-period", 24*60*60*1000, "The lifetime of a faulty member in ms. After that the member becomes a tombstone.")
	tombstonePeriod = flag.Int("tombstone-period", 5000, "The lifetime of a tombstone member in ms. After that the member is removed from the membership.")

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

	options := []ringpop.Option{ringpop.Channel(ch),
		ringpop.Identity(*hostport),
		ringpop.Logger(bark.NewLoggerFromLogrus(logger)),

		ringpop.SuspectPeriod(time.Duration(*suspectPeriod) * time.Millisecond),
		ringpop.FaultyPeriod(time.Duration(*faultyPeriod) * time.Millisecond),
		ringpop.TombstonePeriod(time.Duration(*tombstonePeriod) * time.Millisecond),
	}

	if *statsUDP != "" && *statsFile != "" {
		log.Fatalf("-stats-udp and stats-file are mutually exclusive.")
	}

	if *statsUDP != "" || *statsFile != "" {
		var statsdClient statsd.Statter
		if *statsUDP != "" {
			var err error
			statsdClient, err = statsd.New(*statsUDP, "")
			if err != nil {
				log.Fatalf("colud not open stats connection: %v", err)
			}
		}

		if *statsFile != "" {
			statsdClient, err = NewFileStatsd(*statsFile)
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

	go signalHandler(rp, true)  // handle SIGINT
	go signalHandler(rp, false) // handle SIGTERM

	// block
	select {}
}

func signalHandler(rp *ringpop.Ringpop, interactive bool) {
	sigchan := make(chan os.Signal, 1)

	var s os.Signal
	if interactive {
		s = syscall.SIGINT
	} else {
		s = syscall.SIGTERM
	}

	signal.Notify(sigchan, s)

	// wait on a signal
	<-sigchan
	log.Println("received signal, initiating self eviction")

	if interactive {
		log.Error("triggered graceful shutdown. Press Ctrl+C again to force exit.")
		go func() {
			// exit on second signal
			<-sigchan
			log.Error("Force exiting...")
			os.Exit(1)
		}()
	}

	rp.SelfEvict()
	log.Println("member got evicted")

	log.Println("sleeping because of human")
	<-time.After(time.Second * 1)
	log.Println("finished sleeping, shutting down")

	os.Exit(0)
}
