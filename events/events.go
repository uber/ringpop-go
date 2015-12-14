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

import "time"

// Event is an empty interface that is type switched when handeled.
type Event interface{}

// An EventListener handles events given to it by the Ringpop, as well as forwarded events from
// the SWIM node contained by the ringpop. HandleEvent should be thread safe.
type EventListener interface {
	HandleEvent(event Event)
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
