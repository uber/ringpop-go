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
	"errors"

	log "github.com/uber-common/bark"
	"github.com/uber/tchannel-go/json"
	"golang.org/x/net/context"
)

// emptyArg is a blank arguments used as filler for making TChannel calls that
// require nothing to be passed to TChannel's arg3.
type emptyArg struct{}

// Endpoint is an identifier for an internal swim endpoint
type Endpoint string

const (
	// PingEndpoint is the identifier for /protocol/ping
	PingEndpoint Endpoint = "ping"

	// PingReqEndpoint is the identifier for /protocol/ping-req
	PingReqEndpoint Endpoint = "ping-req"
)

// Status contains a status string of the response from a handler.
type Status struct {
	Status string `json:"status"`
}

// HealResponse contains a list of nodes where healing was attempted
type HealResponse struct {
	Targets []string `json:"targets"`
	Error   string   `json:"error"`
}

// SetLabelRequest contains a key and a value to set in the labels
type SetLabelRequest struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// notImplementedHandler is a dummy handler that returns an error explaining
// this method is not implemented.
func notImplementedHandler(ctx json.Context, req *emptyArg) (*emptyArg, error) {
	return nil, errors.New("handler not implemented")
}

func (n *Node) registerHandlers() error {
	handlers := map[string]interface{}{
		"/protocol/join":             n.joinHandler,
		"/protocol/ping":             n.pingHandler,
		"/protocol/ping-req":         n.pingRequestHandler,
		"/admin/debugSet":            notImplementedHandler,
		"/admin/debugClear":          notImplementedHandler,
		"/admin/gossip":              n.gossipHandler, // Deprecated
		"/admin/gossip/start":        n.gossipHandlerStart,
		"/admin/gossip/stop":         n.gossipHandlerStop,
		"/admin/healpartition/disco": n.discoverProviderHealerHandler,
		"/admin/tick":                n.tickHandler, // Deprecated
		"/admin/gossip/tick":         n.tickHandler,
		"/admin/member/leave":        n.adminLeaveHandler,
		"/admin/member/join":         n.adminJoinHandler,
		"/admin/member/setLabel":     n.adminSetLabelHandler,
		"/admin/reap":                n.reapFaultyMembersHandler,
	}

	return json.Register(n.channel, handlers, n.errorHandler)
}

func (n *Node) joinHandler(ctx json.Context, req *joinRequest) (*joinResponse, error) {
	res, err := handleJoin(n, req)
	if err != nil {
		n.logger.WithFields(log.Fields{
			"error":       err,
			"joinRequest": req,
		}).Debug("invalid join request received")
		return nil, err
	}

	return res, nil
}

func (n *Node) pingHandler(ctx json.Context, req *ping) (*ping, error) {
	return handlePing(n, req)
}

func (n *Node) pingRequestHandler(ctx json.Context, req *pingRequest) (*pingResponse, error) {
	return handlePingRequest(n, req)
}

func (n *Node) gossipHandler(ctx json.Context, req *emptyArg) (*emptyArg, error) {
	switch n.gossip.Stopped() {
	case true:
		n.gossip.Start()
	case false:
		n.gossip.Stop()
	}

	return &emptyArg{}, nil
}

func (n *Node) gossipHandlerStart(ctx json.Context, req *emptyArg) (*emptyArg, error) {
	n.gossip.Start()
	return &emptyArg{}, nil
}

func (n *Node) gossipHandlerStop(ctx json.Context, req *emptyArg) (*emptyArg, error) {
	n.gossip.Stop()
	return &emptyArg{}, nil
}

func (n *Node) discoverProviderHealerHandler(ctx json.Context, req *emptyArg) (*HealResponse, error) {
	targets, err := n.healer.Heal()
	msg := ""
	if err != nil {
		msg = err.Error()
	}
	return &HealResponse{Targets: targets, Error: msg}, nil
}

func (n *Node) tickHandler(ctx json.Context, req *emptyArg) (*ping, error) {
	n.gossip.ProtocolPeriod()
	return &ping{Checksum: n.memberlist.Checksum()}, nil
}

func (n *Node) adminJoinHandler(ctx json.Context, req *emptyArg) (*Status, error) {
	n.memberlist.SetLocalStatus(Alive)
	return &Status{Status: "rejoined"}, nil
}

func (n *Node) adminLeaveHandler(ctx json.Context, req *emptyArg) (*Status, error) {
	n.memberlist.SetLocalStatus(Leave)
	return &Status{Status: "ok"}, nil
}

func (n *Node) adminSetLabelHandler(ctx json.Context, req *SetLabelRequest) (*Status, error) {
	n.Labels().Set(req.Key, req.Value)
	return &Status{Status: "ok"}, nil
}

// reapFaultyMembersHandler iterates through the local members of this nodes and
// declares all the members marked as faulty as a tombstone. This will clean all
// these members from the membership in the complete cluster due to the gossipy
// nature of swim
func (n *Node) reapFaultyMembersHandler(ctx json.Context, req *emptyArg) (*Status, error) {
	members := n.memberlist.GetMembers()
	for _, member := range members {
		if member.Status == Faulty {
			// declare all faulty members as tombstone
			n.memberlist.MakeTombstone(member.Address, member.Incarnation)
		}
	}
	return &Status{Status: "ok"}, nil
}

// errorHandler is called when one of the handlers returns an error.
func (n *Node) errorHandler(ctx context.Context, err error) {
	n.logger.WithField("error", err).Info("error occurred")
}
