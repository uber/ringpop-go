package ringpop

import (
	"math"

	log "github.com/Sirupsen/logrus"
)

var LOG10 = math.Log(10)

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	DISSEMINATION
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

type dissemination struct {
	ringpop           *Ringpop
	changes           map[string]Change
	maxPiggybackCount uint32
	piggybackFactor   uint32
}

func newDissemination(ringpop *Ringpop) *dissemination {
	return &dissemination{
		ringpop:           ringpop,
		changes:           make(map[string]Change),
		maxPiggybackCount: 1,
		piggybackFactor:   15, // lower factor -> more full syncs
	}
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	METHODS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func (this *dissemination) adjustPiggybackCount() {
	serverCount := this.ringpop.ring.serverCount()
	prevPiggybackCount := this.maxPiggybackCount

	newPiggybackCount := this.piggybackFactor * uint32(math.Ceil(math.Log(float64(serverCount+1))/LOG10))

	if newPiggybackCount != prevPiggybackCount {
		this.maxPiggybackCount = newPiggybackCount
		this.ringpop.stat("gauge", "max-piggyback", int64(this.maxPiggybackCount))
		this.ringpop.logger.WithFields(log.Fields{
			"newPiggybackCount": this.maxPiggybackCount,
			"oldPiggybackCount": prevPiggybackCount,
			"piggybackFactor":   this.piggybackFactor,
			"serverCount":       serverCount,
		}).Debug("adjusted max piggyback count")
	}
}

func (this *dissemination) fullSync() []Change {
	changes := make([]Change, len(this.ringpop.membership.members))

	for _, member := range this.ringpop.membership.members {
		changes = append(changes, Change{
			Source:      this.ringpop.WhoAmI(),
			Address:     member.Address,
			Status:      member.Status,
			Incarnation: member.Incarnation,
		})
	}

	return changes
}

func (this *dissemination) recordChange(change Change) {
	// TODO
}

func (this *dissemination) issueChanges(checksum uint32, source string) []Change {
	var changesToDisseminate []Change
	var changedNodes []string

	for _, change := range this.changes {
		changedNodes = append(changedNodes, change.Address)
	}

	// for _, node := range changedNodes {

	// }

	this.ringpop.stat("gauge", "changes.disseminate", int64(len(changesToDisseminate)))

	if len(changesToDisseminate) > 0 {
		return changesToDisseminate
	} else if this.ringpop.membership.checksum != checksum {
		this.ringpop.stat("increment", "full-sync", 1)
		this.ringpop.logger.WithFields(log.Fields{
			"localChecksum":  this.ringpop.membership.checksum,
			"remoteChecksum": checksum,
			"remoteNode":     source,
		}).Info("full sync")
	}

	return []Change{}
}
