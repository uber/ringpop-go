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

	log "github.com/uber/bark"
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
}

// NewPingRequestSender returns a new PingRequestSender
func newPingRequestSender(node *Node, peer, target string, timeout time.Duration) *pingRequestSender {
	p := &pingRequestSender{
		node:    node,
		peer:    peer,
		target:  target,
		timeout: timeout,
	}

	return p
}

func (p *pingRequestSender) SendPingRequest() (*pingResponse, error) {
	p.node.log.WithFields(log.Fields{
		"peer":   p.peer,
		"target": p.target,
	}).Debug("ping request send")

	ctx, cancel := json.NewContext(p.timeout)
	defer cancel()

	var res pingResponse
	select {
	case err := <-p.MakeCall(ctx, &res):
		return &res, err

	case <-ctx.Done(): // call timed out
		return nil, errors.New("ping request timed out")
	}

}

func (p *pingRequestSender) MakeCall(ctx json.Context, res *pingResponse) <-chan error {
	errC := make(chan error)

	go func() {
		defer close(errC)

		peer := p.node.channel.Peers().GetOrAdd(p.peer)

		req := &pingRequest{
			Source:            p.node.Address(),
			SourceIncarnation: p.node.Incarnation(),
			Checksum:          p.node.memberlist.Checksum(),
			Changes:           p.node.disseminator.IssueAsSender(),
			Target:            p.target,
		}

		err := json.CallPeer(ctx, peer, p.node.service, "/protocol/ping-req", req, &res)
		if err != nil {
			errC <- err
			return
		}

		errC <- nil
	}()

	return errC
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
			p := newPingRequestSender(node, peer.Address, target, timeout)

			p.node.log.WithFields(log.Fields{
				"peer":   peer.Address,
				"target": p.target,
			}).Debug("sending ping request")

			var startTime = time.Now()
			res, err := p.SendPingRequest()
			if err != nil {
				resC <- err
			} else {
				node.emit(PingRequestsSendCompleteEvent{
					Local:    node.Address(),
					Target:   target,
					Peers:    peerAddresses,
					Duration: time.Now().Sub(startTime),
				})
				resC <- res
			}

			wg.Done()
		}(*peer)
	}

	// wait for all sends to complete before closing channel
	go func() {
		wg.Wait()
		close(resC)
	}()

	return resC
}
