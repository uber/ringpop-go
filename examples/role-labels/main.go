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

//go:generate thrift-gen --generateThrift --outputDir gen-go --template github.com/uber/ringpop-go/ringpop.thrift-gen --inputFile role.thrift

package main

import (
	"flag"

	log "github.com/Sirupsen/logrus"
	"github.com/uber-common/bark"
	"github.com/uber/ringpop-go"
	"github.com/uber/ringpop-go/discovery/jsonfile"
	gen "github.com/uber/ringpop-go/examples/role-labels/gen-go/role"
	"github.com/uber/ringpop-go/swim"
	"github.com/uber/tchannel-go"
	"github.com/uber/tchannel-go/thrift"
)

const (
	roleKey = "role"
)

var (
	hostport = flag.String("listen", "127.0.0.1:3000", "hostport to start service on")
	hostfile = flag.String("hosts", "./hosts.json", "path to hosts file")
	rolename = flag.String("role", "master", "name of the role this node takes in the cluster")
)

type worker struct {
	ringpop *ringpop.Ringpop
	channel *tchannel.Channel
	logger  *log.Logger
}

func (w *worker) GetMembers(ctx thrift.Context, role string) ([]string, error) {
	// return empty list
	return w.ringpop.GetReachableMembers(swim.LabeledMember(roleKey, role))
}

func (w *worker) SetRole(ctx thrift.Context, role string) error {
	labels, err := w.ringpop.Labels()
	if err != nil {
		return err
	}
	labels.Set(roleKey, role)
	return nil
}

func (w *worker) RegisterRoleService() error {
	server := thrift.NewServer(w.channel)
	server.Register(gen.NewTChanRoleServiceServer(w))
	return nil
}

func main() {
	flag.Parse()

	ch, err := tchannel.NewChannel("role", nil)
	if err != nil {
		log.Fatalf("channel did not create successfully: %v", err)
	}

	logger := log.StandardLogger()

	rp, err := ringpop.New("ping-app",
		ringpop.Channel(ch),
		ringpop.Identity(*hostport),
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

	if err := worker.RegisterRoleService(); err != nil {
		log.Fatalf("could not register role service: %v", err)
	}

	if err := worker.channel.ListenAndServe(*hostport); err != nil {
		log.Fatalf("could not listen on given hostport: %v", err)
	}

	opts := new(swim.BootstrapOptions)
	opts.DiscoverProvider = jsonfile.New(*hostfile)

	log.Println("Bootstrapping")

	if _, err := worker.ringpop.Bootstrap(opts); err != nil {
		log.Fatalf("ringpop bootstrap failed: %v", err)
	}

	log.Println("Bootstrapped")

	if labels, err := worker.ringpop.Labels(); err != nil {
		log.Fatalf("unable to get access to ringpop labels: %v", err)
	} else {
		labels.Set(roleKey, *rolename)
	}

	log.Println("Started")

	select {}
}
