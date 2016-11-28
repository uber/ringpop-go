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
type addressChecksum struct{}

func (i *addressChecksum) Compute(ring *HashRing) uint32 {
	addresses := ring.copyServersNoLock()
	sort.Strings(addresses)
	bytes := []byte(strings.Join(addresses, ";"))
	return farm.Fingerprint32(bytes)
}

type identityChecksum struct{}

func (i *identityChecksum) Compute(ring *HashRing) uint32 {
	identitySet := make(map[string]struct{})
	ring.tree.root.walk(func(node *redBlackNode) bool {
		identitySet[node.key.(replicaPoint).identity] = struct{}{}
		return true
	})

	identities := make([]string, 0, len(identitySet))
	for identity := range identitySet {
		identities = append(identities, identity)
	}

	sort.Strings(identities)
	bytes := []byte(strings.Join(identities, ";"))
	return farm.Fingerprint32(bytes)
}

type replicaPointChecksum struct{}

func (r *replicaPointChecksum) Compute(ring *HashRing) uint32 {
	buffer := bytes.Buffer{}

	ring.tree.root.walk(func(node *redBlackNode) bool {
		buffer.WriteString(strconv.Itoa(node.key.(replicaPoint).point))
		buffer.WriteString("-")
		buffer.WriteString(node.value.(string))
		buffer.WriteString(";")
		return true
	})

	return farm.Fingerprint32(buffer.Bytes())
}
