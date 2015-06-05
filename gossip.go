package ringpop

import (
	"math"
	"math/rand"
	"time"

	log "github.com/Sirupsen/logrus"

	"github.com/rcrowley/go-metrics"
)

const DefaultMinProtocolPeriod = time.Millisecond * 200

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// GOSSIP
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

type Gossip struct {
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

func NewGossip(ringpop *Ringpop, minProtocolPeriod time.Duration) *Gossip {
	if minProtocolPeriod <= 0 {
		minProtocolPeriod = DefaultMinProtocolPeriod
	}

	gossip := &Gossip{
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

func (this *Gossip) computeProtocolDelay() time.Duration {
	if this.numProtocolPeriods != 0 {
		target := this.lastProtocolPeriod + time.Duration(this.lastProtocolRate)
		return time.Duration(math.Max(float64(int64(target)-TimeNow()), float64(this.minProtocolPeriod))) // REVISIT
	} else {
		// delay for first tick staggered in [0, minProtocolPeriod] ms
		return time.Duration(math.Floor(rand.Float64() * float64(this.minProtocolPeriod+1)))
	}
}

func (this *Gossip) computeProtocolRate() int64 {
	observed := this.protocolTiming.Percentiles([]float64{0.5})[1] * 2.0
	return int64(math.Max(observed, float64(this.minProtocolPeriod)))
}

// TODO
func (this *Gossip) run() {
	protocolDelay := this.computeProtocolDelay()
	println(protocolDelay)

	this.protocolPeriodTimer = time.NewTicker(protocolDelay + 1)
	this.ringpop.stat("timing", "protocol.delay", int64(protocolDelay))

	go func() {
		for {
			if this.protocolPeriodTimer != nil {
				<-this.protocolPeriodTimer.C
				// pingStartTime := time.Now()

				// do something here - todo

			} else {
				break
			}
		}
	}()
}

func (this *Gossip) start() {
	if !this.stopped {
		this.ringpop.logger.WithFields(log.Fields{
			"local": this.ringpop.WhoAmI(),
		}).Debug("gossip has already started")

		return
	}

	// this.ringpop.membership.shuffle() // REVISIT
	this.run()
	this.startProtocolRateTimer()
	this.stopped = false

	this.ringpop.logger.WithFields(log.Fields{
		"local": this.ringpop.WhoAmI(),
	}).Debug("started gossip protocol")
}

func (this *Gossip) stop() {
	if this.stopped {
		this.ringpop.logger.WithFields(log.Fields{
			"local": this.ringpop.WhoAmI(),
		}).Warn("gossip is already stopped")

		return
	}

	this.protocolRateTimer.Stop()
	this.protocolRateTimer = nil

	this.protocolPeriodTimer.Stop()
	this.protocolPeriodTimer = nil

	this.stopped = true

	this.ringpop.logger.WithFields(log.Fields{
		"local": this.ringpop.WhoAmI(),
	}).Debug("stopped gossip protocol")
}

// StartProtocolRateTimer creates a ticker and launches a goroutine that
// sets lastProtocolRate every 1000ms, returns a channel that can be used
// to exit out of the function
func (this *Gossip) startProtocolRateTimer() {
	this.protocolRateTimer = time.NewTicker(1000 * time.Millisecond)
	// launch goroutine that calculates last protocol rate periodically
	go func() {
		for {
			if this.protocolRateTimer != nil {
				<-this.protocolRateTimer.C
				this.lastProtocolRate = this.computeProtocolRate()
			} else {
				break
			}
		}
	}()
}
