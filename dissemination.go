package ringpop

import (
	"math"
	"sync"

	log "github.com/Sirupsen/logrus"
)

var log10 = math.Log(10)

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	DISSEMINATION
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

type dissemination struct {
	ringpop           *Ringpop
	changes           map[string]disseminationChange
	maxPiggybackCount int
	piggybackFactor   int
	lock              sync.RWMutex
}

type disseminationChange struct {
	Change
	piggybackCount int
}

func newDissemination(ringpop *Ringpop) *dissemination {
	d := &dissemination{
		ringpop:           ringpop,
		changes:           make(map[string]disseminationChange),
		maxPiggybackCount: 1,
		piggybackFactor:   2, // lower factor -> more full syncs
	}

	return d
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	METHODS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func (d *dissemination) adjustPiggybackCount() {
	serverCount := d.ringpop.ring.serverCount()
	prevPiggybackCount := d.maxPiggybackCount

	newPiggybackCount := d.piggybackFactor * int(math.Ceil(math.Log(float64(serverCount+1))/log10))

	if newPiggybackCount != prevPiggybackCount {
		d.maxPiggybackCount = newPiggybackCount
		d.ringpop.stat("gauge", "max-piggyback", int64(d.maxPiggybackCount))
		d.ringpop.logger.WithFields(log.Fields{
			"newPiggybackCount": d.maxPiggybackCount,
			"oldPiggybackCount": prevPiggybackCount,
			"piggybackFactor":   d.piggybackFactor,
			"serverCount":       serverCount,
		}).Debug("[ringpop] adjusted max piggyback count")
	}
}

func (d *dissemination) fullSync() []Change {
	changes := make([]Change, 0, len(d.ringpop.membership.members))

	for _, member := range d.ringpop.membership.members {
		changes = append(changes, Change{
			Source:      d.ringpop.WhoAmI(),
			Address:     member.Address,
			Status:      member.Status,
			Incarnation: member.Incarnation,
		})
	}

	return changes
}

func (d *dissemination) issueChangesAsSender() []Change {
	return d.issueChangesV2(nil)
}

func (d *dissemination) issueChangesAsReceiver(senderAddress string,
	senderIncarnation int64, senderChecksum uint32) ([]Change, bool) {

	filter := func(c disseminationChange) bool {
		return senderAddress == c.Source &&
			senderIncarnation == c.SourceIncarnation
	}

	changes := d.issueChangesV2(filter)

	if len(changes) > 0 {
		return changes, false
	} else if d.ringpop.membership.checksum != senderChecksum {
		d.ringpop.stat("increment", "full-sync", 1)
		d.ringpop.logger.WithFields(log.Fields{
			"local":          d.ringpop.WhoAmI(),
			"localChecksum":  d.ringpop.membership.checksum,
			"remote":         senderAddress,
			"remoteChecksum": senderChecksum,
		}).Warn("[ringpop] full sync")

		return d.fullSync(), true
	}

	return []Change{}, false
}

func (d *dissemination) issueChangesV2(filter func(disseminationChange) bool) []Change {
	var changesToDisseminate []Change

	for _, change := range d.changes {
		// filter thing?
		change.piggybackCount++

		if filter != nil && filter(change) {
			d.ringpop.stat("increment", "filtered-change", 1)
			continue
		}

		if change.piggybackCount > d.maxPiggybackCount {
			delete(d.changes, change.Address)
			continue
		}

		changesToDisseminate = append(changesToDisseminate, change.Change)
	}

	return changesToDisseminate
}

// issueChanges returns a slice of changes to be propogated
func (d *dissemination) issueChanges(checksum uint32, source string) []Change {
	var changesToDisseminate []Change

	d.lock.Lock()
	for _, change := range d.changes {
		change.piggybackCount++

		if change.piggybackCount > d.maxPiggybackCount {
			d.ringpop.logger.Warn("deleting change")
			delete(d.changes, change.Address)
			continue
		}

		changesToDisseminate = append(changesToDisseminate, change.Change)
	}
	d.lock.Unlock()

	d.ringpop.stat("gauge", "changes.disseminate", int64(len(changesToDisseminate)))

	if len(changesToDisseminate) > 0 {
		return changesToDisseminate
	} else if checksum != 0 && d.ringpop.membership.checksum != checksum {
		d.ringpop.stat("increment", "full-sync", 1)
		d.ringpop.logger.WithFields(log.Fields{
			"localChecksum":  d.ringpop.membership.checksum,
			"remoteChecksum": checksum,
			"remoteNode":     source,
		}).Warn("[ringpop] full sync")

		return d.fullSync()
	}

	return []Change{}
}

// recordchange records a change in the dissemination for later propogation
func (d *dissemination) recordChange(change Change) {
	d.lock.Lock()
	d.changes[change.Address] = disseminationChange{change, 0}
	d.lock.Unlock()
}

func (d *dissemination) clearChanges() {
	d.lock.Lock()
	d.changes = make(map[string]disseminationChange)
	d.lock.Unlock()
}

func (d *dissemination) onRingChange() {
	d.adjustPiggybackCount()
}
