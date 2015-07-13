package ringpop

import (
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
)

const defaultSuspicionTimeout = time.Millisecond * 5000

// interface so that both changes and members can be passed to Suspicion methods
type suspect interface {
	address() string
	incarnation() int64
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	SUSPICION
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

type suspicion struct {
	ringpop *Ringpop
	period  time.Duration
	stopped bool
	timers  map[string]*time.Timer
	lock    sync.Mutex
}

// NewSuspicion creates a new suspicion protocol
func newSuspicion(ringpop *Ringpop, suspicionTimeout time.Duration) *suspicion {
	period := defaultSuspicionTimeout
	if suspicionTimeout != time.Duration(0) {
		period = suspicionTimeout
	}

	suspicion := &suspicion{
		ringpop: ringpop,
		period:  period,
		timers:  make(map[string]*time.Timer),
		lock:    sync.Mutex{},
	}

	return suspicion
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// SUSPICION METHODS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

// Start enables suspicion protocol for a particular member given by a change
func (s *suspicion) start(suspect suspect) {
	if s.stopped {
		s.ringpop.logger.WithFields(log.Fields{
			"local": s.ringpop.WhoAmI(),
		}).Debug("[ringpop] cannot start a suspect period because suspicion has not been reenabled")

		return
	}

	if suspect.address() == s.ringpop.WhoAmI() {
		s.ringpop.logger.WithFields(log.Fields{
			"local":   s.ringpop.WhoAmI(),
			"suspect": suspect.address(),
		}).Debug("[ringpop] cannot start a suspect period for the local member")

		return
	}

	s.lock.Lock()
	defer s.lock.Unlock()

	if timer, ok := s.timers[suspect.address()]; ok {
		timer.Stop()
	}

	// declare member faulty when timer runs out
	s.timers[suspect.address()] = time.AfterFunc(s.period, func() {
		s.ringpop.logger.WithFields(log.Fields{
			"local":  s.ringpop.WhoAmI(),
			"faulty": suspect.address(),
		}).Info("[ringpop] member declared faulty")

		s.ringpop.membership.makeFaulty(suspect.address(), suspect.incarnation())
	})

	s.ringpop.logger.WithFields(log.Fields{
		"local":     s.ringpop.WhoAmI(),
		"suspect":   suspect.address(),
		"timestamp": time.Now(),
	}).Debug("[ringpop] started suspect period")
}

// stops the suspicion timer for a specific member
func (s *suspicion) stop(suspect suspect) {
	s.lock.Lock()
	defer s.lock.Unlock()

	if timer, ok := s.timers[suspect.address()]; ok {
		timer.Stop()
		delete(s.timers, suspect.address())

		s.ringpop.logger.WithFields(log.Fields{
			"local":   s.ringpop.WhoAmI(),
			"suspect": suspect.address(),
		}).Debug("[ringpop] stopped members suspect timer")
	}
}

// reenables the suspicion protocol
func (s *suspicion) reenable() {
	if !s.stopped {
		s.ringpop.logger.WithFields(log.Fields{
			"local": s.ringpop.WhoAmI(),
		}).Warn("[ringpop] cannot reenable suspicion protocol because it was never disabled")

		return
	}

	s.stopped = false

	s.ringpop.logger.WithFields(log.Fields{
		"local": s.ringpop.WhoAmI(),
	}).Debug("[ringpop] reenabled suspicion protocol")
}

// stops all suspicion timers
func (s *suspicion) stopAll() {
	s.lock.Lock()
	defer s.lock.Unlock()

	s.stopped = true

	numtimers := len(s.timers)

	if numtimers == 0 {
		s.ringpop.logger.WithFields(log.Fields{
			"local": s.ringpop.WhoAmI(),
		}).Debug("[ringpop] stopped no suspect timers")

		return
	}

	for addr, timer := range s.timers {
		timer.Stop()
		delete(s.timers, addr)
	}

	s.ringpop.logger.WithFields(log.Fields{
		"local":     s.ringpop.WhoAmI(),
		"numTimers": numtimers,
	}).Debug("[ringpop] stopped all suspect timers")
}
