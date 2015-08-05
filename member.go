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

// A Member is a node in the ring
type Member struct {
	Address     string `json:"address"`
	Status      string `json:"status"`
	Incarnation int64  `json:"incarnationNumber"`
}

func (m Member) address() string {
	return m.Address
}

func (m Member) incarnation() int64 {
	return m.Incarnation
}
