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
	"strings"

	log "github.com/Sirupsen/logrus"
	"github.com/uber-common/bark"
	"github.com/uber/ringpop-go"
	"github.com/uber/tchannel-go"
	"github.com/uber/tchannel-go/json"
	"golang.org/x/net/context"
)

var (
	attach    = flag.Bool("attach", false, "set this to true to just attach to the ring and not bootstrap")
	boothosts = flag.String("boothosts", "127.0.0.1:3000,127.0.0.1:3001,127.0.0.1:3002", "comma separated list of hostports to bootstrap ringpop on or attach to")
	hostport  = flag.String("hostport", "127.0.0.1:3000", "hostport to listen for ring changes")
)

type worker struct {
	ringpop *ringpop.Ringpop
	channel *tchannel.Channel
	logger  *log.Logger
	hosts   []string
}

// Lookup is the request
type Lookup struct {
	Key      string `json:"key"`
	Taphost  string `json:"taphost"`
	Replicas int    `json:"replicas"`
}

// Result returns the servers
type Result struct {
	Servers string `json:"servers"`
}

func (w *worker) RegisterLookup() error {
	hmap := map[string]interface{}{"/lookup": w.LookupHandler}

	return json.Register(w.channel, hmap, func(ctx context.Context, err error) {
		w.logger.Debug("error occured: %v", err)
	})
}

func (w *worker) LookupHandler(ctx json.Context, look *Lookup) (*Result, error) {
	var servers []string
	log.Infof("tapping host: %v: remote ring: %v", look.Taphost, "parent-app")
	err := w.ringpop.TapRing(look.Taphost, "parent-app")
	if err == nil {
		servers = w.ringpop.LookupN(look.Key, look.Replicas)
	}
	singleServer := strings.Join(servers, ",")
	log.Infof("lookup returning: %v", singleServer)

	return &Result{
		Servers: singleServer,
	}, nil
}

func InitWorker(app string, ch *tchannel.Channel, hostport string) *worker {
	logger := log.StandardLogger()
	worker := &worker{
		channel: ch,
		ringpop: ringpop.NewRingpop(app, hostport, ch, &ringpop.Options{
			Logger: bark.NewLoggerFromLogrus(logger),
		}),
		logger: logger,
	}
	if err := worker.channel.ListenAndServe(hostport); err != nil {
		log.Fatalf("could not listen on given hostport: %v", err)
	}
	return worker
}

func main() {
	flag.Parse()

	hosts := []string{}
	for _, item := range strings.Split(*boothosts, ",") {
		hosts = append(hosts, item)
	}

	var worker *worker
	if !*attach {
		ch, _ := tchannel.NewChannel("parentring", nil)
		worker = InitWorker("parent-app", ch, *hostport)
		opts := new(ringpop.BootstrapOptions)
		opts.Hosts = hosts

		if _, err := worker.ringpop.Bootstrap(opts); err != nil {
			log.Fatalf("ringpop bootstrap failed: %v", err)
		}
	} else {
		// now the original ring is bootstrapped.. setup the listen ring
		listenCh, _ := tchannel.NewChannel("listenring", nil)
		worker = InitWorker("listen-app", listenCh, *hostport)

		worker.hosts = hosts
		log.Infof("registering lookup handler for host: %v", *hostport)
		if err := worker.RegisterLookup(); err != nil {
			log.Fatalf("could not register lookup handler: %v", err)
		}
	}

	select {}
}
