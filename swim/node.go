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

package swim

import (
	"errors"
	"io/ioutil"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/rcrowley/go-metrics"
	log "github.com/uber-common/bark"
	"github.com/uber/ringpop-go/shared"
	"github.com/uber/ringpop-go/swim/util"
)

// Options to create a SWIM with
type Options struct {
	SuspicionTimeout  time.Duration
	MinProtocolPeriod time.Duration

	JoinTimeout, PingTimeout, PingRequestTimeout time.Duration

	PingRequestSize int

	RollupFlushInterval time.Duration
	RollupMaxUpdates    int

	BootstrapFile  string
	BootstrapHosts []string

	Logger log.Logger
}

func defaultOptions() *Options {
	opts := &Options{
		BootstrapFile: "./hosts.json",

		SuspicionTimeout:  5000 * time.Millisecond,
		MinProtocolPeriod: 200 * time.Millisecond,

		JoinTimeout:        1000 * time.Millisecond,
		PingTimeout:        1500 * time.Millisecond,
		PingRequestTimeout: 5000 * time.Millisecond,

		PingRequestSize: 3,

		RollupFlushInterval: 5000 * time.Millisecond,
		RollupMaxUpdates:    250,

		Logger: log.NewLoggerFromLogrus(&logrus.Logger{
			Out: ioutil.Discard,
		}),
	}

	return opts
}

func mergeDefaultOptions(opts *Options) *Options {
	def := defaultOptions()

	if opts == nil {
		return def
	}

	if opts.Logger == nil {
		opts.Logger = def.Logger
	}

	opts.SuspicionTimeout = util.SelectDuration(opts.SuspicionTimeout,
		def.SuspicionTimeout)

	opts.MinProtocolPeriod = util.SelectDuration(opts.MinProtocolPeriod,
		def.MinProtocolPeriod)

	opts.RollupMaxUpdates = util.SelectInt(opts.RollupMaxUpdates,
		def.RollupMaxUpdates)
	opts.RollupFlushInterval = util.SelectDuration(opts.RollupFlushInterval,
		def.RollupFlushInterval)

	opts.JoinTimeout = util.SelectDuration(opts.JoinTimeout, def.JoinTimeout)
	opts.PingTimeout = util.SelectDuration(opts.PingTimeout, def.PingTimeout)
	opts.PingRequestTimeout = util.SelectDuration(opts.PingRequestTimeout,
		def.PingRequestTimeout)

	opts.PingRequestSize = util.SelectInt(opts.PingRequestSize,
		def.PingRequestSize)

	return opts
}

// A Node is a SWIM member
type Node struct {
	app     string
	service string
	address string

	state struct {
		stopped, destroyed, pinging, ready bool
		sync.RWMutex
	}

	bootstrapFile  string
	bootstrapHosts map[string][]string

	channel      shared.SubChannel
	memberlist   *memberlist
	memberiter   memberIter
	disseminator *disseminator
	suspicion    *suspicion
	gossip       *gossip
	rollup       *updateRollup

	joinTimeout, pingTimeout, pingRequestTimeout time.Duration

	pingRequestSize int

	listeners []EventListener

	clientRate metrics.Meter
	serverRate metrics.Meter
	totalRate  metrics.Meter

	startTime time.Time

	logger log.Logger
	log    log.Logger // wraps the provided logger with a 'local: address' field
}

// NewNode returns a new SWIM node
func NewNode(app, address string, channel shared.SubChannel, opts *Options) *Node {
	// use defaults for options that are unspecified
	opts = mergeDefaultOptions(opts)

	node := &Node{
		address: address,
		app:     app,
		channel: channel,
		logger:  opts.Logger,
		log:     opts.Logger.WithField("local", address),

		joinTimeout:        opts.JoinTimeout,
		pingTimeout:        opts.PingTimeout,
		pingRequestTimeout: opts.PingRequestTimeout,

		pingRequestSize: opts.PingRequestSize,

		clientRate: metrics.NewMeter(),
		serverRate: metrics.NewMeter(),
		totalRate:  metrics.NewMeter(),
	}

	node.memberlist = newMemberlist(node)
	node.memberiter = newMemberlistIter(node.memberlist)
	node.suspicion = newSuspicion(node, opts.SuspicionTimeout)
	node.gossip = newGossip(node, opts.MinProtocolPeriod)
	node.disseminator = newDisseminator(node)
	node.rollup = newUpdateRollup(node, opts.RollupFlushInterval,
		opts.RollupMaxUpdates)

	if node.channel != nil {
		node.registerHandlers()
		node.service = node.channel.ServiceName()
	}

	return node
}

// Address returns the address of the SWIM node
func (n *Node) Address() string {
	return n.address
}

// App returns the node's app
func (n *Node) App() string {
	return n.app
}

// Incarnation returns the incarnation number of the local node
func (n *Node) Incarnation() int64 {
	if n.memberlist != nil && n.memberlist.local != nil {
		return n.memberlist.local.Incarnation
	}
	return -1
}

func (n *Node) emit(event interface{}) {
	for _, listener := range n.listeners {
		go listener.HandleEvent(event)
	}
}

// RegisterListener adds a listener that will be sent swim events
func (n *Node) RegisterListener(l EventListener) {
	n.listeners = append(n.listeners, l)
}

// Start starts the SWIM protocol and all sub-protocols
func (n *Node) Start() {
	n.gossip.Start()
	n.suspicion.Reenable()

	n.state.Lock()
	n.state.stopped = false
	n.state.Unlock()
}

// Stop stops the SWIM protocol and all sub-protocols
func (n *Node) Stop() {
	n.gossip.Stop()
	n.suspicion.Disable()

	n.state.Lock()
	n.state.stopped = true
	n.state.Unlock()
}

// Stopped returns whether or not the SWIM protocol is currently running
func (n *Node) Stopped() bool {
	n.state.RLock()
	stopped := n.state.stopped
	n.state.RUnlock()

	return stopped
}

// Destroy stops the SWIM protocol and all sub-protocols
func (n *Node) Destroy() {
	n.Stop()
	n.rollup.Destroy()

	n.state.Lock()
	n.state.destroyed = true
	n.state.Unlock()
}

// Destroyed returns whether or not the node has been destroyed or node
func (n *Node) Destroyed() bool {
	n.state.RLock()
	destroyed := n.state.destroyed
	n.state.RUnlock()

	return destroyed
}

// Ready returns whether or not the node has bootstrapped fully and is ready for use
func (n *Node) Ready() bool {
	n.state.RLock()
	ready := n.state.ready
	n.state.RUnlock()

	return ready
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	Bootstrapping
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

// BootstrapOptions provides options to bootstrap the node with
type BootstrapOptions struct {
	// Slice of hosts to bootstrap with, prioritized over provided file
	Hosts []string

	// File containing a JSON array of hosts to bootstrap with
	File string

	// Whether or not gossip should start
	Stopped bool

	// Amount of time before join requests time out
	JoinTimeout time.Duration

	// Minimum number of nodes to join
	JoinSize int

	// Maximum time to attempt joins before joining cluster times out
	MaxJoinDuration time.Duration

	// number of nodes to attempt to join
	ParallelismFactor int
}

// Bootstrap joins a node to a cluster. The channel provided to the node must be
// listening for the bootstrap to complete.
func (n *Node) Bootstrap(opts *BootstrapOptions) ([]string, error) {
	if n.channel == nil {
		return nil, errors.New("channel required")
	}

	if opts == nil {
		opts = &BootstrapOptions{}
	}

	if err := n.seedBootstrapHosts(opts); err != nil {
		return nil, err
	}

	if len(n.bootstrapHosts) == 0 {
		return nil, errors.New("bootstrap hosts required, but none found")
	}

	err := util.CheckLocalMissing(n.address, n.bootstrapHosts[util.CaptureHost(n.address)])
	if err != nil {
		n.log.Warn(err.Error())
	}

	mismatched, err := util.CheckHostnameIPMismatch(n.address, n.bootstrapHosts)
	if err != nil {
		n.log.WithField("mismatched", mismatched).Warn(err.Error())
	}

	n.memberlist.MakeAlive(n.address, util.TimeNowMS())

	joinOpts := &joinOpts{
		timeout:           opts.JoinTimeout,
		size:              opts.JoinSize,
		maxJoinDuration:   opts.MaxJoinDuration,
		parallelismFactor: opts.ParallelismFactor,
	}

	joined, err := sendJoin(n, joinOpts)
	if err != nil {
		n.log.WithFields(log.Fields{
			"err": err.Error(),
		}).Error("bootstrap failed")
		return nil, err
	}

	if n.Destroyed() {
		n.log.Error("destroyed during bootstrap process")
		return nil, errors.New("destroyed during bootstrap process")
	}

	if !opts.Stopped {
		n.gossip.Start()
	}

	n.state.Lock()
	n.state.ready = true
	n.state.Unlock()

	n.startTime = time.Now()

	return joined, nil
}

func (n *Node) seedBootstrapHosts(opts *BootstrapOptions) (err error) {
	var hostports []string

	n.bootstrapHosts = make(map[string][]string)

	if opts.Hosts != nil {
		hostports = opts.Hosts
	} else {
		switch true {
		case true:
			if opts.File != "" {
				hostports, err = util.ReadHostsFile(opts.File)
				if err == nil {
					break
				}
				n.log.WithField("file", opts.File).Warnf("could not read host file: %v", err)
			}
			fallthrough
		case true:
			if n.bootstrapFile != "" {
				hostports, err = util.ReadHostsFile(n.bootstrapFile)
				if err == nil {
					break
				}
				n.log.WithField("file", n.bootstrapFile).Warnf("could not read host file: %v", err)
			}
			fallthrough
		case true:
			hostports, err = util.ReadHostsFile("./hosts.json")
			if err == nil {
				break
			}
			n.log.WithField("file", "./hosts.json").Warnf("could not read host file: %v", err)
			return errors.New("unable to read hosts file")
		}
	}

	for _, hostport := range hostports {
		host := util.CaptureHost(hostport)
		if host != "" {
			n.bootstrapHosts[host] = append(n.bootstrapHosts[host], hostport)
		}
	}

	return
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	Change Handling
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func (n *Node) handleChanges(changes []Change) {
	for _, change := range changes {
		n.disseminator.RecordChange(change)

		switch change.Status {
		case Alive:
			n.suspicion.Stop(change)
			n.disseminator.AdjustMaxPropogations()

		case Faulty:
			n.suspicion.Stop(change)

		case Suspect:
			n.suspicion.Start(change)
			n.disseminator.AdjustMaxPropogations()

		case Leave:
			n.suspicion.Stop(change)
			n.disseminator.AdjustMaxPropogations()
		}
	}
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// Gossip
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func (n *Node) pinging() bool {
	n.state.RLock()
	pinging := n.state.pinging
	n.state.RUnlock()

	return pinging
}

func (n *Node) setPinging(pinging bool) {
	n.state.Lock()
	n.state.pinging = pinging
	n.state.Unlock()
}

// pingNextMember pings the next member in the memberlist
func (n *Node) pingNextMember() {
	member, ok := n.memberiter.Next()
	if !ok {
		n.log.Warn("no pingable members")
		return
	}

	if n.pinging() {
		n.log.Warn("node already pinging")
		return
	}

	n.setPinging(true)
	defer n.setPinging(false)

	// send ping
	res, err := sendPing(n, member.Address, n.pingTimeout)
	if err == nil {
		n.memberlist.Update(res.Changes)
		return
	}

	// ping failed, send ping requests
	var errs []error
	for response := range sendPingRequests(n, member.Address, n.pingRequestSize, n.pingRequestTimeout) {
		switch res := response.(type) {
		case *pingResponse:
			if res.Ok {
				n.log.WithField("target", member.Address).Info("ping request target reachable")
				n.memberlist.Update(res.Changes)
				return
			}

			n.log.WithField("target", member.Address).Info("ping request target unreachable")
			n.memberlist.MakeSuspect(member.Address, member.Incarnation)
			return

		case error:
			errs = append(errs, res)
		}
	}

	n.log.WithFields(log.Fields{
		"target":    member.Address,
		"errors":    errs,
		"numErrors": len(errs),
	}).Warn("ping request inconclusive due to errors")
}

func (n *Node) GetReachableMembers() []string {
	return n.memberlist.GetReachableMembers()
}
