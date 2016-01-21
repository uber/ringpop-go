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
	"github.com/uber/tchannel-go/json"
	"golang.org/x/net/context"
)

// TODO: EVERYTHING!

// Arg is a blank arg
type Arg struct{}

func (rp *Ringpop) registerHandlers() error {
	handlers := map[string]interface{}{
		"/health":       rp.health,
		"/admin/stats":  rp.adminStatsHandler,
		"/admin/lookup": rp.adminLookupHandler,
	}

	return json.Register(rp.subChannel, handlers, func(ctx context.Context, err error) {
		rp.logger.WithField("error", err).Info("error occured")
	})
}

func (rp *Ringpop) health(ctx json.Context, req *Arg) (*Arg, error) {
	return nil, nil
}

func (rp *Ringpop) adminStatsHandler(ctx json.Context, req *Arg) (map[string]interface{}, error) {
	return handleStats(rp), nil
}

type lookupRequest struct {
	Key string `json:"key"`
}

type lookupResponse struct {
	Dest string `json:"dest"`
}

func (rp *Ringpop) adminLookupHandler(ctx json.Context, req *lookupRequest) (*lookupResponse, error) {
	dest, err := rp.Lookup(req.Key)
	if err != nil {
		return nil, err
	}
	return &lookupResponse{Dest: dest}, nil
}

func (rp *Ringpop) adminReloadHandler(ctx json.Context, req *Arg) (*Arg, error) {
	return nil, nil
}
