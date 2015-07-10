package ringpop

import log "github.com/Sirupsen/logrus"

func handlePing(ringpop *Ringpop, body pingBody) pingBody {
	ringpop.stat("increment", "ping.recv", 1)

	// ringpop.serverRate.Mark(1)
	// ringpop.totalRate.Mark(1)

	ringpop.logger.WithFields(log.Fields{
		"local":  ringpop.WhoAmI(),
		"source": body.Source,
	}).Debug("[ringpop] ping receive")

	ringpop.membership.update(body.Changes)

	changes, fullSync := ringpop.dissemination.issueChangesAsReceiver(body.Source,
		body.SourceIncarnation, body.Checksum)

	if fullSync {
		// TODO: something...
	}

	resBody := pingBody{
		Checksum: ringpop.membership.checksum,
		Changes:  changes,
		Source:   ringpop.WhoAmI(),
	}

	return resBody
}
