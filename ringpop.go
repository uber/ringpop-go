package ringpop

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/uber/tchannel/golang"

	"github.com/quipo/statsd"
	"github.com/rcrowley/go-metrics"
)

var hostPortPattern, hostPortErr = regexp.Compile("^([0-9]+.[0-9]+.[0-9]+.[0-9]+):[0-9]+$")

const (
	_MAX_JOIN_DURATOIN_               = 300000
	_MEMBERSHIP_UPDATE_FLUSH_INTERVAL = 5000
	_PROXY_REQ_TIMEOUT_               = 30000
)

var (
	_PROXY_REQ_PROPS_ = []string{"keys", "dest", "req", "res"}
)

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	RINGPOP
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

type RingpopOptions struct {
	Channel *tchannel.Channel
	Logger  *log.Logger
	Statsd  statsd.Statsd

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

type Ringpop struct {
	// Exported
	App      string
	HostPort string

	// Unexported
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

	ring       *HashRing
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

// TODO: Finish`
func NewRingpop(app, hostport string, options *RingpopOptions) *Ringpop {
	// use default options if nothing is passed into options
	if options == nil {
		options = &RingpopOptions{}
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
	if ringpop.maxJoinDuration = _MAX_JOIN_DURATOIN_; options.MaxJoinDuration.Nanoseconds() != 0 {
		ringpop.maxJoinDuration = options.MaxJoinDuration
	}

	ringpop.pingTimeout = time.Millisecond * 1500
	ringpop.pingReqSize = 3
	ringpop.pingReqTimeout = time.Millisecond * 5000

	// set proxyReqTimeout to option or default value
	if ringpop.proxyReqTimeout = _PROXY_REQ_TIMEOUT_; options.ProxyReqTimeout.Nanoseconds() != 0 {
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
	ringpop.statKeys = make(map[string]string, 0)
	// changes 0.0.0.0:0000 -> 0_0_0_0_0000
	ringpop.statHostPort = strings.Replace(ringpop.HostPort, ".", "_", -1)
	ringpop.statHostPort = strings.Replace(ringpop.statHostPort, ":", "_", -1)
	ringpop.statPrefix = fmt.Sprintf("ringpop.%s", ringpop.statHostPort)

	ringpop.destroyed = false

	return ringpop
}

func (this *Ringpop) SetupChannel() {
	NewRingpopTChannel(this, this.channel)
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//  METHODS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func (this *Ringpop) WhoAmI() string {
	return this.HostPort
}

func (this *Ringpop) Ready() bool {
	return this.ready
}

func (this *Ringpop) Pinging() bool {
	return this.pinging
}

func (this *Ringpop) SetJoinSize(size int) {
	this.joinSize = size
}

func (this *Ringpop) Lookup(key string) (string, bool) {
	dest, found := this.ring.lookup(key)
	if !found {
		this.logger.WithField("key", key).Debug("could not find destination for a key")
		return this.WhoAmI(), false
	}

	return dest, true
}

// EmitEvents enables or disables the channel returned by EventCh
// Passing true will enable the channel
// Passing false will disable the channel
func (this *Ringpop) EmitEvents(emit bool) {
	this.emitting = emit
}

// EventC returns a read only channel of events from ringpop.
func (this *Ringpop) EventC() <-chan string {
	return this.eventC
}

// Emit sends a message on the ringpop's eventC in a nonblocking fashion
func (this *Ringpop) emit(message string) {
	if this.emitting {
		go func() {
			this.eventC <- message
		}()
	}
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	RING EVENT HANDLING
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func (this *Ringpop) onRingChecksumComputed() {
	this.stat("increment", "ring.checksum-computed", 1)
	this.emit("ringChecksumComputed")
}

func (this *Ringpop) onRingServerAdded() {
	this.stat("increment", "ring.server-added", 1)
	this.emit("ringServerAdded")
}

func (this *Ringpop) onRingServerRemoved() {
	this.stat("increment", "ring.server-removed", 1)
	this.emit("ringServerRemoved")
}

func (this *Ringpop) launchRingEventHandler() chan<- bool {
	stop := make(chan bool, 1)

	go func() {
		for {
			select {
			case event := <-this.ringEventC:
				switch event {
				case "added":
					this.onRingServerAdded()
				case "removed":
					this.onRingServerRemoved()
				case "checksumComputed":
					this.onRingChecksumComputed()
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

func (this *Ringpop) onMemberAlive(change Change) {
	// TODO: add .stat call see .js for detail)
	// this.stat('increment', 'membership-update.alive');

	this.logger.WithFields(log.Fields{
		"local": this.WhoAmI(),
		"alive": change.Address,
	}).Debug("member is alive")

	this.dissemination.recordChange(change) // Propogate change
	this.ring.addServer(change.Address)     // Add server from ring
	this.suspicion.stop(change)             // Stop suspicion for server
}

func (this *Ringpop) onMemberFaulty(change Change) {
	// TODO: add .stat call (see .js for detail)
	// this.stat('increment', 'membership-update.faulty');

	this.logger.WithFields(log.Fields{
		"local":  this.WhoAmI(),
		"faulty": change.Address,
	}).Debug("member is faulty")

	this.ring.removeServer(change.Address)  // Remove server from ring
	this.suspicion.stop(change)             // Stop suspicion for server
	this.dissemination.recordChange(change) // Propogate change
}

func (this *Ringpop) onMemberSuspect(change Change) {
	// TODO: add .stat call and .log call (see .js for detail)
	// this.stat('increment', 'membership-update.suspect');

	this.logger.WithFields(log.Fields{
		"local":   this.WhoAmI(),
		"suspect": change.Address,
	}).Debug("member is suspect")

	this.suspicion.start(change)            // Start suspicion for server
	this.dissemination.recordChange(change) // Propogate change
}

func (this *Ringpop) onMemberLeave(change Change) {
	// TODO: add .stat call (see .js for detail)
	// this.stat('increment', 'membership-update.leave');

	this.logger.WithFields(log.Fields{
		"local": this.WhoAmI(),
		"leave": change.Address,
	}).Debug("member has left")

	this.dissemination.recordChange(change) // Propogate change
	this.ring.removeServer(change.Address)  // Remove server from ring
	this.suspicion.stop(change)             // Stop suspicion for server
}

func (this *Ringpop) launchMembershipChangeHandler() chan<- bool {
	var changes []Change

	stop := make(chan bool, 1)

	go func() {
		var membershipChanged, ringChanged bool

		for {
			membershipChanged, ringChanged = false, false

			select {
			case changes = <-this.membershipChangeC:
				for _, change := range changes {
					switch change.Status {
					case ALIVE:
						this.onMemberAlive(change)
						membershipChanged, ringChanged = true, true
					case FAULTY:
						this.onMemberFaulty(change)
						membershipChanged, ringChanged = true, true
					case SUSPECT:
						this.onMemberSuspect(change)
						membershipChanged, ringChanged = true, false
					case LEAVE:
						this.onMemberLeave(change)
						membershipChanged, ringChanged = true, true
					}
				}
			case <-stop:
				break
			}

			if membershipChanged {
				this.emit("membershipChanged")
			}

			if ringChanged {
				this.emit("ringChanged")
			}

			this.membershipUpdateRollup.trackUpdates(changes)

			this.stat("gauge", "num-members", int64(this.membership.memberCount()))
			this.stat("timing", "updates", int64(len(changes)))
		}
	}()

	return stop
}

// testing func
func (this *Ringpop) PingMember(address string) error {
	res, err := sendPing(this, address)
	if err != nil {
		this.logger.Errorf("error: %v", err)
		return err
	}
	this.pinging = false
	//this.membership.update(responseBody.changes)
	fmt.Printf("Checksum: %v\n", res.Checksum)
	fmt.Printf("Source: %s\n", res.Source)
	for _, change := range res.Changes {
		fmt.Printf("Address: %s, Status: %s, Incarnation: %v\n",
			change.Address, change.Status, change.Incarnation)
	}
	return nil
}

func (this *Ringpop) pingMemberNow() error {
	if this.pinging {
		this.logger.Warn("aborting ping because one is already in progress")
		return errors.New("ping aborted because a ping already in progress")
	}

	if !this.ready {
		this.logger.Warn("ping started before ring is initialized")
		return errors.New("ping started before ring is initialized")
	}

	iter := this.membership.iter()
	member, err := iter.next()
	if err != nil {
		this.logger.Warn("no usable nodes at protocol period")
		return errors.New("no usable nodes at protocol period")
	}

	this.pinging = true

	res, err := sendPing(this, member.Address)

	this.logger.Infof("Ping response checksum: %v", res.Checksum)

	if err == nil {
		this.pinging = false
		//this.membership.update(responseBody.changes)
		// TODO
		return nil
	}

	if this.destroyed {
		return errors.New("destroyed whilst pinging")
	}

	// Do a ping request otherwise

	return nil
}

func (this *Ringpop) TestPing(target string) {

}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	STATS(D) HANDLING
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func (this *Ringpop) stat(statType, key string, val int64) {
	fqkey, ok := this.statKeys[key]
	if !ok {
		fqkey = fmt.Sprintf("%s.%s", this.statPrefix, key)
		this.statKeys[key] = fqkey
	}

	switch statType {
	case "increment":
		this.statsd.Incr(fqkey, val)
	case "gauge":
		this.statsd.Gauge(fqkey, val)
	case "timing":
		this.statsd.Timing(fqkey, val)
	}
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	TESTING FUNCTIONS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func (this *Ringpop) allowJoins() {
	this.isDenyingJoins = false
}

func (this *Ringpop) denyJoins() {
	this.isDenyingJoins = true
}
