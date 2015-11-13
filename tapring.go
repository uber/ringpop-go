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

package ringpop

import (
	"errors"
	"time"

	"github.com/uber/tchannel-go/json"
)

// An tapRequest is used to query the ring on the remote node
type tapRequest struct {
	Checksum uint32 `json:"checksum"`
}

// An tapResponse is used to send the ring to the remote node
type tapResponse struct {
	Checksum uint32   `json:"checksum"`
	Servers  []string `json:"servers"`
}

// tapRemoteRing taps on the ring and gets the info from the remote node
func tapRemoteRing(host string, rp *Ringpop) ([]string, uint32, error) {
	ctx, cancel := json.NewContext(3 * time.Second)
	defer cancel()

	var resp tapResponse
	var err error

	select {
	case err = <-sendTap(ctx, rp, host, &resp):
	case <-ctx.Done():
		err = errors.New("ctx timed out")
	}

	return resp.Servers, resp.Checksum, err
}

// sendTap sends the message to the specific node to get the info about the ring
func sendTap(ctx json.Context, rp *Ringpop, node string, res *tapResponse) <-chan error {
	errC := make(chan error)

	go func() {
		defer close(errC)

		peer := rp.channel.Peers().GetOrAdd(node)

		req := &tapRequest{
			Checksum: rp.ring.Checksum(),
		}

		err := json.CallPeer(ctx, peer, rp.channel.ServiceName(), "/tapring", req, res)
		if err != nil {
			errC <- err
			return
		}

		errC <- nil
	}()

	return errC
}

// handleTapRing handles the message coming from the remote node and sends
// all the servers on the ring. If the checksum coming from the request is
// the same as the one here, no need to return anything
func handleTapRing(rp *Ringpop, req *tapRequest) *tapResponse {
	res := &tapResponse{
		Servers:  []string{},
		Checksum: req.Checksum,
	}
	if req.Checksum != rp.ring.Checksum() {
		res.Checksum = rp.ring.Checksum()
		res.Servers = rp.ring.GetServers()
	}
	return res
}
