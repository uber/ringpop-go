package ringpop

const (
	ALIVE   = "alive"
	FAULTY  = "fault"
	LEAVE   = "leave"
	SUSPECT = "suspect"
)

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// MEMER
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

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

func (this Member) suspectAddress() string {
	return this.Address
}

func (this Member) suspectStatus() string {
	return this.Status
}

func (this Member) suspectIncarnation() int64 {
	return this.Incarnation
}
