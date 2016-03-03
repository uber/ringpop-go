// Copyright (c) 2016 Uber Technologies, Inc.
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
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/stretchr/testify/suite"
	"github.com/uber/ringpop-go/discovery/statichosts"
	"github.com/uber/ringpop-go/events"
	"github.com/uber/ringpop-go/util"
)

type PartitionHealTestSuite struct {
	suite.Suite
	tnode       *testNode
	node        *Node
	peers       []*testNode
	incarnation int64
}

func TestPartitionHealTestSuite(t *testing.T) {
	suite.Run(t, new(PartitionHealTestSuite))
}

func (s *PartitionHealTestSuite) SetupTest() {
	s.incarnation = util.TimeNowMS()
	s.tnode = newChannelNode(s.T())
	s.node = s.tnode.node
	s.peers = genChannelNodes(s.T(), 10)
}

func (s *PartitionHealTestSuite) TearDownTest() {
	destroyNodes(s.peers...)
	destroyNodes(s.tnode)
}

func (s *PartitionHealTestSuite) TestPartitionHeal() {
	startTime := time.Now()

	p1 := newPartition(s.T(), 5)
	p2 := newPartition(s.T(), 5)
	defer destroyNodes(p1...)
	defer destroyNodes(p2...)

	p1.AddPartitionWithStatus(p2, Faulty)
	p2.AddPartitionWithStatus(p1, Faulty)

	for _, n := range p1 {
		n.node.disseminator.ClearChanges()
		c := clock.NewMock()
		n.node.clock = c
		c.Add(time.Millisecond)
	}
	for _, n := range p2 {
		n.node.disseminator.ClearChanges()
		c := clock.NewMock()
		n.node.clock = c
		c.Add(time.Millisecond)
	}

	h1, err := p1[0].node.discoverProvider.Hosts()
	s.NoError(err, "expected discover provider to work")
	h2, err := p2[0].node.discoverProvider.Hosts()
	s.NoError(err, "expected discover provider to work")

	p1[0].node.discoverProvider = statichosts.New(append(h1, h2...)...)

	p1[0].node.RegisterListener(onAttemptHealEvent(func() {
		fmt.Println("ATTEMPT")
	}))

	p1[0].node.healer.Start()

	go func() {
		c := p1[0].node.clock.(*clock.Mock)
		for {
			c.Add(time.Second)
		}
	}()

	// err = AttemptHeal(p1[0].node, p2[0].node.Address())
	// s.NoError(err, "expected that the partition healed on it's own")

	waitForPartitionHeal(s.T(), 3000*time.Millisecond, p1, p2)
	fmt.Println(time.Now().Sub(startTime))
	fmt.Println(p1[0].node.clock.Now().Sub(clock.NewMock().Now()))
	time.Sleep(time.Second)
}

type partition []*testNode

func newPartition(t *testing.T, n int) partition {
	p := partition(genChannelNodes(t, n))
	bootstrapNodes(t, p...)
	waitForConvergence(t, 500*time.Millisecond, p...)
	return p
}

// onPingRequestReceive can be registered on an EventListener and fires f only
// when the PingRequest handler receives a request.
func onAttemptHealEvent(f func()) ListenerFunc {
	return ListenerFunc(func(e events.Event) {
		switch e.(type) {
		case AttemptHealEvent:
			f()
		}
	})
}

func (p partition) AddPartitionWithStatus(p2 partition, status string) {
	for _, n := range p {
		for _, n2 := range p2 {
			n.node.memberlist.MakeChange(n2.node.Address(), n2.node.Incarnation(), status)
		}
	}
}

// waitForPartitionHeal is same as waitForConvergence but unlike that function,
// waitForPatitionHeal doesn't stop when no dissemination is detected.
func waitForPartitionHeal(t *testing.T, timeout time.Duration, ps ...partition) {
	var nodes []*Node
	for _, p := range ps {
		nodes = append(nodes, testNodesToNodes(p)...)
	}

	deadline := time.Now().Add(timeout)

Tick:
	for {
		// check deadline
		if time.Now().After(deadline) {
			t.Errorf("timeout during wait for convergence")
			return
		}

		// let nodes gossip
		for _, node := range nodes {
			node.gossip.ProtocolPeriod()
		}

		// continue until there are no more changes
		for _, node := range nodes {
			if node.HasChanges() {
				continue Tick
			}
		}

		// return when converged
		if nodesConverged(nodes) {
			return
		}
	}

}
