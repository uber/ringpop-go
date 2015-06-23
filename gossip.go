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

	minProtocolPeriod  time.Duration
	numProtocolPeriods int64

	lastProtocolPeriod time.Time
	lastProtocolRate   time.Duration
	protocolRateTimer  *time.Ticker

	protocolTiming metrics.Histogram
}

func newGossip(ringpop *Ringpop, minProtocolPeriod time.Duration) *gossip {
	if minProtocolPeriod <= 0 {
		minProtocolPeriod = defaultMinProtocolPeriod
	}

	gossip := &gossip{
		ringpop:           ringpop,
		stopped:           true,
		minProtocolPeriod: minProtocolPeriod,
		protocolTiming:    metrics.NewHistogram(metrics.NilSample{}),
	}

	gossip.protocolTiming.Update(int64(gossip.minProtocolPeriod))

	return gossip
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// METHODS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func (g *gossip) computeProtocolDelay() time.Duration {
	if g.numProtocolPeriods != 0 {
		target := g.lastProtocolPeriod.Add(g.lastProtocolRate)
		return time.Duration(math.Max(float64(target.UnixNano()-time.Now().UnixNano()), float64(g.minProtocolPeriod))) // REVISIT
	}

	// delay for first tick staggered in [0, minProtocolPeriod] ms
	return time.Duration(math.Floor(rand.Float64() * float64(g.minProtocolPeriod+1)))
}

func (g *gossip) computeProtocolRate() time.Duration {
	observed := g.protocolTiming.Percentiles([]float64{0.5})[1] * 2.0
	return time.Duration(math.Max(observed, float64(g.minProtocolPeriod)))
}

// TODO
func (g *gossip) run() {
	go func() {
		for {
			protocolDelay := g.computeProtocolDelay()
			g.ringpop.stat("timing", "protocol.delay", milliseconds(protocolDelay))
			time.Sleep(protocolDelay)

			if g.stopped {
				g.ringpop.logger.WithField("local", g.ringpop.WhoAmI()).
					Debug("stopped recurring gossip loop")
				break
			}

			// pingStartTime := time.Now()
			// err := g.ringpop.PingMemberNow()

			g.lastProtocolPeriod = time.Now()
			g.numProtocolPeriods++
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
		}).Debug("gossip is already stopped")

		return
	}

	g.protocolRateTimer.Stop()

	g.stopped = true

	g.ringpop.logger.WithFields(log.Fields{
		"local": g.ringpop.WhoAmI(),
	}).Debug("stopped gossip protocol")
}

// StartProtocolRateTimer creates a ticker and launches a goroutine that
// sets lastProtocolRate every 1000ms
func (g *gossip) startProtocolRateTimer() {
	g.protocolRateTimer = time.NewTicker(1000 * time.Millisecond)
	// launch goroutine that calculates last protocol rate periodically
	go func() {
		for _ = range g.protocolRateTimer.C {
			g.lastProtocolRate = g.computeProtocolRate()
		}
	}()
}
