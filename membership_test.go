package ringpop

import (
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	MEMBERSHIP TESTS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func TestMembershipChecksumChanges(t *testing.T) {
	// testPop makes the local member alive
	ringpop := testPop("127.0.0.1:3000", 0, nil)
	defer ringpop.Destroy()

	oldchecksum := ringpop.membership.checksum

	ringpop.membership.makeAlive("127.0.0.1:3001", unixMilliseconds(time.Now()))

	assert.NotEqual(t, oldchecksum, ringpop.membership.checksum,
		"expected checksum to have changed on membership change")
}

func TestMembershipChecksumEqual(t *testing.T) {
	incarnation := unixMilliseconds(time.Now())

	ringpop := testPop("127.0.0.1:3000", incarnation, nil)
	defer ringpop.Destroy()
	ringpop2 := testPop("127.0.0.1:3000", incarnation, nil)
	defer ringpop2.Destroy()

	ringpop.membership.makeAlive("127.0.0.1:3001", incarnation)
	ringpop.membership.makeAlive("127.0.0.1:3002", incarnation)
	ringpop.membership.makeAlive("127.0.0.1:3003", incarnation)

	ringpop2.membership.makeAlive("127.0.0.1:3003", incarnation)
	ringpop2.membership.makeAlive("127.0.0.1:3001", incarnation)
	ringpop2.membership.makeAlive("127.0.0.1:3002", incarnation)

	assert.Equal(t, ringpop.membership.checksum, ringpop2.membership.checksum,
		"expected checksums to be equal, regardless of input order")
}

// Higher incarnation should result in a leave override
func TestMembershipLeaveOverrideHigher(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000", 0, nil)
	defer ringpop.Destroy()

	member, found := ringpop.membership.getMemberByAddress(ringpop.WhoAmI())
	assert.True(t, found, "expected new member to be found")
	assert.Equal(t, ALIVE, member.Status, "expected member to start as alive")

	ringpop.membership.update([]Change{Change{
		Address:     member.Address,
		Status:      LEAVE,
		Incarnation: member.Incarnation + 1,
	}})

	assert.Equal(t, LEAVE, member.Status, "expected member status to be leave")
}

func TestMembershipLeaveOverrideEqual(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000", 0, nil)
	defer ringpop.Destroy()

	member, found := ringpop.membership.getMemberByAddress(ringpop.WhoAmI())
	assert.True(t, found, "expected member to be found")
	assert.Equal(t, ALIVE, member.Status, "expected member to start as alive")

	ringpop.membership.update([]Change{Change{
		Address:     member.Address,
		Status:      LEAVE,
		Incarnation: member.Incarnation,
	}})

	assert.Equal(t, LEAVE, member.Status, "expected member status to still be alive")
}

// Lower incarnation should not result in a leave override
func TestMembershipLeaveOverrideLower(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000", 0, nil)
	defer ringpop.Destroy()

	member, found := ringpop.membership.getMemberByAddress(ringpop.WhoAmI())
	assert.True(t, found, "expected member to be found")
	assert.Equal(t, ALIVE, member.Status, "expected member to start as alive")

	ringpop.membership.update([]Change{Change{
		Address:     member.Address,
		Status:      LEAVE,
		Incarnation: member.Incarnation - 1,
	}})

	assert.Equal(t, ALIVE, member.Status, "expected member status to still be alive")
}

// Attempting to make the local member faulty should not change local member status
func TestMembershipLocalFaultyOverride(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000", 0, nil)
	defer ringpop.Destroy()

	member, found := ringpop.membership.getMemberByAddress(ringpop.WhoAmI())
	assert.True(t, found, "expected member to be found")
	assert.Equal(t, ALIVE, member.Status, "expected local member to start as alive")

	ringpop.membership.update([]Change{Change{
		Address:     member.Address,
		Status:      FAULTY,
		Incarnation: member.Incarnation + 1,
	}})

	assert.Equal(t, ALIVE, member.Status, "expected local member to still be alive")
}

// Attempting to make the local member faulty should not change local member status
func TestMembershipLocalSuspectOverride(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000", 0, nil)
	defer ringpop.Destroy()

	member, found := ringpop.membership.getMemberByAddress(ringpop.WhoAmI())
	assert.True(t, found, "expected member to be found")
	assert.Equal(t, ALIVE, member.Status, "expected local member to start as alive")

	ringpop.membership.update([]Change{Change{
		Address:     member.Address,
		Status:      SUSPECT,
		Incarnation: member.Incarnation + 1,
	}})

	assert.Equal(t, ALIVE, member.Status, "expected local member to still be alive")
}

// Update method properly handles multiple updates in input
// Also tests that an update for a never before seen member works for all statuses
func TestMembershipHandleMultipleUpdates(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000", 0, nil)
	defer ringpop.Destroy()

	oldchecksum := ringpop.membership.checksum

	updates := ringpop.membership.update([]Change{
		Change{
			Address:     "127.0.0.1:3001",
			Status:      ALIVE,
			Incarnation: unixMilliseconds(time.Now()),
		},
		Change{
			Address:     "127.0.0.1:3002",
			Status:      SUSPECT,
			Incarnation: unixMilliseconds(time.Now()),
		},
		Change{
			Address:     "127.0.0.1:3003",
			Status:      FAULTY,
			Incarnation: unixMilliseconds(time.Now()),
		},
		Change{
			Address:     "127.0.0.1:3004",
			Status:      LEAVE,
			Incarnation: unixMilliseconds(time.Now()),
		},
	})

	assert.Equal(t, 4, len(updates), "expected 4 updates to be applied")

	member, found := ringpop.membership.getMemberByAddress("127.0.0.1:3001")
	assert.True(t, found, "expected member to be found")
	assert.Equal(t, ALIVE, member.Status, "expected member to be alive")

	member, found = ringpop.membership.getMemberByAddress("127.0.0.1:3002")
	assert.True(t, found, "expected member to be found")
	assert.Equal(t, SUSPECT, member.Status, "expected member to be suspect")

	member, found = ringpop.membership.getMemberByAddress("127.0.0.1:3003")
	assert.True(t, found, "expected member to be found")
	assert.Equal(t, FAULTY, member.Status, "expected member to be faulty ")

	member, found = ringpop.membership.getMemberByAddress("127.0.0.1:3004")
	assert.True(t, found, "expected member to be found")
	assert.Equal(t, LEAVE, member.Status, "expected member to be leave")

	assert.NotEqual(t, oldchecksum, ringpop.membership.checksum,
		"expected checksum to change after updates applied")
}

// A member should be able to go from alive -> faulty immediately without having to be suspect inbetween
func TestMembershipAliveToFaulty(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000", 0, nil)
	defer ringpop.Destroy()

	newMemberAddr := "127.0.0.2:3001"
	ringpop.membership.makeAlive(newMemberAddr, unixMilliseconds(time.Now()))

	newMember, found := ringpop.membership.getMemberByAddress(newMemberAddr)
	assert.True(t, found, "expected new membwer to be found")
	assert.Equal(t, ALIVE, newMember.Status, "expected new member to start as alive")

	ringpop.membership.update([]Change{
		Change{
			Address:     newMember.Address,
			Status:      FAULTY,
			Incarnation: newMember.Incarnation - 1,
		},
	})

	assert.Equal(t, ALIVE, newMember.Status, "expected no override with lower incarnation number")

	ringpop.membership.update([]Change{
		Change{
			Address:     newMember.Address,
			Status:      FAULTY,
			Incarnation: newMember.Incarnation,
		},
	})

	assert.Equal(t, FAULTY, newMember.Status, "expected override with same incarnation number")
}

func TestMembershipString(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000", 0, nil)
	defer ringpop.Destroy()

	ringpop.membership.makeAlive("127.0.0.2:3000", unixMilliseconds(time.Now()))
	ringpop.membership.makeAlive("127.0.0.3:3000", unixMilliseconds(time.Now()))

	_, err := ringpop.membership.String()
	assert.NoError(t, err, "membership should successfully be marshalled into JSON")
}

func TestMembershipLeaveEnds(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000", 0, nil)
	defer ringpop.Destroy()

	newMemberAddr := "127.0.0.1:3001"
	newMemberInc := unixMilliseconds(time.Now())

	updates := ringpop.membership.makeAlive(newMemberAddr, newMemberInc)
	assert.Equal(t, 1, len(updates), "expected alive update to be applied")

	updates = ringpop.membership.makeLeave(newMemberAddr, newMemberInc)
	assert.Equal(t, 1, len(updates), "expected leave update to be applied")

	updates = ringpop.membership.makeLeave(newMemberAddr, newMemberInc)
	assert.Equal(t, 0, len(updates), "expected no update to be applied")
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	MEMBERSHIP ITERATOR TESTS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func TestIterNoUsable(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000", 0, nil)
	defer ringpop.Destroy()

	iter := ringpop.membership.iter()

	member, ok := iter.next()
	assert.False(t, ok, "expected error, no usable members")
	assert.Nil(t, member, "expected member to be nil")
}

func TestIterNoUsableWithNonLocal(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000", 0, nil)
	defer ringpop.Destroy()

	ringpop.membership.makeFaulty("127.0.0.1:3001", unixMilliseconds(time.Now()))
	ringpop.membership.makeLeave("127.0.0.1:3002", unixMilliseconds(time.Now()))

	iter := ringpop.membership.iter()

	member, ok := iter.next()
	assert.False(t, ok, "expected error, no useable members")
	assert.Nil(t, member, "expected member to be nil")
}

func TestIterOverTen(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000", 0, nil)
	defer ringpop.Destroy()

	for i := 1; i < 11; i++ {
		ringpop.membership.makeAlive(fmt.Sprintf("127.0.0.1:300%v", i), unixMilliseconds(time.Now()))
	}

	iter := ringpop.membership.iter()
	iterated := make(map[string]bool)

	for i := 0; i < 15; i++ {
		member, ok := iter.next()
		assert.True(t, ok, "expected no error")
		assert.NotNil(t, member, "expected useable member")
		iterated[member.Address] = true
	}

	assert.Len(t, iterated, 10, "expected 10 members to be iterated over")
}

func TestIterSkipsFaultyAndLocal(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000", 0, nil)
	defer ringpop.Destroy()

	ringpop.membership.makeAlive("127.0.0.1:3001", unixMilliseconds(time.Now()))
	ringpop.membership.makeFaulty("127.0.0.1:3002", unixMilliseconds(time.Now()))
	ringpop.membership.makeAlive("127.0.0.1:3003", unixMilliseconds(time.Now()))

	iter := ringpop.membership.iter()

	iterated := make(map[string]bool)

	member, ok := iter.next()
	assert.True(t, ok, "expected no error")
	assert.NotNil(t, member, "expected useable member")
	iterated[member.Address] = true

	member, ok = iter.next()
	assert.True(t, ok, "expected no error")
	assert.NotNil(t, member, "expected useable member")
	iterated[member.Address] = true

	member, ok = iter.next()
	assert.True(t, ok, "expected no error")
	assert.NotNil(t, member, "expected useable member")
	iterated[member.Address] = true

	member, ok = iter.next()
	assert.True(t, ok, "expected no error")
	assert.NotNil(t, member, "expected useable member")
	iterated[member.Address] = true

	assert.Len(t, iterated, 2, "expected two pingable members to be iterated on")
}
