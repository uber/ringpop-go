package ringpop

type Dissemination struct{}

func NewDissemination() *Dissemination {
	return &Dissemination{}
}

func (this *Dissemination) recordChange(change Change) {
	// TODO
}
