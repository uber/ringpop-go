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

	"github.com/uber/ringpop-go/logging"
	"github.com/uber/ringpop-go/shared"
	"github.com/uber/tchannel-go/json"
)

// A Ping is used as an Arg3 for the ping TChannel call / response
type ping struct {
	Changes           []Change `json:"changes"`
	Checksum          uint32   `json:"checksum"`
	Source            string   `json:"source"`
	SourceIncarnation int64    `json:"sourceIncarnationNumber"`
	App               string   `json:"app"`
}

// sendPing sends a ping to target node that times out after timeout
func sendPing(node *Node, target string, timeout time.Duration) (*ping, error) {
	changes, bumpPiggybackCounters := node.disseminator.IssueAsSender()

	res, err := sendPingWithChanges(node, target, changes, timeout)
	if err != nil {
		return res, err
	}

	// when ping was successful
	bumpPiggybackCounters()

	return res, err
}

// sendPingWithChanges sends a special ping to the target with the given changes.
// In normal pings the disseminator is consulted to create issue the changes,
// this is not the case in this function. Only the given changes are transmitted.
func sendPingWithChanges(node *Node, target string, changes []Change, timeout time.Duration) (*ping, error) {
	req := ping{
		Checksum:          node.memberlist.Checksum(),
		Changes:           changes,
		Source:            node.Address(),
		SourceIncarnation: node.Incarnation(),
		App:               node.app,
	}

	node.EmitEvent(PingSendEvent{
		Local:   node.Address(),
		Remote:  target,
		Changes: req.Changes,
	})

	logging.Logger("ping").WithFields(log.Fields{
		"local":   node.Address(),
		"remote":  target,
		"changes": req.Changes,
	}).Debug("ping send")

	ctx, cancel := shared.NewTChannelContext(timeout)
	defer cancel()

	peer := node.channel.Peers().GetOrAdd(target)
	startTime := time.Now()

	// send the ping
	errC := make(chan error, 1)
	res := &ping{}
	go func() {
		errC <- json.CallPeer(ctx, peer, node.service, "/protocol/ping", req, res)
	}()

	// get result or timeout
	var err error
	select {
	case err = <-errC:
	case <-ctx.Done():
		err = errors.New("ping timed out")
	}

	if err != nil {
		// ping failed
		logging.Logger("ping").WithFields(log.Fields{
			"local":  node.Address(),
			"remote": target,
			"error":  err,
		}).Debug("ping failed")

		return nil, err
	}

	node.EmitEvent(PingSendCompleteEvent{
		Local:    node.Address(),
		Remote:   target,
		Changes:  req.Changes,
		Duration: time.Now().Sub(startTime),
	})

	return res, err
}
