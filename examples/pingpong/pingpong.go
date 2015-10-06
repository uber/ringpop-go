package main

import (
	"flag"
	"log"

	"github.com/Sirupsen/logrus"
	"github.com/uber/bark"
	"github.com/uber/ringpop-go"
	"github.com/uber/ringpop-go/examples/pingpong/gen-go/pingpong"
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

	return &worker{
		address: address,
		ringpop: ringpop.NewRingpop("pingpong", address, channel, &ringpop.Options{
			Logger: logger,
		}),
	}
}

func (w *worker) Ping(ctx thrift.Context, request *pingpong.Ping) (*pingpong.Pong, error) {
	var req, res []byte
	var err error

	if req, err = ringpop.SerializeThrift(&pingpong.PingArgs{Request: request}); err != nil {
		return nil, err
	}

	handle, err := w.ringpop.HandleOrForward(request.Key, req, &res, "pingpong",
		"PingPong::Ping", tchannel.Thrift, nil)

	if !handle {
		if err != nil {
			return nil, err
		}

		var pongResult pingpong.PingResult
		if err := ringpop.DeserializeThrift(res, &pongResult); err != nil {
			return nil, err
		}

		return pongResult.GetSuccess(), nil
	}

	// handle request locally
	return &pingpong.Pong{From: w.address}, nil
}

func main() {
	flag.Parse()

	channel, err := tchannel.NewChannel("worker", &tchannel.ChannelOptions{
	// Logger: tchannel.NewLevelLogger(tchannel.SimpleLogger, tchannel.LogLevelWarn),
	})
	if err != nil {
		log.Fatalf("could not create channel: %v", err)
	}

	worker := newWorker(*hostport, channel)
	server := thrift.NewServer(channel.GetSubChannel("pingpong"))
	server.Register(pingpong.NewTChanPingPongServer(worker))

	if err := channel.ListenAndServe(*hostport); err != nil {
		log.Fatalf("could not listen on hostport: %v", err)
	}

	bsopts := new(ringpop.BootstrapOptions)
	bsopts.File = *hostfile
	bsopts.Stopped = true
	if _, err := worker.ringpop.Bootstrap(bsopts); err != nil {
		log.Fatalf("could not bootstrap ringpop: %v", err)
	}

	// block
	select {}
}
