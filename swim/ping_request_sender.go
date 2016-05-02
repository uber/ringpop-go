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
	"sync"
	"time"

	log "github.com/uber-common/bark"
	"github.com/uber/ringpop-go/logging"
	"github.com/uber/ringpop-go/shared"
	"github.com/uber/tchannel-go/json"
)

// A PingRequest is used to make a ping request to a remote node
type pingRequest struct {
	Source            string   `json:"source"`
	SourceIncarnation int64    `json:"sourceIncarnationNumber"`
	Target            string   `json:"target"`
	Checksum          uint32   `json:"checksum"`
	Changes           []Change `json:"changes"`
}

// A PingRequestSender is used to make a ping request to a remote node
type pingRequestSender struct {
	node    *Node
	peer    string
	target  string
	timeout time.Duration
	logger  log.Logger
}

// NewPingRequestSender returns a new PingRequestSender
func newPingRequestSender(node *Node, peer, target string, timeout time.Duration) *pingRequestSender {
	p := &pingRequestSender{
		node:    node,
		peer:    peer,
		target:  target,
		timeout: timeout,
		logger:  logging.Logger("ping").WithField("local", node.Address()),
	}

	return p
}

func (p *pingRequestSender) SendPingRequest() (*pingResponse, error) {
	p.logger.WithFields(log.Fields{
		"peer":   p.peer,
		"target": p.target,
	}).Debug("ping request send")

	ctx, cancel := shared.NewTChannelContext(p.timeout)
	defer cancel()

	var res pingResponse
	select {
	case err := <-p.MakeCall(ctx, &res):
		if err == nil {
			p.node.memberlist.Update(res.Changes)
		}
		return &res, err

	case <-ctx.Done(): // call timed out
		return nil, errors.New("ping request timed out")
	}

}

func (p *pingRequestSender) MakeCall(ctx json.Context, res *pingResponse) <-chan error {
	errC := make(chan error, 1)

	go func() {
		defer close(errC)

		changes, bumpPiggybackCounters := p.node.disseminator.IssueAsSender()
		req := &pingRequest{
			Source:            p.node.Address(),
			SourceIncarnation: p.node.Incarnation(),
			Checksum:          p.node.memberlist.Checksum(),
			Changes:           changes,
			Target:            p.target,
		}

		peer := p.node.channel.Peers().GetOrAdd(p.peer)
		err := json.CallPeer(ctx, peer, p.node.service, "/protocol/ping-req", req, &res)
		if err != nil {
			bumpPiggybackCounters()
			errC <- err
			return
		}

		errC <- nil
	}()

	return errC
}

// indirectPing is used to check if a target node can be reached indirectly.
// The indirectPing is performed by sending a specifiable amount of ping
// requests nodes in n's membership.
func indirectPing(n *Node, target string, amount int, timeout time.Duration) (reached bool, errs []error) {
	resCh := sendPingRequests(n, target, amount, timeout)

	// wait for responses from the ping-reqs
	for result := range resCh {
		switch res := result.(type) {
		case *pingResponse:
			if res.Ok {
				return true, errs
			}
			// If the ping to the target was not-ok we want to wait for more results.

		case error:
			errs = append(errs, res)
		}
	}

	return false, errs
}

// sendPingRequests sends ping requests to the target address and returns a channel
//containing the responses. Responses can be one of type:
//  (1) error:          if the call to peer failed
//  (2) PingResponse:   if the peer performed the ping request
func sendPingRequests(node *Node, target string, size int, timeout time.Duration) <-chan interface{} {
	var peerAddresses []string
	peers := node.memberlist.RandomPingableMembers(size, map[string]bool{target: true})

	for _, peer := range peers {
		peerAddresses = append(peerAddresses, peer.Address)
	}

	node.emit(PingRequestsSendEvent{
		Local:  node.Address(),
		Target: target,
		Peers:  peerAddresses,
	})

	var wg sync.WaitGroup
	resC := make(chan interface{}, size)

	for _, peer := range peers {
		wg.Add(1)

		go func(peer Member) {
			defer wg.Done()

			p := newPingRequestSender(node, peer.Address, target, timeout)

			p.logger.WithFields(log.Fields{
				"peer":   peer.Address,
				"target": p.target,
			}).Debug("sending ping request")

			var startTime = time.Now()
			res, err := p.SendPingRequest()

			if err != nil {
				node.emit(PingRequestSendErrorEvent{
					Local:  node.Address(),
					Target: target,
					Peers:  peerAddresses,
					Peer:   peer.Address,
				})

				resC <- err
				return
			}

			node.emit(PingRequestsSendCompleteEvent{
				Local:    node.Address(),
				Target:   target,
				Peers:    peerAddresses,
				Peer:     peer.Address,
				Duration: time.Now().Sub(startTime),
			})

			resC <- res
		}(*peer)
	}

	// wait for all sends to complete before closing channel
	go func() {
		wg.Wait()
		close(resC)
	}()

	return resC
}
