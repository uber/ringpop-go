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
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber/ringpop-go/shared"
	"github.com/uber/ringpop-go/util"
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

type testNode struct {
	node    *Node
	channel *tchannel.Channel
}

func (n *testNode) Destroy() {
	n.node.Destroy()
	n.channel.Close()
}

func (n *testNode) closeAndWait(sender *tchannel.Channel) {
	n.channel.Close()

	// wait for tchannel to respond with connection refused errors.
	for {
		ctx, cancel := shared.NewTChannelContext(time.Millisecond * 50)

		err := sender.Ping(ctx, n.node.Address())
		if err == nil {
			cancel()
			continue
		}

		_, ok := err.(tchannel.SystemError)
		if ok {
			cancel()
			continue
		}

		cancel()
		break
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
	node := NewNode("test", hostport, ch.GetSubChannel("test"), &Options{
		Clock: clock.NewMock(),
	})

	return &testNode{node, ch}
}

// newChannelNodeWithHostPort creates a testNode with the address specified by
// the hostport parameter.
func newChannelNodeWithHostPort(t *testing.T, hostport string) *testNode {
	ch, err := tchannel.NewChannel("test", nil)
	require.NoError(t, err, "channel must create successfully")

	node := NewNode("test", hostport, ch.GetSubChannel("test"), &Options{
		Clock: clock.NewMock(),
	})

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

func bootstrapNodes(t *testing.T, testNodes ...*testNode) []string {
	var hostports []string

	for _, tn := range testNodes {
		hostports = append(hostports, tn.node.Address())

		_, err := tn.node.Bootstrap(&BootstrapOptions{
			Hosts:   hostports,
			Stopped: true,
		})
		require.NoError(t, err, "node must bootstrap successfully")
	}
	return hostports
}

func waitForConvergence(t *testing.T, timeout time.Duration, testNodes ...*testNode) {
	timeoutCh := time.After(timeout)

	nodes := testNodesToNodes(testNodes)

	// To get the cluster to a converged state we will let the nodes gossip until
	// there are no more changes. After the cluster finished gossipping we double
	// check that all nodes have the same checksum for the memberlist, this means
	// that the cluster is converged.
Tick:
	for {
		select {
		case <-timeoutCh:
			t.Errorf("timeout during wait for convergence")
			return
		default:
			for _, node := range nodes {
				node.gossip.ProtocolPeriod()
			}
			for _, node := range nodes {
				if node.HasChanges() {
					continue Tick
				}
			}

			if !nodesConverged(nodes) {
				t.Errorf("nodes did not converge to 1 checksum")
			}

			return
		}
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

// swimCluster is a group of real swim nodes listening on randomly-assigned
// ports.
type swimCluster struct {
	nodes    []*Node
	channels []*tchannel.Channel
}

// newSwimCluster creates a new swimCluster with the number of nodes specified
// by size. These nodes are not joined together at creation. Use Bootstrap if you
// need a bootstrapped cluster.
func newSwimCluster(size int) *swimCluster {
	var nodes []*Node
	var channels []*tchannel.Channel
	for i := 0; i < size; i++ {
		ch, err := tchannel.NewChannel("test", nil)
		if err != nil {
			panic(err)
		}

		if err := ch.ListenAndServe("127.0.0.1:0"); err != nil {
			panic(err)
		}
		channels = append(channels, ch)

		hostport := ch.PeerInfo().HostPort
		node := NewNode("test", hostport, ch.GetSubChannel("test"), nil)

		nodes = append(nodes, node)
	}
	return &swimCluster{nodes: nodes, channels: channels}
}

// Add adds the specified node to the cluster and bootstraps it so that it is
// joined to the existing nodes.
func (c *swimCluster) Add(n *Node) {
	n.Bootstrap(&BootstrapOptions{
		DiscoverProvider: &StaticHostList{c.Addresses()},
	})
	c.nodes = append(c.nodes, n)
}

// Addresses returns a slice of addresses of all nodes in the cluster.
func (c *swimCluster) Addresses() (hostports []string) {
	for _, node := range c.nodes {
		hostports = append(hostports, node.Address())
	}
	return
}

// Bootstrap joins all the nodes in this cluster together using Bootstrap calls.
func (c *swimCluster) Bootstrap() {
	for _, node := range c.nodes {
		node.Bootstrap(&BootstrapOptions{
			DiscoverProvider: &StaticHostList{c.Addresses()},
		})
	}
}

// WaitForConvergence polls the checksums of each node in the cluster and returns when they all match, or until timeout occurs.
func (c *swimCluster) WaitForConvergence(timeout time.Duration) error {
	timeoutCh := time.After(timeout)

	for {
		select {
		case <-time.After(time.Millisecond):
			if nodesConverged(c.nodes) {
				return nil
			}
		case <-timeoutCh:
			return errors.New("timeout during converging")
		}
	}
}

func testNodesToNodes(testNodes []*testNode) []*Node {
	nodes := make([]*Node, len(testNodes))
	for i, tn := range testNodes {
		nodes[i] = tn.node
	}
	return nodes
}

func nodesConverged(nodes []*Node) bool {
	if len(nodes) == 0 {
		return true
	}

	first := nodes[0].memberlist.Checksum()
	for _, node := range nodes {
		if node.memberlist.Checksum() != first {
			return false
		}
	}

	return true
}

// Destroy destroys all nodes in this cluster. It also closes channels, but
// only those that were created as part of newSwimCluster.
func (c *swimCluster) Destroy() {
	for _, node := range c.nodes {
		node.Destroy()
	}
	for _, ch := range c.channels {
		ch.Close()
	}
}

// Nodes returns a slice of all nodes in the cluster.
func (c *swimCluster) Nodes() []*Node {
	return c.nodes
}
