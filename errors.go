package ringpop

import "errors"

var (
	// ErrNotBootstrapped is returned by public methods which require the ring to
	// be bootstrapped before they can operate correctly.
	ErrNotBootstrapped = errors.New("ringpop is not bootstrapped")

	// ErrEphemeralAddress is returned by the address resolver if TChannel is
	// using port 0 and is not listening (and thus has not been assigned a port by
	// the OS).
	ErrEphemeralAddress = errors.New("unable to resolve this node's address from channel that is not yet listening")

	// ErrChannelNotListening is returned on bootstrap if TChannel is not
	// listening.
	ErrChannelNotListening = errors.New("tchannel is not listening")

	// ErrInvalidIdentity is returned when the identity value is invalid.
	ErrInvalidIdentity = errors.New("a hostport is not valid as an identity")
)
