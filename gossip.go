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

func NewGossip(ringpop *Ringpop) *Gossip {
	gossip := &Gossip{
		ringpop: ringpop,
	}

	log.Info("ahh")

	// TODO

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
	// REVIST THIS LINE!!!
	observed := this.protocolTiming.Percentiles([]float64{0.5})[1] * 2.0
	return int64(math.Max(observed, float64(this.minProtocolPeriod)))
}

// TODO
// func (this *Gossip) run() {
// 	protocolDelay := this.computeProtocolDelay()

// 	this.protocolPeriodTimer = time.NewTicker(protocolDelay)
// 	this.ringpop.stat("timing", "protocol.delay", int64(protocolDelay))

// 	go func() {
// 		returnCh := make(chan string)
// 		for {
// 			select {
// 			case <-this.protocolPeriodTimer.C:
// 				pingStartTime := time.Now()

// 				this.ringpop.pingMemberNow(returnCh)

// 				go func() {
// 					select {
// 					case rtn := <-returnCh:
// 						// do something
// 					}
// 				}()
// 			}
// 		}
// 	}()

// }

func (this *Gossip) start() {
	if !this.stopped {
		this.ringpop.logger.WithFields(log.Fields{
			"local": this.ringpop.WhoAmI(),
		}).Debug("gossip has already started")

		return
	}

	this.ringpop.membership.shuffle() // REVISIT
	// this.Run()
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
func (this *Gossip) startProtocolRateTimer() chan<- bool {
	this.protocolRateTimer = time.NewTicker(1000 * time.Millisecond)
	quit := make(chan bool, 1)
	// launch goroutine that calculates last protocol rate periodically
	go func() {
		for {
			select {
			case <-this.protocolRateTimer.C:
				this.lastProtocolRate = this.computeProtocolRate()
			case <-quit:
				this.protocolRateTimer.Stop()
				close(quit)
				break
			}
		}
	}()

	// return channel that provides method to break out of goroutine
	return quit
}
