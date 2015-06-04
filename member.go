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
	address     string
	status      string
	incarnation int64
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
// METHODS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func (this Member) Address() string {
	return this.address
}

func (this Member) Status() string {
	return this.status
}

func (this Member) Incarnation() int64 {
	return this.incarnation
}
