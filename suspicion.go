package ringpop

import (
	"time"

	log "github.com/Sirupsen/logrus"
)

const defaultTimeout = time.Millisecond * 5000

// interface so that both changes and members can be passed to Suspicion methods
type Suspect interface {
	Address() string
	Status() string
	Incarnation() int64
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	S U S P I C I O N
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

type Suspicion struct {
	ringpop *Ringpop
	period  time.Duration
	stopped bool
	timers  map[string]*time.Timer
}

// NewSuspicion creates a new suspicion protocol
func NewSuspicion(ringpop *Ringpop, suspicionTimeout time.Duration) *Suspicion {
	period := defaultTimeout
	if suspicionTimeout != time.Duration(0) {
		period = suspicionTimeout
	}

	suspicion := &Suspicion{
		ringpop: ringpop,
		period:  period,
		timers:  map[string]*time.Timer{},
	}

	return suspicion
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	M E T H O D S
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

// Start enables suspicion protocol for a particular member given by a change
func (this *Suspicion) start(suspect Suspect) {
	if this.stopped {
		this.ringpop.logger.WithFields(log.Fields{
			"local": this.ringpop.WhoAmI(),
		}).Debug("cannot start a suspect period because suspicion has not been reenabled")

		return
	}

	if suspect.Address() == this.ringpop.WhoAmI() {
		this.ringpop.logger.WithFields(log.Fields{
			"local":   this.ringpop.WhoAmI(),
			"suspect": suspect.Address(),
		}).Debug("cannot start a suspect period for the local member")

		return
	}

	if timer, ok := this.timers[suspect.Address()]; ok {
		timer.Stop()
	}

	// declare member faulty when timer runs out
	this.timers[suspect.Address()] = time.AfterFunc(time.Millisecond*time.Duration(this.period), func() {
		this.ringpop.logger.WithFields(log.Fields{
			"local":  this.ringpop.WhoAmI(),
			"faulty": suspect.Address(),
		}).Info("ringpop declares member faulty")

		this.ringpop.membership.makeFaulty(suspect.Address(), suspect.Incarnation(), "")
	})

	this.ringpop.logger.WithFields(log.Fields{
		"local":   this.ringpop.WhoAmI(),
		"suspect": suspect.Address(),
	}).Debug("started suspect period")
}

// Stop stops the suspicion timer for a specific member
func (this *Suspicion) stop(suspect Suspect) {
	if timer, ok := this.timers[suspect.Address()]; ok {
		timer.Stop()
		delete(this.timers, suspect.Address())

		this.ringpop.logger.WithFields(log.Fields{
			"local":   this.ringpop.WhoAmI(),
			"suspect": suspect.Address(),
		}).Debug("stopped members suspect timer")
	}
}

// Reenable reenables the suspicion protocol
func (this *Suspicion) reenable() {
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
func (this *Suspicion) stopAll() {
	this.stopped = true

	if len(this.timers) == 0 {
		this.ringpop.logger.WithFields(log.Fields{
			"local": this.ringpop.WhoAmI(),
		}).Debug("stopped no suspect timers")

		return
	}

	numtimers := len(this.timers)
	for addr, timer := range this.timers {
		timer.Stop()
		delete(this.timers, addr)
	}

	this.ringpop.logger.WithFields(log.Fields{
		"local":     this.ringpop.WhoAmI(),
		"numTimers": numtimers,
	}).Debug("stopped all suspect timers")
}
