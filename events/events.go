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

package events

import (
	"sync"
	"time"
)

// Event is an empty interface that is type switched when handeled.
type Event interface{}

// An EventListener handles events given to it by the Ringpop, as well as forwarded events from
// the SWIM node contained by the ringpop. HandleEvent should be thread safe.
type EventListener interface {
	HandleEvent(event Event)
}

// EventEmitter can add and remove listeners which will be invoked when an event
// is emitted.
type EventEmitter interface {
	// AddListener adds a listener that will be invoked when an evemit is
	// emitted via EmitEvent. The value returned indicates if the listener have
	// been added or not. The operation will not fail, but a listener will only
	// be added once.
	AddListener(EventListener) bool

	// RemoveListener removes a listener and prevents it being invoked in
	// subsequent emitted events. The return value indicates if a value has been
	// removed or not.
	RemoveListener(EventListener) bool

	// EmitEvent invokes all the listeners that are registered with the
	// EventEmitter
	EmitEvent(Event)
}

type sharedEventEmitter struct {
	// listeners is a slice keeping all added EvenListener interfaces. The slice
	// is only assinged, but never altered in place. When the slice is accessed
	// an approriate lock needs to be aquired on listenersLock. After reading
	// the slice it is safe to release the lock because the underlying array is
	// never changed in place.
	listeners     []EventListener
	listenersLock sync.RWMutex
}

// AddListener adds a listener to the EventEmitter. Events emitted on this
// emitter will be invoked on the listener. The return value indicates if the
// listener has been added or not. It can't be added if it is already added and
// therefore registered to receive events
func (a *sharedEventEmitter) AddListener(l EventListener) bool {
	if l == nil {
		// do not register nil listener, will cause nil pointer dereference during
		// event emitting
		return false
	}

	a.listenersLock.Lock()
	defer a.listenersLock.Unlock()

	// Check if listener is already registered
	for _, listener := range a.listeners {
		if listener == l {
			return false
		}
	}

	// by making a copy the backing array will never be changed after its creation.
	// this allowes to copy the slice while locked but iterate while not locked
	// preventing deadlocks when listeners are added/removed in the handler of a
	// listener
	listenersCopy := make([]EventListener, 0, len(a.listeners)+1)
	listenersCopy = append(listenersCopy, a.listeners...)
	listenersCopy = append(listenersCopy, l)

	a.listeners = listenersCopy

	return true
}

// RemoveListener removes a listener from the EventEmitter. Subsequent calls to
// EmitEvent will not cause HandleEvent to be called on this listener. The
// return value indicates if a listener has been removed or not. The listener
// can't be removed if it was not present before.
func (a *sharedEventEmitter) RemoveListener(l EventListener) bool {
	a.listenersLock.Lock()
	defer a.listenersLock.Unlock()

	for i := range a.listeners {
		if a.listeners[i] == l {
			// create a new list excluding the listener that needs removal
			listenersCopy := make([]EventListener, 0, len(a.listeners)-1)
			listenersCopy = append(listenersCopy, a.listeners[:i]...)
			listenersCopy = append(listenersCopy, a.listeners[i+1:]...)
			a.listeners = listenersCopy

			return true
		}
	}

	return false
}

// AsyncEventEmitter is an implementation of both an EventRegistar and EventEmitter
// that emits events in their own go routine.
type AsyncEventEmitter struct {
	sharedEventEmitter
}

// EmitEvent will send the event to all registered listeners
func (a *AsyncEventEmitter) EmitEvent(event Event) {
	a.listenersLock.RLock()
	for _, listener := range a.listeners {
		go listener.HandleEvent(event)
	}
	a.listenersLock.RUnlock()
}

// SyncEventEmitter is an implementation of both an EventRegistar and EventEmitter
// that emits events in the calling go routine.
type SyncEventEmitter struct {
	sharedEventEmitter
}

// EmitEvent will send the event to all registered listeners
func (a *SyncEventEmitter) EmitEvent(event Event) {
	// we copy the slice to a local variable before calling the listeners. This
	// makes it possible for the listener to remove itself during its invocation
	// without running into a deadlock. Since the underlying array is immutable
	// (by convention) reading it without the lock is safe to do
	a.listenersLock.RLock()
	listeners := a.listeners
	a.listenersLock.RUnlock()

	for _, listener := range listeners {
		listener.HandleEvent(event)
	}
}

// A RingChangedEvent is sent when servers are added and/or removed from the ring
type RingChangedEvent struct {
	ServersAdded   []string
	ServersUpdated []string
	ServersRemoved []string
}

// RingChecksumEvent is sent when a server is removed or added and a new checksum
// for the ring is calculated
type RingChecksumEvent struct {
	// OldChecksum contains the previous legacy checksum. Note: might be deprecated in the future.
	OldChecksum uint32
	// NewChecksum contains the new legacy checksum. Note: might be deprecated in the future.
	NewChecksum uint32
	// OldChecksums contains the map of previous checksums
	OldChecksums map[string]uint32
	// NewChecksums contains the map with new checksums
	NewChecksums map[string]uint32
}

// A LookupEvent is sent when a lookup is performed on the Ringpop's ring
type LookupEvent struct {
	Key      string
	Duration time.Duration
}

// A LookupNEvent is sent when a lookupN is performed on the Ringpop's ring
type LookupNEvent struct {
	Key      string
	N        int
	Duration time.Duration
}

// Ready is fired when ringpop has successfully bootstrapped and is ready to receive requests and other method calls.
type Ready struct{}

// Destroyed is fired when ringpop has been destroyed and should not be responding to requests or lookup requests.
type Destroyed struct{}
