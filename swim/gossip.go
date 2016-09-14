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
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/rcrowley/go-metrics"
	log "github.com/uber-common/bark"
	"github.com/uber/ringpop-go/logging"
)

// Gossip handles the protocol period of the SWIM protocol
type gossip struct {
	node *Node

	state struct {
		sync.RWMutex
		running bool

		// control channels for background tasks
		protocolPeriodStop  chan bool
		protocolPeriodWait  <-chan bool
		protocolRateChannel chan bool
	}

	minProtocolPeriod time.Duration

	protocol struct {
		numPeriods int
		lastPeriod time.Time
		lastRate   time.Duration
		timing     metrics.Histogram
		sync.RWMutex
	}

	logger log.Logger
}

// newGossip returns a new gossip SWIM sub-protocol with the given protocol period
func newGossip(node *Node, minProtocolPeriod time.Duration) *gossip {
	gossip := &gossip{
		node:              node,
		minProtocolPeriod: minProtocolPeriod,
		logger:            logging.Logger("gossip").WithField("local", node.Address()),
	}

	gossip.protocol.timing = metrics.NewHistogram(metrics.NewUniformSample(10))
	gossip.protocol.timing.Update(int64(gossip.minProtocolPeriod))

	return gossip
}

// computes a delay for the gossip protocol period
func (g *gossip) ComputeProtocolDelay() time.Duration {
	g.protocol.RLock()
	defer g.protocol.RUnlock()

	var delay time.Duration

	if g.protocol.numPeriods != 0 {
		target := g.protocol.lastPeriod.Add(g.protocol.lastRate)
		delay = time.Duration(math.Max(float64(target.Sub(time.Now())), float64(g.minProtocolPeriod)))
	} else {
		// delay for first tick in [0, minProtocolPeriod]ms
		delay = time.Duration(rand.Intn(int(g.minProtocolPeriod + 1)))
	}

	g.node.EmitEvent(ProtocolDelayComputeEvent{
		Duration: delay,
	})
	return delay
}

func (g *gossip) ProtocolRate() time.Duration {
	g.protocol.RLock()
	rate := g.protocol.lastRate
	g.protocol.RUnlock()

	return rate
}

// computes a ProtocolRate for the Gossip
func (g *gossip) AdjustProtocolRate() {
	g.protocol.Lock()
	observed := g.protocol.timing.Percentile(0.5) * 2.0
	g.protocol.lastRate = time.Duration(math.Max(observed, float64(g.minProtocolPeriod)))
	g.protocol.Unlock()
}

func (g *gossip) ProtocolTiming() metrics.Histogram {
	g.protocol.RLock()
	histogram := g.protocol.timing
	g.protocol.RUnlock()

	return histogram
}

// start the gossip protocol
func (g *gossip) Start() {
	g.state.Lock()
	defer g.state.Unlock()

	if g.state.running {
		g.logger.Warn("gossip already started")
		return
	}

	// mark the state to be running
	g.state.running = true

	// schedule repeat execution in the background
	g.state.protocolPeriodStop, g.state.protocolPeriodWait = scheduleRepeaditly(g.ProtocolPeriod, g.ComputeProtocolDelay, g.node.clock)
	g.state.protocolRateChannel, _ = scheduleRepeaditly(g.AdjustProtocolRate, func() time.Duration {
		return time.Second
	}, g.node.clock)

	g.logger.Debug("started gossip protocol")
}

// stop the gossip protocol
func (g *gossip) Stop() {
	g.state.Lock()
	defer g.state.Unlock()

	if !g.state.running {
		g.logger.Warn("gossip already stopped")
		return
	}

	g.state.running = false

	// stop background execution of running tasks
	close(g.state.protocolPeriodStop)
	close(g.state.protocolRateChannel)

	// wait for the goroutine to be stopped
	_ = <-g.state.protocolPeriodWait

	g.logger.Debug("stopped gossip protocol")
}

// returns whether or not the gossip sub-protocol is stopped
func (g *gossip) Stopped() bool {
	g.state.RLock()
	stopped := !g.state.running
	g.state.RUnlock()

	return stopped
}

// run a gossip protocol period
func (g *gossip) ProtocolPeriod() {
	startTime := time.Now()

	g.node.pingNextMember()
	endTime := time.Now()

	g.protocol.Lock()

	lag := endTime.Sub(g.protocol.lastPeriod)
	wasFirst := (g.protocol.numPeriods == 0)

	g.protocol.lastPeriod = endTime
	g.protocol.numPeriods++
	g.protocol.timing.Update(int64(time.Now().Sub(startTime)))
	g.protocol.Unlock()

	if !wasFirst {
		g.node.EmitEvent(ProtocolFrequencyEvent{
			Duration: lag,
		})
	}
}
