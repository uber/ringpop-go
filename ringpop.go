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

package ringpop

import (
	"fmt"
	"io/ioutil"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/dgryski/go-farm"
	"github.com/quipo/statsd"
	"github.com/uber/ringpop-go/forward"
	"github.com/uber/ringpop-go/swim"
	"github.com/uber/ringpop-go/swim/util"
	"github.com/uber/tchannel/golang"
)

// Options to create a Ringpop with
type Options struct {
	Logger *log.Logger
	Stats  Stats
}

func defaultOptions() *Options {
	opts := &Options{
		Logger: &log.Logger{
			Out: ioutil.Discard,
		},
		Stats: statsd.NoopClient{},
	}

	return opts
}

func mergeDefault(opts *Options) *Options {
	def := defaultOptions()

	if opts == nil {
		return def
	}

	if opts.Logger == nil {
		opts.Logger = def.Logger
	}

	if opts.Stats == nil {
		opts.Stats = def.Stats
	}

	return opts
}

// Ringpop is a consistent hash-ring that uses a gossip prtocol to disseminate changes around the
// ring
type Ringpop struct {
	app     string
	address string

	ready, destroyed bool
	// protects ready, destroyed
	l sync.RWMutex

	channel *tchannel.SubChannel
	node    *swim.Node
	ring    *hashRing

	listeners []EventListener

	logger       *log.Logger
	stats        Stats
	statHostport string
	statPrefix   string
	statKeys     map[string]string
	statHooks    []string   // type ?
	sl           sync.Mutex // protects stat keys

	forwarder *forward.Forwarder

	startTime time.Time
}

// NewRingpop returns a new Ringpop instance
func NewRingpop(app, address string, channel *tchannel.Channel, opts *Options) *Ringpop {
	opts = mergeDefault(opts)

	ringpop := &Ringpop{
		app:      app,
		address:  address,
		logger:   opts.Logger,
		stats:    opts.Stats,
		statKeys: make(map[string]string),
	}

	if channel != nil {
		ringpop.channel = channel.GetSubChannel("ringpop")
		ringpop.registerHandlers()
	}

	ringpop.node = swim.NewNode(app, address, ringpop.channel, &swim.Options{
		Logger: opts.Logger,
	})
	ringpop.node.RegisterListener(ringpop)

	ringpop.ring = newHashRing(ringpop, farm.Hash32)

	ringpop.statHostport = strings.Replace(ringpop.address, ".", "_", -1)
	ringpop.statHostport = strings.Replace(ringpop.statHostport, ":", "_", -1)
	ringpop.statPrefix = fmt.Sprintf("ringpop.%s", ringpop.statHostport)

	ringpop.forwarder = forward.NewForwarder(ringpop, ringpop.channel, ringpop.logger)

	return ringpop
}

// Destroy Ringpop
func (rp *Ringpop) Destroy() {
	rp.l.Lock()
	defer rp.l.Unlock()

	rp.node.Destroy()

	rp.destroyed = true
}

// Destroyed returns
func (rp *Ringpop) Destroyed() bool {
	rp.l.Lock()
	defer rp.l.Unlock()

	return rp.destroyed
}

// App returns the app the ringpop belongs to
func (rp *Ringpop) App() string {
	return rp.app
}

// WhoAmI returns the local address of the Ringpop node
func (rp *Ringpop) WhoAmI() string {
	return rp.address
}

// Uptime returns the amount of time that the ringpop has been running for
func (rp *Ringpop) Uptime() time.Duration {
	return time.Now().Sub(rp.startTime)
}

func (rp *Ringpop) emit(event interface{}) {
	for _, listener := range rp.listeners {
		go listener.HandleEvent(event)
	}
}

// RegisterListener adds a listener to the ringpop
func (rp *Ringpop) RegisterListener(l EventListener) {
	rp.listeners = append(rp.listeners, l)
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	Bootstrap
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

// BootstrapOptions used to bootstrap the ringpop with
type BootstrapOptions struct {
	swim.BootstrapOptions
}

// Bootstrap starts the Ringpop
func (rp *Ringpop) Bootstrap(opts *BootstrapOptions) ([]string, error) {
	time.Sleep(10 * time.Millisecond)

	joined, err := rp.node.Bootstrap(&opts.BootstrapOptions)
	if err != nil {
		rp.logger.WithFields(log.Fields{
			"local": rp.WhoAmI(),
			"error": err,
		}).Info("bootstrap failed")
		return nil, err
	}

	rp.logger.WithFields(log.Fields{
		"local":  rp.WhoAmI(),
		"joined": joined,
	}).Info("bootstrap complete")

	return joined, nil
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	SWIM Events
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

// HandleEvent is used to satisfy the swim.EventListener interface. No touchy.
func (rp *Ringpop) HandleEvent(event interface{}) {
	rp.emit(event)

	switch event := event.(type) {
	case swim.MemberlistChangesReceivedEvent:
		// TODO: stat

	case swim.MemberlistChangesAppliedEvent:
		rp.stat("gauge", "changes.apply", int64(len(event.Changes)))
		rp.handleChanges(event.Changes)

	case swim.FullSyncEvent:
		rp.stat("increment", "full-sync", 1)

	case swim.MaxPAdjustedEvent:
		rp.stat("gauge", "max-p", int64(event.NewPCount))

	case swim.JoinReceiveEvent:
		rp.stat("increment", "join.recv", 1)

	case swim.JoinCompleteEvent:
		rp.stat("increment", "join.complete", 1)
		rp.stat("timing", "join", util.MS(event.Duration))

	case swim.PingSendEvent:
		rp.stat("increment", "ping.send", 1)

	case swim.PingReceiveEvent:
		rp.stat("increment", "ping.recv", 1)

	case swim.PingRequestsSendEvent:
		rp.stat("increment", "ping-req.send", int64(len(event.Peers)))
		rp.stat("increment", "ping-req.other-members", int64(len(event.Peers)))

	case swim.PingRequestReceiveEvent:
		rp.stat("increment", "ping-req.recv", 1)

	case swim.PingRequestPingEvent:
		rp.stat("timing", "ping-req.ping", util.MS(event.Duration))

	default:
		// do nothing
	}
}

func (rp *Ringpop) handleChanges(changes []swim.Change) {
	var serversToAdd, serversToRemove []string

	for _, change := range changes {
		switch change.Status {
		case swim.Alive:
			serversToAdd = append(serversToAdd, change.Address)
		case swim.Faulty, swim.Leave:
			serversToRemove = append(serversToRemove, change.Address)
		}
	}

	rp.ring.AddRemoveServers(serversToAdd, serversToRemove)
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	Ring
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

// Checksum returns the checksum of the ringpop's hashring
func (rp *Ringpop) Checksum() uint32 {
	return rp.ring.Checksum()
}

// Lookup hashes a key to a server in the ring
func (rp *Ringpop) Lookup(key string) string {
	startTime := time.Now()

	dest, ok := rp.ring.Lookup(key)

	rp.emit(LookupEvent{key, time.Now().Sub(startTime)})

	if !ok {
		rp.logger.WithFields(log.Fields{
			"local": rp.WhoAmI(),
			"key":   key,
		}).Error("could not find destination for key")

		return rp.WhoAmI()
	}

	return dest
}

// TODO: LookupN

func (rp *Ringpop) ringEvent(event interface{}) {
	rp.emit(event)

	switch event := event.(type) {
	case RingChecksumEvent:
		rp.stat("increment", "ring.checksum-computed", 1)

	case RingChangedEvent:
		rp.stat("increment", "ring.server-added", int64(len(event.ServersAdded)))
		rp.stat("increment", "ring.server-added", int64(len(event.ServersAdded)))
	}

}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	Stats
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

// Stats is an interface for where ringpop should send statistics
type Stats interface {
	Incr(key string, val int64) error
	Gauge(key string, val int64) error
	Timing(key string, val int64) error
}

func (rp *Ringpop) stat(sType, sKey string, val int64) {
	rp.sl.Lock()
	defer rp.sl.Unlock()

	psKey, ok := rp.statKeys[sKey]
	if !ok {
		psKey = fmt.Sprintf("%s.%s", rp.statPrefix, sKey)
		rp.statKeys[sKey] = psKey
	}

	switch sType {
	case "increment":
		rp.stats.Incr(psKey, val)
	case "gauge":
		rp.stats.Gauge(psKey, val)
	case "timing":
		rp.stats.Timing(psKey, val)
	}
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	Forwarding
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

// HandleOrForward returns true if the request should be handled locally, or false
// if it should be forwarded to a different node. If false is returned, forwarding
// is taken care of internally by the method, and, if no error has occured, the
// response is written in the provided response field.
func (rp *Ringpop) HandleOrForward(key string, request, response interface{},
	service, endpoint string, opts *forward.Options) (bool, error) {

	dest := rp.Lookup(key)
	if dest == rp.WhoAmI() {
		return true, nil
	}

	// else forward request
	err := rp.forwarder.ForwardRequest(request, response, dest, service, endpoint, []string{key}, opts)
	return false, err
}
