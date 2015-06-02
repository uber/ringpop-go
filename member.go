package ringpop

const (
	ALIVE   = "alive"
	FAULTY  = "fault"
	LEAVE   = "leave"
	SUSPECT = "suspect"
)

type Member struct {
	Address     string
	Status      string
	Incarnation int64
}
