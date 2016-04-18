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

// A PingResponse is the response from a successful ping request call
type pingResponse struct {
	Ok      bool     `json:"pingStatus"`
	Target  string   `json:"target"`
	Changes []Change `json:"changes"`
}

func handlePingRequest(node *Node, req *pingRequest) (*pingResponse, error) {
	if !node.Ready() {
		node.emit(RequestBeforeReadyEvent{PingReqEndpoint})
		return nil, ErrNodeNotReady
	}

	node.emit(PingRequestReceiveEvent{
		Local:   node.Address(),
		Source:  req.Source,
		Target:  req.Target,
		Changes: req.Changes,
	})

	node.serverRate.Mark(1)
	node.totalRate.Mark(1)

	node.memberlist.Update(req.Changes)

	pingStartTime := time.Now()

	res, err := sendPing(node, req.Target, node.pingTimeout)
	pingOk := err == nil

	if pingOk {
		node.emit(PingRequestPingEvent{
			Local:    node.Address(),
			Source:   req.Source,
			Target:   req.Target,
			Duration: time.Now().Sub(pingStartTime),
		})

		node.memberlist.Update(res.Changes)
	}

	changes, _ :=
		node.disseminator.IssueAsReceiver(req.Source, req.SourceIncarnation, req.Checksum)

	// ignore full sync

	return &pingResponse{
		Target:  req.Target,
		Ok:      pingOk,
		Changes: changes,
	}, nil
}
