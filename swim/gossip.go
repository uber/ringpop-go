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
)

// Gossip handles the protocol period of the SWIM protocol
type gossip struct {
	node *Node

	state struct {
		stopped bool
		sync.RWMutex
	}

	minProtocolPeriod time.Duration

	protocol struct {
		numPeriods int
		lastPeriod time.Time
		lastRate   time.Duration
		timing     metrics.Histogram
		sync.RWMutex
	}
}

// newGossip returns a new gossip SWIM sub-protocol with the given protocol period
func newGossip(node *Node, minProtocolPeriod time.Duration) *gossip {
	gossip := &gossip{
		node:              node,
		minProtocolPeriod: minProtocolPeriod,
	}

	gossip.SetStopped(true)
	gossip.protocol.timing = metrics.NewHistogram(metrics.NewUniformSample(10))
	gossip.protocol.timing.Update(int64(gossip.minProtocolPeriod))

	return gossip
}

// returns whether or not the gossip sub-protocol is stopped
func (g *gossip) Stopped() bool {
	g.state.RLock()
	stopped := g.state.stopped
	g.state.RUnlock()

	return stopped
}

// sets the gossip sub-protocol to stopped or not stopped
func (g *gossip) SetStopped(stopped bool) {
	g.state.Lock()
	g.state.stopped = stopped
	g.state.Unlock()
}

// computes a delay for the gossip protocol period
func (g *gossip) ComputeProtocolDelay() time.Duration {
	g.protocol.RLock()
	defer g.protocol.RUnlock()

	if g.protocol.numPeriods != 0 {
		target := g.protocol.lastPeriod.Add(g.protocol.lastRate)
		delay := math.Max(float64(target.Sub(time.Now())), float64(g.minProtocolPeriod))
		return time.Duration(delay)
	}
	// delay for first tick in [0, minProtocolPeriod]ms
	return time.Duration(rand.Intn(int(g.minProtocolPeriod + 1)))
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
	if !g.Stopped() {
		g.node.log.Warn("gossip already started")
		return
	}

	g.SetStopped(false)
	g.RunProtocolRateLoop()
	g.RunProtocolPeriodLoop()

	g.node.log.Debug("started gossip protocol")
}

// stop the gossip protocol
func (g *gossip) Stop() {
	if g.Stopped() {
		g.node.log.Warn("gossip already stopped")
		return
	}

	g.SetStopped(true)
	g.node.log.Debug("stopped gossip protocol")
}

// run the gossip protocol period loop
func (g *gossip) RunProtocolPeriodLoop() {
	go func() {
		startTime := time.Now()
		for !g.Stopped() {
			delay := g.ComputeProtocolDelay()
			g.node.emit(ProtocolDelayComputeEvent{
				Duration: delay,
			})

			startTimeFreq := time.Now()

			g.ProtocolPeriod()
			time.Sleep(delay)

			g.node.emit(ProtocolFrequencyEvent{
				Duration: time.Now().Sub(startTimeFreq),
			})
		}

		g.node.log.WithFields(log.Fields{
			"start": startTime,
			"end":   time.Now(),
		}).Debug("stopped protocol period loop")
	}()
}

// run a gossip protocol period
func (g *gossip) ProtocolPeriod() {
	startTime := time.Now()

	g.node.pingNextMember()

	g.protocol.Lock()
	g.protocol.lastPeriod = time.Now()
	g.protocol.numPeriods++
	g.protocol.timing.Update(int64(time.Now().Sub(startTime)))
	g.protocol.Unlock()
}

//
func (g *gossip) RunProtocolRateLoop() {
	go func() {
		for !g.Stopped() {
			time.Sleep(time.Second)
			g.AdjustProtocolRate()
		}
	}()
}
