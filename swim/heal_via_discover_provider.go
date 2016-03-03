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
	"math/rand"
	"time"

	"github.com/benbjohnson/clock"
	"github.com/uber/ringpop-go/discovery"
)

// discoverProviderHealer attempts to heal a ringpop partition by consulting
// the discoverProvider for nodes that are faulty or not available in its
// membership. It attempts the heal probabalisticly so that the discovery provider
// doesn't get overloaded with request -- with default settings 6
// times per minutes on avarage for the entire cluster.
type discoverProviderHealer struct {
	node *Node

	discoverProvider *discovery.DiscoverProvider
	baseProbabillity float64
	period           time.Duration

	previousHostListSize int
	quit                 chan bool
}

func newDiscoverProviderHealer(n *Node, baseProbability float64, period time.Duration) *discoverProviderHealer {
	return &discoverProviderHealer{
		node:             n,
		baseProbabillity: baseProbability,
		period:           period,
		quit:             make(chan bool, 1),
	}
}

// Start the partition healing loop
func (h *discoverProviderHealer) Start() {
	go func() {
		for {
			// attempt heal with the pro
			if rand.Float64() < h.Probability() {
				fmt.Println(h.Probability(), h.node.clock.Now().Sub(clock.NewMock().Now()))
				h.Heal()
			}

			// loop or quit
			select {
			case <-h.node.clock.After(h.period):
			case <-h.quit:
				return
			}
		}
	}()
}

// Stop the partition healing loop
func (h *discoverProviderHealer) Stop() {
	h.quit <- true
}

// Probability returns the probability when a heal should be attempted
// we want to throttle the heal attempts to alleviate pressure on the
// discover provider.
func (h *discoverProviderHealer) Probability() float64 {
	// avoid division by zero.
	if h.previousHostListSize < h.node.CountReachableMembers() {
		h.previousHostListSize = h.node.CountReachableMembers()
	}
	if h.previousHostListSize < 1 {
		h.previousHostListSize = 1
	}
	return h.baseProbabillity / float64(h.previousHostListSize)
}

// Heal iterates over the hostList that the discoverProvider provider. If the
// node encounters a host that is faulty or not in the membership, we pick that
// node as a target to perform a partition heal with.
func (h *discoverProviderHealer) Heal() {
	h.node.emit(&AttemptHealEvent{})

	// get list from discovery provider
	if h.node.discoverProvider == nil {
		return
	}
	hostList, err := h.node.discoverProvider.Hosts()
	if err != nil {
		// log discover provider failure
		return
	}

	h.previousHostListSize = len(hostList)

	// filter hosts that we already know about and attempt to heal nodes that
	// are complementary to the membership of this node.
	for _, target := range hostList {
		m, ok := h.node.memberlist.Member(target)
		if ok && m.Status != Faulty {
			continue
		}

		// try to heal partition
		fmt.Println("REALLY HEAL")
		err = AttemptHeal(h.node, target)
		if err != nil {
			// TODO(wieger): log err
		}
		break
	}
}
