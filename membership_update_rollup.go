package ringpop

import (
	"sync"
	"time"
)
import log "github.com/Sirupsen/logrus"

const defaultMaxNumUpdates = 250

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// MEMBERSHIP UPDATE ROLLUP
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

type membershipUpdateRollup struct {
	ringpop         *Ringpop
	flushInterval   time.Duration
	maxNumUpdates   int
	buffer          map[string][]Change
	firstUpdateTime time.Time
	lastFlushTime   time.Time
	lastUpdateTime  time.Time
	flushTimer      *time.Timer
	eventC          chan string

	lock sync.RWMutex
}

// NewMembershipUpdateRollup returns a new MembershipUpdateRollup
func newMembershipUpdateRollup(ringpop *Ringpop, flushInterval time.Duration, maxNumUpdates int) *membershipUpdateRollup {
	if maxNumUpdates <= 0 {
		maxNumUpdates = defaultMaxNumUpdates
	}

	membershipUpdateRollup := &membershipUpdateRollup{
		ringpop:       ringpop,
		flushInterval: flushInterval,
		maxNumUpdates: maxNumUpdates,
		buffer:        map[string][]Change{},
		eventC:        make(chan string),
	}

	return membershipUpdateRollup
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// METHODS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

// addUpdates adds changes to the buffer
func (mr *membershipUpdateRollup) addUpdates(changes []Change) {
	ts := time.Now()

	mr.lock.Lock()
	defer mr.lock.Unlock()

	for _, change := range changes {
		// update timestamp
		change.Timestamp = ts
		// add change to corresponding slice of changes
		mr.buffer[change.Address] = append(mr.buffer[change.Address], change)
	}
}

func (mr *membershipUpdateRollup) trackUpdates(changes []Change) (flushed bool) {
	if len(changes) == 0 {
		return flushed
	}

	mr.lock.RLock()
	flushInterval := mr.flushInterval
	sinceLastUpdate := time.Now().Sub(mr.lastUpdateTime)
	mr.lock.RUnlock()
	if sinceLastUpdate >= flushInterval {
		mr.flushBuffer()
		flushed = true
	}

	mr.lock.Lock()
	if mr.firstUpdateTime.IsZero() {
		mr.firstUpdateTime = time.Now()
	}
	mr.lock.Unlock()

	mr.renewFlushTimer()
	mr.addUpdates(changes)

	mr.lock.Lock()
	mr.lastUpdateTime = time.Now()
	mr.lock.Unlock()

	return flushed
}

// numUpdates returns the total number of changes contained in the buffer
func (mr *membershipUpdateRollup) numUpdates() int {
	total := 0

	for _, changes := range mr.buffer {
		total += len(changes)
	}
	return total
}

// renewFlushTimer starts or restarts the flush timer
func (mr *membershipUpdateRollup) renewFlushTimer() {
	mr.lock.Lock()
	defer mr.lock.Unlock()

	if mr.flushTimer != nil {
		mr.flushTimer.Stop()
	}

	mr.flushTimer = time.AfterFunc(mr.flushInterval, func() {
		mr.flushBuffer()
		mr.renewFlushTimer()
	})
}

// flushBuffer flushes contents of buffer and resets update times to zero
func (mr *membershipUpdateRollup) flushBuffer() {
	mr.lock.Lock()
	defer mr.lock.Unlock()

	// nothing to flush if no updates in buffer
	if len(mr.buffer) == 0 {
		// mr.ringpop.logger.WithField("local", mr.ringpop.WhoAmI()).Info("ringpop flushed no updates")
		return
	}

	now := time.Now()

	sinceLastUpdate := time.Duration(0)
	if !mr.lastUpdateTime.IsZero() {
		sinceLastUpdate = time.Duration(mr.lastUpdateTime.UnixNano() - mr.firstUpdateTime.UnixNano())
	}

	sinceLastFlush := time.Duration(0)
	if !mr.lastFlushTime.IsZero() {
		sinceLastFlush = time.Duration(now.UnixNano() - mr.lastFlushTime.UnixNano())
	}

	numUpdates := mr.numUpdates()
	mr.ringpop.logger.WithFields(log.Fields{
		"local":            mr.ringpop.WhoAmI(),
		"checksum":         mr.ringpop.membership.checksum,
		"sinceFirstUpdate": sinceLastUpdate,
		"sinceLastFlush":   sinceLastFlush,
		"numUpdates":       numUpdates,
	}).Info("[ringpop] flushed membership update buffer")

	mr.buffer = make(map[string][]Change)
	mr.firstUpdateTime = time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC) // Zero time
	mr.lastUpdateTime = time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
	mr.lastFlushTime = now

	go func() {
		mr.eventC <- "flushed"
	}()
}

// Destroy stops timer
func (mr *membershipUpdateRollup) destroy() {
	mr.lock.Lock()
	defer mr.lock.Unlock()

	if mr.flushTimer != nil {
		mr.flushTimer.Stop()
		mr.flushTimer = nil
	}
}
