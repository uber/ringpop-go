package ringpop

import (
	"math"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

var localMemberUpdate = Change{
	Address:     "127.0.0.1:3000",
	Status:      ALIVE,
	Incarnation: 123456789,
}

var remoteMemberUpdate = Change{
	Address:     "127.0.0.1:3001",
	Status:      ALIVE,
	Incarnation: 123456789,
}

var updates = []Change{localMemberUpdate, remoteMemberUpdate}

func testRollup(interval time.Duration) *membershipUpdateRollup {
	if interval == time.Duration(0) {
		interval = time.Duration(math.MaxInt64)
	}

	ringpop := testPop("127.0.0.1:3000", 0, &Options{
		MembershipUpdateFlushInterval: interval,
	})

	return ringpop.membershipUpdateRollup
}

func TestTrackUpdatesEmpty(t *testing.T) {

	rollup := testRollup(time.Duration(0))

	rollup.trackUpdates([]Change{})
	rollup.destroy()
}

func TestTrackUpdates(t *testing.T) {
	rollup := testRollup(time.Duration(0))

	rollup.trackUpdates(updates)
	assert.NotNil(t, rollup.flushTimer, "expected flushTimer to be set")
	rollup.destroy()
}

func TestFlushEmptyBuffer(t *testing.T) {
	rollup := testRollup(time.Duration(0))

	rollup.addUpdates(updates)
	assert.NotEmpty(t, rollup.buffer, "expected updates to be in buffer")
	rollup.flushBuffer()
	assert.Empty(t, rollup.buffer, "expected buffer to be empty")
	rollup.flushBuffer()
	assert.Empty(t, rollup.buffer, "expected buffer to be empty")
}

func TestBufferFlushedOnIntervalExceed(t *testing.T) {
	rollup := testRollup(time.Hour)

	rollup.trackUpdates(updates)
	assert.NotNil(t, rollup.flushTimer, "expected flushTimer to be set")

	rollup.lastUpdateTime = time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)

	flushed := rollup.trackUpdates(updates)
	assert.True(t, flushed, "expected buffer to be flushed")
	assert.Equal(t, "flushed", <-rollup.eventC, "expected buffer to be flushed")
}

func TestMultipleFlush(t *testing.T) {
	rollup := testRollup(time.Hour)

	rollup.trackUpdates(updates)
	rollup.flushBuffer()
	assert.Equal(t, "flushed", <-rollup.eventC, "expected flush")
	assert.Empty(t, rollup.buffer, "expected buffer to be empty")

	rollup.trackUpdates(updates)
	rollup.flushBuffer()
	assert.Equal(t, "flushed", <-rollup.eventC, "expected flush")
	assert.Empty(t, rollup.buffer, "expected buffer to be empty")

	rollup.destroy()
}

func TestFlushing(t *testing.T) {
	rollup := testRollup(1 * time.Millisecond)

	rollup.trackUpdates(updates)

	time.Sleep(2 * time.Millisecond)

	assert.Equal(t, "flushed", <-rollup.eventC, "expected flush")

	rollup.trackUpdates(updates)

	time.Sleep(2 * time.Millisecond)

	assert.Equal(t, "flushed", <-rollup.eventC, "expected flush")

	rollup.destroy()
}
