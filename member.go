package ringpop

const (
	// ALIVE is the member "alive" state
	ALIVE = "alive"

	// FAULTY is the member "faulty" state
	FAULTY = "faulty"

	// LEAVE is the member "leave" state
	LEAVE = "leave"

	// SUSPECT is the memeber "suspect" state
	SUSPECT = "suspect"
)

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// MEMBER
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

// Member is a node in the membership list
type Member struct {
	Address     string
	Status      string
	Incarnation int64
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// METHODS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func (m Member) suspectAddress() string {
	return m.Address
}

func (m Member) suspectStatus() string {
	return m.Status
}

func (m Member) suspectIncarnation() int64 {
	return m.Incarnation
}
