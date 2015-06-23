package ringpop

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"strings"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/uber/tchannel/golang"

	"github.com/quipo/statsd"
	"github.com/rcrowley/go-metrics"
)

const (
	// defaultMaxJoinDuration                = 300000 * time.Millisecond
	defaultMembershipUpdateFlushInterval = 5000 * time.Millisecond
	defaultProxyReqTimeout               = 30000 * time.Millisecond
)

var (
	proxyReqProps = []string{"keys", "dest", "req", "res"}
)

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	RINGPOP
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

// Options provides opts for creation of a ringpop
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

	bootstrapFile     string
	bootstrapFromFile bool
	bootstrapHosts    map[string][]string
	joinSize          int

	ready     bool
	pinging   bool
	destroyed bool

	pingReqSize int

	pingReqTimeout                time.Duration
	pingTimeout                   time.Duration
	proxyReqTimeout               time.Duration
	maxJoinDuration               time.Duration
	membershipUpdateFlushInterval time.Duration

	ring *hashRing

	// membership
	dissemination          *dissemination
	membership             *membership
	MembershipIter         *membershipIter
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
	statLock     sync.Mutex

	clientRate metrics.Meter
	serverRate metrics.Meter
	totalRate  metrics.Meter

	// logging
	logger *log.Logger

	// testing
	isDenyingJoins bool
}

// NewRingpop creates a new ringpop on the specified hostport
func NewRingpop(app, hostport string, opts Options) *Ringpop {
	// verify valid hostport
	if match := hostPortPattern.Match([]byte(hostport)); !match {
		log.Fatal("Invalid host port ", hostport)
	}

	ringpop := &Ringpop{
		App:      app,
		HostPort: hostport,
	}

	ringpop.eventC = make(chan string, 25)

	ringpop.ready = false
	ringpop.pinging = false

	// set channel to option
	if ringpop.channel = nil; opts.Channel != nil {
		ringpop.channel = opts.Channel
	}
	// set logger to option or default value
	if ringpop.logger = log.StandardLogger(); opts.Logger != nil {
		ringpop.logger = opts.Logger
	}
	// set statsd client to option or default value
	if ringpop.statsd = (&statsd.NoopClient{}); opts.Statsd != nil {
		ringpop.statsd = opts.Statsd
	}

	ringpop.bootstrapFile = opts.BootstrapFile
	ringpop.bootstrapHosts = make(map[string][]string)
	ringpop.joinSize = opts.JoinSize
	ringpop.maxJoinDuration = selectDurationOrDefault(opts.MaxJoinDuration, defaultMaxJoinDuration)

	ringpop.pingTimeout = time.Millisecond * 1500
	ringpop.pingReqSize = 3
	ringpop.pingReqTimeout = time.Millisecond * 5000

	// set proxyReqTimeout to option or default value
	ringpop.proxyReqTimeout = selectDurationOrDefault(opts.ProxyReqTimeout, defaultProxyReqTimeout)

	ringpop.membershipUpdateFlushInterval = selectDurationOrDefault(
		opts.MembershipUpdateFlushInterval,
		defaultMembershipUpdateFlushInterval)

	// membership and gossip
	ringpop.ring = newHashRing(ringpop)
	ringpop.dissemination = newDissemination(ringpop)
	ringpop.membership = newMembership(ringpop)
	ringpop.membershipUpdateRollup = newMembershipUpdateRollup(ringpop,
		ringpop.membershipUpdateFlushInterval, 0)
	ringpop.suspicion = newSuspicion(ringpop, opts.SuspicionTimeout)
	ringpop.gossip = newGossip(ringpop, 0)
	// statsd
	// changes 0.0.0.0:0000 -> 0_0_0_0_0000
	ringpop.statKeys = make(map[string]string)
	ringpop.statHostPort = strings.Replace(ringpop.HostPort, ".", "_", -1)
	ringpop.statHostPort = strings.Replace(ringpop.statHostPort, ":", "_", -1)
	ringpop.statPrefix = fmt.Sprintf("ringpop.%s", ringpop.statHostPort)

	ringpop.destroyed = false

	return ringpop
}

// Destroy stops Ringpops
func (rp *Ringpop) Destroy() {
	rp.logger.WithField("local", rp.WhoAmI()).Debug("destroyed ringpop")
	rp.destroyed = true
	rp.gossip.stop()
	rp.suspicion.stopAll()
	rp.membershipUpdateRollup.destroy()

	// Sleep momentarily to allow sends on the ring event channel that may
	// have been started by the membership change channel to complete
	close(rp.eventC)

	// clientRate/serverRate/totalRate stuff

	if rp.channel != nil {
		rp.channel.Close()
	}
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//  RINGPOP SETUP AND BOOTSTRAPPING
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

// SetupChannel sets up the ringpop's TChannel. Has no effect if the ringpop's
// channel is nil.
func (rp *Ringpop) SetupChannel() error {
	if rp.channel != nil {
		newRingpopTChannel(rp, rp.channel)
	}
	return errors.New("ringpop must have a non-nil channel")
}

// BootstrapOptions provieds options to bootstrap ringpop with. The bootstrapper
// will prioritize a slice of given hosts over a file if both are provided.
type BootstrapOptions struct {
	Hosts []string
	File  string
}

// Bootstrap seeds the hash ring, joins nodes in the seed list, and then starts
// the gossip protocol. Returns a slice of strings containing the hostports of
// the nodes joined to the ringpop and a nil error, or a nil slice and an error
// if the bootstrap failed.
func (rp *Ringpop) Bootstrap(opts BootstrapOptions) ([]string, error) {
	if err := rp.seedBootstrapHosts(opts); err != nil {
		return nil, fmt.Errorf("Failed to bootstrap ringpop: %v", err)
	}

	if len(rp.bootstrapHosts) == 0 {
		message := `Ringpop cannot be bootstrapped without bootstrap hosts.
			Make sure you specify a valid bootstrap hosts file to the ringpop
			or have a valid hosts.json file in the current working directory.`
		return nil, errors.New(message)
	}

	rp.checkForMissingBootstrapHost()
	rp.checkForHostnameIPMismatch()

	// make self alive
	rp.membership.makeAlive(rp.WhoAmI(), unixMilliseconds(time.Now()), "")

	nodesJoined, err := sendJoin(rp)
	if err != nil {
		rp.logger.WithFields(log.Fields{
			"err":     err.Error(),
			"address": rp.HostPort,
		}).Error("ringpop bootstrap failed")
		return nil, err
	}

	if rp.destroyed {
		message := "ringpop was destroyed during bootstrap"
		rp.logger.WithField("address", rp.HostPort).Error(message)
		return nil, errors.New(message)
	}

	// rp.gossip.start()
	rp.ready = true
	rp.emit("ready")

	return nodesJoined, nil
}

func (rp *Ringpop) checkForMissingBootstrapHost() bool {
	if indexOf(rp.bootstrapHosts[captureHost(rp.HostPort)], rp.HostPort) == -1 {
		message := `Bootstrap hosts does not include the host:port of the local
			node. This may be fine because your hosts file may just be slightly
			out of date, but it could also be an indication that your node is
			identifying itself incorrectly.`

		rp.logger.WithField("address", rp.HostPort).Warn(message)
		return false
	}
	return true
}

func (rp *Ringpop) checkForHostnameIPMismatch() bool {
	testMismatch := func(message string, filter func(string) bool) bool {
		var mismatched []string
		for _, hosts := range rp.bootstrapHosts {
			for _, host := range hosts {
				if filter(host) {
					mismatched = append(mismatched, host)
				}
			}
		}

		if len(mismatched) > 0 {
			rp.logger.WithFields(log.Fields{
				"address":                  rp.HostPort,
				"mismatchedBootstrapHosts": mismatched,
			}).Warn(message)

			return false
		}

		return true
	}

	if hostPortPattern.Match([]byte(rp.HostPort)) {
		ipMessage := `Your ringpop host identifier looks like an IP address and 
			there are bootstrap hosts that appear to be specified with hostnames. 
			These inconsistencies may lead to subtle node communication issues.`

		return testMismatch(ipMessage, func(host string) bool {
			return !hostPortPattern.Match([]byte(host))
		})
	}

	hostMessage := `Your ringpop host identifier looks like a hsotname and there 
		are bootstrap hosts that appear to be specified with IP addresses. These 
		inconsistencies may lead to subtle node communication issues`

	return testMismatch(hostMessage, func(host string) bool {
		return hostPortPattern.Match([]byte(host))
	})
}

// PrintBootstrapHosts is for testing
func (rp *Ringpop) PrintBootstrapHosts() {
	for _, hosts := range rp.bootstrapHosts {
		for _, host := range hosts {
			fmt.Println(host)
		}
	}
}

// PrintMembership is for testing
func (rp *Ringpop) PrintMembership() {
	for _, member := range rp.membership.members {
		fmt.Println(member.Address, member.Status)
	}
}

func (rp *Ringpop) seedBootstrapHosts(opts BootstrapOptions) error {
	var hosts []string
	var err error

	if opts.Hosts != nil {
		hosts = opts.Hosts
	} else {
		switch true {
		case true:
			if opts.File != "" {
				hosts, err = rp.readHostsFile(opts.File)
				if err == nil {
					break
				}
				rp.logger.WithField("file", opts.File).Warnf("Could not read host file: %v", err)
			}
			fallthrough
		case true:
			if rp.bootstrapFile != "" {
				hosts, err = rp.readHostsFile(rp.bootstrapFile)
				if err == nil {
					break
				}
				rp.logger.WithField("file", rp.bootstrapFile).Warnf("Could not read host file: %v", err)
			}
			fallthrough
		case true:
			hosts, err = rp.readHostsFile("./hosts.json")
			if err == nil {
				break
			}
			rp.logger.WithField("file", "./hosts.json").Warnf("Could not read host file: %v", err)
			return errors.New("unable to read hosts file")
		}
	}

	for _, hostport := range hosts {
		host := captureHost(hostport)
		if host == "" {
			continue
		}
		rp.bootstrapHosts[host] = append(rp.bootstrapHosts[host], hostport)
	}

	return nil
}

func (rp *Ringpop) readHostsFile(file string) ([]string, error) {
	var hosts []string

	data, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	err = json.Unmarshal(data, &hosts)
	if err != nil {
		return nil, err
	}

	return hosts, err
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
	dest, ok := rp.ring.lookup(key)
	if !ok {
		rp.logger.WithField("key", key).Debug("could not find destination for a key")
		return rp.WhoAmI(), false
	}

	return dest, true
}

// SetEmitting enables or disables the channel returned by EventC.
func (rp *Ringpop) SetEmitting(emit bool) {
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

func (rp *Ringpop) handleRingEvent(event string) {
	switch event {
	case "added":
		rp.onRingServerAdded()
	case "removed":
		rp.onRingServerRemoved()
	case "checksumComputed":
		rp.onRingChecksumComputed()
	}
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	MEMBERSHIP EVENT HANDLING
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func (rp *Ringpop) onMemberAlive(change Change) {
	rp.stat("increment", "membership-update.alive", 1)
	rp.logger.WithFields(log.Fields{
		"local":       rp.WhoAmI(),
		"alive":       change.Address,
		"incarnation": change.Incarnation,
	}).Debug("member is alive")

	rp.dissemination.recordChange(change) // Propogate change
	rp.ring.addServer(change.Address)     // Add node to ring
	rp.suspicion.stop(change)             // Stop suspicion for node
}

func (rp *Ringpop) onMemberFaulty(change Change) {
	rp.stat("increment", "membership-update.faulty", 1)
	rp.logger.WithFields(log.Fields{
		"local":       rp.WhoAmI(),
		"faulty":      change.Address,
		"incarnation": change.Incarnation,
	}).Debug("member is faulty")

	rp.ring.removeServer(change.Address)  // Remove node from ring
	rp.suspicion.stop(change)             // Stop suspicion for node
	rp.dissemination.recordChange(change) // Propogate change
}

func (rp *Ringpop) onMemberSuspect(change Change) {
	rp.stat("increment", "membership-update.suspect", 1)
	rp.logger.WithFields(log.Fields{
		"local":       rp.WhoAmI(),
		"suspect":     change.Address,
		"incarnation": change.Incarnation,
	}).Debug("member is suspect")

	rp.suspicion.start(change)            // Start suspicion for node
	rp.dissemination.recordChange(change) // Propogate change
}

func (rp *Ringpop) onMemberLeave(change Change) {
	rp.stat("increment", "membership-update.leave", 1)
	rp.logger.WithFields(log.Fields{
		"local":       rp.WhoAmI(),
		"leave":       change.Address,
		"incarnation": change.Incarnation,
	}).Debug("member has left")

	rp.dissemination.recordChange(change) // Propogate change
	rp.ring.removeServer(change.Address)  // Remove server from ring
	rp.suspicion.stop(change)             // Stop suspicion for node
}

func (rp *Ringpop) handleChanges(changes []Change) {
	var membershipChanged, ringChanged bool

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

	if membershipChanged {

	}

	if ringChanged {
		rp.dissemination.eventC <- "changed"
		rp.emit("ringChanged")
	}

	rp.membershipUpdateRollup.trackUpdates(changes)

	rp.stat("gauge", "num-members", int64(rp.membership.memberCount()))
	rp.stat("timing", "updates", int64(len(changes)))
}

// PingMember is temporary testing func
func (rp *Ringpop) PingMember(address string) error {
	res, err := sendPing(rp, address)
	if err != nil {
		rp.logger.Debugf("ping failed: %v", err)
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
	if err != nil {
		rp.logger.WithFields(log.Fields{
			"local":  rp.WhoAmI(),
			"remote": member.Address,
		}).Debugf("ping failed: %v", err)

		// do a ping-req
	}

	if rp.destroyed {
		return errors.New("destroyed whilst pinging")
	}

	rp.pinging = false
	// rp.membership.update(res.Changes)
	// TODO

	fmt.Printf("Checksum: %v\n", res.Checksum)
	fmt.Printf("Source: %s\n", res.Source)
	for _, change := range res.Changes {
		fmt.Printf("Address: %s, Status: %s, Incarnation: %v\n",
			change.Address, change.Status, change.Incarnation)
	}

	return nil
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	STATS(D) HANDLING
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func (rp *Ringpop) stat(statType, key string, val int64) {
	rp.statLock.Lock()
	fqkey, ok := rp.statKeys[key]
	if !ok {
		fqkey = fmt.Sprintf("%s.%s", rp.statPrefix, key)
		rp.statKeys[key] = fqkey
	}
	rp.statLock.Unlock()

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

func (rp *Ringpop) testBootstrapper() {
	rp.membership.makeAlive(rp.WhoAmI(), unixMilliseconds(time.Now()), "")
	rp.ready = true
}

// SetDebug enables or disables the logging debug flag
func (rp *Ringpop) SetDebug(debug bool) {
	if debug {
		log.SetLevel(log.DebugLevel)
	} else {
		log.SetLevel(log.InfoLevel)
	}
}
