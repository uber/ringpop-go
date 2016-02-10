// Copyright (c) 2015 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
//
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
//
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package swim

import (
	"math"
	"sync"

	log "github.com/uber-common/bark"
	"github.com/uber/ringpop-go/logging"
)

var log10 = math.Log(10)

// defaultPFactor is the piggyback factor value, described in the swim paper.
const defaultPFactor int = 15

// A pChange is a change with a p count representing the number of times the
// change has been propagated to other nodes.
type pChange struct {
	Change
	p int
}

// A disseminator propagates changes to other nodes.
type disseminator struct {
	node    *Node
	changes map[string]*pChange

	// maxP indicates how many times a change is disseminated. It is declared
	// in the swim paper as piggybackfactor * log(# nodes).
	maxP    int
	pFactor int

	sync.RWMutex

	logger log.Logger
}

// newDisseminator returns a new Disseminator instance.
func newDisseminator(n *Node) *disseminator {
	d := &disseminator{
		node:    n,
		changes: make(map[string]*pChange),
		maxP:    defaultPFactor,
		pFactor: defaultPFactor,
		logger:  logging.Logger("disseminator").WithField("local", n.Address()),
	}

	return d
}

func (d *disseminator) AdjustMaxPropagations() {
	d.Lock()

	numPingable := d.node.memberlist.NumPingableMembers()
	prevMaxP := d.maxP

	newMaxP := d.pFactor * int(math.Ceil(math.Log(float64(numPingable+1))/log10))

	if newMaxP != prevMaxP {
		d.maxP = newMaxP

		d.node.emit(MaxPAdjustedEvent{prevMaxP, newMaxP})

		d.logger.WithFields(log.Fields{
			"newMax":            newMaxP,
			"prevMax":           prevMaxP,
			"propagationFactor": d.pFactor,
			"numPingable":       numPingable,
		}).Debug("adjusted max propagation count")
	}

	d.Unlock()
}

// HasChanges reports whether disseminator has changes to disseminate.
func (d *disseminator) HasChanges() bool {
	d.RLock()
	result := len(d.changes) > 0
	d.RUnlock()
	return result
}

func (d *disseminator) FullSync() (changes []Change) {
	d.Lock()

	for _, member := range d.node.memberlist.GetMembers() {
		changes = append(changes, Change{
			Address:           member.Address,
			Incarnation:       member.Incarnation,
			Source:            d.node.Address(),
			SourceIncarnation: d.node.Incarnation(),
			Status:            member.Status,
		})
	}

	d.Unlock()

	return changes
}

// IssueAsSender collects all changes a node needs when sending a ping or
// ping-req. The second return value is a callback that raises the piggyback
// counters of the given changes.
func (d *disseminator) IssueAsSender() (changes []Change, bumpPiggybackCounters func()) {
	changes = d.issueChanges()
	return changes, func() {
		d.bumpPiggybackCounters(changes)
	}
}

func (d *disseminator) bumpPiggybackCounters(changes []Change) {
	d.Lock()
	for _, change := range changes {
		c, ok := d.changes[change.Address]
		if !ok {
			continue
		}

		c.p += 1
		if c.p >= d.maxP {
			delete(d.changes, c.Address)
		}
	}
	d.Unlock()
}

// IssueAsReceiver collects all changes a node needs when responding to a ping
// or ping-req. Unlike IssueAsSender, IssueAsReceiver automatically increments
// the piggyback counters because it's difficult to find out whether a response
// reaches the client. The second return value indicates whether a full sync
// is triggered.
func (d *disseminator) IssueAsReceiver(
	senderAddress string,
	senderIncarnation int64,
	senderChecksum uint32) (changes []Change, fullSync bool) {

	changes = d.issueChanges()

	// filter out changes that came from the sender previously
	changes = d.filterChangesFromSender(changes, senderAddress, senderIncarnation)

	d.bumpPiggybackCounters(changes)

	if len(changes) > 0 || d.node.memberlist.Checksum() == senderChecksum {
		return changes, false
	}

	d.node.emit(FullSyncEvent{senderAddress, senderChecksum})

	d.node.logger.WithFields(log.Fields{
		"localChecksum":  d.node.memberlist.Checksum(),
		"remote":         senderAddress,
		"remoteChecksum": senderChecksum,
	}).Info("full sync")

	return d.FullSync(), true
}

// filterChangesFromSender returns changes that didn't originate at the sender.
// Attention, this function reorders the underlaying input array.
func (d *disseminator) filterChangesFromSender(cs []Change, source string, incarnation int64) []Change {
	for i := 0; i < len(cs); i++ {
		if incarnation == cs[i].SourceIncarnation && source == cs[i].Source {
			d.node.emit(ChangeFilteredEvent{cs[i]})

			// swap, and not just overwrite, so that in the end only the order
			// of the underlying array has changed.
			cs[i], cs[len(cs)-1] = cs[len(cs)-1], cs[i]

			cs = cs[:len(cs)-1]
			i--
		}
	}
	return cs
}

func (d *disseminator) issueChanges() []Change {
	d.Lock()

	// To make JSON output [] instead of null on empty change list
	result := make([]Change, 0)
	for _, change := range d.changes {
		result = append(result, change.Change)
	}

	d.Unlock()

	d.node.emit(ChangesCalculatedEvent{result})

	return result
}

func (d *disseminator) ClearChanges() {
	d.Lock()
	d.changes = make(map[string]*pChange)
	d.Unlock()
}

func (d *disseminator) RecordChange(change Change) {
	d.Lock()
	d.changes[change.Address] = &pChange{change, 0}
	d.Unlock()
}

func (d *disseminator) ClearChange(address string) {
	d.Lock()
	delete(d.changes, address)
	d.Unlock()
}

func (d *disseminator) ChangesByAddress(address string) (Change, bool) {
	d.RLock()
	change, ok := d.changes[address]
	var c Change
	if ok {
		c = change.Change
	}
	d.RUnlock()
	return c, ok
}

func (d *disseminator) ChangesCount() int {
	d.RLock()
	c := len(d.changes)
	d.RUnlock()
	return c
}
