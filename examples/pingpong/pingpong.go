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
	"github.com/uber/ringpop-go/examples/pingpong/gen-go/pingpong"
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

func (w *worker) Ping(ctx thrift.Context, request *pingpong.Ping) (*pingpong.Pong, error) {
	return &pingpong.Pong{Source: w.address}, nil
}

func main() {
	flag.Parse()

	channel, err := tchannel.NewChannel("pingpong", &tchannel.ChannelOptions{
	// Logger: tchannel.NewLevelLogger(tchannel.SimpleLogger, tchannel.LogLevelWarn),
	})
	if err != nil {
		log.Fatalf("could not create channel: %v", err)
	}

	worker := newWorker(*hostport, channel)
	server := thrift.NewServer(channel)

	var handler pingpong.TChanPingPong
	handler = worker

	// wrap the PingPong handler by a ringpop adapter for routing RPC calls
	handler, err = pingpong.NewRingpopPingPongAdapter(handler, worker.ringpop, channel, pingpong.PingPongConfiguration{
		Ping: &pingpong.PingPongPingConfiguration{
			Key: func(ctx thrift.Context, request *pingpong.Ping) (string, error) {
				if request == nil {
					return "", errors.New("missing request in call to Ping")
				}
				return request.Key, nil
			},
		},
	})
	if err != nil {
		log.Fatalf("unable to wrap the handler: %q", err)
	}

	server.Register(pingpong.NewTChanPingPongServer(handler))

	if err := channel.ListenAndServe(*hostport); err != nil {
		log.Fatalf("could not listen on hostport: %v", err)
	}

	bsopts := new(swim.BootstrapOptions)
	bsopts.File = *hostfile
	bsopts.Stopped = true
	if _, err := worker.ringpop.Bootstrap(bsopts); err != nil {
		log.Fatalf("could not bootstrap ringpop: %v", err)
	}

	// block
	select {}
}
