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
	"math/rand"
	"time"

	log "github.com/uber-common/bark"
	"github.com/uber/ringpop-go/logging"
	"github.com/uber/ringpop-go/util"
)

// discoverProviderHealer attempts to heal a ringpop partition by consulting
// the discoverProvider for nodes that are faulty or not available in its
// membership. It attempts the heal probabalisticly so that the discovery provider
// doesn't get overloaded with request -- with default settings 6
// times per minutes on avarage for the entire cluster.
type discoverProviderHealer struct {
	node *Node

	baseProbabillity float64
	period           time.Duration

	previousHostListSize int
	quit                 chan struct{}
	started              chan struct{}

	logger log.Logger
}

func newDiscoverProviderHealer(n *Node, baseProbability float64, period time.Duration) *discoverProviderHealer {
	return &discoverProviderHealer{
		node:             n,
		baseProbabillity: baseProbability,
		period:           period,
		logger:           logging.Logger("healer").WithField("local", n.Address()),
		started:          make(chan struct{}, 1),
		quit:             make(chan struct{}),
	}
}

// Start the partition healing loop
func (h *discoverProviderHealer) Start() {
	rand.Seed(time.Now().UnixNano())
	// check if started channel is already filled
	// if not, we start a new loop
	select {
	case h.started <- struct{}{}:
	default:
		return
	}

	go func() {
		for {
			// attempt heal with the pro
			if rand.Float64() < h.Probability() {
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

// Stop the partition healing loop.
func (h *discoverProviderHealer) Stop() {
	// if started, consume and send quit signal
	// if not started this is noop
	select {
	case <-h.started:
		h.quit <- struct{}{}
	default:
	}
}

// Probability returns the probability when a heal should be attempted
// we want to throttle the heal attempts to alleviate pressure on the
// discover provider.
func (h *discoverProviderHealer) Probability() float64 {
	// avoid division by zero.
	if h.previousHostListSize < h.node.CountMembers(ReachableMember) {
		h.previousHostListSize = h.node.CountMembers(ReachableMember)
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
func (h *discoverProviderHealer) Heal() ([]string, error) {
	h.node.emit(DiscoHealEvent{})
	// get list from discovery provider
	if h.node.discoverProvider == nil {
		return []string{}, errors.New("discoverProvider not available to healer")
	}

	hostList, err := h.node.discoverProvider.Hosts()
	if err != nil {
		h.logger.Warn("healer unable to receive host list from discover provider")
		return []string{}, err
	}

	h.previousHostListSize = len(hostList)

	// collect the targets this node might want to heal with
	var targets []string
	for _, address := range hostList {
		m, ok := h.node.memberlist.Member(address)
		if !ok || statePrecedence(m.Status) >= statePrecedence(Faulty) {
			targets = append(targets, address)
		}
	}
	util.ShuffleStringsInPlace(targets)

	// filter hosts that we already know about and attempt to heal nodes that
	// are complementary to the membership of this node.
	var ret []string
	failures := 0
	maxFailures := 10
	for len(targets) != 0 && failures < maxFailures {
		target := targets[0]
		targets = del(targets, target)

		// try to heal partition
		hostsOnOtherSide, err := AttemptHeal(h.node, target)

		if err != nil {
			h.logger.WithFields(log.Fields{
				"error":   err.Error(),
				"failure": failures,
			}).Warn("heal attempt failed (10 in total)")
			failures++
			continue
		}

		for _, host := range hostsOnOtherSide {
			targets = del(targets, host)
		}

		ret = append(ret, target)
	}

	if failures == maxFailures {
		h.logger.WithField("reachedNodes", len(ret)).Warn("healer reached max failures")
	}
	return ret, nil
}

// del returns a slice where all ocurences of s are filtered out. This modifies
// the original slice.
func del(strs []string, s string) []string {
	for i := 0; i < len(strs); i++ {
		if strs[i] != s {
			continue
		}
		strs[i] = strs[len(strs)-1]
		strs = strs[:len(strs)-1]
		i--
	}
	return strs
}
