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

	"github.com/benbjohnson/clock"
	log "github.com/uber-common/bark"
	"github.com/uber/ringpop-go/logging"
	"github.com/uber/ringpop-go/util"
)

var timeZero = time.Date(1, 1, 1, 0, 0, 0, 0, time.UTC)

type updateRollup struct {
	node          *Node
	maxUpdates    int
	flushInterval time.Duration

	buffer struct {
		updates map[string][]Change
		sync.RWMutex
	}

	flushTimer struct {
		t *clock.Timer
		sync.Mutex
	}

	timings struct {
		firstUpdate time.Time
		lastFlush   time.Time
		lastUpdate  time.Time
		sync.RWMutex
	}

	logger log.Logger
}

func newUpdateRollup(node *Node, flushInterval time.Duration, maxUpdates int) *updateRollup {
	rollup := &updateRollup{
		node:          node,
		maxUpdates:    maxUpdates,
		flushInterval: flushInterval,
		logger:        logging.Logger("rollup").WithField("local", node.Address()),
	}

	rollup.buffer.updates = make(map[string][]Change)

	return rollup
}

func (r *updateRollup) Destroy() {
	r.flushTimer.Lock()

	if r.flushTimer.t != nil {
		r.flushTimer.t.Stop()
	}

	r.flushTimer.Unlock()
}

func (r *updateRollup) AddUpdates(changes []Change) {
	r.buffer.Lock()

	timestamp := time.Now()
	for _, change := range changes {
		change.Timestamp = util.Timestamp(timestamp)
		r.buffer.updates[change.Address] = append(r.buffer.updates[change.Address], change)
	}

	r.buffer.Unlock()
}

func (r *updateRollup) TrackUpdates(changes []Change) (flushed bool) {
	if len(changes) == 0 {
		return false
	}

	now := time.Now()

	r.timings.Lock()
	sinceLastUpdate := now.Sub(r.timings.lastUpdate)
	r.timings.Unlock()

	if sinceLastUpdate >= r.flushInterval {
		r.FlushBuffer() // locks buffer and timings
		flushed = true
	}

	r.timings.Lock()
	if r.timings.firstUpdate.IsZero() {
		r.timings.firstUpdate = now
	}

	r.RenewFlushTimer()   // locks flushTimer
	r.AddUpdates(changes) // locks buffer

	r.timings.lastUpdate = now
	r.timings.Unlock()

	return flushed
}

func (r *updateRollup) RenewFlushTimer() {
	r.flushTimer.Lock()

	if r.flushTimer.t != nil {
		r.flushTimer.t.Stop()
	}

	r.flushTimer.t = r.node.clock.AfterFunc(r.flushInterval, func() {
		r.FlushBuffer()
		r.RenewFlushTimer()
	})

	r.flushTimer.Unlock()
}

func (r *updateRollup) numUpdates() (n int) {
	for _, updates := range r.buffer.updates {
		n += len(updates)
	}

	return
}

func (r *updateRollup) FlushBuffer() {
	r.buffer.Lock()

	if len(r.buffer.updates) == 0 {
		r.logger.Debug("no updates flushed")
		r.buffer.Unlock()
		return
	}

	now := time.Now()

	r.timings.Lock()

	sinceFirstUpdate := time.Duration(0)
	if !r.timings.lastUpdate.IsZero() {
		sinceFirstUpdate = r.timings.lastUpdate.Sub(r.timings.firstUpdate)
	}

	sinceLastFlush := time.Duration(0)
	if !r.timings.lastFlush.IsZero() {
		sinceLastFlush = now.Sub(r.timings.lastFlush)
	}

	r.logger.WithFields(log.Fields{
		"checksum":         r.node.memberlist.Checksum(),
		"sinceFirstUpdate": sinceFirstUpdate,
		"sinceLastFlush":   sinceLastFlush,
		"numUpdates":       r.numUpdates(),
		"updates":          r.buffer,
	}).Debug("rollup flushed update buffer")

	r.buffer.updates = make(map[string][]Change)
	r.timings.lastFlush = now
	r.timings.firstUpdate = util.TimeZero()
	r.timings.lastUpdate = util.TimeZero()

	r.buffer.Unlock()
	r.timings.Unlock()
}

// testing func to avoid data races
func (r *updateRollup) FlushTimer() *clock.Timer {
	r.flushTimer.Lock()
	timer := r.flushTimer.t
	r.flushTimer.Unlock()

	return timer
}

// testing func to avoid data races
func (r *updateRollup) Buffer() map[string][]Change {
	r.buffer.RLock()
	buffer := r.buffer.updates
	r.buffer.RUnlock()

	return buffer
}
