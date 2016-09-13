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
	"reflect"
	"sync"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uber/ringpop-go/discovery/statichosts"
	"github.com/uber/ringpop-go/events"
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
	clock   *clock.Mock
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
	c := clock.NewMock()
	node := NewNode("test", hostport, ch.GetSubChannel("test"), &Options{
		Clock: c,
	})

	return &testNode{node, ch, c}
}

// newChannelNodeWithHostPort creates a testNode with the address specified by
// the hostport parameter.
func newChannelNodeWithHostPort(t *testing.T, hostport string) *testNode {
	ch, err := tchannel.NewChannel("test", nil)
	require.NoError(t, err, "channel must create successfully")

	c := clock.NewMock()
	node := NewNode("test", hostport, ch.GetSubChannel("test"), &Options{
		Clock: c,
	})

	return &testNode{node, ch, c}
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
		assert.True(t, ok, "expected member to be in memberlist")
		require.NotNil(t, member, "member cannot be nil")
		assert.Equal(t, expected.Status, member.Status, "expected statuses to be the same")
	}
}

func bootstrapNodes(t *testing.T, testNodes ...*testNode) []string {
	var hostports []string

	for _, tn := range testNodes {
		hostports = append(hostports, tn.node.Address())

		_, err := tn.node.Bootstrap(&BootstrapOptions{
			DiscoverProvider: statichosts.New(hostports...),
			Stopped:          true,
		})
		require.NoError(t, err, "node must bootstrap successfully")
	}
	return hostports
}

// waitForConvergence lets the nodes gossip and returns when the nodes are converged.
// After the cluster finished gossiping we double check that all nodes have the
// same checksum for the memberlist, this means that the cluster is converged.
func waitForConvergence(t *testing.T, maxIterations int, testNodes ...*testNode) {
	waitForConvergenceNodes(t, maxIterations, testNodesToNodes(testNodes)...)
}

func waitForConvergenceNodes(t *testing.T, maxIterations int, nodes ...*Node) {
	// 1 node is a special case because it can't drain its changes since it
	// cannot ping or be pinged by another member
	if len(nodes) == 1 {
		nodes[0].disseminator.ClearChanges()
		return
	}

	// mark which nodes were gossiping and stop them from gossiping
	wasGossiping := make([]bool, len(nodes))
	for i, node := range nodes {
		wasGossiping[i] = !node.gossip.Stopped()
		if wasGossiping[i] {
			node.gossip.Stop()
		}
	}

	// after the forceful convergence start all nodes that were gossiping before
	defer func() {
		for i, shouldStart := range wasGossiping {
			if shouldStart {
				nodes[i].gossip.Start()
			}
		}
	}()

Tick:
	maxIterations--
	// return when deadline is reached
	if maxIterations < 0 {
		t.Errorf("exhausted the maximum number of iterations while waiting for convergence")
		return
	}

	// tick all the nodes
	for _, node := range nodes {
		node.gossip.ProtocolPeriod()
	}

	// repeat if we stil have changes
	for _, node := range nodes {
		if node.HasChanges() {
			goto Tick
		}
	}

	// check for convergence if there are no changes left
	if !nodesConverged(nodes) {
		t.Errorf("nodes did not converge to 1 checksum")
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
		DiscoverProvider: statichosts.New(c.Addresses()...),
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
			DiscoverProvider: statichosts.New(c.Addresses()...),
		})
	}
}

// WaitForConvergence polls the checksums of each node in the cluster and returns when they all match, or until timeout occurs.
func (c *swimCluster) WaitForConvergence(t *testing.T, maxIterations int) {
	waitForConvergenceNodes(t, maxIterations, c.nodes...)
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

// DoThenWaitFor executes a function and then waits for a specific type
// of event to occur. This function shouldn't be used outside tests because
// there is no way to unsubscribe the event handler.
//
// Often we want to execute some code and then wait for an event be emitted due
// to the code being executed. However in order to not miss the event we must
// first register an event handler before we can execute the code:
// - register listener that signals we can continue on receiving the correct event;
// - execute the code that will lead to an event being emitted;
// - wait for the continue signal.
//
// This can be quite hard to follow. Ideally we want it to look like
// - execute the code that will lead to an event being emitted;
// - and then wait for a specific event.
//
// This function helps with making the code read like the latter.
func DoThenWaitFor(f func(), er events.EventEmitter, t interface{}) {
	block := make(chan struct{}, 1)

	var once sync.Once

	er.AddListener(on(t, func(e events.Event) {
		once.Do(func() {
			block <- struct{}{}
		})
	}))

	f()

	// wait for first occurence of the event, close thereafter
	<-block
	close(block)
}

// The ListenerFunc type is an adapter to allow the use of ordinary functions
// as EventListeners.
type ListenerFunc struct {
	fn func(events.Event)
}

// HandleEvent calls f(e).
func (f *ListenerFunc) HandleEvent(e events.Event) {
	f.fn(e)
}

// on is returns an EventListener that executes a function f upon receiving an
// event of type t.
func on(t interface{}, f func(e events.Event)) *ListenerFunc {
	return &ListenerFunc{
		fn: func(e events.Event) {
			if reflect.TypeOf(t) == reflect.TypeOf(e) {
				f(e)
			}
		},
	}
}
