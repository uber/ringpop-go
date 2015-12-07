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
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/apache/thrift/lib/go/thrift"
	"github.com/dgryski/go-farm"
	log "github.com/uber-common/bark"
	"github.com/uber/ringpop-go/events"
	"github.com/uber/ringpop-go/forward"
	"github.com/uber/ringpop-go/shared"
	"github.com/uber/ringpop-go/swim"
	"github.com/uber/tchannel-go"
)

// Interface specifies the public facing methods a user of ringpop is able to
// use.
type Interface interface {
	Destroy()
	Destroyed() bool
	App() string
	WhoAmI() string
	Uptime() time.Duration
	RegisterListener(l events.EventListener)
	Bootstrap(opts *swim.BootstrapOptions) ([]string, error)
	HandleEvent(event interface{})
	Checksum() uint32
	Lookup(key string) string
	LookupN(key string, n int) []string

	HandleOrForward(key string, request []byte, response *[]byte, service, endpoint string, format tchannel.Format, opts *forward.Options) (bool, error)
	Forward(dest string, keys []string, request []byte, service, endpoint string, format tchannel.Format, opts *forward.Options) ([]byte, error)
}

// Ringpop is a consistent hash-ring that uses a gossip prtocol to disseminate changes around the
// ring
type Ringpop struct {
	config         *Configuration
	configHashRing *HashRingConfiguration

	identityResolver IdentityResolver

	state      state
	stateMutex sync.RWMutex

	channel    shared.TChannel
	subChannel shared.SubChannel
	node       *swim.Node
	ring       *hashRing
	forwarder  *forward.Forwarder

	listeners []events.EventListener

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

// state represents the internal state of a Ringpop instance.
type state uint

const (
	Created state = iota
	Initialized
	Bootstrapping
	Ready
	Destroyed
)

// New returns a new Ringpop instance!
func New(app string, opts ...Option) (*Ringpop, error) {

	var err error

	ringpop := &Ringpop{
		config: &Configuration{
			App: app,
		},
	}

	err = applyOptions(ringpop, defaultOptions)
	if err != nil {
		panic(fmt.Errorf("Error applying default Ringpop options: %v", err))
	}

	err = applyOptions(ringpop, opts)
	if err != nil {
		return nil, err
	}

	errs := checkOptions(ringpop)
	if len(errs) != 0 {
		return nil, fmt.Errorf("%v", errs)
	}

	ringpop.setState(Created)

	return ringpop, nil
}

// init configures a Ringpop instance and makes it ready to do comms.
func (rp *Ringpop) init() error {

	if rp.channel == nil {
		return errors.New("Missing channel")
	}

	address, err := rp.identity()
	if err != nil {
		return err
	}

	rp.subChannel = rp.channel.GetSubChannel("ringpop", tchannel.Isolated)
	rp.registerHandlers()

	rp.node = swim.NewNode(rp.config.App, address, rp.subChannel, &swim.Options{
		Logger: rp.logger,
	})
	rp.node.RegisterListener(rp)

	rp.ring = newHashRing(rp, farm.Fingerprint32, rp.configHashRing.ReplicaPoints)

	rp.stats.hostport = genStatsHostport(address)
	rp.stats.prefix = fmt.Sprintf("ringpop.%s", rp.stats.hostport)
	rp.stats.keys = make(map[string]string)

	rp.forwarder = forward.NewForwarder(rp, rp.subChannel, rp.logger)

	rp.setState(Initialized)

	return nil
}

// identity returns a host:port string of the address that Ringpop should
// use as its identifier.
func (rp *Ringpop) identity() (string, error) {
	return rp.identityResolver()
}

// r.channelIdentityResolver resolves the hostport identity from the current
// TChannel object on the Ringpop instance.
func (rp *Ringpop) channelIdentityResolver() (string, error) {
	hostport := rp.channel.PeerInfo().HostPort
	// Check that TChannel is listening. By default, TChannel listens on an
	// ephemeral host/port. The real port is then assigned by the OS when
	// ListenAndService is called. If the hostport is 0.0.0.0:0, it means
	// TChannel is not yet listening and the hostport cannot be resolved.
	if hostport == "0.0.0.0:0" {
		return "", fmt.Errorf("unable to resolve valid listen address (TChannel hostport is %s)", hostport)
	}
	return hostport, nil
}

// Destroy Ringpop
func (rp *Ringpop) Destroy() {

	// Only destroy the node if it has been created
	if rp.getState() > Created {
		rp.node.Destroy()
	}

	rp.setState(Destroyed)
}

// Destroyed returns
func (rp *Ringpop) Destroyed() bool {
	return rp.getState() == Destroyed
}

// App returns the app the ringpop belongs to
func (rp *Ringpop) App() string {
	return rp.config.App
}

// WhoAmI returns the address of the current/local Ringpop node. It returns an
// error if Ringpop is not yet initialised/bootstrapped.
func (rp *Ringpop) WhoAmI() (string, error) {
	if !rp.Ready() {
		return "", ErrNotBootstrapped
	}
	address, _ := rp.identity()
	return address, nil
}

// Uptime returns the amount of time that the ringpop has been running for
func (rp *Ringpop) Uptime() (time.Duration, error) {
	if !rp.Ready() {
		return 0, ErrNotBootstrapped
	}
	return time.Now().Sub(rp.startTime), nil
}

func (rp *Ringpop) emit(event interface{}) {
	for _, listener := range rp.listeners {
		go listener.HandleEvent(event)
	}
}

// RegisterListener adds a listener to the ringpop. The listener's HandleEvent method
// should be thread safe
func (rp *Ringpop) RegisterListener(l events.EventListener) {
	rp.listeners = append(rp.listeners, l)
}

// getState gets the state of the current Ringpop instance.
func (rp *Ringpop) getState() state {
	rp.stateMutex.Lock()
	defer rp.stateMutex.Unlock()
	return rp.state
}

// setState sets the state of the current Ringpop instance.
func (rp *Ringpop) setState(s state) {
	rp.stateMutex.Lock()
	rp.state = s
	rp.stateMutex.Unlock()
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	Bootstrap
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

// Bootstrap starts the Ringpop
func (rp *Ringpop) Bootstrap(opts *swim.BootstrapOptions) ([]string, error) {

	if rp.getState() < Initialized {
		rp.init()
	}

	rp.setState(Bootstrapping)

	joined, err := rp.node.Bootstrap(opts)
	if err != nil {
		rp.log.WithField("error", err).Info("bootstrap failed")
		rp.setState(Initialized)
		return nil, err
	}

	rp.setState(Ready)

	rp.log.WithField("joined", joined).Info("bootstrap complete")
	return joined, nil
}

// Ready returns whether or not ringpop is bootstrapped and should receive
// requests
func (rp *Ringpop) Ready() bool {
	if rp.state != Ready {
		return false
	}
	return rp.node.Ready()
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
		for _, change := range event.Changes {
			status := change.Status
			if len(status) == 0 {
				status = "unknown"
			}
			rp.statter.IncCounter(rp.getStatKey("membership-update."+status), nil, 1)
		}

	case swim.MemberlistChangesAppliedEvent:
		rp.statter.UpdateGauge(rp.getStatKey("changes.apply"), nil, int64(len(event.Changes)))
		rp.handleChanges(event.Changes)

	case swim.FullSyncEvent:
		rp.statter.IncCounter(rp.getStatKey("full-sync"), nil, 1)

	case swim.MaxPAdjustedEvent:
		rp.statter.UpdateGauge(rp.getStatKey("max-p"), nil, int64(event.NewPCount))

	case swim.JoinReceiveEvent:
		rp.statter.IncCounter(rp.getStatKey("join.recv"), nil, 1)

	case swim.JoinCompleteEvent:
		rp.statter.IncCounter(rp.getStatKey("join.complete"), nil, 1)
		rp.statter.RecordTimer(rp.getStatKey("join"), nil, event.Duration)

	case swim.PingSendEvent:
		rp.statter.IncCounter(rp.getStatKey("ping.send"), nil, 1)

	case swim.PingSendCompleteEvent:
		rp.statter.RecordTimer(rp.getStatKey("ping"), nil, event.Duration)

	case swim.PingReceiveEvent:
		rp.statter.IncCounter(rp.getStatKey("ping.recv"), nil, 1)

	case swim.PingRequestsSendEvent:
		rp.statter.IncCounter(rp.getStatKey("ping-req.send"), nil, int64(len(event.Peers)))

	case swim.PingRequestsSendCompleteEvent:
		rp.statter.RecordTimer(rp.getStatKey("ping-req"), nil, event.Duration)

	case swim.PingRequestReceiveEvent:
		rp.statter.IncCounter(rp.getStatKey("ping-req.recv"), nil, 1)

	case swim.PingRequestPingEvent:
		rp.statter.RecordTimer(rp.getStatKey("ping-req.ping"), nil, event.Duration)

	case swim.ProtocolDelayComputeEvent:
		rp.statter.RecordTimer(rp.getStatKey("protocol.delay"), nil, event.Duration)

	case swim.ProtocolFrequencyEvent:
		rp.statter.RecordTimer(rp.getStatKey("protocol.frequency"), nil, event.Duration)

	case swim.ChecksumComputeEvent:
		rp.statter.RecordTimer(rp.getStatKey("compute-checksum"), nil, event.Duration)
		rp.statter.UpdateGauge(rp.getStatKey("checksum"), nil, int64(event.Checksum))
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
func (rp *Ringpop) Checksum() (uint32, error) {
	if !rp.Ready() {
		return 0, ErrNotBootstrapped
	}
	return rp.ring.Checksum(), nil
}

// Lookup returns the address of the server in the ring that is responsible
// for the specified key. It returns an error if the Ringpop instance is not
// yet initialised/bootstrapped.
func (rp *Ringpop) Lookup(key string) (string, error) {

	if !rp.Ready() {
		return "", ErrNotBootstrapped
	}

	startTime := time.Now()

	dest, ok := rp.ring.Lookup(key)

	rp.emit(events.LookupEvent{key, time.Now().Sub(startTime)})

	if !ok {
		identity, _ := rp.WhoAmI()
		err := errors.New("could not find destination for key")
		rp.log.WithField("key", key).Warn(err)
		return identity, err
	}

	return dest, nil
}

// LookupN hashes a key to N servers in the ring
func (rp *Ringpop) LookupN(key string, n int) ([]string, error) {
	if !rp.Ready() {
		return nil, ErrNotBootstrapped
	}
	return rp.ring.LookupN(key, n), nil
}

func (rp *Ringpop) ringEvent(e interface{}) {
	rp.emit(e)

	switch e := e.(type) {
	case events.RingChecksumEvent:
		rp.statter.IncCounter(rp.getStatKey("ring.checksum-computed"), nil, 1)

	case events.RingChangedEvent:
		added := int64(len(e.ServersAdded))
		removed := int64(len(e.ServersRemoved))
		rp.statter.IncCounter(rp.getStatKey("ring.server-added"), nil, added)
		rp.statter.IncCounter(rp.getStatKey("ring.server-removed"), nil, removed)
	}
}

func (rp *Ringpop) GetReachableMembers() ([]string, error) {
	if !rp.Ready() {
		return nil, ErrNotBootstrapped
	}
	return rp.node.GetReachableMembers(), nil
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	Stats
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func (rp *Ringpop) getStatKey(key string) string {
	rp.stats.Lock()
	rpKey, ok := rp.stats.keys[key]
	if !ok {
		rpKey = fmt.Sprintf("%s.%s", rp.stats.prefix, key)
		rp.stats.keys[key] = rpKey
	}
	rp.stats.Unlock()

	return rpKey
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

	if !rp.Ready() {
		return false, ErrNotBootstrapped
	}

	dest, err := rp.Lookup(key)
	if err != nil {
		return false, err
	}

	identity, err := rp.WhoAmI()
	if err != nil {
		return false, err
	}

	if dest == identity {
		return true, nil
	}

	res, err := rp.Forward(dest, []string{key}, request, service, endpoint, format, opts)
	*response = res

	return false, err
}

// Forward forwards the request to given destination host and returns the response.
func (rp *Ringpop) Forward(dest string, keys []string, request []byte, service, endpoint string,
	format tchannel.Format, opts *forward.Options) ([]byte, error) {

	return rp.forwarder.ForwardRequest(request, dest, service, endpoint, keys, format, opts)
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
