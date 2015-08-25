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
	"fmt"
	"sync"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/uber/ringpop-go/swim"
	"github.com/uber/tchannel/golang"
)

// fake stats
type dummmyStats struct {
	vals map[string]int64
}

func newDummyStats() *dummmyStats {
	return &dummmyStats{make(map[string]int64)}
}

func (d *dummmyStats) Incr(key string, val int64) error {
	d.vals[key] += val

	return nil
}

func (d *dummmyStats) Gauge(key string, val int64) error {
	d.vals[key] += val

	return nil
}

func (d *dummmyStats) Timing(key string, val int64) error {
	d.vals[key] += val

	return nil
}

// fake event listener
type dummyListener struct {
	l      sync.Mutex
	events int
}

func (d *dummyListener) EventCount() int {
	d.l.Lock()
	defer d.l.Unlock()

	return d.events
}

func (d *dummyListener) HandleEvent(event interface{}) {
	d.l.Lock()
	d.events++
	d.l.Unlock()
}

func testPop(t *testing.T, hostport string) (*Ringpop, func()) {
	ch, err := tchannel.NewChannel("test-app", nil)
	require.NoError(t, err, "cannot have error when creating channel")

	ringpop := NewRingpop("test-app", hostport, ch, nil)

	destroy := func() {
		ringpop.Destroy()
		ch.Close()
	}

	return ringpop, destroy
}

func genAddresses(host, fromPort, toPort int) []string {
	var addresses []string

	for i := fromPort; i <= toPort; i++ {
		addresses = append(addresses, fmt.Sprintf("127.0.0.%v:%v", host, 3000+i))
	}

	return addresses
}

func genChanges(addresses []string, status string) []swim.Change {
	var changes []swim.Change

	for _, address := range addresses {
		changes = append(changes, swim.Change{
			Address: address,
			Status:  status,
		})
	}

	return changes
}
