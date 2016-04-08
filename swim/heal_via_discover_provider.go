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
	"math/rand"
	"time"

	log "github.com/uber-common/bark"
	"github.com/uber/ringpop-go/discovery"
	"github.com/uber/ringpop-go/logging"
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

	logger log.Logger
}

func newDiscoverProviderHealer(n *Node, baseProbability float64, period time.Duration) *discoverProviderHealer {
	return &discoverProviderHealer{
		node:             n,
		baseProbabillity: baseProbability,
		period:           period,
		quit:             make(chan bool, 1),
		logger:           logging.Logger("healer").WithField("local", n.Address()),
	}
}

// Start the partition healing loop
func (h *discoverProviderHealer) Start() {
	go func() {
		for {
			// attempt heal with the pro
			p := rand.Float64()
			P := h.Probability()
			if p < P {
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

// Heal iterates over the hostList that the discoverProvider provides. If the
// node encounters a host that is faulty or not in the membership, we pick that
// node as a target to perform a partition heal with.
//
// If heal was attempted, returns identities of the target nodes.
func (h *discoverProviderHealer) Heal() []string {
	h.node.emit(DiscoHealEvent{})
	// get list from discovery provider
	if h.node.discoverProvider == nil {
		return []string{}
	}
	hostList, err := h.node.discoverProvider.Hosts()
	if err != nil {
		h.logger.Warn("unable to receive host list from discover provider")
		return []string{}
	}

	h.previousHostListSize = len(hostList)

	var targets []string
	for _, address := range hostList {
		m, ok := h.node.memberlist.Member(address)
		if !ok || m.Status == Faulty {
			targets = append(targets, address)
		}
	}

	// filter hosts that we already know about and attempt to heal nodes that
	// are complementary to the membership of this node.
	var ret []string

	for len(targets) != 0 {
		target := targets[0]
		targets = del(targets, target)

		// try to heal partition
		hostsOnOtherSide, err := AttemptHeal(h.node, target)
		if err != nil {
			h.logger.WithField("error", err.Error()).Warn("heal failed")
			continue
		}

		for _, host := range hostsOnOtherSide {
			targets = del(targets, host)
		}

		ret = append(ret, target)
	}

	return ret
}
