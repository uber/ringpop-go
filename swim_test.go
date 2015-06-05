package ringpop

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	GOSSIP TESTS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

// TODO

func TestStartStopGossip(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000")

	ringpop.gossip.start()
	assert.NotNil(t, ringpop.gossip.protocolRateTimer, "expected protocol rate timer to be set")
	assert.NotNil(t, ringpop.gossip.protocolPeriodTimer, "expected protocol period timer to be set")

	ringpop.gossip.stop()
	assert.Nil(t, ringpop.gossip.protocolRateTimer, "expected protocol rate timer to be cleared")
	assert.Nil(t, ringpop.gossip.protocolPeriodTimer, "expected protocol period timer to be cleared")
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	SUSPICION TESTS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func TestCannotStartSuspectPeriodWhileDisabled(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000")

	ringpop.membership.makeAlive("127.0.0.1:3001", time.Now().UnixNano(), "")

	ringpop.suspicion.stopAll()
	assert.True(t, ringpop.suspicion.stopped, "expected suspicion protocol to be stopped")

	member, _ := ringpop.membership.getMemberByAddress("127.0.0.1:3001")
	ringpop.suspicion.start(member)
	assert.Nil(t, ringpop.suspicion.timers[member.Address()], "expected timer for member to be nil")

	ringpop.suspicion.reenable()
	assert.False(t, ringpop.suspicion.stopped, "expected suspcion protocol to be reenabled")

	ringpop.suspicion.start(member)
	assert.NotNil(t, ringpop.suspicion.timers[member.Address()], "expected time for member to be set")
}

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
	ringpop.suspicion.period = time.Duration(1 * time.Millisecond)

	ringpop.membership.makeAlive("127.0.0.1:3001", time.Now().UnixNano(), "")

	member, _ := ringpop.membership.getMemberByAddress("127.0.0.1:3001")

	ringpop.suspicion.start(member)

	time.Sleep(1 * time.Millisecond)

	assert.Equal(t, FAULTY, member.Status(), "expected suspicion to make member faulty")
}

func TestSuspectTimerStoppedOnDuplicateSuspicion(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000")
	ringpop.suspicion.period = time.Duration(math.MaxInt64)

	ringpop.membership.makeAlive("127.0.0.1:3001", time.Now().UnixNano(), "")

	member, _ := ringpop.membership.getMemberByAddress("127.0.0.1:3001")

	ringpop.suspicion.start(member)
	assert.NotNil(t, ringpop.suspicion.timers[member.Address()], "expected timer to be set for member suspect period")

	oldtimer := ringpop.suspicion.timers[member.Address()]

	ringpop.suspicion.start(member)
	assert.NotNil(t, ringpop.suspicion.timers[member.Address()], "expected timer to be set for member suspect period")

	assert.NotEqual(t, oldtimer, ringpop.suspicion.timers[member.Address()], "expected timer to be changed")
}

func TestStopTimer(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000")
	ringpop.suspicion.period = time.Duration(math.MaxInt64)

	ringpop.membership.makeAlive("127.0.0.1:3001", time.Now().UnixNano(), "")

	member, _ := ringpop.membership.getMemberByAddress("127.0.0.1:3001")

	ringpop.suspicion.start(member)
	assert.NotNil(t, ringpop.suspicion.timers[member.Address()], "expected timer to be set for member suspect period")

	ringpop.suspicion.stop(member)
	assert.Nil(t, ringpop.suspicion.timers[member.Address()], "expected timer to be stopped")
}

func TestStopAllStopsAllTimers(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000")
	ringpop.suspicion.period = time.Duration(math.MaxInt64)

	ringpop.membership.makeAlive("127.0.0.1:3001", time.Now().UnixNano(), "")
	ringpop.membership.makeAlive("127.0.0.1:3002", time.Now().UnixNano(), "")

	member1, _ := ringpop.membership.getMemberByAddress("127.0.0.1:3001")
	member2, _ := ringpop.membership.getMemberByAddress("127.0.0.1:3002")

	ringpop.suspicion.start(member1)
	ringpop.suspicion.start(member2)

	assert.NotNil(t, ringpop.suspicion.timers[member1.Address()],
		"expected timer to be set for member suspect period")
	assert.NotNil(t, ringpop.suspicion.timers[member2.Address()],
		"expected timer to be set for member suspect period")

	ringpop.suspicion.stopAll()

	assert.True(t, ringpop.suspicion.stopped, "expected suspicion protocol to be stopped")
	assert.Nil(t, ringpop.suspicion.timers[member1.Address()], "expected timer for member to be stopped")
	assert.Nil(t, ringpop.suspicion.timers[member2.Address()], "expected timer for member to be stopped")
}
