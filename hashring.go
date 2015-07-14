package ringpop

import (
	"bytes"
	"ringpop/rbtree"
	"sort"
	"sync"

	"github.com/dgryski/go-farm"
)

func offsetFarmHash32(b []byte, off int) uint32 {
	b = append(b, byte(off))
	return farm.Hash32(b)
}

type hashRing struct {
	ringpop       *Ringpop
	tree          rbtree.RBTree
	servers       map[string]bool
	checksum      uint32
	replicaPoints int
	lock          sync.RWMutex
}

func newHashRing(ringpop *Ringpop) *hashRing {
	hashring := &hashRing{
		ringpop:       ringpop,
		tree:          rbtree.RBTree{},
		servers:       make(map[string]bool),
		replicaPoints: 3,
	}

	return hashring
}

// computeChecksum computes checksum of all servers in the ring
func (r *hashRing) computeChecksum() {
	var addresses sort.StringSlice
	var buffer bytes.Buffer

	for address := range r.servers {
		addresses = append(addresses, address)
	}

	addresses.Sort()

	for _, address := range addresses {
		buffer.WriteString(address)
		buffer.WriteString(";")
	}

	r.checksum = farm.Hash32(buffer.Bytes())
	r.ringpop.handleRingEvent("checksumComputed")
}

// AddServer does what you would expect
func (r *hashRing) addServer(address string) {
	if r.hasServer(address) {
		return
	}

	r.lock.Lock()
	// add server to servers
	r.servers[address] = true
	// insert replications into ring
	for i := 0; i < r.replicaPoints; i++ {
		r.tree.Insert(int(offsetFarmHash32([]byte(address), i)), address)
	}
	r.lock.Unlock()

	r.computeChecksum()
	r.ringpop.handleRingEvent("added")
}

// removeServer does what you would expect
func (r *hashRing) removeServer(address string) {
	if !r.hasServer(address) {
		return
	}

	r.lock.Lock()
	// remove server from servers
	delete(r.servers, address)
	// remove replications from ring
	for i := 0; i < r.replicaPoints; i++ {
		r.tree.Delete(int(offsetFarmHash32([]byte(address), i)))
	}
	r.lock.Unlock()

	r.computeChecksum()
	r.ringpop.handleRingEvent("removed")
}

// hasServer returns true if the server exists in the ring, false otherwise
func (r *hashRing) hasServer(address string) bool {
	r.lock.RLock()
	exists, ok := r.servers[address]
	r.lock.RUnlock()
	return ok && exists
}

// ServerCount returns the number of servers in the ring
func (r *hashRing) serverCount() int {
	r.lock.RLock()
	defer r.lock.RUnlock()
	return len(r.servers)
}

func (r *hashRing) lookup(key string) (string, bool) {
	iter := r.tree.IterAt(int(farm.Hash32([]byte(key))))
	if !iter.Nil() {
		return iter.Str(), true
	}
	return "", false
}
