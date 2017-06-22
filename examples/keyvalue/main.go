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

//go:generate thrift-gen --generateThrift --outputDir gen-go --template github.com/uber/ringpop-go/ringpop.thrift-gen --inputFile keyvalue.thrift

package main

import (
	"flag"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/uber-common/bark"
	"github.com/uber/ringpop-go"
	"github.com/uber/ringpop-go/discovery/jsonfile"
	gen "github.com/uber/ringpop-go/examples/keyvalue/gen-go/keyvalue"
	"github.com/uber/ringpop-go/swim"
	"github.com/uber/tchannel-go"
	"github.com/uber/tchannel-go/thrift"
)

var (
	hostport = flag.String("listen", "127.0.0.1:3000", "hostport to start service on")
	hostfile = flag.String("hosts", "./hosts.json", "path to hosts file")
)

type worker struct {
	adapter gen.TChanKeyValueService

	memoryLock sync.RWMutex
	memory     map[string]string
}

func (w *worker) Set(ctx thrift.Context, key string, value string) error {
	log.Printf("setting key %q to %q", key, value)

	w.memoryLock.Lock()
	defer w.memoryLock.Unlock()

	if w.memory == nil {
		w.memory = make(map[string]string)
	}
	w.memory[key] = value

	return nil
}

func (w *worker) Get(ctx thrift.Context, key string) (string, error) {
	log.Printf("getting key %q", key)

	w.memoryLock.RLock()
	defer w.memoryLock.RUnlock()

	value := w.memory[key]

	return value, nil
}

func (w *worker) GetAll(ctx thrift.Context, keys []string) ([]string, error) {
	log.Printf("getting keys: %v", keys)

	m := make([]string, 0, len(keys))
	for _, key := range keys {
		// sharded self call
		value, err := w.adapter.Get(ctx, key)
		if err != nil {
			return nil, err
		}
		m = append(m, value)
	}
	return m, nil
}

func main() {
	flag.Parse()

	ch, err := tchannel.NewChannel("keyvalue", nil)
	if err != nil {
		log.Fatalf("channel did not create successfully: %v", err)
	}

	logger := log.StandardLogger()

	rp, err := ringpop.New("keyvalue",
		ringpop.Channel(ch),
		ringpop.Logger(bark.NewLoggerFromLogrus(logger)),
	)
	if err != nil {
		log.Fatalf("Unable to create Ringpop: %v", err)
	}

	worker := &worker{}
	adapter, _ := gen.NewRingpopKeyValueServiceAdapter(worker, rp, ch,
		gen.KeyValueServiceConfiguration{
			Get: &gen.KeyValueServiceGetConfiguration{
				Key: func(ctx thrift.Context, key string) (shardKey string, err error) {
					return key, nil
				},
			},

			GetAll: &gen.KeyValueServiceGetAllConfiguration{
				Key: func(ctx thrift.Context, keys []string) (shardKey string, err error) {
					// use the node listening on 127.0.0.1:3004 as the node that should answer
					return "127.0.0.1:30040", nil
				},
			},

			Set: &gen.KeyValueServiceSetConfiguration{
				Key: func(ctx thrift.Context, key string, value string) (shardKey string, err error) {
					return key, nil
				},
			},
		},
	)
	worker.adapter = adapter

	// register sharded endpoits
	thrift.NewServer(ch).Register(gen.NewTChanKeyValueServiceServer(adapter))

	if err := ch.ListenAndServe(*hostport); err != nil {
		log.Fatalf("could not listen on given hostport: %v", err)
	}

	bootstrapOpts := &swim.BootstrapOptions{
		DiscoverProvider: jsonfile.New(*hostfile),
	}
	if _, err := rp.Bootstrap(bootstrapOpts); err != nil {
		log.Fatalf("ringpop bootstrap failed: %v", err)
	}

	select {}
}
