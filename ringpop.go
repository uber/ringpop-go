package ringpop

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/quipo/statsd"
	"github.com/rcrowley/go-metrics"
)

var hostPortPattern, hostPortErr = regexp.Compile("^([0-9]+.[0-9]+.[0-9]+.[0-9]+):[0-9]+$")

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	RINGPOP
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

type Ringpop struct {
	// Exported
	App      string
	HostPort string

	// Unexported
	eventC   chan string
	emitting bool

	bootstrapFile string
	joinSize      int

	ready     bool
	pinging   bool
	destroyed bool

	pingReqSize    int
	pingReqTimeout time.Duration
	pingTimeout    time.Duration

	ring       *HashRing
	ringEventC chan string

	// membership
	dissemination          *Dissemination
	membership             *Membership
	membershipChangeC      chan []Change
	membershipUpdateRollup *MembershipUpdateRollup

	// swim
	suspicion *Suspicion
	gossip    *Gossip

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

func NewRingpop(app, hostport string) *Ringpop {
	// verify valid hostport
	if match := hostPortPattern.Match([]byte(hostport)); !match {
		log.Fatal("Invalid host port ", hostport)
	}

	ringpop := &Ringpop{
		App:      app,
		HostPort: hostport,
	}

	ringpop.pingReqSize = 3
	ringpop.pingReqTimeout = time.Millisecond * 3000
	ringpop.pingTimeout = time.Millisecond * 200

	ringpop.ready = false
	ringpop.pinging = false

	ringpop.membership = NewMembership(ringpop)
	ringpop.membershipChangeC = make(chan []Change)
	// ringpop.launchMembershipChangeHandler()
	ringpop.membershipUpdateRollup = NewMembershipUpdateRollup(ringpop, 5000*time.Millisecond, 0)

	ringpop.suspicion = NewSuspicion(ringpop, 3000*time.Millisecond)
	ringpop.gossip = NewGossip(ringpop, -1)

	// launch event handling functions
	ringpop.ring = NewHashring()
	// ringpop.launchRingEventHandler()

	ringpop.statsd = statsd.NoopClient{} // TODO: change this to an actual client
	ringpop.statKeys = make(map[string]string, 0)
	// changes 0.0.0.0:0000 -> 0_0_0_0_0000
	ringpop.statHostPort = strings.Replace(ringpop.HostPort, ".", "_", -1)
	ringpop.statHostPort = strings.Replace(ringpop.statHostPort, ":", "_", -1)
	ringpop.statPrefix = fmt.Sprintf("ringpop.%s", ringpop.statHostPort)

	ringpop.logger = log.StandardLogger() // temporary
	ringpop.logger.Formatter = new(log.JSONFormatter)

	ringpop.destroyed = false
	// ringpop.joiner = nil TODO

	return ringpop
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
		// make this happen in a goroutine for non-blocking mode?
		this.eventC <- message
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
	this.ring.addServer(change.Address())   // Add server from ring
	this.suspicion.stop(change)             // Stop suspicion for server
}

func (this *Ringpop) onMemberFaulty(change Change) {
	// TODO: add .stat call (see .js for detail)
	// this.stat('increment', 'membership-update.faulty');

	this.logger.WithFields(log.Fields{
		"local":  this.WhoAmI(),
		"faulty": change.Address(),
	}).Debug("member is faulty")

	this.ring.removeServer(change.Address()) // Remove server from ring
	this.suspicion.stop(change)              // Stop suspicion for server
	this.dissemination.recordChange(change)  // Propogate change
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
		"leave": change.Address(),
	}).Debug("member has left")

	this.dissemination.recordChange(change)  // Propogate change
	this.ring.removeServer(change.Address()) // Remove server from ring
	this.suspicion.stop(change)              // Stop suspicion for server
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
					switch change.Status() {
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

func (this *Ringpop) pingMemberNow(returnCh chan<- string) {
	if this.pinging {
		this.logger.Warn("aborting ping because one is already in progress")
		returnCh <- "already pinging"
		return
	}

	if !this.ready {
		this.logger.Warn("ping started before ring is initialized")
		returnCh <- "not ready"
		return
	}

	iter := this.membership.iter()
	_, err := iter.next()
	if err != nil {
		this.logger.Warn("no usable nodes at protocol period")
		returnCh <- "no usable nodes at protocol period"
		return
	}

	this.pinging = true

	// sendPing(this, member) // this should internally block with goroutine

	//
	// TODO
	//
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
