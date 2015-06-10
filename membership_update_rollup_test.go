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
	return newMembershipUpdateRollup(testPop("127.0.0.1:3000"), interval, 0)
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

func TestBufferFlushedOnIntervalExceed(t *testing.T) {
	rollup := testRollup(time.Duration(123456789))

	rollup.trackUpdates(updates)
	assert.NotNil(t, rollup.flushTimer, "expected flushTimer to be set")

	rollup.lastUpdateTime = time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC) // simulate time passing

	// revisit... should this be flushing even the updates we are adding
	// or only what's previously in the buffer?
	rollup.trackUpdates(updates)

	assert.Equal(t, "flushed", <-rollup.eventC, "expected flush")
	assert.Equal(t, 0, len(rollup.buffer), "expected buffer to be empty")
}

func TestMultipleFlush(t *testing.T) {
	rollup := testRollup(time.Duration(123456789))

	rollup.trackUpdates(updates)
	rollup.flushBuffer()
	assert.Equal(t, "flushed", <-rollup.eventC, "expected flush")
	assert.Equal(t, 0, len(rollup.buffer), "expected buffer to be empty")

	rollup.trackUpdates(updates)
	rollup.flushBuffer()
	assert.Equal(t, "flushed", <-rollup.eventC, "expected flush")
	assert.Equal(t, 0, len(rollup.buffer), "expected buffer to be empty")

	rollup.destroy()
}

func testFlushing(t *testing.T) {
	rollup := testRollup(2000 * time.Millisecond)

	rollup.trackUpdates(updates)

	time.Sleep(2100 * time.Millisecond)

	assert.Equal(t, "flushed", <-rollup.eventC, "expected flush")

	rollup.addUpdates(updates)

	time.Sleep(2100 * time.Millisecond)

	rollup.destroy()
}
