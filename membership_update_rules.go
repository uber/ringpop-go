package ringpop

func validStatus(status string) bool {
	return status == ALIVE || status == FAULTY || status == LEAVE || status == SUSPECT
}

func IsAliveOverride(member *Member, change Change) bool {
	return change.Status() == ALIVE &&
		validStatus(member.Status()) && // this line is maybe not necessary?
		change.Incarnation() > member.Incarnation()
}

func IsFaultyOverride(member *Member, change Change) bool {
	return change.Status() == FAULTY &&
		((member.Status() == SUSPECT && change.Incarnation() >= member.Incarnation()) ||
			(member.Status() == FAULTY && change.Incarnation() > member.Incarnation()) ||
			(member.Status() == ALIVE && change.Incarnation() >= member.Incarnation()))
}

func IsLeaveOverride(member *Member, change Change) bool {
	return change.Status() == LEAVE &&
		member.Status() != LEAVE &&
		change.Incarnation() >= member.Incarnation()
}

func IsSuspectOverride(member *Member, change Change) bool {
	return change.Status() == SUSPECT &&
		((member.Status() == SUSPECT && change.Incarnation() > member.Incarnation()) ||
			(member.Status() == FAULTY && change.Incarnation() > member.Incarnation()) ||
			(member.Status() == ALIVE && change.Incarnation() >= member.Incarnation()))
}

func IsLocalFaultyOverride(ringpop *Ringpop, member *Member, change Change) bool {
	return member.Address() == ringpop.WhoAmI() && change.Status() == FAULTY
}

func IsLocalSuspectOverride(ringpop *Ringpop, member *Member, change Change) bool {
	return member.Address() == ringpop.WhoAmI() && change.Status() == SUSPECT
}
