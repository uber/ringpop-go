package ringpop

import (
	"math"
	"math/rand"
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

	minProtocolPeriod   time.Duration
	numProtocolPeriods  int64
	protocolPeriodTimer *time.Ticker

	lastProtocolPeriod time.Duration
	lastProtocolRate   int64
	protocolRateTimer  *time.Ticker

	protocolTiming metrics.StandardHistogram
}

func newGossip(ringpop *Ringpop, minProtocolPeriod time.Duration) *gossip {
	if minProtocolPeriod <= 0 {
		minProtocolPeriod = defaultMinProtocolPeriod
	}

	gossip := &gossip{
		ringpop:           ringpop,
		stopped:           true,
		minProtocolPeriod: minProtocolPeriod,
		protocolTiming:    metrics.StandardHistogram{},
	}

	return gossip
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// METHODS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func (g *gossip) computeProtocolDelay() time.Duration {
	if g.numProtocolPeriods != 0 {
		target := g.lastProtocolPeriod + time.Duration(g.lastProtocolRate)
		return time.Duration(math.Max(float64(int64(target)-unixMilliseconds()), float64(g.minProtocolPeriod))) // REVISIT
	}

	// delay for first tick staggered in [0, minProtocolPeriod] ms
	return time.Duration(math.Floor(rand.Float64() * float64(g.minProtocolPeriod+1)))
}

func (g *gossip) computeProtocolRate() int64 {
	observed := g.protocolTiming.Percentiles([]float64{0.5})[1] * 2.0
	return int64(math.Max(observed, float64(g.minProtocolPeriod)))
}

// TODO
func (g *gossip) run() {
	protocolDelay := g.computeProtocolDelay()

	g.protocolPeriodTimer = time.NewTicker(protocolDelay + 1)
	g.ringpop.stat("timing", "protocol.delay", int64(protocolDelay))

	go func() {
		for {
			if g.protocolPeriodTimer != nil {
				<-g.protocolPeriodTimer.C
				// pingStartTime := time.Now()

				// TODO: ...something something ping something something

			} else {
				break
			}
		}
	}()
}

func (g *gossip) start() {
	if !g.stopped {
		g.ringpop.logger.WithFields(log.Fields{
			"local": g.ringpop.WhoAmI(),
		}).Debug("gossip has already started")

		return
	}

	// g.ringpop.membership.shuffle() // REVISIT
	g.run()
	g.startProtocolRateTimer()
	g.stopped = false

	g.ringpop.logger.WithFields(log.Fields{
		"local": g.ringpop.WhoAmI(),
	}).Debug("started gossip protocol")
}

func (g *gossip) stop() {
	if g.stopped {
		g.ringpop.logger.WithFields(log.Fields{
			"local": g.ringpop.WhoAmI(),
		}).Warn("gossip is already stopped")

		return
	}

	g.protocolRateTimer.Stop()
	g.protocolRateTimer = nil

	g.protocolPeriodTimer.Stop()
	g.protocolPeriodTimer = nil

	g.stopped = true

	g.ringpop.logger.WithFields(log.Fields{
		"local": g.ringpop.WhoAmI(),
	}).Debug("stopped gossip protocol")
}

// StartProtocolRateTimer creates a ticker and launches a goroutine that
// sets lastProtocolRate every 1000ms, returns a channel that can be used
// to exit out of the function
func (g *gossip) startProtocolRateTimer() {
	g.protocolRateTimer = time.NewTicker(1000 * time.Millisecond)
	// launch goroutine that calculates last protocol rate periodically
	go func() {
		for {
			if g.protocolRateTimer != nil {
				<-g.protocolRateTimer.C
				g.lastProtocolRate = g.computeProtocolRate()
			} else {
				break
			}
		}
	}()
}
