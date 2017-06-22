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
	"bytes"
	"flag"

	log "github.com/sirupsen/logrus"
	"github.com/uber-common/bark"
	"github.com/uber/ringpop-go"
	"github.com/uber/ringpop-go/discovery/jsonfile"
	gen "github.com/uber/ringpop-go/examples/ping-thrift/gen-go/ping"
	"github.com/uber/ringpop-go/forward"
	"github.com/uber/ringpop-go/swim"
	"github.com/uber/tchannel-go"
	"github.com/uber/tchannel-go/thrift"
)

var (
	hostport = flag.String("listen", "127.0.0.1:3000", "hostport to start ringpop on")
	hostfile = flag.String("hosts", "./hosts.json", "path to hosts file")
)

type worker struct {
	ringpop *ringpop.Ringpop
	channel *tchannel.Channel
	logger  *log.Logger
}

func (w *worker) RegisterPing() error {
	server := thrift.NewServer(w.channel)
	server.Register(gen.NewTChanPingPongServiceServer(w))
	return nil
}

func (w *worker) Ping(ctx thrift.Context, request *gen.Ping) (*gen.Pong, error) {
	var pongResult gen.PingPongServicePingResult
	var res []byte

	headers := ctx.Headers()
	var marshaledHeaders bytes.Buffer
	err := thrift.WriteHeaders(&marshaledHeaders, headers)
	if err != nil {
		return nil, err
	}

	pingArgs := &gen.PingPongServicePingArgs{
		Request: request,
	}
	req, err := ringpop.SerializeThrift(pingArgs)
	if err != nil {
		return nil, err
	}

	forwardOptions := &forward.Options{Headers: marshaledHeaders.Bytes()}
	handle, err := w.ringpop.HandleOrForward(request.Key, req, &res, "pingchannel", "PingPongService::Ping", tchannel.Thrift, forwardOptions)
	if handle {
		address, err := w.ringpop.WhoAmI()
		if err != nil {
			return nil, err
		}
		pHeader := headers["p"]
		return &gen.Pong{
			Message: "Hello, world!",
			From_:   address,
			Pheader: &pHeader,
		}, nil
	}

	if err := ringpop.DeserializeThrift(res, &pongResult); err != nil {
		return nil, err
	}

	// else request was forwarded
	return pongResult.Success, err
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
