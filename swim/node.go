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
	"sync"
	"time"

	"github.com/uber/ringpop-go/discovery"
	"github.com/uber/ringpop-go/events"

	"github.com/benbjohnson/clock"
	"github.com/rcrowley/go-metrics"
	log "github.com/uber-common/bark"
	"github.com/uber/ringpop-go/logging"
	"github.com/uber/ringpop-go/shared"
	"github.com/uber/ringpop-go/util"
)

var (
	// ErrNodeNotReady is returned when a remote request is being handled while the node is not yet ready
	ErrNodeNotReady = errors.New("node is not ready to handle requests")
)

// Options is a configuration struct passed the NewNode constructor.
type Options struct {
	StateTimeouts     StateTimeouts
	MinProtocolPeriod time.Duration

	JoinTimeout, PingTimeout, PingRequestTimeout time.Duration

	PingRequestSize int

	RollupFlushInterval time.Duration
	RollupMaxUpdates    int

	MaxReverseFullSyncJobs int

	// When started, the partition healing algorithm attempts a partition heal
	// every PartitionHealPeriod with a probability of:
	// PartitionHealBaseProbabillity / # Nodes in discoverProvider.
	//
	// When in a 100 node cluster BaseProbabillity = 3 and Period = 30s,
	// every 30 seconds a node will have a probability of 3/100 to start the
	// partition healing procedure. This means that for the entire cluster
	// the discover provider receives 6 calls per minute on average.
	PartitionHealPeriod           time.Duration
	PartitionHealBaseProbabillity float64

	Clock clock.Clock
}

func defaultOptions() *Options {
	opts := &Options{
		StateTimeouts: StateTimeouts{
			Suspect:   5 * time.Second,
			Faulty:    24 * time.Hour,
			Tombstone: 1 * time.Minute,
		},

		MinProtocolPeriod: 200 * time.Millisecond,

		JoinTimeout:        1000 * time.Millisecond,
		PingTimeout:        1500 * time.Millisecond,
		PingRequestTimeout: 5000 * time.Millisecond,

		PingRequestSize: 3,

		RollupFlushInterval: 5000 * time.Millisecond,
		RollupMaxUpdates:    250,

		PartitionHealPeriod:           30 * time.Second,
		PartitionHealBaseProbabillity: 3,

		Clock: clock.New(),

		MaxReverseFullSyncJobs: 5,
	}

	return opts
}

func mergeDefaultOptions(opts *Options) *Options {
	def := defaultOptions()

	if opts == nil {
		return def
	}

	opts.StateTimeouts = mergeStateTimeouts(opts.StateTimeouts, def.StateTimeouts)

	opts.MinProtocolPeriod = util.SelectDuration(opts.MinProtocolPeriod, def.MinProtocolPeriod)

	opts.RollupMaxUpdates = util.SelectInt(opts.RollupMaxUpdates, def.RollupMaxUpdates)
	opts.RollupFlushInterval = util.SelectDuration(opts.RollupFlushInterval, def.RollupFlushInterval)

	opts.PartitionHealPeriod = util.SelectDuration(opts.PartitionHealPeriod, def.PartitionHealPeriod)

	opts.PartitionHealBaseProbabillity = util.SelectFloat(opts.PartitionHealBaseProbabillity, def.PartitionHealBaseProbabillity)

	opts.JoinTimeout = util.SelectDuration(opts.JoinTimeout, def.JoinTimeout)
	opts.PingTimeout = util.SelectDuration(opts.PingTimeout, def.PingTimeout)
	opts.PingRequestTimeout = util.SelectDuration(opts.PingRequestTimeout, def.PingRequestTimeout)

	opts.PingRequestSize = util.SelectInt(opts.PingRequestSize, def.PingRequestSize)

	opts.MaxReverseFullSyncJobs = util.SelectInt(opts.MaxReverseFullSyncJobs, def.MaxReverseFullSyncJobs)

	if opts.Clock == nil {
		opts.Clock = def.Clock
	}

	return opts
}

// NodeInterface specifies the public-facing methods that a SWIM Node
// implements.
type NodeInterface interface {
	Bootstrap(opts *BootstrapOptions) ([]string, error)
	CountMembers(predicates ...MemberPredicate) int
	Destroy()
	GetChecksum() uint32
	GetMembers(predicates ...MemberPredicate) []Member
	MemberStats() MemberStats
	ProtocolStats() ProtocolStats
	Ready() bool
	RegisterListener(l events.EventListener)

	Labels() *NodeLabels
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

	channel          shared.SubChannel
	discoverProvider discovery.DiscoverProvider
	memberlist       *memberlist
	memberiter       memberIter
	disseminator     *disseminator
	stateTransitions *stateTransitions
	gossip           *gossip
	rollup           *updateRollup

	// When we get more healer strategies we can abstract to a healer interface.
	healer *discoverProviderHealer

	joinTimeout, pingTimeout, pingRequestTimeout time.Duration

	pingRequestSize int

	maxReverseFullSyncJobs int

	listeners []events.EventListener

	clientRate metrics.Meter
	serverRate metrics.Meter
	totalRate  metrics.Meter

	startTime time.Time

	logger log.Logger

	// clock is used to generate incarnation numbers; it is typically the
	// system clock, wrapped via clock.New()
	clock clock.Clock
}

// NewNode returns a new SWIM Node.
func NewNode(app, address string, channel shared.SubChannel, opts *Options) *Node {
	// use defaults for options that are unspecified
	opts = mergeDefaultOptions(opts)

	node := &Node{
		address: address,
		app:     app,
		channel: channel,
		logger:  logging.Logger("node").WithField("local", address),

		joinTimeout:        opts.JoinTimeout,
		pingTimeout:        opts.PingTimeout,
		pingRequestTimeout: opts.PingRequestTimeout,

		pingRequestSize: opts.PingRequestSize,

		maxReverseFullSyncJobs: opts.MaxReverseFullSyncJobs,

		clientRate: metrics.NewMeter(),
		serverRate: metrics.NewMeter(),
		totalRate:  metrics.NewMeter(),
		clock:      opts.Clock,
	}

	node.memberlist = newMemberlist(node)
	node.memberiter = newMemberlistIter(node.memberlist)
	node.stateTransitions = newStateTransitions(node, opts.StateTimeouts)

	node.healer = newDiscoverProviderHealer(
		node,
		opts.PartitionHealBaseProbabillity,
		opts.PartitionHealPeriod,
	)
	node.gossip = newGossip(node, opts.MinProtocolPeriod)
	node.disseminator = newDisseminator(node)

	for _, member := range node.memberlist.GetMembers() {
		change := Change{}
		change.populateSubject(&member)
		node.disseminator.RecordChange(change)
	}

	node.rollup = newUpdateRollup(node, opts.RollupFlushInterval,
		opts.RollupMaxUpdates)

	if node.channel != nil {
		node.registerHandlers()
		node.service = node.channel.ServiceName()
	}

	return node
}

// Address returns the address of the SWIM node.
func (n *Node) Address() string {
	return n.address
}

// App returns the Node's application name.
func (n *Node) App() string {
	return n.app
}

// HasChanges reports whether Node has changes to disseminate.
func (n *Node) HasChanges() bool {
	return n.disseminator.HasChanges()
}

// Incarnation returns the incarnation number of the Node.
func (n *Node) Incarnation() int64 {
	if n.memberlist != nil && n.memberlist.local != nil {
		n.memberlist.RLock()
		incarnation := n.memberlist.local.Incarnation
		n.memberlist.RUnlock()
		return incarnation
	}
	return -1
}

func (n *Node) emit(event interface{}) {
	for _, listener := range n.listeners {
		listener.HandleEvent(event)
	}
}

// RegisterListener adds an EventListener to the node. When a swim event e is
// emitted, l.HandleEvent(e) is called for every registered listener l.
// Attention, all listeners are called synchronously. Be careful with
// registering blocking and other slow calls.
func (n *Node) RegisterListener(l events.EventListener) {
	n.listeners = append(n.listeners, l)
}

// Start starts the SWIM protocol and all sub-protocols.
func (n *Node) Start() {
	n.gossip.Start()
	n.stateTransitions.Enable()
	n.healer.Start()

	n.state.Lock()
	n.state.stopped = false
	n.state.Unlock()
}

// Stop stops the SWIM protocol and all sub-protocols.
func (n *Node) Stop() {
	n.gossip.Stop()
	n.stateTransitions.Disable()
	n.healer.Stop()

	n.state.Lock()
	n.state.stopped = true
	n.state.Unlock()
}

// Stopped returns whether or not the SWIM protocol is currently running.
func (n *Node) Stopped() bool {
	n.state.RLock()
	stopped := n.state.stopped
	n.state.RUnlock()

	return stopped
}

// Destroy stops the SWIM protocol and all sub-protocols.
func (n *Node) Destroy() {
	n.state.Lock()
	if n.state.destroyed {
		n.state.Unlock()
		return
	}
	n.state.destroyed = true
	n.state.Unlock()

	n.Stop()
	n.rollup.Destroy()
}

// Destroyed returns whether or not the node has been destroyed.
func (n *Node) Destroyed() bool {
	n.state.RLock()
	destroyed := n.state.destroyed
	n.state.RUnlock()

	return destroyed
}

// Ready returns whether or not the node has bootstrapped and is ready for use.
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

// BootstrapOptions is a configuration struct passed to Node.Bootstrap.
type BootstrapOptions struct {
	// The DiscoverProvider resolves a list of bootstrap hosts.
	DiscoverProvider discovery.DiscoverProvider

	// Whether or not gossip should be started immediately after a successful
	// bootstrap.
	Stopped bool

	// Amount of time before individual join requests time out.
	JoinTimeout time.Duration

	// Minimum number of nodes to join to satisfy a bootstrap.
	JoinSize int

	// Maximum time to attempt joins before the entire bootstrap process times
	// out.
	MaxJoinDuration time.Duration

	// A higher ParallelismFactor increases the number of nodes that a
	// bootstrapping node will attempt to reach out to in order to satisfy
	// `JoinSize` (the number of nodes that will be contacted at a time is
	// `ParallelismFactor * JoinSize`).
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

	n.discoverProvider = opts.DiscoverProvider
	joinOpts := &joinOpts{
		timeout:           opts.JoinTimeout,
		size:              opts.JoinSize,
		maxJoinDuration:   opts.MaxJoinDuration,
		parallelismFactor: opts.ParallelismFactor,
	}

	joined, err := sendJoin(n, joinOpts)
	if err != nil {
		n.logger.WithFields(log.Fields{
			"err": err.Error(),
		}).Error("bootstrap failed")
		return nil, err
	}

	if !opts.Stopped {
		n.gossip.Start()
		n.healer.Start()
	}

	n.state.Lock()
	n.state.ready = true
	n.state.Unlock()

	n.startTime = time.Now()

	return joined, nil
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	Change Handling
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func (n *Node) handleChanges(changes []Change) {
	n.disseminator.AdjustMaxPropagations()
	for _, change := range changes {
		n.disseminator.RecordChange(change)

		switch change.Status {
		case Alive:
			n.stateTransitions.Cancel(change)

		case Suspect:
			n.stateTransitions.ScheduleSuspectToFaulty(change)

		case Faulty:
			n.stateTransitions.ScheduleFaultyToTombstone(change)

		case Leave:
			// XXX: should this also reap?
			n.stateTransitions.Cancel(change)

		case Tombstone:
			n.stateTransitions.ScheduleTombstoneToEvict(change)
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
		n.logger.Warn("no pingable members")
		return
	}

	if n.pinging() {
		n.logger.Warn("node already pinging")
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
	target := member.Address
	targetReached, errs := indirectPing(n, target, n.pingRequestSize, n.pingRequestTimeout)

	// if all helper nodes are unreachable, the indirectPing is inconclusive
	if len(errs) == n.pingRequestSize {
		n.logger.WithFields(log.Fields{
			"target":    target,
			"errors":    errs,
			"numErrors": len(errs),
		}).Warn("ping request inconclusive due to errors")
		return
	}

	if !targetReached {
		n.logger.WithField("target", target).Info("ping request target unreachable")
		n.memberlist.MakeSuspect(member.Address, member.Incarnation)
		return
	}

	n.logger.WithField("target", target).Info("ping request target reachable")
}

// GetMembers returns a slice of members containing only members that satisfies
// all predicates passed in. An example usecase is to use the ReachableMember
// predicate only get members that are deemed reachable by the rules of SWIM.
func (n *Node) GetMembers(predicates ...MemberPredicate) []Member {
	return n.memberlist.GetMembers(predicates...)
}

// CountMembers returns the number of members currently in this node's
// membership list that satisfies all predicates passed in. And example usecase
// is to count all reachable members by passing the ReachableMember predicate.
func (n *Node) CountMembers(predicates ...MemberPredicate) int {
	return n.memberlist.CountMembers(predicates...)
}

// Labels returns a mutator for the labels kept on this local node. This mutator
// interacts with the local node and memberlist to change labels on this node
// and gossip those changes around.
func (n *Node) Labels() *NodeLabels {
	return &NodeLabels{n}
}
