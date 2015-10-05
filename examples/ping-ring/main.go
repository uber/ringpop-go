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

	log "github.com/Sirupsen/logrus"
	"github.com/uber/bark"
	"github.com/uber/ringpop-go"
	"github.com/uber/tchannel/golang"
	"github.com/uber/tchannel/golang/json"
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
}

func (w *worker) RegisterPong() error {
	hmap := map[string]interface{}{"/ping": w.PingHandler}

	return json.Register(w.channel, hmap, func(ctx context.Context, err error) {
		w.logger.Debug("error occured: %v", err)
	})
}

func (w *worker) PingHandler(ctx json.Context, ping *Ping) (*Pong, error) {
	var pong Pong
	var res []byte

	handle, err := w.ringpop.HandleOrForward(ping.Key, ping.Bytes(), &res, "ping", "/ping", tchannel.JSON, nil)
	if handle {
		return &Pong{"Hello, world!", w.ringpop.WhoAmI()}, nil
	}

	if err := json2.Unmarshal(res, &pong); err != nil {
		return nil, err
	}

	// else request was forwarded
	return &pong, err
}

func main() {
	flag.Parse()

	ch, err := tchannel.NewChannel("ping", nil)
	if err != nil {
		log.Fatalf("channel did not create successfully: %v", err)
	}

	logger := log.StandardLogger()

	worker := &worker{
		channel: ch,
		ringpop: ringpop.NewRingpop("ping-app", *hostport, ch, &ringpop.Options{
			Logger: bark.NewLoggerFromLogrus(logger),
		}),
		logger: logger,
	}

	if err := worker.RegisterPong(); err != nil {
		log.Fatalf("could not register pong handler: %v", err)
	}

	if err := worker.channel.ListenAndServe(*hostport); err != nil {
		log.Fatalf("could not listen on given hostport: %v", err)
	}

	opts := new(ringpop.BootstrapOptions)
	opts.File = *hostfile

	if _, err := worker.ringpop.Bootstrap(opts); err != nil {
		log.Fatalf("ringpop bootstrap failed: %v", err)
	}

	select {}
}
