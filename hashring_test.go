package ringpop

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func testRing() *hashRing {
	ringpop := testPop("127.0.0.1:3000", unixMilliseconds(time.Now()), nil)
	return ringpop.ring
}

func TestRingAddServer(t *testing.T) {
	ring := testRing()

	ring.addServer("server1")
	ring.addServer("server2")

	assert.True(t, ring.hasServer("server1"), "expected server to be in ring")
	assert.True(t, ring.hasServer("server2"), "expected server to be in ring")
	assert.False(t, ring.hasServer("server3"), "expected server to not be in ring")

	ring.addServer("server1")
	assert.True(t, ring.hasServer("server1"), "expected server to be in ring")
}

func TestRingRemoveServer(t *testing.T) {
	ring := testRing()

	ring.addServer("server1")
	ring.addServer("server2")

	assert.True(t, ring.hasServer("server1"), "expected server to be in ring")
	assert.True(t, ring.hasServer("server2"), "expected server to be in ring")

	ring.removeServer("server1")

	assert.False(t, ring.hasServer("server1"), "expected server to not be in ring")
	assert.True(t, ring.hasServer("server2"), "expected server to be in ring")

	ring.removeServer("server3")

	assert.False(t, ring.hasServer("server3"), "expected server to not be in ring")
}

func TestRingChecksumChanges(t *testing.T) {
	ring := testRing()

	checksum := ring.checksum

	ring.addServer("server1")
	ring.addServer("server2")

	assert.NotEqual(t, checksum, ring.checksum, "expected checksum to have changed on server add")

	checksum = ring.checksum

	ring.removeServer("server1")

	assert.NotEqual(t, checksum, ring.checksum, "expected checksum to have changed on server remove")
}

func TestRingServerCount(t *testing.T) {
	ring := testRing()

	assert.Equal(t, 1, ring.serverCount(), "expected one server to be in ring")

	ring.addServer("server1")
	ring.addServer("server2")

	assert.Equal(t, 3, ring.serverCount(), "expected three servers to be in ring")

	ring.removeServer("server1")

	assert.Equal(t, 2, ring.serverCount(), "expected two servers to be in ring")
}

func TestRingLookup(t *testing.T) {
	ring := testRing()

	ring.addServer("server1")
	ring.addServer("server2")

	_, ok := ring.lookup("key")

	assert.True(t, ok, "expected lookup to hash key to a server")

	ring.removeServer("server1")
	ring.removeServer("server2")
	ring.removeServer(ring.ringpop.membership.localMember.Address)

	_, ok = ring.lookup("key")

	assert.False(t, ok, "expected lookup to find no server for key to hash to")
}
