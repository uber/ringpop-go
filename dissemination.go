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

type Dissemination struct {
	ringpop           *Ringpop
	changes           []Change
	maxPiggybackCount uint32
	piggybackFactor   uint32
}

func NewDissemination(ringpop *Ringpop) *Dissemination {
	return &Dissemination{
		ringpop:           ringpop,
		changes:           make([]Change, 0, 10),
		maxPiggybackCount: 1,
		piggybackFactor:   15, // lower factor -> more full syncs
	}
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	METHODS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func (this *Dissemination) adjustPiggybackCount() {
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

func (this *Dissemination) fullSync() []Change {
	changes := []Change{}

	for _, member := range this.ringpop.membership.members {
		changes = append(changes, Change{
			source:      this.ringpop.WhoAmI(),
			address:     member.Address(),
			status:      member.Status(),
			incarnation: member.Incarnation(),
		})
	}

	return changes
}

func (this *Dissemination) recordChange(change Change) {
	// TODO
}

func (this *Dissemination) issueChanges() {

}
