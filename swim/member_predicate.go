package swim

// MemberPredicate is a function that tests if a Member satisfies a condition.
// It is advised to use exported functions on Member instead of its exported
// fields in case we want to extract the functionality of Member to an Interface
// in the future. This is likely to happen if we pursue plugable membership.
type MemberPredicate func(member Member) bool

// TestMember can take multiple predicates and test them against a member
// returning if the member satisfies all the predicates. This means that if one
// test fails it will stop executing and return with false.
func TestMember(member Member, predicates ...MemberPredicate) bool {
	for _, p := range predicates {
		if !p(member) {
			return false
		}
	}
	return true
}

// ReachableMember tests if a member is deemed to be reachable. This filters out
// all members that are known to be unresponsive. Most operations will only ever
// be concerned with Members that are in a Reachable state. In SWIM terms a
// member is considered reachable when it is either in Alive status or in
// Suspect status. All other Members are considered to not be reachable.
func ReachableMember(member Member) bool {
	return member.Status == Alive || member.Status == Suspect
}

// LabeledMember returns a predicate able to test if the value of a label on a
// member is equal to the provided value.
func LabeledMember(key, value string) MemberPredicate {
	return func(member Member) bool {
		v, ok := member.Labels[key]

		if !ok {
			return false
		}

		// test if the values match
		return v == value
	}
}
