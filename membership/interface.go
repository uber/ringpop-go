package membership

const (
	// IdentityLabelKey is the key used to identify the identity label of a
	// Member
	IdentityLabelKey = "__identity"
)

// Member defines a member of the membership. It can be used by applications to
// apply specific business logic on Members. Examples are:
// - Get the address of a member for RPC calls, both forwarding of internal
//   calls that should target a Member
// - Decissions to include a Member in a query via predicates.
type Member interface {
	// GetAddress returns the external address used by the rpc layer to
	// communicate to the member.
	//
	// Note: It is prefixed with Get for legacy reasons and can be removed after
	// a refactor of the swim.Member to free up the `Address` name.
	GetAddress() string

	// Label reads the label for a given key from the member. It also returns
	// wether or not the label was present on the member
	Label(key string) (value string, has bool)

	// Identity returns the logical identity the member takes within the
	// hashring, this is experimental and might move away from the membership to
	// the Hashring
	Identity() string
}
