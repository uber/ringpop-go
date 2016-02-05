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

// Package ringpop is a library that maintains a consistent hash ring atop a
// gossip-based membership protocol. It can be used by applications to
// arbitrarily shard data in a scalable and fault-tolerant manner.
package ringpop

import (
	"bytes"
	"errors"
	"fmt"
	"sync"
	"time"

	athrift "github.com/apache/thrift/lib/go/thrift"
	"github.com/benbjohnson/clock"
	"github.com/dgryski/go-farm"
	log "github.com/uber-common/bark"
	"github.com/uber/ringpop-go/events"
	"github.com/uber/ringpop-go/forward"
	"github.com/uber/ringpop-go/hashring"
	"github.com/uber/ringpop-go/logging"
	"github.com/uber/ringpop-go/shared"
	"github.com/uber/ringpop-go/swim"
	"github.com/uber/ringpop-go/util"
	"github.com/uber/tchannel-go"
)

// Interface specifies the public facing methods a user of ringpop is able to
// use.
type Interface interface {
	Destroy()
	App() string
	WhoAmI() (string, error)
	Uptime() (time.Duration, error)
	RegisterListener(l events.EventListener)
	Bootstrap(opts *swim.BootstrapOptions) ([]string, error)
	Checksum() (uint32, error)
	Lookup(key string) (string, error)
	LookupN(key string, n int) ([]string, error)
	GetReachableMembers() ([]string, error)
	CountReachableMembers() (int, error)

	HandleOrForward(key string, request []byte, response *[]byte, service, endpoint string, format tchannel.Format, opts *forward.Options) (bool, error)
	Forward(dest string, keys []string, request []byte, service, endpoint string, format tchannel.Format, opts *forward.Options) ([]byte, error)
}

// Ringpop is a consistent hashring that uses a gossip protocol to disseminate
// changes around the ring.
type Ringpop struct {
	config         *configuration
	configHashRing *hashring.Configuration

	identityResolver IdentityResolver

	state      state
	stateMutex sync.RWMutex

	clock clock.Clock

	channel    shared.TChannel
	subChannel shared.SubChannel
	node       swim.NodeInterface
	ring       *hashring.HashRing
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

	tickers   chan *clock.Ticker
	startTime time.Time
}

// state represents the internal state of a Ringpop instance.
type state uint

const (
	// created means the Ringpop instance has been created but the swim node,
	// stats and hasring haven't been set up. The listen address has not been
	// resolved yet either.
	created state = iota
	// initialized means the listen address has been resolved and the swim
	// node, stats and hashring have been instantiated onto the Ringpop
	// instance.
	initialized
	// ready means Bootstrap has been called, the ring has successfully
	// bootstrapped and is now ready to receive requests.
	ready
	// destroyed means the Ringpop instance has been shut down, is no longer
	// ready for requests and cannot be revived.
	destroyed
)

// New returns a new Ringpop instance.
func New(app string, opts ...Option) (*Ringpop, error) {
	var err error

	ringpop := &Ringpop{
		config: &configuration{
			App: app,
		},
		logger: logging.Logger("ringpop"),
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

	ringpop.setState(created)

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
		Clock: rp.clock,
	})
	rp.node.RegisterListener(rp)

	rp.ring = hashring.New(farm.Fingerprint32, rp.configHashRing.ReplicaPoints)
	rp.ring.RegisterListener(rp)

	rp.stats.hostport = genStatsHostport(address)
	rp.stats.prefix = fmt.Sprintf("ringpop.%s", rp.stats.hostport)
	rp.stats.keys = make(map[string]string)

	rp.forwarder = forward.NewForwarder(rp, rp.subChannel)
	rp.forwarder.RegisterListener(rp)

	rp.startTimers()
	rp.setState(initialized)

	return nil
}

// Starts periodic timers in a single goroutine. Can be turned back off via
// stopTimers. At present, only 1 timer exists, to emit ring.checksum-periodic.
func (rp *Ringpop) startTimers() {
	if rp.tickers != nil {
		return
	}
	rp.tickers = make(chan *clock.Ticker, 1) // 1 == max number of tickers

	if rp.config.RingChecksumStatPeriod != RingChecksumStatPeriodNever {
		ticker := rp.clock.Ticker(rp.config.RingChecksumStatPeriod)
		rp.tickers <- ticker
		go func() {
			for _ = range ticker.C {
				rp.statter.UpdateGauge(
					rp.getStatKey("ring.checksum-periodic"),
					nil,
					int64(rp.ring.Checksum()))
			}
		}()
	}
}

func (rp *Ringpop) stopTimers() {
	if rp.tickers != nil {
		close(rp.tickers)
		for ticker := range rp.tickers {
			ticker.Stop()
		}
		rp.tickers = nil
	}
}

// identity returns a host:port string of the address that Ringpop should
// use as its identifier.
func (rp *Ringpop) identity() (string, error) {
	return rp.identityResolver()
}

// r.channelIdentityResolver resolves the hostport identity from the current
// TChannel object on the Ringpop instance.
func (rp *Ringpop) channelIdentityResolver() (string, error) {
	peerInfo := rp.channel.PeerInfo()
	// Check that TChannel is listening on a real hostport. By default,
	// TChannel listens on an ephemeral host/port. The real port is then
	// assigned by the OS when ListenAndServe is called. If the hostport is
	// ephemeral, it means TChannel is not yet listening and the hostport
	// cannot be resolved.
	if peerInfo.IsEphemeralHostPort() {
		return "", ErrEphemeralIdentity
	}
	return peerInfo.HostPort, nil
}

// Destroy stops all communication. Note that this does not close the TChannel
// instance that was passed to Ringpop in the constructor. Once an instance is
// destroyed, it cannot be restarted.
func (rp *Ringpop) Destroy() {
	if rp.node != nil {
		rp.node.Destroy()
	}

	rp.stopTimers()

	rp.setState(destroyed)
}

// destroyed returns
func (rp *Ringpop) destroyed() bool {
	return rp.getState() == destroyed
}

// App returns the name of the application this Ringpop instance belongs to.
// The application name is set in the constructor when the Ringpop instance is
// created.
func (rp *Ringpop) App() string {
	return rp.config.App
}

// WhoAmI returns the address of the current/local Ringpop node. It returns an
// error if Ringpop is not yet initialized/bootstrapped.
func (rp *Ringpop) WhoAmI() (string, error) {
	if !rp.Ready() {
		return "", ErrNotBootstrapped
	}
	return rp.identity()
}

// Uptime returns the amount of time that this Ringpop instance has been
// bootstrapped for.
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
// should be thread safe.
func (rp *Ringpop) RegisterListener(l events.EventListener) {
	rp.listeners = append(rp.listeners, l)
}

// getState gets the state of the current Ringpop instance.
func (rp *Ringpop) getState() state {
	rp.stateMutex.RLock()
	r := rp.state
	rp.stateMutex.RUnlock()
	return r
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

// Bootstrap starts communication for this Ringpop instance.
//
// When Bootstrap is called, this Ringpop instance will attempt to contact
// other instances from a seed list provided either in the BootstrapOptions or
// as a JSON file.
//
// If no seed hosts are provided, a single-node cluster will be created.
func (rp *Ringpop) Bootstrap(userBootstrapOpts *swim.BootstrapOptions) ([]string, error) {
	if rp.getState() < initialized {
		err := rp.init()
		if err != nil {
			return nil, err
		}
	}

	identity, err := rp.identity()
	if err != nil {
		return nil, err
	}

	// If the user has provided a list of hosts (and not a bootstrap file),
	// check we're in the bootstrap host list and add ourselves if we're not
	// there. If the host list is empty, this will create a single-node
	// cluster.
	bootstrapOpts := *userBootstrapOpts
	if len(bootstrapOpts.File) == 0 && !util.StringInSlice(bootstrapOpts.Hosts, identity) {
		bootstrapOpts.Hosts = append(bootstrapOpts.Hosts, identity)
	}

	joined, err := rp.node.Bootstrap(&bootstrapOpts)
	if err != nil {
		rp.logger.WithField("error", err).Info("bootstrap failed")
		rp.setState(initialized)
		return nil, err
	}

	rp.setState(ready)

	rp.logger.WithField("joined", joined).Info("bootstrap complete")
	return joined, nil
}

// Ready returns whether or not ringpop is bootstrapped and ready to receive
// requests.
func (rp *Ringpop) Ready() bool {
	if rp.getState() != ready {
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
func (rp *Ringpop) HandleEvent(event events.Event) {
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
		for _, change := range event.Changes {
			status := change.Status
			if len(status) == 0 {
				status = "unknown"
			}
			rp.statter.IncCounter(rp.getStatKey("membership-set."+status), nil, 1)
		}
		mc, err := rp.CountReachableMembers()
		if err != nil {
			rp.logger.Errorf("unable to count members of the ring for statting: %q", err)
		} else {
			rp.statter.UpdateGauge(rp.getStatKey("num-members"), nil, int64(mc))
		}
		rp.statter.IncCounter(rp.getStatKey("updates"), nil, int64(len(event.Changes)))

	case swim.FullSyncEvent:
		rp.statter.IncCounter(rp.getStatKey("full-sync"), nil, 1)

	case swim.MaxPAdjustedEvent:
		rp.statter.UpdateGauge(rp.getStatKey("max-piggyback"), nil, int64(event.NewPCount))

	case swim.JoinReceiveEvent:
		rp.statter.IncCounter(rp.getStatKey("join.recv"), nil, 1)

	case swim.JoinCompleteEvent:
		rp.statter.IncCounter(rp.getStatKey("join.complete"), nil, 1)
		rp.statter.IncCounter(rp.getStatKey("join.succeeded"), nil, 1)
		rp.statter.RecordTimer(rp.getStatKey("join"), nil, event.Duration)

	case swim.PingSendEvent:
		rp.statter.IncCounter(rp.getStatKey("ping.send"), nil, 1)

	case swim.PingSendCompleteEvent:
		rp.statter.RecordTimer(rp.getStatKey("ping"), nil, event.Duration)

	case swim.PingReceiveEvent:
		rp.statter.IncCounter(rp.getStatKey("ping.recv"), nil, 1)

	case swim.PingRequestsSendEvent:
		rp.statter.IncCounter(rp.getStatKey("ping-req.send"), nil, 1)
		rp.statter.IncCounter(rp.getStatKey("ping-req.other-members"), nil, int64(len(event.Peers)))

	case swim.PingRequestsSendCompleteEvent:
		rp.statter.RecordTimer(rp.getStatKey("ping-req"), nil, event.Duration)

	case swim.PingRequestReceiveEvent:
		rp.statter.IncCounter(rp.getStatKey("ping-req.recv"), nil, 1)

	case swim.PingRequestPingEvent:
		rp.statter.RecordTimer(rp.getStatKey("ping-req-ping"), nil, event.Duration)

	case swim.ProtocolDelayComputeEvent:
		rp.statter.RecordTimer(rp.getStatKey("protocol.delay"), nil, event.Duration)

	case swim.ProtocolFrequencyEvent:
		rp.statter.RecordTimer(rp.getStatKey("protocol.frequency"), nil, event.Duration)

	case swim.ChecksumComputeEvent:
		rp.statter.RecordTimer(rp.getStatKey("compute-checksum"), nil, event.Duration)
		rp.statter.UpdateGauge(rp.getStatKey("checksum"), nil, int64(event.Checksum))
		rp.statter.IncCounter(rp.getStatKey("membership.checksum-computed"), nil, 1)

	case swim.ChangesCalculatedEvent:
		rp.statter.UpdateGauge(rp.getStatKey("changes.disseminate"), nil, int64(len(event.Changes)))

	case swim.ChangeFilteredEvent:
		rp.statter.IncCounter(rp.getStatKey("filtered-change"), nil, 1)

	case swim.JoinFailedEvent:
		rp.statter.IncCounter(rp.getStatKey("join.failed."+string(event.Reason)), nil, 1)

	case swim.JoinTriesUpdateEvent:
		rp.statter.UpdateGauge(rp.getStatKey("join.retries"), nil, int64(event.Retries))

	case events.LookupEvent:
		rp.statter.RecordTimer(rp.getStatKey("lookup"), nil, event.Duration)

	case swim.MakeNodeStatusEvent:
		rp.statter.IncCounter(rp.getStatKey("make-"+event.Status), nil, 1)

	case swim.RequestBeforeReadyEvent:
		rp.statter.IncCounter(rp.getStatKey("not-ready."+string(event.Endpoint)), nil, 1)

	case swim.RefuteUpdateEvent:
		rp.statter.IncCounter(rp.getStatKey("refuted-update"), nil, 1)

	case events.RingChecksumEvent:
		rp.statter.IncCounter(rp.getStatKey("ring.checksum-computed"), nil, 1)
		rp.statter.UpdateGauge(rp.getStatKey("ring.checksum"), nil, int64((event.NewChecksum)))

	case events.RingChangedEvent:
		added := int64(len(event.ServersAdded))
		removed := int64(len(event.ServersRemoved))
		rp.statter.IncCounter(rp.getStatKey("ring.server-added"), nil, added)
		rp.statter.IncCounter(rp.getStatKey("ring.server-removed"), nil, removed)
		rp.statter.IncCounter(rp.getStatKey("ring.changed"), nil, 1)

	case forward.RequestForwardedEvent:
		rp.statter.IncCounter(rp.getStatKey("requestProxy.egress"), nil, 1)

	case forward.InflightRequestsChangedEvent:
		rp.statter.UpdateGauge(rp.getStatKey("requestProxy.inflight"), nil, event.Inflight)

	case forward.InflightRequestsMiscountEvent:
		rp.statter.IncCounter(rp.getStatKey("requestProxy.miscount."+string(event.Operation)), nil, 1)

	case forward.FailedEvent:
		rp.statter.IncCounter(rp.getStatKey("requestProxy.send.error"), nil, 1)

	case forward.SuccessEvent:
		rp.statter.IncCounter(rp.getStatKey("requestProxy.send.success"), nil, 1)

	case forward.MaxRetriesEvent:
		rp.statter.IncCounter(rp.getStatKey("requestProxy.retry.failed"), nil, 1)

	case forward.RetryAttemptEvent:
		rp.statter.IncCounter(rp.getStatKey("requestProxy.retry.attempted"), nil, 1)

	case forward.RetryAbortEvent:
		rp.statter.IncCounter(rp.getStatKey("requestProxy.retry.aborted"), nil, 1)

	case forward.RerouteEvent:
		me, _ := rp.WhoAmI()
		if event.NewDestination == me {
			rp.statter.IncCounter(rp.getStatKey("requestProxy.retry.reroute.local"), nil, 1)
		} else {
			rp.statter.IncCounter(rp.getStatKey("requestProxy.retry.reroute.remote"), nil, 1)
		}

	case forward.RetrySuccessEvent:
		rp.statter.IncCounter(rp.getStatKey("requestProxy.retry.succeeded"), nil, 1)
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

// Checksum returns the current checksum of this Ringpop instance's hashring.
func (rp *Ringpop) Checksum() (uint32, error) {
	if !rp.Ready() {
		return 0, ErrNotBootstrapped
	}
	return rp.ring.Checksum(), nil
}

// Lookup returns the address of the server in the ring that is responsible
// for the specified key. It returns an error if the Ringpop instance is not
// yet initialized/bootstrapped.
func (rp *Ringpop) Lookup(key string) (string, error) {
	if !rp.Ready() {
		return "", ErrNotBootstrapped
	}

	startTime := time.Now()

	dest, success := rp.ring.Lookup(key)

	rp.emit(events.LookupEvent{key, time.Now().Sub(startTime)})

	if !success {
		err := errors.New("could not find destination for key")
		rp.logger.WithField("key", key).Warn(err)
		return "", err
	}

	return dest, nil
}

// LookupN returns the addresses of all the servers in the ring that are
// responsible for the specified key. It returns an error if the Ringpop
// instance is not yet initialized/bootstrapped.
func (rp *Ringpop) LookupN(key string, n int) ([]string, error) {
	if !rp.Ready() {
		return nil, ErrNotBootstrapped
	}
	return rp.ring.LookupN(key, n), nil
}

func (rp *Ringpop) ringEvent(e interface{}) {
	rp.HandleEvent(e)
}

// GetReachableMembers returns a slice of members currently in this instance's
// membership list that aren't faulty.
func (rp *Ringpop) GetReachableMembers() ([]string, error) {
	if !rp.Ready() {
		return nil, ErrNotBootstrapped
	}
	return rp.node.GetReachableMembers(), nil
}

// CountReachableMembers returns the number of members currently in this
// instance's membership list that aren't faulty.
func (rp *Ringpop) CountReachableMembers() (int, error) {
	if !rp.Ready() {
		return 0, ErrNotBootstrapped
	}
	return rp.node.CountReachableMembers(), nil
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
func SerializeThrift(s athrift.TStruct) ([]byte, error) {
	var b []byte
	var buffer = bytes.NewBuffer(b)

	transport := athrift.NewStreamTransportW(buffer)
	if err := s.Write(athrift.NewTBinaryProtocolTransport(transport)); err != nil {
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
func DeserializeThrift(b []byte, s athrift.TStruct) error {
	reader := bytes.NewReader(b)
	transport := athrift.NewStreamTransportR(reader)
	return s.Read(athrift.NewTBinaryProtocolTransport(transport))
}
