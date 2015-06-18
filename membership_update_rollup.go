package ringpop

import "time"
import log "github.com/Sirupsen/logrus"

const defaultMaxNumUpdates = 250

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// MEMBERSHIP UPDATE ROLLUP
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

type membershipUpdateRollup struct {
	ringpop       *Ringpop
	flushInterval time.Duration
	maxNumUpdates int

	buffer          map[string][]Change
	firstUpdateTime time.Time
	lastFlushTime   time.Time
	lastUpdateTime  time.Time
	flushTimer      *time.Ticker
	eventC          chan string
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

	for _, change := range changes {
		// update timestamp
		change.Timestamp = ts
		// add change to corresponding slice of changes
		mr.buffer[change.Address] = append(mr.buffer[change.Address], change)
	}
}

func (mr *membershipUpdateRollup) trackUpdates(changes []Change) {
	if len(changes) == 0 {
		return
	}

	sinceLastUpdate := time.Duration(mr.lastUpdateTime.UnixNano() - time.Now().UnixNano())
	if sinceLastUpdate >= mr.flushInterval {
		mr.flushBuffer()
		// <-mr.eventC // block until flush completes
	}

	if mr.firstUpdateTime.IsZero() {
		mr.firstUpdateTime = time.Now()
	}

	mr.renewFlushTimer()
	mr.addUpdates(changes)
	mr.lastUpdateTime = time.Now()
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
	mr.flushTimer = nil
	mr.flushTimer = time.NewTicker(mr.flushInterval)

	go func() {
		for {
			if mr.flushTimer != nil {
				select {
				case <-mr.flushTimer.C:
					mr.flushBuffer()
				}
			} else {
				break
			}
		}
	}()
}

// flushBuffer flushes contents of buffer and resets update times to zero
func (mr *membershipUpdateRollup) flushBuffer() {
	// nothing to flush if no updates in buffer
	if len(mr.buffer) == 0 {
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
	}).Info("ringpop flushed membership update buffer")

	mr.buffer = map[string][]Change{}
	mr.firstUpdateTime = time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC) // Zero time
	mr.lastUpdateTime = time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
	mr.lastFlushTime = now

	go func() {
		mr.eventC <- "flushed"
	}()
}

// Destroy stops timer
func (mr *membershipUpdateRollup) destroy() {
	if mr.flushTimer == nil {
		return
	}

	mr.flushTimer.Stop()
	mr.flushTimer = nil
}
