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

import (
	"runtime"
	"testing"
	"time"

	. "github.com/uber/tchannel-go"

	"github.com/uber/tchannel-go/testutils"
)

func waitForChannelClose(t *testing.T, ch *Channel) bool {
	var state ChannelState
	for i := 0; i < 10; i++ {
		if state = ch.State(); state == ChannelClosed {
			return true
		}

		if i > 5 {
			time.Sleep(time.Millisecond)
		} else {
			runtime.Gosched()
		}
	}

	// Channel is not closing, fail the test.
	t.Errorf("Channel did not close, last state: %v", state)
	return false
}

// WithVerifiedServer runs the given test function with a server channel that is verified
// at the end to make sure there are no leaks (e.g. no exchanges leaked).
func WithVerifiedServer(t *testing.T, opts *testutils.ChannelOpts, f func(serverCh *Channel, hostPort string)) {
	var ch *Channel
	testutils.WithServer(t, opts, func(serverCh *Channel, hostPort string) {
		f(serverCh, hostPort)
		ch = serverCh
	})

	if !waitForChannelClose(t, ch) {
		return
	}

	// Check the message exchanges and make sure they are all empty.
	if exchangesLeft := CheckEmptyExchanges(ch); exchangesLeft != "" {
		t.Errorf("Found uncleared message exchanges:\n%v", exchangesLeft)
	}
}
