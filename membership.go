package ringpop

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"math/rand"
	"sort"
	"sync"
	"time"

	"github.com/dgryski/go-farm"
)

// Change is a struct used to make changes to the membership
type Change struct {
	Source            string    `json:"source"`
	SourceIncarnation int64     `json:"sourceIncarnation"`
	Address           string    `json:"address"`
	Status            string    `json:"status"`
	Incarnation       int64     `json:"incarnation"`
	Timestamp         time.Time `json:"timestamp"`
}

// methods to satisfy `suspect` interface
func (c Change) address() string {
	return c.Address
}

func (c Change) incarnation() int64 {
	return c.Incarnation
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	MEMBERSHIP
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

type membership struct {
	ringpop     *Ringpop
	members     map[string]*Member
	memberlist  []*Member
	localMember *Member
	checksum    uint32
	lock        sync.RWMutex
}

// NewMembership returns a pointer to a new membership
func newMembership(ringpop *Ringpop) *membership {
	membership := &membership{
		ringpop: ringpop,
		members: make(map[string]*Member),
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
func (m *membership) computeChecksum() {
	m.lock.Lock()
	m.checksum = farm.Hash32([]byte(m.genChecksumString()))
	m.lock.Unlock()
}

func (m *membership) genChecksumString() string {
	var checksumStrings sort.StringSlice

	for _, member := range m.members {
		s := fmt.Sprintf("%s%s%v", member.Address, member.Status, member.Incarnation)
		checksumStrings = append(checksumStrings, s)
	}

	checksumStrings.Sort()

	buffer := bytes.NewBuffer([]byte{})
	for _, str := range checksumStrings {
		buffer.WriteString(str)
		buffer.WriteString(";")
	}

	return buffer.String()
}

func (m *membership) getMemberByAddress(address string) (*Member, bool) {
	member, ok := m.members[address]
	if !ok {
		member = nil
	}
	return member, ok
}

func (m *membership) getJoinPosition() int {
	return int(math.Floor(rand.Float64() * float64(len(m.memberlist))))
}

func (m *membership) memberCount() int {
	m.lock.RLock()
	defer m.lock.RUnlock()
	return len(m.members)
}

func (m *membership) randomPingablemembers(n int, excluding []string) []*Member {
	var pingableMembers []*Member

	for _, member := range m.memberlist {
		if m.isPingable(member) && indexOf(excluding, member.Address) == -1 {
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

func (m *membership) isPingable(member *Member) bool {
	return member.Address != m.ringpop.WhoAmI() &&
		(member.Status == ALIVE || member.Status == SUSPECT)
}

func (m *membership) makeAlive(address string, incarnation int64) []Change {
	if m.localMember == nil {
		m.localMember = &Member{Address: m.ringpop.WhoAmI()}
	}

	return m.update([]Change{Change{
		Source:            m.localMember.Address,
		SourceIncarnation: m.localMember.Incarnation,
		Address:           address,
		Status:            ALIVE,
		Incarnation:       incarnation,
		Timestamp:         time.Now(),
	}})
}

func (m *membership) makeFaulty(address string, incarnation int64) []Change {
	if m.localMember == nil {
		m.localMember = &Member{Address: m.ringpop.WhoAmI()}
	}

	return m.update([]Change{Change{
		Source:            m.localMember.Address,
		SourceIncarnation: m.localMember.Incarnation,
		Address:           address,
		Status:            FAULTY,
		Incarnation:       incarnation,
		Timestamp:         time.Now(),
	}})
}

func (m *membership) makeLeave(address string, incarnation int64) []Change {
	if m.localMember == nil {
		m.localMember = &Member{Address: m.ringpop.WhoAmI(), Incarnation: unixMilliseconds(time.Now())}
	}

	return m.update([]Change{Change{
		Source:            m.localMember.Address,
		SourceIncarnation: m.localMember.Incarnation,
		Address:           address,
		Status:            LEAVE,
		Incarnation:       incarnation,
		Timestamp:         time.Now(),
	}})
}

func (m *membership) makeSuspect(address string, incarnation int64) []Change {
	if m.localMember == nil {
		m.localMember = &Member{Address: m.ringpop.WhoAmI()}
	}

	return m.update([]Change{Change{
		Source:            m.localMember.Address,
		SourceIncarnation: m.localMember.Incarnation,
		Address:           address,
		Status:            SUSPECT,
		Incarnation:       incarnation,
		Timestamp:         time.Now(),
	}})
}

func (m *membership) makeUpdate(change Change) {
	member, ok := m.members[change.Address]

	if !ok {
		member = &Member{
			Address:     change.Address,
			Status:      change.Status,
			Incarnation: change.Incarnation,
		}

		if member.Address == m.ringpop.WhoAmI() {
			m.localMember = member
		}

		m.members[change.Address] = member
		joinpos := m.getJoinPosition()
		m.memberlist = append(m.memberlist[:joinpos], append([]*Member{member},
			m.memberlist[joinpos:]...)...)
	}

	member.Status = change.Status
	member.Incarnation = change.Incarnation
}

func (m *membership) update(changes []Change) []Change {
	var updates []Change

	if len(changes) == 0 {
		return nil
	}

	if m.ringpop.Destroyed() {
		return nil
	}

	m.lock.Lock()

	for _, change := range changes {
		member, found := m.getMemberByAddress(change.Address)

		// If m is the first time we see the member, take change wholesale
		if !found {
			m.makeUpdate(change)
			updates = append(updates, change)
			continue
		}

		// If the change is a local override, reassert member is alive
		if isLocalSuspectOverride(m.ringpop, member, change) ||
			isLocalFaultyOverride(m.ringpop, member, change) {
			newchange := Change{
				Source:      change.Source,
				Address:     change.Address,
				Status:      ALIVE,
				Incarnation: unixMilliseconds(time.Now()),
				Timestamp:   change.Timestamp,
			}

			m.makeUpdate(newchange)
			updates = append(updates, newchange)
			continue
		}

		// If non-local update, take change wholesale
		if isAliveOverride(member, change) || isSuspectOverride(member, change) ||
			isFaultyOverride(member, change) || isLeaveOverride(member, change) {
			m.makeUpdate(change)
			updates = append(updates, change)
		}
	}

	m.lock.Unlock()

	if len(updates) > 0 {
		m.computeChecksum()
		m.ringpop.emit("updated")
		m.ringpop.handleChanges(changes)
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
func (m *membership) shuffle() {
	shuffled := []*Member{}

	m.lock.Lock()
	defer m.lock.Unlock()

	for _, member := range m.memberlist {
		shuffled = append(shuffled, member)
	}

	m.memberlist = shuffle(shuffled)

}

// String returns a JSON string
func (m *membership) String() (string, error) {
	var members []string

	m.lock.RLock()
	defer m.lock.RUnlock()

	for _, member := range m.members {
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

func (m *membership) iter() membershipIter {
	return newMembershipIter(m)
}

type membershipIter struct {
	membership   *membership
	currentIndex int
	currentRound int
}

func newMembershipIter(membership *membership) membershipIter {
	iter := membershipIter{
		membership:   membership,
		currentIndex: -1,
		currentRound: 0,
	}

	iter.membership.shuffle()

	return iter
}

// Returns the next pingable member in the membership list, if it
// visits all members but none are pingable, returns nil, false
func (it *membershipIter) next() (*Member, bool) {
	maxToVisit := len(it.membership.memberlist)
	visited := make(map[string]bool)

	for len(visited) < maxToVisit {
		it.currentIndex++

		if it.currentIndex >= len(it.membership.memberlist) {
			it.currentIndex = 0
			it.currentRound++
			it.membership.shuffle()
		}

		member := it.membership.memberlist[it.currentIndex]
		visited[member.Address] = true

		if it.membership.isPingable(member) {
			return member, true
		}
	}

	return nil, false
}
