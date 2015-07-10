package ringpop

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

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
