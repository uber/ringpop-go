package ringpop

import "time"
import log "github.com/Sirupsen/logrus"

const MAX_NUM_UPDATES = 250

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// MEMBERSHIP UPDATE ROLLUP
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

type MembershipUpdateRollup struct {
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
func NewMembershipUpdateRollup(ringpop *Ringpop, flushInterval time.Duration, maxNumUpdates int) *MembershipUpdateRollup {
	if maxNumUpdates <= 0 {
		maxNumUpdates = MAX_NUM_UPDATES
	}

	membershipUpdateRollup := &MembershipUpdateRollup{
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
func (this *MembershipUpdateRollup) addUpdates(changes []Change) {
	ts := time.Now()

	for _, change := range changes {
		// update timestamp
		change.timestamp = ts
		// add change to corresponding slice of changes
		this.buffer[change.Address()] = append(this.buffer[change.Address()], change)
	}
}

func (this *MembershipUpdateRollup) trackUpdates(changes []Change) {
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
func (this *MembershipUpdateRollup) numUpdates() int {
	total := 0

	for _, changes := range this.buffer {
		total += len(changes)
	}
	return total
}

// renewFlushTimer starts or restarts the flush timer
func (this *MembershipUpdateRollup) renewFlushTimer() {
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
func (this *MembershipUpdateRollup) flushBuffer() {
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
func (this *MembershipUpdateRollup) destroy() {
	if this.flushTimer == nil {
		return
	}

	this.flushTimer.Stop()
	this.flushTimer = nil
}
