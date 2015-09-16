// Copyright (c) 2015 Uber Technologies, Inc.

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

package tchannel_test

// This file contains functions for tests to access internal tchannel state.
// Since it has a _test.go suffix, it is only compiled with tests in this package.

import (
	"bytes"
	"math/rand"
	"sync"
	"testing"
	"time"

	. "github.com/uber/tchannel/golang"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber/tchannel/golang/raw"
	"github.com/uber/tchannel/golang/testutils"
	"golang.org/x/net/context"
)

type swapper struct {
	t *testing.T
}

func (s *swapper) OnError(ctx context.Context, err error) {
	s.t.Errorf("OnError: %v", err)
}

func (*swapper) Handle(ctx context.Context, args *raw.Args) (*raw.Res, error) {
	return &raw.Res{
		Arg2: args.Arg3,
		Arg3: args.Arg2,
	}, nil
}

func TestFramesReleased(t *testing.T) {
	CheckStress(t)

	defer testutils.SetTimeout(t, 10*time.Second)()
	const (
		requestsPerGoroutine = 10
		numGoroutines        = 10
		maxRandArg           = 512 * 1024
	)

	var connections []*Connection
	pool := NewRecordingFramePool()
	WithVerifiedServer(t, &testutils.ChannelOpts{
		ServiceName: "swap-server",
		DefaultConnectionOptions: ConnectionOptions{
			FramePool: pool,
		},
	}, func(serverCh *Channel, hostPort string) {
		serverCh.Register(raw.Wrap(&swapper{t}), "swap")

		clientCh, err := NewChannel("swap-client", nil)
		require.NoError(t, err)
		defer clientCh.Close()

		// Create an active connection that can be shared by the goroutines by calling Ping.
		ctx, cancel := NewContext(time.Second)
		defer cancel()
		require.NoError(t, clientCh.Ping(ctx, hostPort))

		var wg sync.WaitGroup
		worker := func() {
			for i := 0; i < requestsPerGoroutine; i++ {
				ctx, cancel := NewContext(time.Second * 5)
				defer cancel()

				require.NoError(t, clientCh.Ping(ctx, hostPort))

				arg2 := testutils.RandBytes(rand.Intn(maxRandArg))
				arg3 := testutils.RandBytes(rand.Intn(maxRandArg))
				resArg2, resArg3, _, err := raw.Call(ctx, clientCh, hostPort, "swap-server", "swap", arg2, arg3)
				if !assert.NoError(t, err, "error during sendRecv") {
					continue
				}

				// We expect the arguments to be swapped.
				if bytes.Compare(arg3, resArg2) != 0 {
					t.Errorf("returned arg2 does not match expected:\n  got %v\n want %v", resArg2, arg3)
				}
				if bytes.Compare(arg2, resArg3) != 0 {
					t.Errorf("returned arg2 does not match expected:\n  got %v\n want %v", resArg3, arg2)
				}
			}
			wg.Done()
		}

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go worker()
		}

		wg.Wait()

		connections = append(connections, GetConnections(serverCh)...)
		connections = append(connections, GetConnections(clientCh)...)
	})

	// Wait a few milliseconds for the closing of channels to take effect.
	time.Sleep(10 * time.Millisecond)

	if unreleasedCount, isEmpty := pool.CheckEmpty(); isEmpty != "" || unreleasedCount > 0 {
		t.Errorf("Frame pool has %v unreleased frames, errors:\n%v", unreleasedCount, isEmpty)
	}

	// Check the message exchanges and make sure they are all empty.
	if exchangesLeft := CheckEmptyExchangesConns(connections); exchangesLeft != "" {
		t.Errorf("Found uncleared message exchanges:\n%v", exchangesLeft)
	}
}
