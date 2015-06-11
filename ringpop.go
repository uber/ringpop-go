package ringpop

import (
	"errors"
	"fmt"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/uber/tchannel/golang"

	"github.com/quipo/statsd"
	"github.com/rcrowley/go-metrics"
)

const (
	ringpopMaxJoingDuration       = 300000 * time.Millisecond
	membershipUpdateFlushInterval = 5000 * time.Millisecond
	proxyReqTimeout               = 30000 * time.Millisecond
)

var (
	proxyReqProps = []string{"keys", "dest", "req", "res"}
)

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	RINGPOP
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

// Options provides options for creation of a ringpop
type Options struct {
	// Channel for the ringpop to operate on
	Channel *tchannel.Channel
	// Logger to send logs to, defaults to StandardLogger
	Logger *log.Logger
	// Statsd client to send stats to, if none provided no stats are sent
	Statsd statsd.Statsd

	// File to bootstrap from
	BootstrapFile string

	JoinSize                      int
	SetTimeout                    time.Duration
	ProxyReqTimeout               time.Duration
	MaxJoinDuration               time.Duration
	MembershipUpdateFlushInterval time.Duration

	RequestProxyMaxRetries    int
	RequestProxyRetrySchedule time.Duration // revist

	MinProtocolPeriod time.Duration
	SuspicionTimeout  time.Duration
}

// TODO: ringpop comment description

// Ringpop is a ringpop is a ringpop is a ringpop
type Ringpop struct {
	App      string
	HostPort string

	channel *tchannel.Channel

	eventC   chan string
	emitting bool

	bootstrapFile string
	joinSize      int

	ready     bool
	pinging   bool
	destroyed bool

	pingReqSize int

	pingReqTimeout                time.Duration
	pingTimeout                   time.Duration
	proxyReqTimeout               time.Duration
	maxJoinDuration               time.Duration
	membershipUpdateFlushInterval time.Duration

	ring       *hashRing
	ringEventC chan string

	// membership
	dissemination          *dissemination
	membership             *membership
	MembershipIter         *membershipIter
	membershipChangeC      chan []Change
	membershipUpdateRollup *membershipUpdateRollup

	// gossip
	suspicion *suspicion
	gossip    *gossip

	// statsd
	statsd       statsd.Statsd
	statHostPort string
	statPrefix   string
	statKeys     map[string]string
	statHooks    []string // type?

	clientRate metrics.Meter
	serverRate metrics.Meter
	totalRate  metrics.Meter

	// logging
	logger *log.Logger

	// testing
	isDenyingJoins bool
}

// NewRingpop creates a new ringpop on the specified hostport
func NewRingpop(app, hostport string, options *Options) *Ringpop {
	// use default options if nothing is passed into options
	if options == nil {
		options = &Options{}
	}

	// verify valid hostport
	if match := hostPortPattern.Match([]byte(hostport)); !match {
		log.Fatal("Invalid host port ", hostport)
	}

	ringpop := &Ringpop{
		App:      app,
		HostPort: hostport,
	}

	ringpop.ready = false
	ringpop.pinging = false

	// set channel to option
	if ringpop.channel = nil; options.Channel != nil {
		ringpop.channel = options.Channel
	}
	// set logger to option or default value
	if ringpop.logger = log.StandardLogger(); options.Logger != nil {
		ringpop.logger = options.Logger
	}
	// set statsd client to option or default value
	if ringpop.statsd = (&statsd.NoopClient{}); options.Statsd != nil {
		ringpop.statsd = options.Statsd
	}

	ringpop.bootstrapFile = options.BootstrapFile
	ringpop.joinSize = options.JoinSize
	// set maxJoinDuration to option or default value
	if ringpop.maxJoinDuration = ringpopMaxJoingDuration; options.MaxJoinDuration.Nanoseconds() != 0 {
		ringpop.maxJoinDuration = options.MaxJoinDuration
	}

	ringpop.pingTimeout = time.Millisecond * 1500
	ringpop.pingReqSize = 3
	ringpop.pingReqTimeout = time.Millisecond * 5000

	// set proxyReqTimeout to option or default value
	if ringpop.proxyReqTimeout = proxyReqTimeout; options.ProxyReqTimeout.Nanoseconds() != 0 {
		ringpop.proxyReqTimeout = options.ProxyReqTimeout
	}
	// membership and gossip
	ringpop.dissemination = newDissemination(ringpop)
	ringpop.membership = newMembership(ringpop)
	ringpop.membershipUpdateRollup = newMembershipUpdateRollup(ringpop,
		ringpop.membershipUpdateFlushInterval, 0)
	ringpop.suspicion = newSuspicion(ringpop, options.SuspicionTimeout)
	ringpop.gossip = newGossip(ringpop, 0)
	// statsd
	// changes 0.0.0.0:0000 -> 0_0_0_0_0000
	ringpop.statKeys = make(map[string]string, 0)
	ringpop.statHostPort = strings.Replace(ringpop.HostPort, ".", "_", -1)
	ringpop.statHostPort = strings.Replace(ringpop.statHostPort, ":", "_", -1)
	ringpop.statPrefix = fmt.Sprintf("ringpop.%s", ringpop.statHostPort)

	ringpop.destroyed = false

	return ringpop
}

// SetupChannel sets up the ringpop's TChannel. Has no effect if the ringpop's channel
// is `nil`.
func (rp *Ringpop) SetupChannel() {
	if rp.channel != nil {
		newRingpopTChannel(rp, rp.channel)
	}
}

// Bootstrap seeds the hash ring, joins nodes in the seed list, and then starts the
// gossip protocol.
func (rp *Ringpop) Bootstrap() {

}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//  METHODS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

// WhoAmI returns the ringpop's hostport
func (rp *Ringpop) WhoAmI() string {
	return rp.HostPort
}

// Ready returns true if the ringpop is ready for use
func (rp *Ringpop) Ready() bool {
	return rp.ready
}

// Lookup hashes a key to a node and returns that node, and true. If no node can
// be found, it returns the hostport of the local ringpop and false.
func (rp *Ringpop) Lookup(key string) (string, bool) {
	dest, found := rp.ring.lookup(key)
	if !found {
		rp.logger.WithField("key", key).Debug("could not find destination for a key")
		return rp.WhoAmI(), false
	}

	return dest, true
}

// EmitEvents enables or disables the channel returned by EventC.
func (rp *Ringpop) EmitEvents(emit bool) {
	rp.emitting = emit
}

// EventC returns a read only channel of events from ringpop.
func (rp *Ringpop) EventC() <-chan string {
	return rp.eventC
}

// Emit sends a message on the ringpop's eventC in a nonblocking fashion
func (rp *Ringpop) emit(message string) {
	if rp.emitting {
		go func() {
			rp.eventC <- message
		}()
	}
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	RING EVENT HANDLING
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func (rp *Ringpop) onRingChecksumComputed() {
	rp.stat("increment", "ring.checksum-computed", 1)
	rp.emit("ringChecksumComputed")
}

func (rp *Ringpop) onRingServerAdded() {
	rp.stat("increment", "ring.server-added", 1)
	rp.emit("ringServerAdded")
}

func (rp *Ringpop) onRingServerRemoved() {
	rp.stat("increment", "ring.server-removed", 1)
	rp.emit("ringServerRemoved")
}

func (rp *Ringpop) launchRingEventHandler() chan<- bool {
	stop := make(chan bool, 1)

	go func() {
		for {
			select {
			case event := <-rp.ringEventC:
				switch event {
				case "added":
					rp.onRingServerAdded()
				case "removed":
					rp.onRingServerRemoved()
				case "checksumComputed":
					rp.onRingChecksumComputed()
				}
			}
		}
	}()

	return stop
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	MEMBERSHIP EVENT HANDLING
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func (rp *Ringpop) onMemberAlive(change Change) {
	// TODO: add .stat call see .js for detail)
	// rp.stat('increment', 'membership-update.alive');

	rp.logger.WithFields(log.Fields{
		"local": rp.WhoAmI(),
		"alive": change.Address,
	}).Debug("member is alive")

	rp.dissemination.recordChange(change) // Propogate change
	rp.ring.addServer(change.Address)     // Add server from ring
	rp.suspicion.stop(change)             // Stop suspicion for server
}

func (rp *Ringpop) onMemberFaulty(change Change) {
	// TODO: add .stat call (see .js for detail)
	// rp.stat('increment', 'membership-update.faulty');

	rp.logger.WithFields(log.Fields{
		"local":  rp.WhoAmI(),
		"faulty": change.Address,
	}).Debug("member is faulty")

	rp.ring.removeServer(change.Address)  // Remove server from ring
	rp.suspicion.stop(change)             // Stop suspicion for server
	rp.dissemination.recordChange(change) // Propogate change
}

func (rp *Ringpop) onMemberSuspect(change Change) {
	// TODO: add .stat call and .log call (see .js for detail)
	// rp.stat('increment', 'membership-update.suspect');

	rp.logger.WithFields(log.Fields{
		"local":   rp.WhoAmI(),
		"suspect": change.Address,
	}).Debug("member is suspect")

	rp.suspicion.start(change)            // Start suspicion for server
	rp.dissemination.recordChange(change) // Propogate change
}

func (rp *Ringpop) onMemberLeave(change Change) {
	// TODO: add .stat call (see .js for detail)
	// rp.stat('increment', 'membership-update.leave');

	rp.logger.WithFields(log.Fields{
		"local": rp.WhoAmI(),
		"leave": change.Address,
	}).Debug("member has left")

	rp.dissemination.recordChange(change) // Propogate change
	rp.ring.removeServer(change.Address)  // Remove server from ring
	rp.suspicion.stop(change)             // Stop suspicion for server
}

func (rp *Ringpop) launchMembershipChangeHandler() chan<- bool {
	var changes []Change

	stop := make(chan bool, 1)

	go func() {
		var membershipChanged, ringChanged bool

		for {
			membershipChanged, ringChanged = false, false

			select {
			case changes = <-rp.membershipChangeC:
				for _, change := range changes {
					switch change.Status {
					case ALIVE:
						rp.onMemberAlive(change)
						membershipChanged, ringChanged = true, true
					case FAULTY:
						rp.onMemberFaulty(change)
						membershipChanged, ringChanged = true, true
					case SUSPECT:
						rp.onMemberSuspect(change)
						membershipChanged, ringChanged = true, false
					case LEAVE:
						rp.onMemberLeave(change)
						membershipChanged, ringChanged = true, true
					}
				}
			case <-stop:
				break
			}

			if membershipChanged {
				rp.emit("membershipChanged")
			}

			if ringChanged {
				rp.emit("ringChanged")
			}

			rp.membershipUpdateRollup.trackUpdates(changes)

			rp.stat("gauge", "num-members", int64(rp.membership.memberCount()))
			rp.stat("timing", "updates", int64(len(changes)))
		}
	}()

	return stop
}

// PingMember is temporary testing func
func (rp *Ringpop) PingMember(address string) error {
	res, err := sendPing(rp, address)
	if err != nil {
		rp.logger.Errorf("error: %v", err)
		return err
	}
	rp.pinging = false
	//rp.membership.update(responseBody.changes)
	fmt.Printf("Checksum: %v\n", res.Checksum)
	fmt.Printf("Source: %s\n", res.Source)
	for _, change := range res.Changes {
		fmt.Printf("Address: %s, Status: %s, Incarnation: %v\n",
			change.Address, change.Status, change.Incarnation)
	}
	return nil
}

// PingMemberNow temporarily exported for testing purposes
func (rp *Ringpop) PingMemberNow() error {
	if rp.pinging {
		rp.logger.Warn("aborting ping because one is already in progress")
		return errors.New("ping aborted because a ping already in progress")
	}

	if !rp.ready {
		rp.logger.Warn("ping started before ring is initialized")
		return errors.New("ping started before ring is initialized")
	}

	iter := rp.membership.iter()
	member, ok := iter.next()
	if !ok {
		rp.logger.Warn("no usable nodes at protocol period")
		return errors.New("no usable nodes at protocol period")
	}

	rp.pinging = true

	res, err := sendPing(rp, member.Address)

	if err == nil {
		rp.pinging = false
		//rp.membership.update(responseBody.changes)
		// TODO

		fmt.Printf("Checksum: %v\n", res.Checksum)
		fmt.Printf("Source: %s\n", res.Source)
		for _, change := range res.Changes {
			fmt.Printf("Address: %s, Status: %s, Incarnation: %v\n",
				change.Address, change.Status, change.Incarnation)
		}

		return nil
	}

	if rp.destroyed {
		return errors.New("destroyed whilst pinging")
	}

	// Do a ping request otherwise

	return nil
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	STATS(D) HANDLING
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func (rp *Ringpop) stat(statType, key string, val int64) {
	fqkey, ok := rp.statKeys[key]
	if !ok {
		fqkey = fmt.Sprintf("%s.%s", rp.statPrefix, key)
		rp.statKeys[key] = fqkey
	}

	switch statType {
	case "increment":
		rp.statsd.Incr(fqkey, val)
	case "gauge":
		rp.statsd.Gauge(fqkey, val)
	case "timing":
		rp.statsd.Timing(fqkey, val)
	}
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	TESTING FUNCTIONS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func (rp *Ringpop) allowJoins() {
	rp.isDenyingJoins = false
}

func (rp *Ringpop) denyJoins() {
	rp.isDenyingJoins = true
}
