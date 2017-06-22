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

//go:generate thrift-gen --generateThrift --outputDir gen-go --inputFile ping.thrift --template github.com/uber/ringpop-go/ringpop.thrift-gen

package main

import (
	"errors"
	"flag"

	log "github.com/sirupsen/logrus"
	"github.com/uber-common/bark"
	"github.com/uber/ringpop-go"
	"github.com/uber/ringpop-go/discovery/jsonfile"
	gen "github.com/uber/ringpop-go/examples/ping-thrift-gen/gen-go/ping"
	"github.com/uber/ringpop-go/swim"
	"github.com/uber/tchannel-go"
	"github.com/uber/tchannel-go/thrift"
)

var (
	hostport = flag.String("listen", "127.0.0.1:3000", "hostport to start service on")
	hostfile = flag.String("hosts", "./hosts.json", "path to hosts file")
)

type worker struct {
	ringpop *ringpop.Ringpop
	channel *tchannel.Channel
	logger  *log.Logger
}

func (w *worker) RegisterPing() error {
	server := thrift.NewServer(w.channel)
	// wrap the PingPongService worker by a ringpop adapter for routing RPC calls
	// NewRingpopPingPongServiceAdapter is in the package containing the generated code
	// Its name is derived from the thrift service name as follows: `NewRingpop[Service Name]Adapter`
	adapter, err := gen.NewRingpopPingPongServiceAdapter(w, w.ringpop, w.channel,
		// PingPongServiceConfiguration contains the configuration for ringpop forwarding regaring this service
		// The name is derived as follows `[Service Name]Configuration`
		gen.PingPongServiceConfiguration{
			// The ping member of the configuration refers to the Ping endpoint within the service.
			// Configuring an endpoint is optional and unconfigured endpoints will work but not be
			// forwarded to a different ringpop node during invocation
			// The name of the configuration struct passed in here is derived as follows:
			// `[Service Name][Endpoint Name]Configuration`
			Ping: &gen.PingPongServicePingConfiguration{
				// The configuration structs only member is the Key closure. The purpose of this closure
				// is to return the key used for ringpop sharding and forwarding. Calls that are sharded
				// on the same key are guaranteed to be forwarded to the same node.
				// The input signature for this closure is the same as the signature for your
				// implementation. The output is always the tuple (shardKey string, err error).
				Key: func(ctx thrift.Context, request *gen.Ping) (shardKey string, err error) {
					// The body of the Key closure can perform whatever logic is needed to come to a
					// meaningful shardKey for the the request
					if request == nil {
						return "", errors.New("missing request in call to Ping")
					}
					// Here we route the ping's for the same key to the same machine
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
	address, err := w.ringpop.WhoAmI()
	if err != nil {
		return nil, err
	}
	headers := ctx.Headers()
	pHeader := headers["p"]
	return &gen.Pong{
		Message: "Hello, world!",
		From_:   address,
		Pheader: &pHeader,
	}, nil
}

func main() {
	flag.Parse()

	ch, err := tchannel.NewChannel("pingchannel", nil)
	if err != nil {
		log.Fatalf("channel did not create successfully: %v", err)
	}

	logger := log.StandardLogger()

	rp, err := ringpop.New("ping-app",
		ringpop.Channel(ch),
		ringpop.Address(*hostport),
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

	if err := worker.channel.ListenAndServe(*hostport); err != nil {
		log.Fatalf("could not listen on given hostport: %v", err)
	}

	opts := new(swim.BootstrapOptions)
	opts.DiscoverProvider = jsonfile.New(*hostfile)

	if _, err := worker.ringpop.Bootstrap(opts); err != nil {
		log.Fatalf("ringpop bootstrap failed: %v", err)
	}

	select {}
}
