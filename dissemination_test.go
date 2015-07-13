package ringpop

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFullSync(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000", 0, nil)
	defer ringpop.Destroy()

	d := ringpop.dissemination

	ringpop.membership.makeAlive("127.0.0.1:3001", 1)
	ringpop.membership.makeAlive("127.0.0.1:3002", 1)

	changes := ringpop.dissemination.fullSync()
	var addrs []string
	for _, change := range changes {
		addrs = append(addrs, change.Address)
	}

	_, ok := d.changes["127.0.0.1:3000"]
	assert.True(t, ok, "expected address to be in changes")

	_, ok = d.changes["127.0.0.1:3001"]
	assert.True(t, ok, "expected address to be in changes")

	_, ok = d.changes["127.0.0.1:3002"]
	assert.True(t, ok, "expected address to be in changes")

	_, ok = d.changes["127.0.0.1:3003"]
	assert.False(t, ok, "expected address to not be in changes")
}

func TestIssueChangesAsSender(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000", 0, nil)
	defer ringpop.Destroy()

	d := ringpop.dissemination

	addressAlive := "127.0.0.1:3001"
	addressSuspect := "127.0.0.1:3002"
	addressFaulty := "127.0.0.1:3003"
	incarnation := unixMilliseconds(time.Now())

	ringpop.dissemination.clearChanges()

	ringpop.membership.makeAlive(addressAlive, incarnation)
	ringpop.membership.makeSuspect(addressSuspect, incarnation)
	ringpop.membership.makeFaulty(addressFaulty, incarnation)

	changes := d.issueChangesAsSender()
	assert.Len(t, changes, 3, "expected to get three changes")

}

func TestIssueChangesAsReceiver(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000", 0, nil)
	defer ringpop.Destroy()

	d := ringpop.dissemination

	local := ringpop.membership.localMember

	addressAlive := "127.0.0.1:3001"
	addressSuspect := "127.0.0.1:3002"
	addressFaulty := "127.0.0.1:3003"
	incarnation := unixMilliseconds(time.Now())

	d.clearChanges()

	ringpop.membership.makeAlive(addressAlive, incarnation)
	ringpop.membership.makeSuspect(addressSuspect, incarnation)
	ringpop.membership.makeFaulty(addressFaulty, incarnation)

	changes, fullSync := d.issueChangesAsReceiver(local.Address, local.Incarnation,
		ringpop.membership.checksum)

	assert.Len(t, changes, 0, "expected no changes to be issued for same sender/receiver")
	assert.False(t, fullSync, "expected no changes to be issued for same sender/receiver")

	changes, fullSync = d.issueChangesAsReceiver(addressAlive, incarnation, ringpop.membership.checksum)

	assert.Len(t, changes, 3, "expected to get three changes")
	assert.False(t, fullSync, "expected changes to not be a fullsync")

	d.clearChanges()

	changes, fullSync = d.issueChangesAsReceiver(addressAlive, incarnation, ringpop.membership.checksum)

	assert.Len(t, changes, 0, "expected to get no changes")
	assert.False(t, fullSync, "expected changes to not be a fullsync")

	changes, fullSync = d.issueChangesAsReceiver(addressAlive, incarnation, ringpop.membership.checksum+1)

	assert.Len(t, changes, 4, "expected change to be issued for each memeber in membership")
	assert.True(t, fullSync, "expected changes to be a fullsync")
}

func TestPiggybackCountDeletes(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000", 0, nil)
	defer ringpop.Destroy()

	d := ringpop.dissemination
	d.clearChanges()

	d.maxPiggybackCount = 2

	address := "127.0.0.1:3001"
	incarnation := unixMilliseconds(time.Now())

	ringpop.membership.makeAlive(address, incarnation)

	assert.Equal(t, 0, d.changes[address].piggybackCount, "expected piggy back count to be 0")

	changes := d.issueChangesAsSender()
	assert.Equal(t, 1, d.changes[address].piggybackCount, "expected piggybackCount for change to be 1")
	assert.Len(t, changes, 1, "expected one change to be issued")

	changes = d.issueChangesAsSender()
	assert.Equal(t, 2, d.changes[address].piggybackCount, "expected piggybackCount for change to be 2")
	assert.Len(t, changes, 1, "expected one change to be issued")

	changes = d.issueChangesAsSender()
	assert.Len(t, changes, 0, "expected no changes to be issued")

	_, ok := d.changes[address]
	assert.False(t, ok, "expected change to have been deleted from dissemination changes")
}
