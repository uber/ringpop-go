package ringpop

func isAliveOverride(member *Member, change Change) bool {
	return change.Status == ALIVE && change.Incarnation > member.Incarnation
}

func isFaultyOverride(member *Member, change Change) bool {
	return change.Status == FAULTY &&
		((member.Status == SUSPECT && change.Incarnation > member.Incarnation) ||
			(member.Status == FAULTY && change.Incarnation > member.Incarnation) ||
			(member.Status == ALIVE && change.Incarnation >= member.Incarnation))
}

func isLeaveOverride(member *Member, change Change) bool {
	return change.Status == LEAVE &&
		member.Status != LEAVE &&
		change.Incarnation >= member.Incarnation
}

func isSuspectOverride(member *Member, change Change) bool {
	return change.Status == SUSPECT &&
		((member.Status == SUSPECT && change.Incarnation > member.Incarnation) ||
			(member.Status == FAULTY && change.Incarnation > member.Incarnation) ||
			(member.Status == ALIVE && change.Incarnation >= member.Incarnation))
}

func isLocalFaultyOverride(ringpop *Ringpop, member *Member, change Change) bool {
	return member.Address == ringpop.WhoAmI() && change.Status == FAULTY
}

func isLocalSuspectOverride(ringpop *Ringpop, member *Member, change Change) bool {
	return member.Address == ringpop.WhoAmI() && change.Status == SUSPECT
}
