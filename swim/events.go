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

import "time"

// An EventListener handles events given to it by the SWIM node. HandleEvent should be thread safe.
type EventListener interface {
	HandleEvent(interface{})
}

// A MaxPAdjustedEvent occurs when the disseminator adjusts the max propogation
// count for changes
type MaxPAdjustedEvent struct {
	OldPCount int
	NewPCount int
}

// A MemberlistChangesReceivedEvent contains changes received by the node's
// memberlist, pending application
type MemberlistChangesReceivedEvent struct {
	Changes []Change
}

// A MemberlistChangesAppliedEvent contains changes that were applied to the
// node's memberlist as well as the previous and new checksums and the
// number of members in the memberlist
type MemberlistChangesAppliedEvent struct {
	Changes     []Change
	OldChecksum uint32
	NewChecksum uint32
	NumMembers  int
}

// A FullSyncEvent is sent when the disseminator's node issues changes a
// full sync of the memberlist
type FullSyncEvent struct {
	Remote         string
	RemoteChecksum uint32
}

// A JoinReceiveEvent is sent when a join request is received by a node
type JoinReceiveEvent struct {
	Local  string
	Source string
}

// A JoinCompleteEvent is sent when a join request to remote node successfully
// completes
type JoinCompleteEvent struct {
	Duration  time.Duration
	NumJoined int
	Joined    []string
}

// A PingSendEvent is sent when the node sends a ping to a remote node
type PingSendEvent struct {
	Local   string
	Remote  string
	Changes []Change
}

// A PingReceiveEvent is sent when the node receives a ping from a remote node
type PingReceiveEvent struct {
	Local   string
	Source  string
	Changes []Change
}

// A PingRequestsSendEvent is sent when the node sends a ping requests to remote nodes
type PingRequestsSendEvent struct {
	Local  string
	Target string
	Peers  []string
}

// A PingRequestReceiveEvent is sent when the node receives a pign request from a remote node
type PingRequestReceiveEvent struct {
	Local   string
	Source  string
	Target  string
	Changes []Change
}

// A PingRequestPingEvent is sent when the node sends a ping to the target node at the
// behest of the source node and receives a response
type PingRequestPingEvent struct {
	Local    string
	Source   string
	Target   string
	Duration time.Duration
}
