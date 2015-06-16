package ringpop

func receivePing(ringpop *Ringpop, body pingBody) []Change {
	ringpop.stat("increment", "ping.recv", 1)

	ringpop.serverRate.Mark(1)
	ringpop.totalRate.Mark(1)

	ringpop.membership.update(body.Changes)

	return ringpop.dissemination.issueChanges(body.Checksum, body.Source)
}
