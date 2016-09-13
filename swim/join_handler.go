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

import "fmt"

// A JoinResponse is sent back as a response to a JoinRequest from a
// remote node
type joinResponse struct {
	App         string   `json:"app"`
	Coordinator string   `json:"coordinator"`
	Membership  []Change `json:"membership"`
	Checksum    uint32   `json:"membershipChecksum"`
}

// TODO: Denying joins?

func validateSourceAddress(node *Node, sourceAddress string) error {
	if node.address == sourceAddress {
		return fmt.Errorf("A node tried joining a cluster by attempting to join itself. "+
			"The node ,%s, must join someone else.", sourceAddress)
	}
	return nil
}

func validateSourceApp(node *Node, sourceApp string) error {
	if node.app != sourceApp {
		return fmt.Errorf("A node tried joining a different app cluster. The "+
			"expected app, %s, did not match the actual app ,%s", node.app, sourceApp)
	}
	return nil
}

func handleJoin(node *Node, req *joinRequest) (*joinResponse, error) {
	node.EmitEvent(JoinReceiveEvent{
		Local:  node.Address(),
		Source: req.Source,
	})

	node.serverRate.Mark(1)
	node.totalRate.Mark(1)

	if err := validateSourceAddress(node, req.Source); err != nil {
		return nil, err
	}

	if err := validateSourceApp(node, req.App); err != nil {
		return nil, err
	}

	res := &joinResponse{
		App:         node.app,
		Coordinator: node.address,
		Membership:  node.disseminator.MembershipAsChanges(),
		Checksum:    node.memberlist.Checksum(),
	}

	return res, nil
}
