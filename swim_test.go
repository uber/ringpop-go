package ringpop

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestSuspectMember(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000")

	ringpop.membership.makeAlive("127.0.0.1:3001", time.Now().UnixNano(), "")

	member, _ := ringpop.membership.getMemberByAddress("127.0.0.1:3001")

	ringpop.suspicion.start(member)

	_, ok := ringpop.suspicion.timers[member.Address()]
	assert.True(t, ok, "expected timer to be set for member suspect period")
}

func TestSuspectLocalMember(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000")

	member, _ := ringpop.membership.getMemberByAddress(ringpop.WhoAmI())

	ringpop.suspicion.start(member)

	_, ok := ringpop.suspicion.timers[member.Address()]
	assert.False(t, ok, "expected timer to not be set for local member suspect period")
}

func TestSuspectBecomesFaulty(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000")

	ringpop.membership.makeAlive("127.0.0.1:3001", time.Now().UnixNano(), "")

	// member, _ := ringpop.membership.getMemberByAddress("127.0.0.1:3001")

}
