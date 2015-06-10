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

type HashRing struct {
	rbtree        rbtree.RBTree
	servers       map[string]bool
	checksum      uint32
	replicaPoints int
	//TODO: give event channel? do we need one?
}

func NewHashring() *HashRing {
	hashring := &HashRing{
		rbtree: rbtree.RBTree{},
	}

	return hashring
}

// computeChecksum computes checksum of all servers in the ring
func (this *HashRing) computeChecksum() {
	var addresses sort.StringSlice
	var buffer bytes.Buffer

	for address := range this.servers {
		addresses = append(addresses, address)
	}

	addresses.Sort()

	for _, address := range addresses {
		buffer.WriteString(address)
		buffer.WriteString(";")
	}

	this.checksum = farmhash32(buffer.String(), -1)
}

// AddServer does what you would expect
func (this *HashRing) addServer(address string) {
	if this.hasServer(address) {
		return
	}
	// add server to servers
	this.servers[address] = true
	// insert replications into ring
	for i := 0; i < this.replicaPoints; i++ {
		this.rbtree.Insert(int64(farmhash32(address, i)), address)
	}

	this.computeChecksum()
}

// removeServer does what you would expect
func (this *HashRing) removeServer(address string) {
	if !this.hasServer(address) {
		return
	}

	delete(this.servers, address)

	for i := 0; i < this.replicaPoints; i++ {
		this.rbtree.Delete(int64(farmhash32(address, i)))
	}

	this.computeChecksum()
}

// hasServer returns true if the server exists in the ring, false otherwise
func (this *HashRing) hasServer(address string) bool {
	exists, ok := this.servers[address]
	return ok && exists
}

// ServerCount returns the number of servers in the ring
func (this *HashRing) serverCount() int {
	return len(this.servers)
}

func (this *HashRing) lookup(key string) (string, bool) {
	hash := farm.Hash32([]byte(key))
	iter := make(<-chan *rbtree.RingNode)
	iter = this.rbtree.Iter(int64(hash))

	if node, ok := <-iter; ok {
		return node.Str(), true
	} else {
		if node = this.rbtree.Min(); node != nil {
			return node.Str(), true
		}
	}

	return "", false
}
