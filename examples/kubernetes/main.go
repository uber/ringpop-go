// Copyright (c) 2016 Uber Technologies, Inc.
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

//go:generate thrift-gen --generateThrift --outputDir gen-go --inputFile ping.thrift --template github.com/uber/ringpop-go/ringpop.thrift-gen

import (
	"errors"
	"os"

	"github.com/uber/ringpop-go/discovery/dns"

	log "github.com/Sirupsen/logrus"
	"github.com/uber-common/bark"
	"github.com/uber/ringpop-go"
	gen "github.com/uber/ringpop-go/examples/ping-thrift-gen/gen-go/ping"
	"github.com/uber/ringpop-go/swim"
	"github.com/uber/tchannel-go"
	"github.com/uber/tchannel-go/thrift"
)

type worker struct {
	ringpop *ringpop.Ringpop
	channel *tchannel.Channel
	logger  *log.Logger
}

func (w *worker) RegisterPing() error {
	server := thrift.NewServer(w.channel)
	adapter, err := gen.NewRingpopPingPongServiceAdapter(w, w.ringpop, w.channel,
		gen.PingPongServiceConfiguration{
			Ping: &gen.PingPongServicePingConfiguration{
				Key: func(ctx thrift.Context, request *gen.Ping) (shardKey string, err error) {
					if request == nil {
						return "", errors.New("missing request in call to Ping")
					}
					return request.Key, nil
				},
			},
		},
	)
	if err != nil {
		log.Fatalf("unable to wrap the worker: %q", err)
	}
	// now pass the ringpop adapter into the TChannel Thrift server for the PingPongService
	server.Register(gen.NewTChanPingPongServiceServer(adapter))
	return nil
}

func (w *worker) Ping(ctx thrift.Context, request *gen.Ping) (*gen.Pong, error) {
	identity, err := w.ringpop.WhoAmI()
	if err != nil {
		return nil, err
	}
	headers := ctx.Headers()
	pHeader := headers["p"]
	return &gen.Pong{
		Message: "Hello, world!",
		From_:   identity,
		Pheader: &pHeader,
	}, nil
}

func main() {
	hostname := os.Getenv("RINGPOP_HOSTNAME")

	ch, err := tchannel.NewChannel("pingchannel", nil)
	if err != nil {
		log.Fatalf("channel did not create successfully: %v", err)
	}

	logger := log.StandardLogger()

	ip, err := tchannel.ListenIP()
	if err != nil {
		log.Fatalf("unable to get listen ip: %v", err)
	}

	hostport := ip.String() + ":3000"

	rp, err := ringpop.New("ping-app",
		ringpop.Channel(ch),
		ringpop.Identity(hostport),
		ringpop.Logger(bark.NewLoggerFromLogrus(logger)),
	)
	if err != nil {
		log.Fatalf("Unable to create Ringpop: %v", err)
	}

	worker := &worker{
		channel: ch,
		ringpop: rp,
		logger:  logger,
	}

	if err := worker.RegisterPing(); err != nil {
		log.Fatalf("could not register ping handler: %v", err)
	}

	if err := worker.channel.ListenAndServe(hostport); err != nil {
		log.Fatalf("could not listen on given hostport: %v", err)
	}
	log.Infof("listening on: %s\n", hostport)

	opts := &swim.BootstrapOptions{}
	opts.DiscoverProvider = &dns.Provider{
		Hostname: hostname,
		Port:     3000,
	}

	if _, err := worker.ringpop.Bootstrap(opts); err != nil {
		log.Fatalf("ringpop bootstrap failed: %v", err)
	}

	select {}
}
