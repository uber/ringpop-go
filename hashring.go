package ringpop

import (
	"bytes"
	"fmt"
	"ringpop/rbtree"
	"sort"
	"sync"

	"github.com/dgryski/go-farm"
)

// Wrapper for farm.Hash32
// if off is < 0, do not append off to the end
func farmhash32(address string, off int) uint32 {
	hashstr := address
	if off >= 0 {
		hashstr = fmt.Sprintf("%s%v", address, off)
	}
	return farm.Hash32([]byte(hashstr))
}

type hashRing struct {
	ringpop       *Ringpop
	rbtree        rbtree.RBTree
	servers       map[string]bool
	checksum      uint32
	replicaPoints int
	lock          sync.RWMutex
}

func newHashRing(ringpop *Ringpop) *hashRing {
	hashring := &hashRing{
		ringpop:       ringpop,
		rbtree:        rbtree.RBTree{},
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

	r.checksum = farmhash32(buffer.String(), -1)
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
		r.rbtree.Insert(int(farmhash32(address, i)), address)
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
		r.rbtree.Delete(int(farmhash32(address, i)))
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
	hash := farm.Hash32([]byte(key))
	iter := make(<-chan *rbtree.RingNode)
	iter = r.rbtree.Iter(int(hash))

	if node, ok := <-iter; ok {
		return node.Str(), true
	}

	if node := r.rbtree.Min(); node != nil {
		return node.Str(), true
	}

	return "", false
}
