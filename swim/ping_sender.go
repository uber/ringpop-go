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

	log "github.com/uber/bark"

	"github.com/uber/tchannel/golang/json"
)

// A Ping is used as an Arg3 for the ping TChannel call / response
type ping struct {
	Changes           []Change `json:"changes"`
	Checksum          uint32   `json:"checksum"`
	Source            string   `json:"source"`
	SourceIncarnation int64    `json:"sourceIncarnation"`
}

// A PingSender is used to send a SWIM gossip ping over TChannel to target node
type pingSender struct {
	node    *Node
	target  string
	timeout time.Duration
}

// NewPingSender returns a new PingSender that can be used to send a ping to target node
func newPingSender(node *Node, target string, timeout time.Duration) *pingSender {
	ps := &pingSender{
		node:    node,
		target:  target,
		timeout: timeout,
	}

	return ps
}

func (p *pingSender) SendPing() (*ping, error) {
	ctx, cancel := json.NewContext(p.timeout)
	defer cancel()

	var res ping
	select {
	case err := <-p.MakeCall(ctx, &res):
		return &res, err

	case <-ctx.Done(): // ping timed out
		return nil, errors.New("ping timed out")
	}
}

func (p *pingSender) MakeCall(ctx json.Context, res *ping) <-chan error {
	errC := make(chan error)

	go func() {
		defer close(errC)

		peer := p.node.channel.Peers().GetOrAdd(p.target)

		req := ping{
			Checksum:          p.node.memberlist.Checksum(),
			Changes:           p.node.disseminator.IssueAsSender(),
			Source:            p.node.Address(),
			SourceIncarnation: p.node.Incarnation(),
		}

		p.node.emit(PingSendEvent{
			Local:   p.node.Address(),
			Remote:  p.target,
			Changes: req.Changes,
		})

		p.node.log.WithFields(log.Fields{
			"remote":  p.target,
			"changes": req.Changes,
		}).Debug("ping send")

		var startTime = time.Now()

		err := json.CallPeer(ctx, peer, p.node.service, "/protocol/ping", req, res)
		if err != nil {
			p.node.log.WithFields(log.Fields{
				"remote": p.target,
				"error":  err,
			}).Debug("ping failed")
			errC <- err
			return
		}

		p.node.emit(PingSendCompleteEvent{
			Local:    p.node.Address(),
			Remote:   p.target,
			Changes:  req.Changes,
			Duration: time.Now().Sub(startTime),
		})

		errC <- nil
	}()

	return errC
}

// SendPing sends a ping to target node that times out after timeout
func sendPing(node *Node, target string, timeout time.Duration) (*ping, error) {
	ps := newPingSender(node, target, timeout)
	res, err := ps.SendPing()
	return res, err
}
