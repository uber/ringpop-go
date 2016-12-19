package membership

// MemberChange shows the state before and after the change of a Member
type MemberChange struct {
	// Before is the state of the member before the change, if the
	// member is a new member the before state is nil
	Before Member
	// After is the state of the member after the change, if the
	// member left the after state will be nil
	After Member
}

// ChangeEvent indicates that the membership has changed. The event will contain
// a list of changes that will show both the old and the new state of a member.
// It is not guaranteed that any of the observable state of a member has in fact
// changed, it might only be an interal state change for the underlying
// membership.
type ChangeEvent struct {
	// Changes is a slice of changes that is related to this event
	Changes []MemberChange
}
