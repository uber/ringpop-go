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
	ringpop := testPop("127.0.0.1:3000", 0)
	defer ringpop.Destroy()

	ringpop.gossip.start()
	assert.False(t, ringpop.gossip.Stopped(), "expected gossip to have started")

	ringpop.gossip.stop()
	assert.True(t, ringpop.gossip.Stopped(), "expected gossip to have stopped")
}

func TestStopWhileStopped(t *testing.T) {
	ringpop := testPop("127.0.0.1:3001", 0)
	defer ringpop.Destroy()

	assert.True(t, ringpop.gossip.Stopped(), "expected gossip to be stopped")

	ringpop.gossip.stop()
	assert.True(t, ringpop.gossip.Stopped(), "expected gossip to still be stopped")
}

func TestCanRestartGossip(t *testing.T) {
	ringpop := testPop("127.0.0.1:3001", 0)
	defer ringpop.Destroy()

	assert.True(t, ringpop.gossip.Stopped(), "expected gossip to be stopped")

	ringpop.gossip.start()
	assert.False(t, ringpop.gossip.Stopped(), "expected gossip to have started")

	ringpop.gossip.stop()
	assert.True(t, ringpop.gossip.Stopped(), "expected gossip to be stopped")

	ringpop.gossip.start()
	assert.False(t, ringpop.gossip.Stopped(), "expected gossip to have started")
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	SUSPICION TESTS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func TestReenableWhileEnabled(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000", 0)
	defer ringpop.Destroy()

	ringpop.suspicion.stopAll()
	assert.True(t, ringpop.suspicion.stopped, "expected suspicion protocol to be stopped")

	ringpop.suspicion.reenable()
	assert.False(t, ringpop.suspicion.stopped, "expected suspicion protocol to be reenabled")

	ringpop.suspicion.reenable()
	assert.False(t, ringpop.suspicion.stopped, "expected suspicion protocol to still be enabled")
}

func TestCannotStartSuspectPeriodWhileDisabled(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000", 0)
	defer ringpop.Destroy()

	ringpop.membership.makeAlive("127.0.0.1:3001", unixMilliseconds(time.Now()), "")

	ringpop.suspicion.stopAll()
	assert.True(t, ringpop.suspicion.stopped, "expected suspicion protocol to be stopped")

	member, _ := ringpop.membership.getMemberByAddress("127.0.0.1:3001")

	ringpop.suspicion.start(*member)
	assert.Nil(t, ringpop.suspicion.timers[member.Address], "expected suspicion timer to be nil")

	ringpop.suspicion.reenable()
	assert.False(t, ringpop.suspicion.stopped, "expected suspicion protocol to be reenabled")

	ringpop.suspicion.start(*member)
	assert.NotNil(t, ringpop.suspicion.timers[member.Address], "expected suspicion timer to be set")
}

func TestSuspectMember(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000", 0)
	defer ringpop.Destroy()

	ringpop.membership.makeAlive("127.0.0.1:3001", unixMilliseconds(time.Now()), "")

	member, _ := ringpop.membership.getMemberByAddress("127.0.0.1:3001")
	ringpop.suspicion.start(*member)
	assert.NotNil(t, ringpop.suspicion.timers[member.Address], "expected suspicion timer to be set")
}

func TestSuspectLocalMember(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000", 0)
	defer ringpop.Destroy()

	member := ringpop.membership.localmember
	ringpop.suspicion.start(*member)
	assert.Nil(t, ringpop.suspicion.timers[member.Address], "expected suspicion timer to be nil")
}

func TestSuspectBecomesFaulty(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000", 0)
	defer ringpop.Destroy()

	ringpop.suspicion.period = time.Duration(1 * time.Millisecond)

	ringpop.membership.makeAlive("127.0.0.1:3001", unixMilliseconds(time.Now()), "")

	member, _ := ringpop.membership.getMemberByAddress("127.0.0.1:3001")

	time.Sleep(1 * time.Millisecond)
	ringpop.suspicion.start(*member)
	time.Sleep(1 * time.Millisecond)

	assert.Equal(t, FAULTY, member.Status, "expected suspicion to make member faulty")
}

func TestSuspectTimerStoppedOnDuplicateSuspicion(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000", 0)
	defer ringpop.Destroy()

	ringpop.suspicion.period = time.Duration(math.MaxInt64)

	ringpop.membership.makeAlive("127.0.0.1:3001", unixMilliseconds(time.Now()), "")

	member, _ := ringpop.membership.getMemberByAddress("127.0.0.1:3001")

	ringpop.suspicion.start(*member)
	assert.NotNil(t, ringpop.suspicion.timers[member.Address], "expected timer to be set for member suspect period")

	oldtimer := ringpop.suspicion.timers[member.Address]

	ringpop.suspicion.start(*member)
	assert.NotNil(t, ringpop.suspicion.timers[member.Address], "expected timer to be set for member suspect period")

	assert.NotEqual(t, oldtimer, ringpop.suspicion.timers[member.Address], "expected timer to be changed")
}

func TestStopTimer(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000", 0)
	defer ringpop.Destroy()

	ringpop.suspicion.period = time.Duration(math.MaxInt64)

	ringpop.membership.makeAlive("127.0.0.1:3001", unixMilliseconds(time.Now()), "")

	member, _ := ringpop.membership.getMemberByAddress("127.0.0.1:3001")

	ringpop.suspicion.start(*member)
	assert.NotNil(t, ringpop.suspicion.timers[member.Address], "expected timer to be set for member suspect period")

	ringpop.suspicion.stop(*member)
	assert.Nil(t, ringpop.suspicion.timers[member.Address], "expected timer to be stopped")
}

func TestStopAllStopsAllTimers(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000", 0)
	defer ringpop.Destroy()

	ringpop.suspicion.period = time.Duration(math.MaxInt64)

	ringpop.membership.makeAlive("127.0.0.1:3001", unixMilliseconds(time.Now()), "")
	ringpop.membership.makeAlive("127.0.0.1:3002", unixMilliseconds(time.Now()), "")

	member1, _ := ringpop.membership.getMemberByAddress("127.0.0.1:3001")
	member2, _ := ringpop.membership.getMemberByAddress("127.0.0.1:3002")

	ringpop.suspicion.start(*member1)
	ringpop.suspicion.start(*member2)

	assert.NotNil(t, ringpop.suspicion.timers[member1.Address],
		"expected timer to be set for member suspect period")
	assert.NotNil(t, ringpop.suspicion.timers[member2.Address],
		"expected timer to be set for member suspect period")

	ringpop.suspicion.stopAll()

	assert.True(t, ringpop.suspicion.stopped, "expected suspicion protocol to be stopped")
	assert.Nil(t, ringpop.suspicion.timers[member1.Address], "expected timer for member to be stopped")
	assert.Nil(t, ringpop.suspicion.timers[member2.Address], "expected timer for member to be stopped")
}
