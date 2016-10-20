package swim

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

var memberIsReachableTests = []struct {
	member    Member
	reachable bool
}{
	{Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Alive, Labels: nil}, true},
	{Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Suspect, Labels: nil}, true},
	{Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Faulty, Labels: nil}, false},
	{Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Leave, Labels: nil}, false},
	{Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Tombstone, Labels: nil}, false},
}

func TestMemberIsReachable(t *testing.T) {
	var predicate MemberPredicate
	predicate = memberIsReachable

	for _, test := range memberIsReachableTests {
		assert.Equal(t, test.reachable, predicate(test.member), "member: %v expected: %b", test.member, test.reachable)
	}
}

var memberWithLabelAndValueTests = []struct {
	member    Member
	reachable bool
}{
	{Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Alive, Labels: nil}, false},
	{Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Alive, Labels: map[string]string{"hello": "world"}}, true},
	{Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Alive, Labels: map[string]string{"hello": "world", "foo": "bar"}}, true},
	{Member{Address: "192.0.2.1:1234", Incarnation: 42, Status: Alive, Labels: map[string]string{"foo": "bar"}}, false},
}

func TestMemberWithLabelAndValue(t *testing.T) {
	var predicate MemberPredicate
	predicate = MemberWithLabelAndValue("hello", "world")

	for _, test := range memberWithLabelAndValueTests {
		assert.Equal(t, test.reachable, predicate(test.member), "member: %v expected: %b", test.member, test.reachable)
	}
}

var truePredicate = func(member Member) bool { return true }
var falsePredicate = func(member Member) bool { return false }

var memberMatchesPredicatesTests = []struct {
	predicates []MemberPredicate
	matches    bool
}{
	{nil, true},
	{[]MemberPredicate{}, true},
	{[]MemberPredicate{truePredicate}, true},
	{[]MemberPredicate{falsePredicate}, false},
	{[]MemberPredicate{truePredicate, truePredicate}, true},
	{[]MemberPredicate{truePredicate, falsePredicate}, false},

	{[]MemberPredicate{MemberWithLabelAndValue("hello", "world")}, true},
	{[]MemberPredicate{MemberWithLabelAndValue("foo", "bar")}, false},
}

func TestMemberMatchesPredicates(t *testing.T) {
	member := Member{
		Address:     "192.0.2.1:1234",
		Incarnation: 42,
		Status:      Alive,
		Labels:      map[string]string{"hello": "world"},
	}

	for _, test := range memberMatchesPredicatesTests {
		assert.Equal(t, test.matches, MemberMatchesPredicates(member, test.predicates...))
	}
}
