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
		timers:  make(map[string]*time.Timer),
	}

	return suspicion
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	M E T H O D S
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

// Start enables suspicion protocol for a particular member given by a change
func (s *suspicion) start(suspect suspect) {
	if s.stopped {
		s.ringpop.logger.WithFields(log.Fields{
			"local": s.ringpop.WhoAmI(),
		}).Debug("cannot start a suspect period because suspicion has not been reenabled")

		return
	}

	if suspect.suspectAddress() == s.ringpop.WhoAmI() {
		s.ringpop.logger.WithFields(log.Fields{
			"local":   s.ringpop.WhoAmI(),
			"suspect": suspect.suspectAddress(),
		}).Debug("cannot start a suspect period for the local member")

		return
	}

	if timer, ok := s.timers[suspect.suspectAddress()]; ok {
		timer.Stop()
	}

	// declare member faulty when timer runs out
	s.timers[suspect.suspectAddress()] = time.AfterFunc(s.period, func() {
		s.ringpop.logger.WithFields(log.Fields{
			"local":  s.ringpop.WhoAmI(),
			"faulty": suspect.suspectAddress(),
		}).Info("ringpop declares member faulty")

		s.ringpop.membership.makeFaulty(suspect.suspectAddress(), suspect.suspectIncarnation(), "")
	})

	s.ringpop.logger.WithFields(log.Fields{
		"local":   s.ringpop.WhoAmI(),
		"suspect": suspect.suspectAddress(),
	}).Debug("started suspect period")
}

// Stop stops the suspicion timer for a specific member
func (s *suspicion) stop(suspect suspect) {
	if timer, ok := s.timers[suspect.suspectAddress()]; ok {
		timer.Stop()
		delete(s.timers, suspect.suspectAddress())

		s.ringpop.logger.WithFields(log.Fields{
			"local":   s.ringpop.WhoAmI(),
			"suspect": suspect.suspectAddress(),
		}).Debug("stopped members suspect timer")
	}
}

// Reenable reenables the suspicion protocol
func (s *suspicion) reenable() {
	if !s.stopped {
		s.ringpop.logger.WithFields(log.Fields{
			"local": s.ringpop.WhoAmI(),
		}).Warn("cannot reenable suspicion protocol because it was never disabled")

		return
	}

	s.stopped = false

	s.ringpop.logger.WithFields(log.Fields{
		"local": s.ringpop.WhoAmI(),
	}).Debug("reenabled suspicion protocol")
}

// StopAll stops all suspicion timers
func (s *suspicion) stopAll() {
	s.stopped = true

	numtimers := len(s.timers)

	if numtimers == 0 {
		s.ringpop.logger.WithFields(log.Fields{
			"local": s.ringpop.WhoAmI(),
		}).Debug("stopped no suspect timers")

		return
	}

	for addr, timer := range s.timers {
		timer.Stop()
		delete(s.timers, addr)
	}

	s.ringpop.logger.WithFields(log.Fields{
		"local":     s.ringpop.WhoAmI(),
		"numTimers": numtimers,
	}).Debug("stopped all suspect timers")
}
