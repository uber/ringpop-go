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
	"fmt"
	"log"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber/ringpop-go/swim/util"
	"github.com/uber/tchannel-go"
)

var testNow = time.Now()
var testInc = util.TimeNowMS()

var testSuspect = Change{
	Address:           "127.0.0.1:3002",
	Incarnation:       testInc,
	Source:            "127.0.0.1:3001",
	SourceIncarnation: testInc,
	Status:            Suspect,
	Timestamp:         util.Timestamp(testNow),
}

type dummyIter struct{}

func (dummyIter) Next() (*Member, bool) {
	return &Member{
		Address:     "127.0.0.1:3010",
		Status:      Alive,
		Incarnation: testInc,
	}, true
}

type lError struct {
	err error
	sync.Mutex
}

func (e *lError) Set(err error) {
	e.Lock()
	e.err = err
	e.Unlock()
}

func (e *lError) Err() error {
	e.Lock()
	err := e.err
	e.Unlock()
	return err
}

type testNode struct {
	node    *Node
	channel *tchannel.Channel
}

func (n *testNode) Destroy() {
	n.node.Destroy()
	n.channel.Close()

	for n.channel.State() != tchannel.ChannelClosed {
		time.Sleep(500 * time.Millisecond)
		log.Print("STATE: ", n.channel.State().String())
	}
}

// newChannelNode creates a testNode with a listening channel and associated
// SWIM node. The channel listens on a random port assigned by the OS.
func newChannelNode(t *testing.T) *testNode {
	ch, err := tchannel.NewChannel("test", nil)
	require.NoError(t, err, "channel must create successfully")

	// Set the channel listening so it binds to the socket and we get a port
	// allocated by the OS
	err = ch.ListenAndServe("127.0.0.1:0")
	require.NoError(t, err, "channel must listen")

	hostport := ch.PeerInfo().HostPort
	node := NewNode("test", hostport, ch.GetSubChannel("test"), nil)

	return &testNode{node, ch}
}

// newChannelNodeWithHostPort creates a testNode with the address specified by
// the hostport parameter.
func newChannelNodeWithHostPort(t *testing.T, hostport string) *testNode {
	ch, err := tchannel.NewChannel("test", nil)
	require.NoError(t, err, "channel must create successfully")

	node := NewNode("test", hostport, ch.GetSubChannel("test"), nil)

	return &testNode{node, ch}
}

func genChannelNodes(t *testing.T, n int) (nodes []*testNode) {
	for i := 0; i < n; i++ {
		node := newChannelNode(t)
		nodes = append(nodes, node)
	}

	return
}

func memberlistHasMembers(t *testing.T, m *memberlist, members []Member) {
	for _, expected := range members {
		member, ok := m.Member(expected.Address)
		require.NotNil(t, member, "member cannot be nil")
		assert.True(t, ok, "expected member to be in memberlist")
		assert.Equal(t, expected.Status, member.Status, "expected statuses to be the same")
	}
}

func bootstrapNodes(t *testing.T, testNodes ...*testNode) {
	var hostports []string

	for _, tn := range testNodes {
		hostports = append(hostports, tn.node.Address())

		_, err := tn.node.Bootstrap(&BootstrapOptions{
			Hosts:   hostports,
			Stopped: true,
		})
		require.NoError(t, err, "node must bootstrap successfully")
	}
}

func destroyNodes(tnodes ...*testNode) {
	for _, tnode := range tnodes {
		tnode.Destroy()
	}
}

// fakeAddresses returns a slice of fake IP address/port combinations that can
// be used during testing. Note that these addresses cannot be used in tests
// which require real communication or binding to a socket.
//
// The IP addresses returned are from the TEST-NET-1 block which are defined in
// RFC 5737 as to be used for documentation and recommended to be unrouteable.
// In practice, these addresses may also just time out.
//
// See:
// http://tools.ietf.org/html/rfc5737
// http://stackoverflow.com/questions/10456044/what-is-a-good-invalid-ip-address-to-use-for-unit-tests
//
func fakeHostPorts(fromHost, toHost, fromPort, toPort int) []string {
	var hostports []string
	for h := fromHost; h <= toHost; h++ {
		for p := fromPort; p <= toPort; p++ {
			hostports = append(hostports, fmt.Sprintf("192.0.2.%v:%v", h, p))
		}
	}
	return hostports
}
