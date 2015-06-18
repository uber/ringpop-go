package ringpop

import (
	"math"

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
	changes           map[string]Change
	maxPiggybackCount int
	piggybackFactor   int
	eventC            chan string
}

func newDissemination(ringpop *Ringpop) *dissemination {
	d := &dissemination{
		ringpop:           ringpop,
		changes:           make(map[string]Change),
		maxPiggybackCount: 1,
		piggybackFactor:   15, // lower factor -> more full syncs
		eventC:            make(chan string),
	}
	d.launchChangeHandler()

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
		}).Debug("adjusted max piggyback count")
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

// issueChanges returns a slice of changes to be propogated
func (d *dissemination) issueChanges(checksum uint32, source string) []Change {
	var changesToDisseminate []Change
	var piggybacks = make(map[string]int)

	var changedNodes []string
	for _, change := range d.changes {
		changedNodes = append(changedNodes, change.Address)
	}

	for _, change := range d.changes {
		piggybacks[change.Address]++

		if piggybacks[change.Address] > d.maxPiggybackCount {
			delete(d.changes, change.Address)
			continue
		}

		changesToDisseminate = append(changesToDisseminate, change)
	}

	d.ringpop.stat("gauge", "changes.disseminate", int64(len(changesToDisseminate)))

	if len(changesToDisseminate) > 0 {
		return changesToDisseminate
	} else if checksum != 0 && d.ringpop.membership.checksum != checksum {
		d.ringpop.stat("increment", "full-sync", 1)
		d.ringpop.logger.WithFields(log.Fields{
			"localChecksum":  d.ringpop.membership.checksum,
			"remoteChecksum": checksum,
			"remoteNode":     source,
		}).Info("full sync")

		return d.fullSync()
	}

	return []Change{}
}

// recordchange records a change in the dissemination for later propogation
func (d *dissemination) recordChange(change Change) {
	d.changes[change.Address] = change
}

// launchChangeHandler launches a goroutine that will adjust the dissemination's
// piggyback count whenever it receives on eventC
func (d *dissemination) launchChangeHandler() {
	go func() {
		for {
			if d.eventC != nil {
				<-d.eventC
				d.adjustPiggybackCount()
			}
			break
		}
	}()
}
