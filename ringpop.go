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

// Package ringpop brings cooperation and coordination to applications that would
// otherwise run as a set of independent worker processes.
//
// Ringpop implements a membership protocol that allows those workers to discover
// one another and use the communication channels established between them as a
// means of disseminating information, detecting failure, and ultimately converging
// on a consistent membership list. Consistent hashing is then applied on top of
// that list and gives an application the ability to define predictable behavior
// and data storage facilities within a custom keyspace. The keyspace is partitioned
// and evenly assigned to the individual instances of an application. Clients of
// the application remain simple and need not know of the underlying cooperation
// between workers nor chosen partitioning scheme. A request can be sent to any
// instance, and Ringpop intelligently forwards the request to the “correct” instance
// as defined by a hash ring lookup.
//
// Ringpop makes it possible to build extremely scalable and fault-tolerant distributed
// systems with 3 main capabilities: membership protocol, consistent hashing, and forwarding.
package ringpop

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/apache/thrift/lib/go/thrift"
	"github.com/dgryski/go-farm"
	log "github.com/uber/bark"
	"github.com/uber/ringpop-go/forward"
	"github.com/uber/ringpop-go/swim"
	"github.com/uber/ringpop-go/swim/util"
	"github.com/uber/tchannel/golang"
)

// Options to create a Ringpop with
type Options struct {
	Logger  log.Logger
	Statter log.StatsReporter
}

func defaultOptions() *Options {
	logger := log.NewLoggerFromLogrus(&logrus.Logger{
		Out: ioutil.Discard,
	})

	opts := &Options{
		Logger:  logger,
		Statter: new(noopStatsReporter),
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

	if opts.Statter == nil {
		opts.Statter = def.Statter
	}

	return opts
}

// Ringpop is a consistent hash-ring that uses a gossip prtocol to disseminate changes around the
// ring
type Ringpop struct {
	app     string
	address string

	state struct {
		read, destroyed bool
		sync.RWMutex
	}

	channel   *tchannel.SubChannel
	node      *swim.Node
	ring      *hashRing
	forwarder *forward.Forwarder

	listeners []EventListener

	statter log.StatsReporter
	stats   struct {
		hostport string
		prefix   string
		keys     map[string]string
		hooks    []string
		sync.RWMutex
	}

	logger log.Logger
	log    log.Logger

	startTime time.Time
}

// NewRingpop returns a new Ringpop instance
func NewRingpop(app, address string, channel *tchannel.Channel, opts *Options) *Ringpop {
	opts = mergeDefault(opts)

	ringpop := &Ringpop{
		app:     app,
		address: address,
		logger:  opts.Logger,
		log:     opts.Logger.WithField("local", address),
		statter: opts.Statter,
	}

	if channel != nil {
		ringpop.channel = channel.GetSubChannel("ringpop")
		ringpop.registerHandlers()
	}

	ringpop.node = swim.NewNode(app, address, ringpop.channel, &swim.Options{
		Logger: ringpop.logger,
	})
	ringpop.node.RegisterListener(ringpop)

	ringpop.ring = newHashRing(ringpop, farm.Hash32)

	ringpop.stats.hostport = genStatsHostport(ringpop.address)
	ringpop.stats.prefix = fmt.Sprintf("ringpop.%s", ringpop.stats.hostport)
	ringpop.stats.keys = make(map[string]string)

	ringpop.forwarder = forward.NewForwarder(ringpop, ringpop.channel, ringpop.logger)

	return ringpop
}

// Destroy Ringpop
func (rp *Ringpop) Destroy() {
	rp.node.Destroy()

	rp.state.Lock()
	rp.state.destroyed = true
	rp.state.Unlock()
}

// Destroyed returns
func (rp *Ringpop) Destroyed() bool {
	rp.state.Lock()
	destroyed := rp.state.destroyed
	rp.state.Unlock()

	return destroyed
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

// RegisterListener adds a listener to the ringpop. The listener's HandleEvent method
// should be thread safe
func (rp *Ringpop) RegisterListener(l EventListener) {
	rp.listeners = append(rp.listeners, l)
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	Bootstrap
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

// BootstrapOptions are used to bootstrap the ringpop
type BootstrapOptions struct {
	swim.BootstrapOptions
}

// Bootstrap starts the Ringpop
func (rp *Ringpop) Bootstrap(opts *BootstrapOptions) ([]string, error) {
	time.Sleep(10 * time.Millisecond)

	joined, err := rp.node.Bootstrap(&opts.BootstrapOptions)
	if err != nil {
		rp.log.WithField("error", err).Info("bootstrap failed")
		return nil, err
	}

	rp.log.WithField("joined", joined).Info("bootstrap complete")
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
		rp.log.WithField("key", key).Warn("could not find destination for key")
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
	rp.stats.Lock()

	psKey, ok := rp.stats.keys[sKey]
	if !ok {
		psKey = fmt.Sprintf("%s.%s", rp.stats.prefix, sKey)
		rp.stats.keys[sKey] = psKey
	}

	rp.stats.Unlock()

	switch sType {
	case "increment":
		rp.statter.IncCounter(psKey, nil, val)
	case "gauge":
		rp.statter.UpdateGauge(psKey, nil, val)
	case "timing":
		rp.statter.RecordTimer(psKey, nil, time.Duration(val))
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
func (rp *Ringpop) HandleOrForward(key string, request []byte, response *[]byte, service, endpoint string,
	format tchannel.Format, opts *forward.Options) (bool, error) {

	dest := rp.Lookup(key)
	if dest == rp.WhoAmI() {
		return true, nil
	}

	res, err := rp.forwarder.ForwardRequest(request, dest, service, endpoint, []string{key}, format, opts)
	*response = res

	return false, err
}

// SerializeThrift takes a thrift struct and returns the serialized bytes
// of that struct using the thrift binary protocol. This is a temporary
// measure before frames can forwarded directly past the endpoint to the proper
// destinaiton.
func SerializeThrift(s thrift.TStruct) ([]byte, error) {
	var b []byte
	var buffer = bytes.NewBuffer(b)

	transport := thrift.NewStreamTransportW(buffer)
	if err := s.Write(thrift.NewTBinaryProtocolTransport(transport)); err != nil {
		return nil, err
	}

	if err := transport.Flush(); err != nil {
		return nil, err
	}
	return buffer.Bytes(), nil
}

// DeserializeThrift takes a byte slice and attempts to write it into the
// given thrift struct using the thrift binary protocol. This is a temporary
// measure before frames can forwarded directly past the endpoint to the proper
// destinaiton.
func DeserializeThrift(b []byte, s thrift.TStruct) error {
	reader := bytes.NewReader(b)
	transport := thrift.NewStreamTransportR(reader)
	return s.Read(thrift.NewTBinaryProtocolTransport(transport))
}
