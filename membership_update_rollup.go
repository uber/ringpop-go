package ringpop

import "time"
import log "github.com/Sirupsen/logrus"

const _MAX_NUM_UPDATES_ = 250

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
		maxNumUpdates = _MAX_NUM_UPDATES_
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
func (this *membershipUpdateRollup) addUpdates(changes []Change) {
	ts := time.Now()

	for _, change := range changes {
		// update timestamp
		change.Timestamp = ts
		// add change to corresponding slice of changes
		this.buffer[change.Address] = append(this.buffer[change.Address], change)
	}
}

func (this *membershipUpdateRollup) trackUpdates(changes []Change) {
	if len(changes) == 0 {
		return
	}

	sinceLastUpdate := time.Duration(this.lastUpdateTime.UnixNano() - time.Now().UnixNano())
	if sinceLastUpdate >= this.flushInterval {
		this.flushBuffer()
		// <-this.eventC // block until flush completes
	}

	if this.firstUpdateTime.IsZero() {
		this.firstUpdateTime = time.Now()
	}

	this.renewFlushTimer()
	this.addUpdates(changes)
	this.lastUpdateTime = time.Now()
}

// numUpdates returns the total number of changes contained in the buffer
func (this *membershipUpdateRollup) numUpdates() int {
	total := 0

	for _, changes := range this.buffer {
		total += len(changes)
	}
	return total
}

// renewFlushTimer starts or restarts the flush timer
func (this *membershipUpdateRollup) renewFlushTimer() {
	this.flushTimer = nil
	this.flushTimer = time.NewTicker(this.flushInterval)

	// shits so broke man fk
	go func() {
		for {
			if this.flushTimer != nil {
				select {
				case <-this.flushTimer.C:
					this.flushBuffer()
				}
			} else {
				break
			}
		}
	}()
}

// flushBuffer flushes contents of buffer and resets update times to zero
func (this *membershipUpdateRollup) flushBuffer() {
	// nothing to flush if no updates in buffer
	if len(this.buffer) == 0 {
		return
	}

	now := time.Now()

	sinceLastUpdate := time.Duration(0)
	if !this.lastUpdateTime.IsZero() {
		sinceLastUpdate = time.Duration(this.lastUpdateTime.UnixNano() - this.firstUpdateTime.UnixNano())
	}

	sinceLastFlush := time.Duration(0)
	if !this.lastFlushTime.IsZero() {
		sinceLastFlush = time.Duration(now.UnixNano() - this.lastFlushTime.UnixNano())
	}

	numUpdates := this.numUpdates()
	this.ringpop.logger.WithFields(log.Fields{
		"local":            this.ringpop.WhoAmI(),
		"checksum":         this.ringpop.membership.checksum,
		"sinceFirstUpdate": sinceLastUpdate,
		"sinceLastFlush":   sinceLastFlush,
		"numUpdates":       numUpdates,
	}).Info("ringpop flushed membership update buffer")

	this.buffer = map[string][]Change{}
	this.firstUpdateTime = time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC) // Zero time
	this.lastUpdateTime = time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)
	this.lastFlushTime = now

	go func() {
		this.eventC <- "flushed"
	}()
}

// Destroy stops timer
func (this *membershipUpdateRollup) destroy() {
	if this.flushTimer == nil {
		return
	}

	this.flushTimer.Stop()
	this.flushTimer = nil
}
