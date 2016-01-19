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
)

var log10 = math.Log(10)

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
	maxP    int
	pFactor int

	sync.RWMutex
}

// newDisseminator returns a new Disseminator instance.
func newDisseminator(n *Node) *disseminator {
	d := &disseminator{
		node:    n,
		changes: make(map[string]*pChange),
		maxP:    1,
		pFactor: 15,
	}

	return d
}

func (d *disseminator) AdjustMaxPropagations() {
	if !d.node.Ready() {
		return
	}

	d.Lock()

	numPingable := d.node.memberlist.NumPingableMembers()
	prevMaxP := d.maxP

	newMaxP := d.pFactor * int(math.Ceil(math.Log(float64(numPingable+1))/log10))

	if newMaxP != prevMaxP {
		d.maxP = newMaxP

		d.node.emit(MaxPAdjustedEvent{prevMaxP, newMaxP})

		d.node.log.WithFields(log.Fields{
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
	return len(d.changes) > 0
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

func (d *disseminator) IssueAsSender() []Change {
	return d.issueChanges(nil)
}

func (d *disseminator) IssueAsReceiver(senderAddress string,
	senderIncarnation int64, senderChecksum uint32) ([]Change, bool) {

	filter := func(c *pChange) bool {
		return senderIncarnation == c.SourceIncarnation &&
			senderAddress == c.Source
	}

	changes := d.issueChanges(filter)

	if len(changes) > 0 {
		return changes, false
	} else if d.node.memberlist.Checksum() != senderChecksum {
		d.node.emit(FullSyncEvent{senderAddress, senderChecksum})

		d.node.log.WithFields(log.Fields{
			"localChecksum":  d.node.memberlist.Checksum(),
			"remote":         senderAddress,
			"remoteChecksum": senderChecksum,
		}).Info("full sync")

		return d.FullSync(), true
	}

	return []Change{}, false
}

func (d *disseminator) issueChanges(filter func(*pChange) bool) (changes []Change) {
	d.Lock()

	// To make JSON output [] instead of null on empty change list
	changes = make([]Change, 0)
	for _, change := range d.changes {
		if filter != nil && filter(change) {
			d.node.emit(ChangeFilteredEvent{change.Change})
			continue
		}

		change.p++

		if change.p > d.maxP {
			delete(d.changes, change.Address)
			continue
		}

		changes = append(changes, change.Change)
	}

	d.Unlock()

	d.node.emit(ChangesCalculatedEvent{changes})

	return changes
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
