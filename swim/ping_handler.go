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

func handlePing(node *Node, req *ping) (*ping, error) {
	if !node.Ready() {
		node.emit(RequestBeforeReadyEvent{PingEndpoint})
		return nil, ErrNodeNotReady
	}

	node.emit(PingReceiveEvent{
		Local:   node.Address(),
		Source:  req.Source,
		Changes: req.Changes,
	})

	node.serverRate.Mark(1)
	node.totalRate.Mark(1)

	node.memberlist.Update(req.Changes)

	changes, fullSync :=
		node.disseminator.IssueAsReceiver(req.Source, req.SourceIncarnation, req.Checksum)

	// Bidirectional full sync.
	if fullSync {
		go func() {
			res, err := sendJoinRequest(node, req.Source, time.Second)
			if err != nil {
				// TODO: log bidirectional full sync failed
			}
			node.memberlist.Update(res.Membership)
		}()
	}

	res := &ping{
		Checksum:          node.memberlist.Checksum(),
		Changes:           changes,
		Source:            node.Address(),
		SourceIncarnation: node.Incarnation(),
	}

	return res, nil
}
