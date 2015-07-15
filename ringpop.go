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
	defaultMembershipUpdateFlushInterval = 5000 * time.Millisecond
	defaultProxyReqTimeout               = 30000 * time.Millisecond
	defaultProxyMaxRetries               = 3
)

var (
	proxyReqProps             = []string{"keys", "dest", "req", "res"}
	defaultProxyRetrySchedule = []time.Duration{3 * time.Millisecond, 6 * time.Millisecond, 12 * time.Millisecond}
)

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	RINGPOP
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

// Options provides opts for creation of a ringpop
type Options struct {
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
	RequestProxyRetrySchedule []time.Duration // revist

	MinProtocolPeriod time.Duration
	SuspicionTimeout  time.Duration
}

// TODO: ringpop comment description

// Ringpop is a ringpop is a ringpop is a ringpop
type Ringpop struct {
	app      string
	hostPort string

	channel *tchannel.Channel

	eventC   chan string
	emitting bool

	bootstrapFile     string
	bootstrapFromFile bool
	bootstrapHosts    map[string][]string
	joinSize          int

	ready         bool
	pinging       bool
	destroyed     bool
	destroyedLock sync.Mutex

	pingReqSize int

	pingReqTimeout                time.Duration
	pingTimeout                   time.Duration
	proxyReqTimeout               time.Duration
	proxyRetrySchedule            []time.Duration
	proxyMaxRetries               int
	maxJoinDuration               time.Duration
	membershipUpdateFlushInterval time.Duration

	// membership
	ring                   *hashRing
	membership             *membership
	membershipIter         *membershipIter
	membershipUpdateRollup *membershipUpdateRollup

	// gossip
	suspicion     *suspicion
	gossip        *gossip
	dissemination *dissemination

	// proxy
	proxy *proxy

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

// NewRingpop creates a new ringpop on the specified hostport.
func NewRingpop(app, hostport string, channel *tchannel.Channel, opts *Options) *Ringpop {
	if opts == nil {
		// opts should contain zero values for each option
		opts = &Options{}
	}

	// verify valid hostport
	if match := hostPortPattern.Match([]byte(hostport)); !match {
		log.Fatal("Invalid host port ", hostport)
	}

	if channel == nil {
		log.Fatal("Ringpop requires non-nil channel to be created")
	}

	ringpop := &Ringpop{
		app:      app,
		hostPort: hostport,
		channel:  channel,
	}

	ringpop.eventC = make(chan string, 25)

	ringpop.ready = false
	ringpop.pinging = false

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

	ringpop.pingTimeout = time.Millisecond * 1500 // 1500
	ringpop.pingReqSize = 3
	ringpop.pingReqTimeout = time.Millisecond * 5000 // 5000

	// set proxyReqTimeout to option or default value
	// set proxyReqTimeout to option or default value
	ringpop.proxyReqTimeout = selectDurationOrDefault(opts.ProxyReqTimeout, defaultProxyReqTimeout)

	ringpop.membershipUpdateFlushInterval = selectDurationOrDefault(
		opts.MembershipUpdateFlushInterval,
		defaultMembershipUpdateFlushInterval)
	ringpop.proxyRetrySchedule = make([]time.Duration, len(defaultProxyRetrySchedule))
	copy(ringpop.proxyRetrySchedule, defaultProxyRetrySchedule)
	ringpop.proxyMaxRetries = defaultProxyMaxRetries

	// membership and gossip
	ringpop.ring = newHashRing(ringpop)
	ringpop.dissemination = newDissemination(ringpop)
	ringpop.membership = newMembership(ringpop)
	ringpop.membershipUpdateRollup = newMembershipUpdateRollup(ringpop,
		ringpop.membershipUpdateFlushInterval, 0)
	ringpop.suspicion = newSuspicion(ringpop, opts.SuspicionTimeout)
	ringpop.gossip = newGossip(ringpop, 0)
	ringpop.proxy = newProxy(ringpop, ringpop.proxyRetrySchedule, ringpop.proxyMaxRetries)

	// statsd
	// changes 0.0.0.0:0000 -> 0_0_0_0_0000
	ringpop.statKeys = make(map[string]string)
	ringpop.statHostPort = strings.Replace(ringpop.hostPort, ".", "_", -1)
	ringpop.statHostPort = strings.Replace(ringpop.statHostPort, ":", "_", -1)
	ringpop.statPrefix = fmt.Sprintf("ringpop.%s", ringpop.statHostPort)

	ringpop.setDestroyed(false)

	return ringpop
}

// Destroyed returns true if the ringpop has been destryoed, and false otherwise
func (rp *Ringpop) Destroyed() bool {
	rp.destroyedLock.Lock()
	defer rp.destroyedLock.Unlock()
	return rp.destroyed
}

func (rp *Ringpop) setDestroyed(destroyed bool) {
	rp.destroyedLock.Lock()
	defer rp.destroyedLock.Unlock()
	rp.destroyed = destroyed
}

// Destroy kills the ringpop
func (rp *Ringpop) Destroy() {
	rp.setDestroyed(true)
	rp.gossip.stop()
	rp.suspicion.stopAll()
	rp.membershipUpdateRollup.destroy()

	close(rp.eventC)

	// clientRate/serverRate/totalRate stuff

	rp.channel.Close()
	rp.logger.WithFields(log.Fields{
		"local":     rp.WhoAmI(),
		"timestamp": time.Now(),
	}).Debug("[ringpop] closed channel")

	rp.logger.WithField("local", rp.WhoAmI()).Debug("[ringpop] destroyed")
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//  RINGPOP SETUP AND BOOTSTRAPPING
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

// listenAndServe registers ringpop's endpoints with its channel and then sets up
// the server
func (rp *Ringpop) listenAndServe() error {
	s, err := newServer(rp)
	if err != nil {
		return err
	}

	return s.listenAndServe()
}

// BootstrapOptions provieds options to bootstrap ringpop with. The bootstrapper
// will prioritize a slice of given hosts over a file if both are provided.
type BootstrapOptions struct {
	// Hosts to bootstrap with
	Hosts []string

	// File containg json array of hosts
	File string

	// Whether or not gossip stopped
	Stopped bool

	// Join timeout
	Timeout time.Duration

	// Maximum time to attempt joins
	MaxJoinDuration time.Duration

	// number of nodes to attempt to join
	ParallelismFactor int

	// number of nodes to join
	JoinSize int
}

// Bootstrap seeds the hash ring, joins nodes in the seed list, and then starts
// the gossip protocol. Returns a slice of strings containing the hostports of
// the nodes joined to the ringpop and a nil error, or a nil slice and an error
// if the bootstrap failed.
func (rp *Ringpop) Bootstrap(opts *BootstrapOptions) ([]string, error) {
	if opts == nil {
		opts = &BootstrapOptions{}
	}

	// set up channel
	if err := rp.listenAndServe(); err != nil {
		rp.logger.WithFields(log.Fields{}).Fatalf("[ringpop] could not setup channel: %v", err)
		return nil, err
	}

	if err := rp.seedBootstrapHosts(opts); err != nil {
		return nil, fmt.Errorf("could not seed bootstrap hosts: %v", err)
	}

	if len(rp.bootstrapHosts) == 0 {
		message := "Ringpop cannot be bootstrapped without bootstrap hosts. Make sure " +
			"you specify a valid bootstrap hosts file to the ringpop or have a valid " +
			"hosts.json file in the current working directory."
		return nil, errors.New(message)
	}

	rp.checkForMissingBootstrapHost()
	rp.checkForHostnameIPMismatch()

	// make self alive
	rp.membership.makeAlive(rp.WhoAmI(), unixMilliseconds(time.Now()))

	joinOpts := &joinerOptions{
		timeout:           opts.Timeout,
		joinSize:          opts.JoinSize,
		maxJoinDuration:   opts.MaxJoinDuration,
		parallelismFactor: opts.ParallelismFactor,
	}

	nodesJoined, err := sendJoin(rp, joinOpts)
	if err != nil {
		rp.logger.WithFields(log.Fields{
			"err":     err.Error(),
			"address": rp.hostPort,
		}).Error("[ringpop] bootstrap failed")
		return nil, err
	}

	if rp.Destroyed() {
		message := "[ringpop] destroyed during bootstrap process"
		rp.logger.WithField("address", rp.hostPort).Error(message)
		return nil, errors.New(message)
	}

	if !opts.Stopped {
		rp.gossip.start()
	}
	rp.ready = true
	rp.emit("ready")

	return nodesJoined, nil
}

// TODO: fix message
func (rp *Ringpop) checkForMissingBootstrapHost() bool {
	if indexOf(rp.bootstrapHosts[captureHost(rp.hostPort)], rp.hostPort) == -1 {
		message := "[ringpop] Bootstrap hosts does not include the host:port of the " +
			"local node. This may be fine because your hosts file may just be slightly " +
			"out of date, but it could also be an indication that your node is " +
			"identifying itself incorrectly."
		rp.logger.WithField("address", rp.hostPort).Warn(message)
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
				"address":                  rp.hostPort,
				"mismatchedBootstrapHosts": mismatched,
			}).Warn(message)

			return false
		}

		return true
	}

	if hostPortPattern.Match([]byte(rp.hostPort)) {
		ipMessage := "[ringpop] Your ringpop host identifier looks like an IP address " +
			"and there are bootstrap hosts that appear to be specified with hostnames. " +
			"These inconsistencies may lead to subtle node communication issues."

		return testMismatch(ipMessage, func(host string) bool {
			return !hostPortPattern.Match([]byte(host))
		})
	}

	hostMessage := "[ringpop] Your ringpop host identifier looks like a hostname and " +
		"there are bootstrap hosts that appear to be specified with IP addresses. " +
		"These inconsistencies may lead to subtle node communication issues"

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

func (rp *Ringpop) seedBootstrapHosts(opts *BootstrapOptions) error {
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
				rp.logger.WithField("file", opts.File).Warnf("[ringpop] could not read host file: %v", err)
			}
			fallthrough
		case true:
			if rp.bootstrapFile != "" {
				hosts, err = rp.readHostsFile(rp.bootstrapFile)
				if err == nil {
					break
				}
				rp.logger.WithField("file", rp.bootstrapFile).Warnf("[ringpop] could not read host file: %v", err)
			}
			fallthrough
		case true:
			hosts, err = rp.readHostsFile("./hosts.json")
			if err == nil {
				break
			}
			rp.logger.WithField("file", "./hosts.json").Warnf("[ringpop] could not read host file: %v", err)
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
	return rp.hostPort
}

// App returns the name of the service ringpop is bound to
func (rp *Ringpop) App() string {
	return rp.app
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
		rp.logger.WithField("key", key).Debug("[ringpop] could not find destination for a key")
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
		"source":      change.Source,
	}).Debug("[ringpop] member is alive")

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
		"source":      change.Source,
	}).Debug("[ringpop] member is faulty")

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
		"source":      change.Source,
	}).Debug("[ringpop] member is suspect")

	rp.suspicion.start(change)            // Start suspicion for node
	rp.dissemination.recordChange(change) // Propogate change
}

func (rp *Ringpop) onMemberLeave(change Change) {
	rp.stat("increment", "membership-update.leave", 1)
	rp.logger.WithFields(log.Fields{
		"local":       rp.WhoAmI(),
		"leave":       change.Address,
		"incarnation": change.Incarnation,
		"source":      change.Source,
	}).Debug("[ringpop] member has left")

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
		rp.emit("membershipChanged")
	}

	if ringChanged {
		rp.dissemination.onRingChange()
		rp.emit("ringChanged")
	}

	rp.membershipUpdateRollup.trackUpdates(changes)

	rp.stat("gauge", "num-members", int64(rp.membership.memberCount()))
	rp.stat("timing", "updates", int64(len(changes)))
}

// PingMember is temporary testing func
func (rp *Ringpop) PingMember(target Member) error {
	_, err := sendPing(rp, target.Address, rp.pingTimeout)
	if err != nil {
		rp.logger.WithFields(log.Fields{
			"local":  rp.WhoAmI(),
			"remote": target.Address,
		}).Infof("ping failed: %v", err)
		sendPingReqs(rp, target, rp.pingReqSize)
		return err
	}
	rp.pinging = false
	//rp.membership.update(responseBody.changes)

	return nil
}

// PingMemberNow temporarily exported for testing purposes
func (rp *Ringpop) PingMemberNow() error {
	if rp.pinging {
		rp.logger.Warn("[ringpop] aborting ping because one is already in progress")
		return errors.New("ping aborted because a ping already in progress")
	}

	if !rp.ready {
		rp.logger.Warn("[ringpop] ping started before ring is initialized")
		return errors.New("ping started before ring is initialized")
	}

	iter := rp.membership.iter()
	member, ok := iter.next()
	if !ok {
		rp.logger.Warn("[ringpop] no usable nodes at protocol period")
		return errors.New("no usable nodes at protocol period")
	}

	rp.pinging = true

	res, err := sendPing(rp, member.Address, rp.pingTimeout)
	if err != nil {
		rp.logger.WithFields(log.Fields{
			"local":  rp.WhoAmI(),
			"remote": member.Address,
			"error":  err,
		}).Info("[ringpop] ping failed")

		sendPingReqs(rp, *member, rp.pingReqSize)

		rp.pinging = false
		return err
	}

	if rp.Destroyed() {
		return errors.New("destroyed whilst pinging")
	}

	rp.pinging = false
	rp.membership.update(res.Changes)

	rp.logger.WithFields(log.Fields{
		"local":  rp.WhoAmI(),
		"remote": member.Address,
	}).Debug("[ringpop] ping success")

	return nil
}

// PingReqNow is for testing
func (rp *Ringpop) PingReqNow(peer, target string) {
	sendPingReq(rp, peer, target)
}

func (rp *Ringpop) validateProps(opts map[string]interface{}, props []string) error {
	for _, val := range props {
		if opts[val] == "" {
			rp.logger.Warn("[ringpop] invalid options for: %s", val)
			return errors.New("invalid options for proxy")
		}
	}
	return nil
}

func (rp *Ringpop) proxyReq(opts map[string]interface{}) error {
	var err error

	if opts != nil {
		if rp.validateProps(opts, proxyReqProps) != nil {
			log.Fatal("invalid options for proxy request")
		}
		err = rp.proxy.proxyRequest(opts)
	} else {
		log.Fatal("specify valid options to proxy the request")
	}
	return err
}

func (rp *Ringpop) handleOrProxy(key string, req *proxyReq, res *proxyReqRes, opts map[string]interface{}) bool {
	rp.logger.WithFields(log.Fields{
		"local": rp.WhoAmI(),
		"url":   req.Header.URL,
		"key":   key,
	}).Debug("[ringpop] handleOrProxy for a key received")

	dest, _ := rp.ring.lookup(key)

	if rp.WhoAmI() == dest {
		rp.logger.WithFields(log.Fields{
			"key":  key,
			"dest": dest,
		}).Debug("[ringpop] handleOrProxy was handled")
		return true
	}

	// Proxy
	rp.logger.WithFields(log.Fields{
		"key":  key,
		"dest": dest,
	}).Debug("[ringpop] handleOrProxy was proxied")
	opts = map[string]interface{}{
		"dest": dest,
		"keys": []string{key},
		"req":  req,
		"res":  res,
	}

	rp.proxyReq(opts)
	return false
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
	rp.membership.makeAlive(rp.WhoAmI(), unixMilliseconds(time.Now()))
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
