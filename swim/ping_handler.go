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
	"time"

	log "github.com/uber-common/bark"
)

func handlePing(node *Node, req *ping) (*ping, error) {
	if !node.Ready() {
		node.EmitEvent(RequestBeforeReadyEvent{PingEndpoint})
		return nil, ErrNodeNotReady
	}

	node.EmitEvent(PingReceiveEvent{
		Local:   node.Address(),
		Source:  req.Source,
		Changes: req.Changes,
	})

	if req.App == "" {
		if node.requiresAppInPing {
			node.logger.WithFields(log.Fields{
				"source": req.Source,
			}).Warn("Rejected ping from unknown ringpop app")
			return nil, errors.New("Pinged ringpop requires app name")
		}
	} else {
		if req.App != node.app {
			node.logger.WithFields(log.Fields{
				"app":    req.App,
				"source": req.Source,
			}).Warn("Rejected ping from wrong ringpop app")
			return nil, errors.New("Pinged ringpop has a different app name")
		}
	}

	node.serverRate.Mark(1)
	node.totalRate.Mark(1)

	node.memberlist.Update(req.Changes)

	changes, fullSync :=
		node.disseminator.IssueAsReceiver(req.Source, req.SourceIncarnation, req.Checksum)

	res := &ping{
		Checksum:          node.memberlist.Checksum(),
		Changes:           changes,
		Source:            node.Address(),
		SourceIncarnation: node.Incarnation(),
	}

	// Start bi-directional full sync.
	if fullSync {
		node.disseminator.tryStartReverseFullSync(req.Source, time.Second)
	}

	return res, nil
}
