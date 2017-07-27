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
	json2 "encoding/json"
	"flag"

	log "github.com/sirupsen/logrus"
	"github.com/uber-common/bark"
	"github.com/uber/ringpop-go"
	"github.com/uber/ringpop-go/discovery/jsonfile"
	"github.com/uber/ringpop-go/forward"
	"github.com/uber/ringpop-go/swim"
	"github.com/uber/tchannel-go"
	"github.com/uber/tchannel-go/json"
	"golang.org/x/net/context"
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

// Ping is a request
type Ping struct {
	Key string `json:"key"`
}

// Bytes returns the byets for a ping
func (p Ping) Bytes() []byte {
	data, _ := json2.Marshal(p)
	return data
}

// Pong is a ping response
type Pong struct {
	Message string `json:"message"`
	From    string `json:"from"`
	PHeader string `json:"pheader"`
}

func (w *worker) RegisterPing() error {
	hmap := map[string]interface{}{"/ping": w.PingHandler}

	return json.Register(w.channel, hmap, func(ctx context.Context, err error) {
		w.logger.Debug("error occured: %v", err)
	})
}

func (w *worker) PingHandler(ctx json.Context, request *Ping) (*Pong, error) {
	var pong Pong
	var res []byte

	headers := ctx.Headers()
	marshaledHeaders, err := json2.Marshal(ctx.Headers())
	if err != nil {
		return nil, err
	}
	forwardOptions := &forward.Options{Headers: []byte(marshaledHeaders)}
	handle, err := w.ringpop.HandleOrForward(request.Key, request.Bytes(), &res, "pingchannel", "/ping", tchannel.JSON, forwardOptions)
	if handle {
		address, err := w.ringpop.WhoAmI()
		if err != nil {
			return nil, err
		}
		return &Pong{"Hello, world!", address, headers["p"]}, nil
	}

	if err := json2.Unmarshal(res, &pong); err != nil {
		return nil, err
	}

	// else request was forwarded
	return &pong, err
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
