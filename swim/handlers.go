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
	log "github.com/uber/bark"
	"github.com/uber/tchannel/golang/json"
	"golang.org/x/net/context"
)

// An Arg is a blank argument used as filler for making TChannel calls that require
// nothing to be passed to Arg3
type Arg struct{}

func (n *Node) registerHandlers() error {
	handlers := map[string]interface{}{
		"/protocol/join":     n.joinHandler,
		"/protocol/ping":     n.pingHandler,
		"/protocol/ping-req": n.pingRequestHandler,
		"/admin/debugSet":    n.debugSetHandler,
		"/admin/debugClear":  n.debugClearHandler,
		"/admin/gossip":      n.gossipHandler,
		"/admin/tick":        n.tickHandler,
	}

	return json.Register(n.channel, handlers, func(ctx context.Context, err error) {
		n.logger.WithField("error", err).Info("error occured")
	})

}

func (n *Node) joinHandler(ctx json.Context, req *joinRequest) (*joinResponse, error) {
	res, err := handleJoin(n, req)
	if err != nil {
		n.logger.WithFields(log.Fields{
			"local":   n.address,
			"error":   err,
			"handler": "join",
		}).Debug("failed to receive join")
		return nil, err
	}

	return res, nil
}

func (n *Node) pingHandler(ctx json.Context, req *ping) (*ping, error) {
	return handlePing(n, req), nil
}

func (n *Node) pingRequestHandler(ctx json.Context, req *pingRequest) (*pingResponse, error) {
	return handlePingRequest(n, req), nil
}

func (n *Node) debugSetHandler(ctx json.Context, req *Arg) (*Arg, error) {
	// n.logger.Level = log.DebugLevel
	return &Arg{}, nil
}

func (n *Node) debugClearHandler(ctx json.Context, req *Arg) (*Arg, error) {
	// n.logger.Level = log.InfoLevel
	return &Arg{}, nil
}

func (n *Node) gossipHandler(ctx json.Context, req *Arg) (*Arg, error) {
	switch n.gossip.Stopped() {
	case true:
		n.gossip.Start()
	case false:
		n.gossip.Stop()
	}

	return &Arg{}, nil
}

func (n *Node) tickHandler(ctx json.Context, req *Arg) (*ping, error) {
	n.gossip.ProtocolPeriod()
	return &ping{Checksum: n.memberlist.Checksum()}, nil
}

func (n *Node) adminJoinHandler(ctx json.Context, req *Arg) (*Arg, error) {
	return nil, nil
}

func (n *Node) adminLeaveHandler(ctx json.Context, req *Arg) (*Arg, error) {
	return nil, nil
}