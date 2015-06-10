package ringpop

import (
	"time"

	log "github.com/Sirupsen/logrus"
)

const defaultTimeout = time.Millisecond * 5000

// interface so that both changes and members can be passed to Suspicion methods
type suspect interface {
	suspectAddress() string
	suspectStatus() string
	suspectIncarnation() int64
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	S U S P I C I O N
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

type suspicion struct {
	ringpop *Ringpop
	period  time.Duration
	stopped bool
	timers  map[string]*time.Timer
}

// NewSuspicion creates a new suspicion protocol
func newSuspicion(ringpop *Ringpop, suspicionTimeout time.Duration) *suspicion {
	period := defaultTimeout
	if suspicionTimeout != time.Duration(0) {
		period = suspicionTimeout
	}

	suspicion := &suspicion{
		ringpop: ringpop,
		period:  period,
		timers:  make(map[string]*time.Timer, 0),
	}

	return suspicion
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	M E T H O D S
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

// Start enables suspicion protocol for a particular member given by a change
func (this *suspicion) start(suspect suspect) {
	if this.stopped {
		this.ringpop.logger.WithFields(log.Fields{
			"local": this.ringpop.WhoAmI(),
		}).Debug("cannot start a suspect period because suspicion has not been reenabled")

		return
	}

	if suspect.suspectAddress() == this.ringpop.WhoAmI() {
		this.ringpop.logger.WithFields(log.Fields{
			"local":   this.ringpop.WhoAmI(),
			"suspect": suspect.suspectAddress(),
		}).Debug("cannot start a suspect period for the local member")

		return
	}

	if timer, ok := this.timers[suspect.suspectAddress()]; ok {
		timer.Stop()
	}

	// declare member faulty when timer runs out
	this.timers[suspect.suspectAddress()] = time.AfterFunc(this.period, func() {
		this.ringpop.logger.WithFields(log.Fields{
			"local":  this.ringpop.WhoAmI(),
			"faulty": suspect.suspectAddress(),
		}).Info("ringpop declares member faulty")

		this.ringpop.membership.makeFaulty(suspect.suspectAddress(), suspect.suspectIncarnation(), "")
	})

	this.ringpop.logger.WithFields(log.Fields{
		"local":   this.ringpop.WhoAmI(),
		"suspect": suspect.suspectAddress(),
	}).Debug("started suspect period")
}

// Stop stops the suspicion timer for a specific member
func (this *suspicion) stop(suspect suspect) {
	if timer, ok := this.timers[suspect.suspectAddress()]; ok {
		timer.Stop()
		delete(this.timers, suspect.suspectAddress())

		this.ringpop.logger.WithFields(log.Fields{
			"local":   this.ringpop.WhoAmI(),
			"suspect": suspect.suspectAddress(),
		}).Debug("stopped members suspect timer")
	}
}

// Reenable reenables the suspicion protocol
func (this *suspicion) reenable() {
	if !this.stopped {
		this.ringpop.logger.WithFields(log.Fields{
			"local": this.ringpop.WhoAmI(),
		}).Warn("cannot reenable suspicion protocol because it was never disabled")

		return
	}

	this.stopped = false

	this.ringpop.logger.WithFields(log.Fields{
		"local": this.ringpop.WhoAmI(),
	}).Debug("reenabled suspicion protocol")
}

// StopAll stops all suspicion timers
func (this *suspicion) stopAll() {
	this.stopped = true

	numtimers := len(this.timers)

	if numtimers == 0 {
		this.ringpop.logger.WithFields(log.Fields{
			"local": this.ringpop.WhoAmI(),
		}).Debug("stopped no suspect timers")

		return
	}

	for addr, timer := range this.timers {
		timer.Stop()
		delete(this.timers, addr)
	}

	this.ringpop.logger.WithFields(log.Fields{
		"local":     this.ringpop.WhoAmI(),
		"numTimers": numtimers,
	}).Debug("stopped all suspect timers")
}
