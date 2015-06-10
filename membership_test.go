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

func TestChecksumChanges(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000")
	ringpop.membership.makeAlive("127.0.0.1:3000", TimeNow(), "")

	oldchecksum := ringpop.membership.checksum

	ringpop.membership.makeAlive("127.0.0.1:3001", TimeNow(), "")

	assert.NotEqual(t, oldchecksum, ringpop.membership.checksum,
		"expected checksum to have changed on membership change")
}

func TestChecksumEqual(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000")
	ringpop2 := testPop("127.0.0.1:3000")

	ringpop.membership.makeAlive("127.0.0.1:3001", 1, "")
	ringpop.membership.makeAlive("127.0.0.1:3002", 1, "")
	ringpop.membership.makeAlive("127.0.0.1:3003", 1, "")

	ringpop2.membership.makeAlive("127.0.0.1:3003", 1, "")
	ringpop2.membership.makeAlive("127.0.0.1:3001", 1, "")
	ringpop2.membership.makeAlive("127.0.0.1:3002", 1, "")

	assert.Equal(t, ringpop.membership.checksum, ringpop2.membership.checksum,
		"expected checksums to be equal, regardless of input order")
}

// Higher incarnation should result in a leave override
func TestLeaveOverrideHigher(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000")

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

// Equal incarnation number should not result in a leave override ... or should it?
func TestLeaveOverrideEqual(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000")

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
func TestLeaveOverrideLower(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000")

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
func TestLocalFaultyOverride(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000")

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
func TestLocalSuspectOverride(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000")

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
func TestHandleMultipleUpdates(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000")

	oldchecksum := ringpop.membership.checksum

	updates := ringpop.membership.update([]Change{
		Change{
			Address:     "127.0.0.1:3001",
			Status:      ALIVE,
			Incarnation: 1,
		},
		Change{
			Address:     "127.0.0.1:3002",
			Status:      SUSPECT,
			Incarnation: 1,
		},
		Change{
			Address:     "127.0.0.1:3003",
			Status:      FAULTY,
			Incarnation: 1,
		},
		Change{
			Address:     "127.0.0.1:3004",
			Status:      LEAVE,
			Incarnation: 1,
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
func TestAliveToFaulty(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000")

	newMemberAddr := "127.0.0.2:3001"
	ringpop.membership.makeAlive(newMemberAddr, TimeNow(), "")

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

func TestString(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000")

	ringpop.membership.makeAlive("127.0.0.2:3000", time.Now().UnixNano(), "")
	ringpop.membership.makeAlive("127.0.0.3:3000", time.Now().UnixNano(), "")

	_, err := ringpop.membership.String()
	assert.NoError(t, err, "membership should successfully be marshalled into JSON")
}

func TestLeaveEnds(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000")

	newMemberAddr := "127.0.0.1:3001"
	newMemberInc := TimeNow()

	updates := ringpop.membership.makeAlive(newMemberAddr, newMemberInc, "")
	assert.Equal(t, 1, len(updates), "expected alive update to be applied")

	updates = ringpop.membership.makeLeave(newMemberAddr, newMemberInc, "")
	assert.Equal(t, 1, len(updates), "expected leave update to be applied")

	updates = ringpop.membership.makeLeave(newMemberAddr, newMemberInc, "")
	assert.Equal(t, 0, len(updates), "expected no update to be applied")
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	MEMBERSHIP ITERATOR TESTS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func TestIterNoUsable(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000")

	iter := ringpop.membership.iter()

	member, err := iter.next()
	assert.Error(t, err, "expected error, no usable members")
	assert.Nil(t, member, "expected member to be nil")
}

func TestIterNoUsableWithNonLocal(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000")

	ringpop.membership.makeFaulty("127.0.0.1:3001", time.Now().UnixNano(), "")
	ringpop.membership.makeLeave("127.0.0.1:3002", time.Now().UnixNano(), "")

	iter := ringpop.membership.iter()

	member, err := iter.next()
	assert.Error(t, err, "expected error, no useable members")
	assert.Nil(t, member, "expected member to be nil")
}

func TestIterOverTen(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000")

	for i := 1; i < 11; i++ {
		ringpop.membership.makeAlive(fmt.Sprintf("127.0.0.1:300%s", i),
			time.Now().UnixNano(), "")
	}

	iter := ringpop.membership.iter()
	iterated := make(map[string]bool)

	for i := 0; i < 15; i++ {
		member, err := iter.next()
		assert.NoError(t, err, "expected no error")
		assert.NotNil(t, member, "expected useable member")
		iterated[member.Address] = true
	}

	assert.Len(t, iterated, 10, "expected 10 members to be iterated over")
}

func TestIterSkipsFaultyAndLocal(t *testing.T) {
	ringpop := testPop("127.0.0.1:3000")

	ringpop.membership.makeAlive("127.0.0.1:3001", time.Now().UnixNano(), "")
	ringpop.membership.makeFaulty("127.0.0.1:3002", time.Now().UnixNano(), "")
	ringpop.membership.makeAlive("127.0.0.1:3003", time.Now().UnixNano(), "")

	iter := ringpop.membership.iter()

	iterated := make(map[string]bool)

	member, err := iter.next()
	assert.NoError(t, err, "expected no error")
	assert.NotNil(t, member, "expected useable member")
	iterated[member.Address] = true

	member, err = iter.next()
	assert.NoError(t, err, "expected no error")
	assert.NotNil(t, member, "expected useable member")
	iterated[member.Address] = true

	member, err = iter.next()
	assert.NoError(t, err, "expected no error")
	assert.NotNil(t, member, "expected useable member")
	iterated[member.Address] = true

	member, err = iter.next()
	assert.NoError(t, err, "expected no error")
	assert.NotNil(t, member, "expected useable member")
	iterated[member.Address] = true

	assert.Len(t, iterated, 2, "expected two pingable members to be iterated on")
}
