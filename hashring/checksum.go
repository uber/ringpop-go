package hashring

import (
	"bytes"
	"sort"
	"strconv"
	"strings"

	"github.com/dgryski/go-farm"
)

// Checksum computes a checksum for an instance of a HashRing. The
// checksum can be used to compare two rings for equality.
type Checksum interface {
	// Compute calculates the checksum for the hashring that is passed in.
	// Compute will be called while having atleast a readlock on the hashring so
	// it is safe to read from the ring, but not safe to chagne the ring. There
	// might be multiple Checksum Computes initiated at the same time, but every
	// Checksum will only be called once per hashring at once
	Compute(ring *HashRing) (checksum uint32)
}

// addressChecksum calculates checksums for all addresses that are added to the
// hashring.
// TODO calculate the checksum on identities. This will be backwards compatible.
// The best case would be to iterate the rbtree in order and hash its content so
// we have more certainty that the rings are looking up the same way but that
// will be a backwards incompatible change
type addressChecksum struct{}

func (i *addressChecksum) Compute(ring *HashRing) uint32 {
	addresses := ring.copyServersNoLock()
	sort.Strings(addresses)
	bytes := []byte(strings.Join(addresses, ";"))
	return farm.Fingerprint32(bytes)
}

type replicapointChecksum struct{}

func (r *replicapointChecksum) Compute(ring *HashRing) uint32 {
	buffer := bytes.Buffer{}

	visitNodes(ring.tree.root, func(node *redBlackNode) {
		buffer.WriteString(strconv.Itoa(node.key.(replicapoint).point))
		buffer.WriteString("-")
		buffer.WriteString(node.value.(string))
		buffer.WriteString(";")
	})

	return farm.Fingerprint32(buffer.Bytes())
}

func visitNodes(node *redBlackNode, visitor func(node *redBlackNode)) {
	if node == nil {
		return
	}

	visitNodes(node.left, visitor)
	visitor(node)
	visitNodes(node.right, visitor)
}
