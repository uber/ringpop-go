package ringpop

import log "github.com/Sirupsen/logrus"

func receivePingReq(ringpop *Ringpop, body pingReqBody) pingReqRes {
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

	return pingReqRes{
		Changes:    ringpop.dissemination.issueChanges(body.Checksum, body.Source),
		PingStatus: true,
		Target:     body.Target,
	}
}
