package membership

// Member defines a member of the membership that can be used by the rest of the
// system
type Member interface {
	// GetAddress returns the external address used by the rpc layer to
	// communicate to the member. It is prefixed with Get for legacy reasons and
	// can be removed after a refactor of the swim.Member
	GetAddress() string

	// Label reads the label for a givin key from the member. It also returns
	// wether or not the label was present on the member
	Label(key string) (value string, has bool)
}
