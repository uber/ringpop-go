package ringpop

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestServerJoin(t *testing.T) {
	hostport1 := "127.0.0.1:3001"
	hostport2 := "127.0.0.1:3002"

	var hosts []string

	hosts = append(hosts, hostport1)
	ringpop1 := newServerRingpop(t, hostport1)
	defer ringpop1.Destroy()

	nodesJoined, err := ringpop1.Bootstrap(&BootstrapOptions{
		Hosts:   hosts,
		Stopped: true,
	})

	assert.NoError(t, err, "expected bootstrap to complete successfully")
	assert.Empty(t, nodesJoined, "expected join of size zero (self only)")
	assert.True(t, ringpop1.Ready(), "expected ringpop to be ready")
	assert.True(t, ringpop1.gossip.Stopped(), "expected gossip to be stopped")

	hosts = append(hosts, hostport2)
	ringpop2 := newServerRingpop(t, hostport2)
	defer ringpop2.Destroy()

	nodesJoined, err = ringpop2.Bootstrap(&BootstrapOptions{
		Hosts:   hosts,
		Stopped: true,
	})

	assert.NoError(t, err, "expected bootstrap to complete successfully")
	assert.Len(t, nodesJoined, 1, "expected join of size one")
	assert.True(t, ringpop2.Ready(), "expected ringpop to be ready")
	assert.True(t, ringpop2.gossip.Stopped(), "expected gossip to be stopped")
}

func TestServerJoinTimesOut(t *testing.T) {
	hosts := genNodes(1, 10)

	ringpop := newServerRingpop(t, hosts[1])
	defer ringpop.Destroy()

	_, err := ringpop.Bootstrap(&BootstrapOptions{
		Hosts:           hosts,
		MaxJoinDuration: 1 * time.Millisecond,
	})

	assert.Error(t, err, "expected join to timeout")
}

func TestServerJoinSix(t *testing.T) {
	ringpops := genRingpops(t, genNodes(1, 7))
	defer destroyRingpops(ringpops)

	var hosts []string
	for _, ringpop := range ringpops {
		hosts = append(hosts, ringpop.WhoAmI())

		nodesJoined, err := ringpop.Bootstrap(&BootstrapOptions{
			Hosts:   hosts,
			Stopped: true,
		})

		assert.Len(t, nodesJoined, len(hosts)-1, "expected to join to all nodes in hosts but self")
		assert.NoError(t, err, "expected bootstrap to complete successfully")
	}
}

func TestServerJoinsTimeout(t *testing.T) {
	ringpop := newServerRingpop(t, "127.0.0.1:3001")
	defer ringpop.Destroy()

	hosts := []string{
		"127.0.0.1:3001",
		"127.0.0.2:3001",
		"127.0.0.2:3002",
	}

	_, err := ringpop.Bootstrap(&BootstrapOptions{
		Hosts:           hosts,
		Timeout:         time.Millisecond / 2,
		MaxJoinDuration: time.Millisecond,
	})

	assert.Error(t, err, "expected join to timeout")
}

func TestServerGossipPing(t *testing.T) {
	ringpop1 := newServerRingpop(t, "127.0.0.1:3001")
	defer ringpop1.Destroy()

	nodesJoined, err := ringpop1.Bootstrap(&BootstrapOptions{
		Hosts:   []string{"127.0.0.1:3001"},
		Stopped: true,
	})

	require.NoError(t, err, "ringpop must bootstrap successfully")
	require.Empty(t, nodesJoined, "ringpop must only join self")

	ringpop2 := newServerRingpop(t, "127.0.0.1:3002")
	defer ringpop2.Destroy()

	nodesJoined, err = ringpop2.Bootstrap(&BootstrapOptions{
		Hosts:   []string{"127.0.0.1:3001", "127.0.0.1:3002"},
		Stopped: true,
	})

	require.NoError(t, err, "ringpop must bootstrap successfully")
	require.Len(t, nodesJoined, 1, "ringpop must only join one other ringpop")
	require.Equal(t, "127.0.0.1:3001", nodesJoined[0], "ringpop must join correct ringpop")

	ringpop1.dissemination.clearChanges()
	ringpop2.dissemination.clearChanges()

	ringpop2.membership.makeAlive("127.0.0.1:3003", unixMilliseconds(time.Now()))
	assert.NotEqual(t, ringpop1.membership.checksum, ringpop2.membership.checksum,
		"expected ringpop checksums to be different")

	ringpop1.gossip.gossip()
	assert.Equal(t, ringpop1.membership.checksum, ringpop1.membership.checksum,
		"expected ringpop checksums to converge on gossip ping")
}

func TestServerGossipPingFails(t *testing.T) {
	ringpop := newServerRingpop(t, "127.0.0.1:3001")
	defer ringpop.Destroy()

	nodesJoined, err := ringpop.Bootstrap(&BootstrapOptions{
		Hosts:   []string{"127.0.0.1:3001"},
		Stopped: true,
	})

	require.NoError(t, err, "expected bootstrap to have completed successfully")
	require.Empty(t, nodesJoined, "ringpop must only join self")

	remote := "127.0.0.1:3002"

	res, err := sendPing(ringpop, remote, time.Millisecond)
	assert.Error(t, err, "expected ping to have failed")
	assert.Nil(t, res, "expected res to be nil")
}

func TestServerGossipPingTimesOut(t *testing.T) {
	ringpop := newServerRingpop(t, "127.0.0.1:3001")
	defer ringpop.Destroy()

	nodesJoined, err := ringpop.Bootstrap(&BootstrapOptions{
		Hosts:   []string{"127.0.0.1:3001"},
		Stopped: true,
	})

	require.NoError(t, err, "expected bootstrap to have completed successfully")
	require.Empty(t, nodesJoined, "ringpop must only join self")

	remote := "127.0.0.2:3001"

	res, err := sendPing(ringpop, remote, time.Millisecond)
	assert.Error(t, err, "expected ping to have timed out")
	assert.Nil(t, res, "expected res to be nil")
}

func TestServerSkipHosts(t *testing.T) {
	ringpop := newServerRingpop(t, "127.0.0.1:3001")
	defer ringpop.Destroy()

	nodesJoined, err := ringpop.Bootstrap(&BootstrapOptions{
		Hosts:     []string{"127.0.0.1:3001"},
		SkipHosts: []string{"127.0.0.1:3002"},
		Stopped:   true,
	})

	require.NoError(t, err, "ringpop must bootstrap successfully")
	require.Empty(t, nodesJoined, "ringpop must only join self")

	ringpop.membership.makeAlive("127.0.0.1:3001", unixMilliseconds(time.Now()))
	ringpop.membership.makeAlive("127.0.0.1:3002", unixMilliseconds(time.Now()))

	ringpop2 := newServerRingpop(t, "127.0.0.1:3002")
	defer ringpop2.Destroy()

	nodesJoined, err = ringpop2.Bootstrap(&BootstrapOptions{
		Hosts:     []string{"127.0.0.1:3001", "127.0.0.1:3002"},
		SkipHosts: []string{"127.0.0.1:3002"},
	})
	require.NoError(t, err, "ringpop must bootstrap successfully")
	require.Len(t, nodesJoined, 1, "ringpop must only join one other ringpop")
	require.Equal(t, "127.0.0.1:3001", nodesJoined[0], "ringpop must join correct ringpop")

	// The ring server count should just be 1, since we skipped one host
	assert.Equal(t, 1, ringpop.ring.serverCount())
	assert.Equal(t, 1, ringpop2.ring.serverCount())
}
