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

func newChannelNode(t *testing.T, address string) *testNode {
	ch, err := tchannel.NewChannel("test", nil)
	require.NoError(t, err, "channel must create successfully")

	node := NewNode("test", address, ch.GetSubChannel("test"), nil)

	require.NoError(t, ch.ListenAndServe(node.Address()), "channel must listen")

	return &testNode{node, ch}
}

func genChannelNodes(t *testing.T, hostports []string) (nodes []*testNode) {
	for _, hostport := range hostports {
		node := newChannelNode(t, hostport)
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

func genAddresses(host, fromPort, toPort int) []string {
	var addresses []string

	for i := fromPort; i <= toPort; i++ {
		addresses = append(addresses, fmt.Sprintf("127.0.0.%v:%v", host, 3000+i))
	}

	return addresses
}

func genAddressesDiffHosts(fromPort, toPort int) []string {
	var hosts []string
	for i := fromPort; i <= toPort; i++ {
		hosts = append(hosts, fmt.Sprintf("127.0.0.%v:3001", i))
	}

	return hosts
}
