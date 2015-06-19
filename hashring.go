package ringpop

import (
	"bytes"
	"fmt"
	"ringpop/rbtree"
	"sort"

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
	rbtree        rbtree.RBTree
	servers       map[string]bool
	checksum      uint32
	replicaPoints int
	eventC        chan string
}

func newHashRing() *hashRing {
	hashring := &hashRing{
		rbtree:  rbtree.RBTree{},
		servers: make(map[string]bool),
		eventC:  make(chan string, 10),
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

	r.eventC <- "checksumComputed"
}

// AddServer does what you would expect
func (r *hashRing) addServer(address string) {
	if r.hasServer(address) {
		return
	}
	// add server to servers
	r.servers[address] = true
	// insert replications into ring
	for i := 0; i < r.replicaPoints; i++ {
		r.rbtree.Insert(int(farmhash32(address, i)), address)
	}

	r.computeChecksum()
	r.eventC <- "added"
}

// removeServer does what you would expect
func (r *hashRing) removeServer(address string) {
	if !r.hasServer(address) {
		return
	}

	delete(r.servers, address)

	for i := 0; i < r.replicaPoints; i++ {
		r.rbtree.Delete(int(farmhash32(address, i)))
	}

	r.computeChecksum()

	r.eventC <- "removed"
}

// hasServer returns true if the server exists in the ring, false otherwise
func (r *hashRing) hasServer(address string) bool {
	exists, ok := r.servers[address]
	return ok && exists
}

// ServerCount returns the number of servers in the ring
func (r *hashRing) serverCount() int {
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
