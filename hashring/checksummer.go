package hashring

import (
	"bytes"
	"sort"
	"strconv"

	"github.com/dgryski/go-farm"
)

// Checksummer computes a checksum for an instance of a HashRing. The
// checksum can be used to compare two rings for equality.
type Checksummer interface {
	// Checksum calculates the checksum for the hashring that is passed in.
	// Compute will be called while having at least a read-lock on the hashring so
	// it is safe to read from the ring, but not safe to change the ring. There
	// might be multiple Checksum Computes initiated at the same time, but every
	// Checksum will only be called once per hashring at once
	Checksum(ring *HashRing) (checksum uint32)
}

type identityChecksummer struct {
	buf bytes.Buffer
}

func (i *identityChecksummer) Checksum(ring *HashRing) uint32 {
	identitySet := make(map[string]struct{})
	ring.tree.root.traverseWhile(func(node *redBlackNode) bool {
		identitySet[node.key.(replicaPoint).identity] = struct{}{}
		return true
	})

	identities := make([]string, 0, len(identitySet))
	for identity := range identitySet {
		identities = append(identities, identity)
	}

	sort.Strings(identities)
	i.buf.Reset()
	for idi, id := range identities {
		if idi > 0 {
			i.buf.WriteByte(';')
		}
		i.buf.WriteString(id)
	}
	return farm.Fingerprint32(i.buf.Bytes())
}

type replicaPointChecksummer struct {
	buf bytes.Buffer
}

func (r *replicaPointChecksummer) Checksum(ring *HashRing) uint32 {
	r.buf.Reset()

	ring.tree.root.traverseWhile(func(node *redBlackNode) bool {
		r.buf.WriteString(strconv.Itoa(node.key.(replicaPoint).hash))
		r.buf.WriteByte('-')
		r.buf.WriteString(node.value.(string))
		r.buf.WriteByte(';')
		return true
	})

	return farm.Fingerprint32(r.buf.Bytes())
}
