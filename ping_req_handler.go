package ringpop

import log "github.com/Sirupsen/logrus"

func handlePingReq(ringpop *Ringpop, body pingReqBody) pingReqRes {
	ringpop.stat("increment", "ping-req.recv", 1)

	ringpop.membership.update(body.Changes)
	ringpop.logger.WithFields(log.Fields{
		"local":  ringpop.WhoAmI(),
		"source": body.Source,
		"target": body.Target,
	}).Debug("[ringpop] ping-req recieve")

	res, err := sendPing(ringpop, body.Target, ringpop.pingTimeout)
	// ringpop.stat("timing", "ping-req-ping", unixMilliseconds(start))
	ringpop.logger.WithFields(log.Fields{
		"local":  ringpop.WhoAmI(),
		"source": body.Source,
		"target": body.Target,
		"isOK":   err == nil,
	}).Debug("[ringpop] ping-req complete")

	if err != nil {
		return pingReqRes{
			Target:     body.Target,
			PingStatus: false,
		}
	}

	ringpop.membership.update(res.Changes)

	changes, fullSync := ringpop.dissemination.issueChangesAsReceiver(body.Source,
		body.SourceIncarnation, body.Checksum)

	if fullSync {
		// TODO: something...
	}

	return pingReqRes{
		Changes:    changes,
		PingStatus: true,
		Target:     body.Target,
	}
}
