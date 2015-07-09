package ringpop

import (
	"math"
	"math/rand"
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/rcrowley/go-metrics"
)

const defaultMinProtocolPeriod = time.Millisecond * 200

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// GOSSIP
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

type gossip struct {
	ringpop *Ringpop
	stopped bool

	minProtocolPeriod  time.Duration
	numProtocolPeriods int64

	lastProtocolPeriod time.Time
	lastProtocolRate   time.Duration

	protocolTiming metrics.Histogram

	lock sync.Mutex // protects stopped flag
}

func newGossip(ringpop *Ringpop, minProtocolPeriod time.Duration) *gossip {
	if minProtocolPeriod <= 0 {
		minProtocolPeriod = defaultMinProtocolPeriod
	}

	gossip := &gossip{
		ringpop:           ringpop,
		stopped:           true,
		minProtocolPeriod: minProtocolPeriod,
		protocolTiming:    metrics.NewHistogram(metrics.NewUniformSample(10)), // best size for this?
	}

	gossip.protocolTiming.Update(int64(gossip.minProtocolPeriod))

	return gossip
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// METHODS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func (g *gossip) Stopped() bool {
	g.lock.Lock()
	defer g.lock.Unlock()
	return g.stopped
}

func (g *gossip) SetStopped(stopped bool) {
	g.lock.Lock()
	defer g.lock.Unlock()
	g.stopped = stopped
}

func (g *gossip) computeProtocolDelay() time.Duration {
	if g.numProtocolPeriods != 0 {
		target := g.lastProtocolPeriod.Add(g.lastProtocolRate)
		return time.Duration(math.Max(float64(target.UnixNano()-time.Now().UnixNano()), float64(g.minProtocolPeriod)))
	}
	// delay for first tick staggered in [0, minProtocolPeriod] ms
	return time.Duration(math.Floor(rand.Float64() * float64(g.minProtocolPeriod+1)))
}

func (g *gossip) computeProtocolRate() time.Duration {
	observed := g.protocolTiming.Percentile(0.5) * 2.0
	return time.Duration(math.Max(observed, float64(g.minProtocolPeriod)))
}

// runs a single gossip protocol period
func (g *gossip) gossip() {
	pingStartTime := time.Now()

	g.ringpop.PingMemberNow()

	g.lastProtocolPeriod = time.Now()
	g.numProtocolPeriods++
	g.protocolTiming.Update(int64(time.Now().Sub(pingStartTime))) // keeps protocol rate in check
}

func (g *gossip) run() {
	startTime := time.Now()

	go func() {
		for {
			if g.Stopped() {
				g.ringpop.logger.WithField("local", g.ringpop.WhoAmI()).
					Debug("[ringpop] stopped recurring gossip loop")
				break
			}

			protocolDelay := g.computeProtocolDelay()
			g.ringpop.stat("timing", "protocol.delay", milliseconds(protocolDelay))
			time.Sleep(protocolDelay)

			g.gossip()

			g.ringpop.stat("timing", "protocol.frequency", unixMilliseconds(startTime))
		}
	}()
}

func (g *gossip) start() {
	if !g.Stopped() {
		g.ringpop.logger.WithFields(log.Fields{
			"local": g.ringpop.WhoAmI(),
		}).Debug("[ringpop] gossip has already started")

		return
	}

	g.ringpop.membership.shuffle()
	g.run()
	g.startProtocolRateLoop()
	g.SetStopped(false)

	g.ringpop.logger.WithFields(log.Fields{
		"local": g.ringpop.WhoAmI(),
	}).Debug("[ringpop] started gossip protocol")
}

func (g *gossip) stop() {
	if g.Stopped() {
		g.ringpop.logger.WithFields(log.Fields{
			"local": g.ringpop.WhoAmI(),
		}).Debug("[ringpop] gossip is already stopped")

		return
	}

	g.SetStopped(true)

	g.ringpop.logger.WithFields(log.Fields{
		"local": g.ringpop.WhoAmI(),
	}).Debug("[ringpop] stopped gossip protocol")
}

// StartProtocolRateTimer launches a goroutine that sets lastProtocolRate every 1000ms
func (g *gossip) startProtocolRateLoop() {
	// launch goroutine that calculates last protocol rate periodically
	go func() {
		for !g.Stopped() {
			time.Sleep(1000 * time.Millisecond)
			g.lastProtocolRate = g.computeProtocolRate()
		}
	}()
}
