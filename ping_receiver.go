package ringpop

import log "github.com/Sirupsen/logrus"

func receivePing(ringpop *Ringpop, body pingBody) pingBody {
	ringpop.stat("increment", "ping.recv", 1)

	// ringpop.serverRate.Mark(1)
	// ringpop.totalRate.Mark(1)

	ringpop.logger.WithFields(log.Fields{
		"local":  ringpop.WhoAmI(),
		"source": body.Source,
	}).Debug("[ringpop] ping receive")

	ringpop.membership.update(body.Changes)

	resBody := pingBody{
		Checksum: ringpop.membership.checksum,
		Changes:  ringpop.dissemination.issueChanges(body.Checksum, body.Source),
		Source:   ringpop.WhoAmI(),
	}

	return resBody
}
