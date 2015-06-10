package ringpop

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"time"

	"github.com/dgryski/go-farm"
)

type Change struct {
	Source      string    `json:"source"`
	Address     string    `json:"address"`
	Status      string    `json:"status"`
	Incarnation int64     `json:"incarnation"`
	Timestamp   time.Time `json:"timestamp"`
}

// methods to satisfy `Suspect` interface
func (this Change) suspectAddress() string {
	return this.Address
}

func (this Change) suspectStatus() string {
	return this.Status
}

func (this Change) suspectIncarnation() int64 {
	return this.Incarnation
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	MEMBERSHIP
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

type membership struct {
	ringpop     *Ringpop
	members     map[string]*Member
	checksum    uint32
	localMember *Member
}

// NewMembership returns a pointer to a new Membership
func newMembership(ringpop *Ringpop) *membership {
	membership := &membership{
		ringpop: ringpop,
		members: map[string]*Member{},
	}
	return membership
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// METHODS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

// computeChecksum calaculates the membership checksum.
// The membership checksum is a farmhash of the checksum string computed
// for each member then joined with all other member checksum strigns by ';'.
// As an example the checksum string for some member might be:
//
// 	localhost:3000alive1414142122274
//
// Joined with another member
//
//	localhost:3000alive1414142122274localhost:3000alive1414142122275
//
// The member fields that are part of the checksum string are:
// address, status, incarnation number
func (this *membership) computeChecksum() {
	this.checksum = farm.Hash32([]byte(this.genChecksumString()))
}

func (this *membership) genChecksumString() string {
	var checksumStrings sort.StringSlice

	for _, member := range this.members {
		cstr := fmt.Sprintf("%s%s%v", member.Address, member.Status, member.Incarnation)
		checksumStrings = append(checksumStrings, cstr)
	}

	checksumStrings.Sort()

	buffer := bytes.NewBuffer([]byte{})
	for _, str := range checksumStrings {
		buffer.WriteString(str)
		buffer.WriteString(";")
	}

	return buffer.String()
}

func (this *membership) getMemberByAddress(address string) (*Member, bool) {
	member, ok := this.members[address]
	if !ok {
		member = nil
	}
	return member, ok
}

func (this *membership) getJoinPosition() int {
	return int(math.Floor(rand.Float64() * float64(len(this.members))))
}

func (this *membership) memberCount() int {
	return len(this.members)
}

func (this *membership) randomPingableMembers(n int, exlcuding map[string]bool) []*Member {
	var pingableMembers []*Member

	for _, member := range this.members {
		if this.isPingable(member) && !exlcuding[member.Address] {
			pingableMembers = append(pingableMembers, member)
		}
	}

	// shuffle members and take first n
	pingableMembers = shuffle(pingableMembers)

	if n > len(pingableMembers) {
		return pingableMembers
	}
	return pingableMembers[:n]
}

func (this *membership) stats() {
	// TODO: decide what to make this return (stuct?)
}

func (this *membership) hasMember(member *Member) bool {
	_, ok := this.members[member.Address]
	return ok
}

func (this *membership) isPingable(member *Member) bool {
	return member.Address != this.ringpop.WhoAmI() &&
		(member.Status == ALIVE || member.Status == SUSPECT)
}

func (this *membership) makeAlive(address string, incarnation int64, source string) []Change {
	if source == "" {
		source = this.ringpop.WhoAmI()
	}

	return this.update([]Change{Change{
		Source:      source,
		Address:     address,
		Status:      ALIVE,
		Incarnation: incarnation,
		Timestamp:   time.Now(),
	}})
}

func (this *membership) makeFaulty(address string, incarnation int64, source string) []Change {
	if source == "" {
		source = this.ringpop.WhoAmI()
	}

	return this.update([]Change{Change{
		Source:      source,
		Address:     address,
		Status:      FAULTY,
		Incarnation: incarnation,
		Timestamp:   time.Now(),
	}})
}

func (this *membership) makeLeave(address string, incarnation int64, source string) []Change {
	if source == "" {
		source = this.ringpop.WhoAmI()
	}

	return this.update([]Change{Change{
		Source:      source,
		Address:     address,
		Status:      LEAVE,
		Incarnation: incarnation,
		Timestamp:   time.Now(),
	}})
}

func (this *membership) makeSuspect(address string, incarnation int64, source string) []Change {
	if source == "" {
		source = this.ringpop.WhoAmI()
	}

	return this.update([]Change{Change{
		Source:      source,
		Address:     address,
		Status:      SUSPECT,
		Incarnation: incarnation,
		Timestamp:   time.Now(),
	}})
}

func (this *membership) makeUpdate(change Change) {
	member, ok := this.members[change.Address]

	if !ok {
		member = &Member{
			Address:     change.Address,
			Status:      change.Status,
			Incarnation: change.Incarnation,
		}

		if member.Address == this.ringpop.WhoAmI() {
			this.localMember = member
		}

		this.members[change.Address] = member
	}

	// joinpos := this.getJoinPosition()
	// this.members = append(this.members[:joinpos], append([]*Member{member}, this.members[joinpos:]...)...)

	member.Status = change.Status
	member.Incarnation = change.Incarnation
}

func (this *membership) update(changes []Change) []Change {
	var updates []Change

	if len(changes) == 0 {
		return updates
	}

	for _, change := range changes {
		member, found := this.getMemberByAddress(change.Address)

		// If this is the first time we see the member, take change wholesale
		if !found {
			this.makeUpdate(change)
			updates = append(updates, change)
			continue
		}

		// If the change is a local override, reassert member is alive
		if IsLocalSuspectOverride(this.ringpop, member, change) ||
			IsLocalFaultyOverride(this.ringpop, member, change) {
			newchange := Change{
				Source:      change.Source,
				Address:     change.Address,
				Status:      ALIVE,
				Incarnation: time.Now().UnixNano() / 1000000,
				Timestamp:   change.Timestamp,
			}

			this.makeUpdate(newchange)
			updates = append(updates, newchange)
			continue
		}

		// If non-local update, take change wholesale
		if IsAliveOverride(member, change) || IsSuspectOverride(member, change) ||
			IsFaultyOverride(member, change) || IsLeaveOverride(member, change) {
			this.makeUpdate(change)
			updates = append(updates, change)
		}
	}

	if len(updates) > 0 {
		this.computeChecksum()
		this.ringpop.emit("updated")
	}

	return updates
}

// shuffles slice of members pseudo-randomly
func shuffle(members []*Member) []*Member {
	newMembers := make([]*Member, len(members), cap(members))
	newIndexes := rand.Perm(len(members))

	for o, n := range newIndexes {
		newMembers[n] = members[o]
	}

	return newMembers
}

// Shuffle returns a slice containing the members in the membership in a random order
func (this *membership) shuffle() []*Member {
	shuffled := []*Member{}

	for _, member := range this.members {
		shuffled = append(shuffled, member)
	}

	shuffled = shuffle(shuffled)

	return shuffled
}

// String returns a JSON string
func (this *membership) String() (string, error) {
	var members []string

	for _, member := range this.members {
		members = append(members, member.Address)
	}

	str, err := json.Marshal(members)
	if err != nil {
		return "", err
	}
	return string(str), err
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	MEMBERSHIP ITERATOR
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func (this *membership) iter() membershipIter {
	return newMembershipIter(this)
}

type membershipIter struct {
	membership   *membership
	members      []*Member
	currentIndex int
	currentRound int
}

func newMembershipIter(membership *membership) membershipIter {
	iter := membershipIter{
		membership:   membership,
		currentIndex: -1,
		currentRound: 0,
	}

	iter.members = iter.membership.shuffle()

	return iter
}

// Returns the next pingable member in the membership list
func (this *membershipIter) next() (*Member, bool) {
	maxToVisit := len(this.members)
	visited := make(map[string]bool)

	for len(visited) < maxToVisit {
		this.currentIndex++

		if this.currentIndex >= len(this.members) {
			this.currentIndex = 0
			this.currentRound++
			this.members = this.membership.shuffle()
		}

		member := this.members[this.currentIndex]
		visited[member.Address] = true

		if this.membership.isPingable(member) {
			return member, true
		}
	}

	return nil, false
}
