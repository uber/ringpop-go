package ringpop

import (
	"time"

	log "github.com/uber-common/bark"
	"github.com/uber/ringpop-go/events"
	"github.com/uber/ringpop-go/swim"
)

type ringpopLogger struct {
	logger  log.Logger
	ringpop *Ringpop
}

func (r *ringpopLogger) HandleEvent(event events.Event) {
	switch event := event.(type) {
	case swim.ChangeMadeEvent:
		// membership logger
		r.logger.WithFields(log.Fields{
			"local":  event.Change.Source,
			"update": event.Change,
		}).Infof("ringpop member declares other member %s", event.Change.Status)

	case swim.ChecksumComputeEvent:
		// membership logger
		if event.OldChecksum != event.Checksum {
			local, _ := r.ringpop.WhoAmI()
			r.logger.WithFields(log.Fields{
				"local":       local,
				"checksum":    event.Checksum,
				"oldChecksum": event.OldChecksum,
				"timestamp":   time.Now().Unix(),
			}).Info("ringpop membership computed new checksum")
		}

	case events.RingChecksumEvent:
		// ring logger
		if event.OldChecksum != event.NewChecksum {
			local, _ := r.ringpop.WhoAmI()
			r.logger.WithFields(log.Fields{
				"local":       local,
				"checksum":    event.NewChecksum,
				"oldChecksum": event.OldChecksum,
				"timestamp":   time.Now().Unix(),
			}).Info("ringpop ring computed new checksum")
		}

	case swim.MemberlistChangesAppliedEvent:
		// membership logger
		for _, change := range event.Changes {
			if me, _ := r.ringpop.WhoAmI(); change.Source != me {
				r.logger.WithFields(log.Fields{
					"local":  me,
					"remote": change.Source,
					// changes in ringpop go do not have an id.
					// "updateId": change.id,
				}).Info("ringpop applied remote update")
			}
		}
	}
}
