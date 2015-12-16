package ringpop

import "errors"

// ErrNotBootstrapped is returned by public methods which require the ring to
// be bootstrapped before they can operate correctly.
var ErrNotBootstrapped = errors.New("ringpop is not bootstrapped")
