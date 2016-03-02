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
	"errors"
	"flag"
	"log"

	"github.com/Sirupsen/logrus"
	"github.com/uber-common/bark"
	"github.com/uber/ringpop-go"
	"github.com/uber/ringpop-go/discovery/jsonfile"
	gen "github.com/uber/ringpop-go/examples/pingpong/gen-go/pingpong"
	"github.com/uber/ringpop-go/swim"
	"github.com/uber/tchannel-go"
	"github.com/uber/tchannel-go/thrift"
)

var (
	hostport = flag.String("listen", "127.0.0.1:3000", "hostport to start service on")
	hostfile = flag.String("hosts", "./hosts.json", "path to hosts file")
)

type worker struct {
	address string
	ringpop *ringpop.Ringpop
}

func newWorker(address string, channel *tchannel.Channel) *worker {
	logger := bark.NewLoggerFromLogrus(logrus.StandardLogger())

	rp, err := ringpop.New("pingpong",
		ringpop.Channel(channel),
		ringpop.Identity(address),
		ringpop.Logger(logger),
	)
	if err != nil {
		log.Fatalf("Unable to create Ringpop: %v", err)
	}

	return &worker{
		address: address,
		ringpop: rp,
	}
}

func (w *worker) Ping(ctx thrift.Context, request *gen.Ping) (*gen.Pong, error) {
	return &gen.Pong{Source: w.address}, nil
}

func main() {
	flag.Parse()

	channel, err := tchannel.NewChannel("pingpong", &tchannel.ChannelOptions{})
	if err != nil {
		log.Fatalf("could not create channel: %v", err)
	}
	server := thrift.NewServer(channel)

	// The actual service implementation
	worker := newWorker(*hostport, channel)
	// wrap the PingPong worker by a ringpop adapter for routing RPC calls
	// NewRingpopPingPongAdapter is in the package containing the generated code
	// Its name is derived from the thrift service name as follows: `NewRingpop[Service Name]Adapter`
	adapter, err := gen.NewRingpopPingPongAdapter(worker, worker.ringpop, channel,
		// PingPongConfiguration contains the configuration for ringpop forwarding regaring this service
		// The name is derived as follows `[Service Name]Configuration`
		gen.PingPongConfiguration{
			// The ping member of the configuration refers to the Ping endpoint within the service.
			// Configuring an endpoint is optional and unconfigured endpoints will work but not be
			// forwarded to a different ringpop node during invocation
			// The name of the configuration struct passed in here is derived as follows:
			// `[Service Name][Endpoint Name]Configuration`
			Ping: &gen.PingPongPingConfiguration{
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

	// now pass the ringpop adapter into the TChannel Thrift server for the PingPong Service
	server.Register(gen.NewTChanPingPongServer(adapter))

	if err := channel.ListenAndServe(*hostport); err != nil {
		log.Fatalf("could not listen on hostport: %v", err)
	}

	bsopts := new(swim.BootstrapOptions)
	bsopts.DiscoverProvider = jsonfile.New(*hostfile)
	bsopts.Stopped = true
	if _, err := worker.ringpop.Bootstrap(bsopts); err != nil {
		log.Fatalf("could not bootstrap ringpop: %v", err)
	}

	// block
	select {}
}
