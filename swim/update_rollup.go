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
	"sync"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/uber/ringpop-go/swim/util"
)

var timeZero = time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)

type updateRollup struct {
	node            *Node
	maxUpdates      int
	buffer          map[string][]Change
	firstUpdateTime time.Time
	lastFlushTime   time.Time
	lastUpdateTime  time.Time
	flushTimer      *time.Timer
	flushInterval   time.Duration

	bl sync.RWMutex // protects buffer
	tl sync.RWMutex // protects flush timer
}

func newUpdateRollup(node *Node, flushInterval time.Duration, maxUpdates int) *updateRollup {
	rollup := &updateRollup{
		node:          node,
		buffer:        make(map[string][]Change),
		maxUpdates:    maxUpdates,
		flushInterval: flushInterval,
	}

	return rollup
}

func (r *updateRollup) Destroy() {
	r.tl.Lock()
	defer r.tl.Unlock()

	if r.flushTimer != nil {
		r.flushTimer.Stop()
	}
}

func (r *updateRollup) AddUpdates(changes []Change) {
	r.bl.Lock()
	defer r.bl.Unlock()

	timestamp := time.Now()
	for _, change := range changes {
		change.Timestamp = timestamp
		r.buffer[change.Address] = append(r.buffer[change.Address], change)
	}
}

func (r *updateRollup) TrackUpdates(changes []Change) (flushed bool) {
	if len(changes) == 0 {
		return false
	}

	now := time.Now()

	sinceLastUpdate := now.Sub(r.lastUpdateTime)
	if sinceLastUpdate >= r.flushInterval {
		r.FlushBuffer()
		flushed = true
	}

	if r.firstUpdateTime.IsZero() {
		r.firstUpdateTime = now
	}

	r.RenewFlushTimer()
	r.AddUpdates(changes)

	r.bl.Lock()
	defer r.bl.Unlock()

	r.lastUpdateTime = now

	return flushed
}

func (r *updateRollup) NumUpdates() (n int) {
	for _, updates := range r.buffer {
		n += len(updates)
	}

	return
}

func (r *updateRollup) RenewFlushTimer() {
	r.tl.Lock()
	defer r.tl.Unlock()

	if r.flushTimer != nil {
		r.flushTimer.Stop()
	}

	r.flushTimer = time.AfterFunc(r.flushInterval, func() {
		r.FlushBuffer()
		r.RenewFlushTimer()
	})
}

func (r *updateRollup) FlushBuffer() {
	r.bl.Lock()
	defer r.bl.Unlock()

	if len(r.buffer) == 0 {
		r.node.logger.WithField("local", r.node.Address()).Debug("no updates flushed")
		return
	}

	now := time.Now()

	sinceFirstUpdate := time.Duration(0)
	if !r.lastUpdateTime.IsZero() {
		sinceFirstUpdate = r.lastUpdateTime.Sub(r.firstUpdateTime)
	}

	sinceLastFlush := time.Duration(0)
	if !r.lastFlushTime.IsZero() {
		sinceLastFlush = now.Sub(r.lastFlushTime)
	}

	r.node.logger.WithFields(log.Fields{
		"local":            r.node.Address(),
		"checksum":         r.node.memberlist.Checksum(),
		"sinceFirstUpdate": sinceFirstUpdate,
		"sinceLastFlush":   sinceLastFlush,
		"numUpdates":       r.NumUpdates(),
		// "updates":          r.buffer,
	}).Debug("rollup flushed update buffer")

	r.buffer = make(map[string][]Change)
	r.lastFlushTime = now
	r.firstUpdateTime = util.TimeZero()
	r.lastUpdateTime = util.TimeZero()
}

// testing func to avoid data races
func (r *updateRollup) FlushTimer() *time.Timer {
	r.tl.RLock()
	defer r.tl.RUnlock()

	return r.flushTimer
}

// testing func to avoid data races
func (r *updateRollup) Buffer() map[string][]Change {
	r.bl.RLock()
	defer r.bl.RUnlock()

	return r.buffer
}
