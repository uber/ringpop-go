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
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/stretchr/testify/assert"
	"github.com/uber/ringpop-go/discovery/statichosts"
	"github.com/uber/ringpop-go/events"
)

// TestPartitionHealWithFaulties creates two partitions A and B where A sees B
// as faulty and visa versa. This test asserts that two calls to heal the
// partition mechanism are needed, one to start the reincarnation phase, the
// other to merge the partitions to a single healed cluster.
func TestPartitionHealWithFaulties(t *testing.T) {
	A := newPartition(t, 5)
	B := newPartition(t, 5)
	defer destroyNodes(A...)
	defer destroyNodes(B...)

	A.AddPartitionWithStatus(B, Faulty)
	B.AddPartitionWithStatus(A, Faulty)

	A.ProgressTime(time.Millisecond * 3)
	B.ProgressTime(time.Millisecond * 5)

	A[0].node.discoverProvider = statichosts.New(append(A.Hosts(), B.Hosts()...)...)

	targets, err := A[0].node.healer.Heal()
	assert.NoError(t, err, "expected no error")
	assert.Len(t, targets, 1, "expected correct amount of targets")
	assert.True(t, B.Contains(targets[0]), "expected target to be a node from the right partition")

	waitForConvergence(t, time.Second, A...)
	waitForConvergence(t, time.Second, B...)

	A.HasPartitionAs(t, A, 3, "alive")
	A.HasPartitionAs(t, B, 0, "faulty")
	B.HasPartitionAs(t, B, 5, "alive")
	B.HasPartitionAs(t, A, 0, "faulty")

	targets, err = A[0].node.healer.Heal()
	assert.NoError(t, err, "expected no error")
	assert.Len(t, targets, 1, "expected correct amount of targets")
	assert.True(t, B.Contains(targets[0]), "expected target to be a node from the right partition")

	waitForPartitionHeal(t, time.Second, A, B)

	A.HasPartitionAs(t, A, 3, "alive")
	A.HasPartitionAs(t, B, 5, "alive")
	B.HasPartitionAs(t, B, 5, "alive")
	B.HasPartitionAs(t, A, 3, "alive")
}

// TestPartitionHealWithMissing creates two partitions A and B where A doesn't
// know about B and visa versa. This test asserts that one call to heal
// partition mechanism is needed because no nodes need to reincarnate.
func TestPartitionHealWithMissing(t *testing.T) {
	A := newPartition(t, 5)
	B := newPartition(t, 5)
	defer destroyNodes(A...)
	defer destroyNodes(B...)

	A[0].node.discoverProvider = statichosts.New(append(A.Hosts(), B.Hosts()...)...)

	targets, err := A[0].node.healer.Heal()
	assert.NoError(t, err, "expected no error")
	assert.Len(t, targets, 1, "expected correct amount of targets")
	assert.True(t, B.Contains(targets[0]), "expected target to be a node from the right partition")

	waitForPartitionHeal(t, time.Second, A, B)

	A.HasPartitionAs(t, A, 0, "alive")
	A.HasPartitionAs(t, B, 0, "alive")
	B.HasPartitionAs(t, B, 0, "alive")
	B.HasPartitionAs(t, A, 0, "alive")
}

// TestPartitionHealWithFaultyAndMissing1 creates two partitions A and B where
// A sees B as faulty and B doesn't know about A. This test asserts that two
// calls to heal the partition mechanism are needed, one to start the reincarnation
// phase, the other to merge the partitions to a single healed cluster.
func TestPartitionHealWithFaultyAndMissing1(t *testing.T) {
	A := newPartition(t, 5)
	B := newPartition(t, 5)
	defer destroyNodes(A...)
	defer destroyNodes(B...)

	A.AddPartitionWithStatus(B, Faulty)

	A.ProgressTime(time.Millisecond * 3)
	B.ProgressTime(time.Millisecond * 5)

	A[0].node.discoverProvider = statichosts.New(append(A.Hosts(), B.Hosts()...)...)

	targets, err := A[0].node.healer.Heal()
	assert.NoError(t, err, "expected no error")
	assert.Len(t, targets, 1, "expected correct amount of targets")
	assert.True(t, B.Contains(targets[0]), "expected target to be a node from the right partition")

	waitForConvergence(t, time.Second, A...)
	waitForConvergence(t, time.Second, B...)

	targets, err = A[0].node.healer.Heal()
	assert.NoError(t, err, "expected no error")
	assert.Len(t, targets, 1, "expected correct amount of targets")
	assert.True(t, B.Contains(targets[0]), "expected target to be a node from the right partition")

	waitForPartitionHeal(t, time.Second, A, B)
}

// TestPartitionHealWithFaultyAndMissing2 creates two partitions A and B where
// B sees A as faulty and A doesn't know about B. This test asserts that two
// calls to heal the partition mechanism are needed, one to start the reincarnation
// phase, the other to merge the partitions to a single healed cluster.
func TestPartitionHealWithFaultyAndMissing2(t *testing.T) {
	A := newPartition(t, 5)
	B := newPartition(t, 5)
	defer destroyNodes(A...)
	defer destroyNodes(B...)

	B.AddPartitionWithStatus(A, Faulty)

	A.ProgressTime(time.Millisecond * 3)
	B.ProgressTime(time.Millisecond * 5)

	A[0].node.discoverProvider = statichosts.New(append(A.Hosts(), B.Hosts()...)...)

	targets, err := A[0].node.healer.Heal()
	assert.NoError(t, err, "expected no error")
	assert.Len(t, targets, 1, "expected correct amount of targets")
	assert.True(t, B.Contains(targets[0]), "expected target to be a node from the right partition")

	waitForConvergence(t, time.Second, A...)
	waitForConvergence(t, time.Second, B...)

	targets, err = A[0].node.healer.Heal()
	assert.NoError(t, err, "expected no error")
	assert.Len(t, targets, 1, "expected correct amount of targets")
	assert.True(t, B.Contains(targets[0]), "expected target to be a node from the right partition")

	waitForPartitionHeal(t, time.Second, A, B)
}

// TestPartitionHealWithFaultyAndMissing2 creates two partitions A and B where
// A sees part of B as faulty and doesn't know about the other part and visa
// versa. This test asserts that two calls to heal the partition mechanism are
// needed, one to start the reincarnation phase, the other to merge the
// partitions to a single healed cluster.
func TestPartitionHealWithFaultyAndMissing3(t *testing.T) {
	A := newPartition(t, 6)
	A1 := partition(A[0:3])
	defer destroyNodes(A...)

	B := newPartition(t, 6)
	B1 := partition(B[0:3])
	defer destroyNodes(B...)

	A.ProgressTime(time.Millisecond * 3)
	B.ProgressTime(time.Millisecond * 5)

	A.AddPartitionWithStatus(B1, Faulty)
	B.AddPartitionWithStatus(A1, Faulty)

	waitForConvergence(t, time.Second, A...)
	waitForConvergence(t, time.Second, B...)

	A[0].node.discoverProvider = statichosts.New(append(A.Hosts(), B.Hosts()...)...)
	A[0].node.healer.Heal()
	waitForConvergence(t, time.Second, A...)
	waitForConvergence(t, time.Second, B...)

	A[0].node.healer.Heal()
	waitForPartitionHeal(t, time.Second, A, B)
}

// TestPartitionHealSemiPartition checks if the ring cluster automatically
// recovers when half partitioned. The bidirectional full sync mechanism
// is responsible to heal partitions of this form. We test for this because
// the partition healing mechanism could leave the cluster in this state
// when it completes only partially.
func TestPartitionHealSemiParition(t *testing.T) {
	A := newPartition(t, 5)
	B := newPartition(t, 5)
	defer destroyNodes(A...)
	defer destroyNodes(B...)

	A.AddPartitionWithStatus(B, Alive)

	waitForPartitionHeal(t, time.Second, A, B)
}

// TestPartitionHealWithMultiplePartitions checks if the heal mechanism heals
// a cluster that is partitioned into multiple partitions. A does not know
// about one of the five partitions. A is able to heal with that partition
// immediately because no reincarnation is required. The other four partitions
// do need two calls to the partition heal mechanism.
func TestPartitionHealWithMultiplePartitions(t *testing.T) {
	A := newPartition(t, 5)
	Bs := []partition{
		newPartition(t, 1),
		newPartition(t, 1),
		newPartition(t, 1),
		newPartition(t, 2),
		newPartition(t, 2),
	}

	A.AddPartitionWithStatus(Bs[0], Faulty)
	A.AddPartitionWithStatus(Bs[1], Faulty)
	A.AddPartitionWithStatus(Bs[2], Faulty)
	A.AddPartitionWithStatus(Bs[3], Faulty)
	// A doesn't know about Bs[4]

	A.ProgressTime(time.Millisecond * 3)
	for _, B := range Bs {
		B.ProgressTime(time.Millisecond * 5)
	}

	hosts := A.Hosts()
	for _, B := range Bs {
		hosts = append(hosts, B.Hosts()...)
	}
	A[0].node.discoverProvider = statichosts.New(hosts...)

	targets, err := A[0].node.healer.Heal()
	assert.NoError(t, err, "expected no error")
	assert.Len(t, targets, 5, "expected correct amount of targets")

	waitForConvergence(t, time.Second, A...)
	for _, B := range Bs {
		waitForConvergence(t, time.Second, B...)
	}
	waitForConvergence(t, time.Second, A...)
	for _, B := range Bs {
		waitForConvergence(t, time.Second, B...)
	}

	targets, err = A[0].node.healer.Heal()
	assert.NoError(t, err, "expected no error")
	assert.Len(t, targets, 4, "expected correct amount of targets")

	waitForPartitionHeal(t, time.Second, append(Bs, A)...)
}

// TestPartitionHealWithTime tests whether partitions are healed over time
// automatically.
func TestPartitionHealWithTime(t *testing.T) {
	A := newPartition(t, 5)
	B := newPartition(t, 5)
	defer destroyNodes(A...)
	defer destroyNodes(B...)

	A.AddPartitionWithStatus(B, Faulty)
	B.AddPartitionWithStatus(A, Faulty)

	A.ProgressTime(time.Millisecond)
	B.ProgressTime(time.Millisecond)

	A[0].node.discoverProvider = statichosts.New(append(A.Hosts(), B.Hosts()...)...)

	A[0].node.healer.Start()

	// Progress time in a background thread. This will cause the node to
	// attempt a heal periodically.

	c := clock.NewMock()
	A[0].node.clock = c
	go func() {
		for {
			c.Add(A[0].node.healer.period)
			time.Sleep(time.Millisecond)
		}
	}()

	waitForPartitionHeal(t, 3000*time.Millisecond, A, B)
}

// TestHealBeforeBootstrap tests that we can heal with a node that is still
// bootstrapping. We put a node in the bootstrapping phase by giving it a
// host-list that is impossible to join. When it is in this phase, we trigger
// a heal attempt from another node. The heal should succeed from the other
// node's perspective.
func TestHealBeforeBootstrap(t *testing.T) {
	a := newChannelNode(t)
	defer a.Destroy()

	b := newChannelNode(t)
	defer b.Destroy()

	// block a from completing bootstrap
	block := make(chan struct{})
	a.node.RegisterListener(on(JoinTriesUpdateEvent{}, func(e events.Event) {
		<-block
	}))

	discoProvider := statichosts.New(a.node.Address(), b.node.Address())

	// bootstrap a and wait for it to be in bootstrap phase
	go func() {
		a.node.Bootstrap(&BootstrapOptions{
			DiscoverProvider: discoProvider,
		})
	}()

	// bootstrap b then add a to its bootstrap provider
	bootstrapNodes(t, b)
	b.node.discoverProvider = discoProvider
	b.node.disseminator.ClearChanges()

	// check that a is not yet part of b's membership
	_, has := b.node.memberlist.Member(a.node.Address())
	assert.False(t, has, "expected that a is not yet part of b's membership")

	// start heal
	targets, err := b.node.healer.Heal()
	assert.NoError(t, err, "expected no error")
	assert.Len(t, targets, 0, "expected that b cannot completely heal with a")

	// a is now part of b's membership
	_, has = b.node.memberlist.Member(a.node.Address())
	assert.True(t, has, "expected that a is now part of b's membership")

	// make sure that a is still not bootstrapped, this causes a to be suspect to b
	DoThenWaitFor(func() {
		b.node.pingNextMember()
	}, a.node, RequestBeforeReadyEvent{})

	// allows a to bootstrap
	DoThenWaitFor(func() {
		close(block)
	}, a.node, JoinCompleteEvent{})

	// progress timers so that incarnation numbers can bump
	c := clock.NewMock()
	c.Add(time.Millisecond)
	a.node.clock = c

	waitForConvergence(t, time.Second, a, b)
}

func TestPartitionHealFail(t *testing.T) {
	A := newPartition(t, 2)
	defer destroyNodes(A...)
	A[0].node.discoverProvider = nil
	targets, err := A[0].node.healer.Heal()
	assert.Len(t, targets, 0, "expected error, no targets")
	assert.Error(t, err, "expected an error")
}

func TestPartitionHealFail2(t *testing.T) {
	A := newPartition(t, 2)
	defer destroyNodes(A...)
	A[0].node.discoverProvider = &BrokenDiscoverProvider{}
	targets, err := A[0].node.healer.Heal()
	assert.Len(t, targets, 0, "expected error, no targets")
	assert.Error(t, err, "expected an error")
}

type BrokenDiscoverProvider struct{}

func (p *BrokenDiscoverProvider) Hosts() ([]string, error) {
	return nil, errors.New("broken discover provider is broken")
}

func TestPartitionHealMaxFails(t *testing.T) {
	A := newPartition(t, 2)
	defer destroyNodes(A...)
	brokenHosts := make([]string, 20)
	for i := range brokenHosts {
		brokenHosts[i] = "invalid host" + fmt.Sprint(i)
	}

	A[0].node.discoverProvider = statichosts.New(brokenHosts...)

	heals := 0
	A[0].node.RegisterListener(on(AttemptHealEvent{}, func(e events.Event) {
		heals++
	}))

	targets, err := A[0].node.healer.Heal()
	assert.Len(t, targets, 0, "expected no nodes are reached during heal")
	assert.NoError(t, err, "expected no error")
	assert.Equal(t, 10, heals, "expected that healer stops after 10 failed heal attempts")
}

// partition is a slice of testNodes. It is used to manipulate the membership
// of all nodes at the same time. This makes it easy to create a partition
// where all the nodes of the partition have the same membership view.
type partition []*testNode

// newPartition creates and bootstraps n new nodes and let's them form a cluster.
func newPartition(t *testing.T, n int) partition {
	p := partition(genChannelNodes(t, n))
	bootstrapNodes(t, p...)
	waitForConvergence(t, 500*time.Millisecond, p...)
	p.ClearChanges()
	return p
}

// AddPartitionWithStatus adds B to the membership of all nodes in this
// partition, with the specified status.
// e.g. after A.AddPartitionWithStatus(B, Faulty), every node of A will
// now have all nodes of B in its membership as Faulty.
func (p partition) AddPartitionWithStatus(B partition, status string) {
	for _, n := range p {
		for _, n2 := range B {
			n.node.memberlist.MakeChange(n2.node.Address(), n2.node.Incarnation(), status)
		}
	}

	p.ClearChanges()
}

// ClearChanges clears all changes from the disseminators for every node in the
// partition.
func (p partition) ClearChanges() {
	for _, n := range p {
		n.node.disseminator.ClearChanges()
	}
}

// Progresses the clock of all the nodes of the partition by T.
func (p partition) ProgressTime(T time.Duration) {
	for _, n := range p {
		now := n.node.clock.Now()
		c := clock.NewMock()
		c.Add(now.Add(T).Sub(clock.NewMock().Now()))
		n.node.clock = c
	}
}

// HasPartitionAs checks that every node from A contains every node from B in
// its membership. It also checks that the members from B have the correct
// incarnation number and status.
func (p partition) HasPartitionAs(t *testing.T, B partition, incarnation int64, status string) {
	for _, a := range p {
		for _, b := range B {
			mem, ok := a.node.memberlist.Member(b.node.Address())
			assert.True(t, ok, "expected members to contain member")
			assert.Equal(t, incarnation, mem.Incarnation, "expected correct incarnation number")
			assert.Equal(t, status, mem.Status, "expected correct status")
		}
	}
}

// Hosts the string slice of addresses for all the nodes in the partition.
func (p partition) Hosts() []string {
	var res []string
	for _, a := range p {
		res = append(res, a.node.Address())
	}
	return res
}

// Contains tells whether a node with the specified address is part of the
// partition.
func (p partition) Contains(address string) bool {
	for _, a := range p {
		if a.node.Address() == address {
			return true
		}
	}
	return false
}

// waitForPartitionHeal lets the nodes of all partitions gossip and returns
// when the nodes are converged. After the cluster finished gossiping we
// double check that all nodes have the same checksum for the memberlist,
// this means that the cluster is converged and healed.
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

		// continue until all members are reachable
		for _, node := range nodes {
			if node.memberlist.CountMembers(ReachableMember) != len(nodes) {
				continue Tick
			}
		}

		// return when converged
		if nodesConverged(nodes) {
			return
		}
	}
}
