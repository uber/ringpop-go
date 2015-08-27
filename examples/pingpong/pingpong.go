package main

import (
	"bytes"
	"flag"
	"log"

	"github.com/Sirupsen/logrus"
	athrift "github.com/apache/thrift/lib/go/thrift"
	"github.com/uber/bark"
	"github.com/uber/ringpop-go"
	"github.com/uber/ringpop-go/examples/pingpong/gen-go/pingpong"
	"github.com/uber/tchannel/golang"
	"github.com/uber/tchannel/golang/thrift"
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

func serialize(s athrift.TStruct) ([]byte, error) {
	var b []byte
	var buffer = bytes.NewBuffer(b)

	transport := athrift.NewStreamTransportW(buffer)
	err := s.Write(athrift.NewTBinaryProtocolTransport(transport))
	if err = transport.Flush(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), err
}

func deserialize(b []byte, s athrift.TStruct) error {
	reader := bytes.NewReader(b)

	transport := athrift.NewStreamTransportR(reader)
	return s.Read(athrift.NewTBinaryProtocolTransport(transport))
}

func (w *worker) Ping(ctx thrift.Context, request *pingpong.Ping) (*pingpong.Pong, error) {
	var req, res []byte
	var err error

	if req, err = serialize(&pingpong.PingArgs{Request: request}); err != nil {
		return nil, err
	}

	handle, err := w.ringpop.HandleOrForward(request.Key, req, &res, "pingpong", "PingPong::Ping",
		tchannel.Thrift, nil)
	if handle {
		return &pingpong.Pong{From: w.address}, nil
	}

	// couldn't forward request
	if err != nil {
		return nil, err
	}

	var pongResult pingpong.PingResult
	if err := deserialize(res, &pongResult); err != nil {
		return nil, err
	}

	return pongResult.GetSuccess(), nil
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
	if _, err := worker.ringpop.Bootstrap(bsopts); err != nil {
		log.Fatalf("could not bootstrap ringpop: %v", err)
	}

	select {}
}
