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

// EventEmitter describes an interface that can be used to emit events
type EventEmitter interface {
	EmitEvent(Event)
}

// EventRegistrar is an object that you can register EventListeners on.
type EventRegistrar interface {
	RegisterListener(EventListener)
	DeregisterListener(EventListener)
}

type baseEventRegistrar struct {
	lock      sync.RWMutex
	listeners []EventListener
}

// RegisterListener adds a listener to the Event Registar. Events emitted on this
// registar will be invoked on the listener
func (a *baseEventRegistrar) RegisterListener(l EventListener) {
	if l == nil {
		// do not register nil listener, will cause nil pointer dereference during
		// event emitting
		return
	}

	a.lock.Lock()
	defer a.lock.Unlock()

	// Check if listener is already registered
	for _, listener := range a.listeners {
		if listener == l {
			return
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
}

// DeregisterListener removes a listener from the Event Registar. Subsequent calls
// to EmitEvent will not cause HandleEvent to be called on this listener.
func (a *baseEventRegistrar) DeregisterListener(l EventListener) {
	a.lock.Lock()
	defer a.lock.Unlock()

	for i := range a.listeners {
		if a.listeners[i] == l {
			// copy the list and remove the listener form the copy
			cpy := append([]EventListener(nil), a.listeners...)
			a.listeners = append(cpy[:i], cpy[i+1:]...)
			break
		}
	}
}

// AsyncEventEmitter is an implementation of both an EventRegistar and EventEmitter
// that emits events in their own go routine.
type AsyncEventEmitter struct {
	baseEventRegistrar
}

// EmitEvent will send the event to all registered listeners
func (a *AsyncEventEmitter) EmitEvent(event Event) {
	a.lock.RLock()
	for _, listener := range a.listeners {
		go listener.HandleEvent(event)
	}
	a.lock.RUnlock()
}

// SyncEventEmitter is an implementation of both an EventRegistar and EventEmitter
// that emits events in the calling go routine.
type SyncEventEmitter struct {
	baseEventRegistrar
}

// EmitEvent will send the event to all registered listeners
func (a *SyncEventEmitter) EmitEvent(event Event) {
	a.lock.RLock()
	listeners := a.listeners
	a.lock.RUnlock()

	for _, listener := range listeners {
		listener.HandleEvent(event)
	}
}

// A RingChangedEvent is sent when servers are added and/or removed from the ring
type RingChangedEvent struct {
	ServersAdded   []string
	ServersRemoved []string
}

// RingChecksumEvent is sent when a server is removed or added and a new checksum
// for the ring is calculated
type RingChecksumEvent struct {
	OldChecksum uint32
	NewChecksum uint32
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
