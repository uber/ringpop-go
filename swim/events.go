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
	"time"

	"github.com/uber/ringpop-go/events"
)

// An EventListener handles events given to it by the SWIM node. HandleEvent should be thread safe.
type EventListener interface {
	HandleEvent(events.Event)
}

// The ListenerFunc type is an adapter to allow the use of ordinary functions
// as EventListeners.
type ListenerFunc func(events.Event)

// HandleEvent calls f(e).
func (f ListenerFunc) HandleEvent(e events.Event) {
	f(e)
}

// A MaxPAdjustedEvent occurs when the disseminator adjusts the max propagation
// count for changes
type MaxPAdjustedEvent struct {
	OldPCount int `json:"oldPCount"`
	NewPCount int `json:"newPCount"`
}

// A MemberlistChangesReceivedEvent contains changes received by the node's
// memberlist, pending application
type MemberlistChangesReceivedEvent struct {
	Changes []Change `json:"changes"`
}

// A MemberlistChangesAppliedEvent contains changes that were applied to the
// node's memberlist as well as the previous and new checksums and the
// number of members in the memberlist
type MemberlistChangesAppliedEvent struct {
	Changes     []Change `json:"changes"`
	OldChecksum uint32   `json:"oldChecksum"`
	NewChecksum uint32   `json:"newChecksum"`
	NumMembers  int      `json:"numMembers"`
}

// A FullSyncEvent is sent when the disseminator's node issues changes a
// full sync of the memberlist
type FullSyncEvent struct {
	Remote         string `json:"remote"`
	RemoteChecksum uint32 `json:"remoteChecksum"`
}

// A JoinReceiveEvent is sent when a join request is received by a node
type JoinReceiveEvent struct {
	Local  string `json:"local"`
	Source string `json:"source"`
}

// A JoinCompleteEvent is sent when a join request to remote node successfully
// completes
type JoinCompleteEvent struct {
	Duration  time.Duration `json:"duration"`
	NumJoined int           `json:"numJoined"`
	Joined    []string      `json:"joined"`
}

// JoinFailedReason indicates the reason a join failed
type JoinFailedReason string

const (
	// Error as a JoinFailedReason indicates that the join failed because of an error
	Error JoinFailedReason = "err"

	// Destroyed as a JoinFailedReason indicates that the join failed because ringpop was destroyed during the join
	Destroyed = "destroyed"
)

// A JoinFailedEvent is sent when a join request to remote node did not successfully
type JoinFailedEvent struct {
	Reason JoinFailedReason
	Error  error
}

// A PingSendEvent is sent when the node sends a ping to a remote node
type PingSendEvent struct {
	Local   string   `json:"local"`
	Remote  string   `json:"remote"`
	Changes []Change `json:"changes"`
}

// A PingSendCompleteEvent is sent when the node finished sending a ping to a remote node
type PingSendCompleteEvent struct {
	Local    string        `json:"local"`
	Remote   string        `json:"remote"`
	Changes  []Change      `json:"changes"`
	Duration time.Duration `json:"duration"`
}

// A PingReceiveEvent is sent when the node receives a ping from a remote node
type PingReceiveEvent struct {
	Local   string   `json:"local"`
	Source  string   `json:"source"`
	Changes []Change `json:"changes"`
}

// A PingRequestsSendEvent is sent when the node sends ping requests to remote nodes
type PingRequestsSendEvent struct {
	Local  string   `json:"local"`
	Target string   `json:"target"`
	Peers  []string `json:"peers"`
}

// A PingRequestSendErrorEvent is sent when the node can't get a response sending ping requests to remote nodes
type PingRequestSendErrorEvent struct {
	Local  string   `json:"local"`
	Target string   `json:"target"`
	Peers  []string `json:"peers"`
	Peer   string   `json:"peer"`
}

// A PingRequestsSendCompleteEvent is sent when the node finished sending ping requests to remote nodes
type PingRequestsSendCompleteEvent struct {
	Local    string        `json:"local"`
	Target   string        `json:"target"`
	Peers    []string      `json:"peers"`
	Peer     string        `json:"peer"`
	Duration time.Duration `json:"duration"`
}

// A PingRequestReceiveEvent is sent when the node receives a pign request from a remote node
type PingRequestReceiveEvent struct {
	Local   string   `json:"local"`
	Source  string   `json:"source"`
	Target  string   `json:"target"`
	Changes []Change `json:"changes"`
}

// A PingRequestPingEvent is sent when the node sends a ping to the target node at the
// behest of the source node and receives a response
type PingRequestPingEvent struct {
	Local    string        `json:"local"`
	Source   string        `json:"source"`
	Target   string        `json:"target"`
	Duration time.Duration `json:"duration"`
}

// A ProtocolDelayComputeEvent is sent when protocol delay is computed during a gossip run
type ProtocolDelayComputeEvent struct {
	Duration time.Duration `json:"duration"`
}

// A ProtocolFrequencyEvent is sent when a gossip run is finished
type ProtocolFrequencyEvent struct {
	Duration time.Duration `json:"duration"`
}

// A ChecksumComputeEvent is sent when a the rings checksum is computed
type ChecksumComputeEvent struct {
	Duration time.Duration `json:"duration"`
	Checksum uint32        `json:"checksum"`
}

// A ChangesCalculatedEvent is sent when the disseminator generated the list of changes to send in a ping or its response
type ChangesCalculatedEvent struct {
	Changes []Change
}

// A ChangeFilteredEvent is sent when a change has been filtered from the list to be disseminated
type ChangeFilteredEvent struct {
	Change Change
}

// A JoinTriesUpdateEvent is sent when the joiner tries to join a group
type JoinTriesUpdateEvent struct {
	Retries int
}

// A MakeNodeStatusEvent is sent when Make[Status] is called on member list
type MakeNodeStatusEvent struct {
	Status string
}

// A RequestBeforeReadyEvent is sent if a remote request came in for a ringpop endpoint while ringpop was not ready to process requests
type RequestBeforeReadyEvent struct {
	Endpoint Endpoint
}

// A RefuteUpdateEvent is sent when a node detects gossip about its own state that needs to be corrected
type RefuteUpdateEvent struct{}
