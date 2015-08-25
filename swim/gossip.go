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

	log "github.com/Sirupsen/logrus"
	"github.com/rcrowley/go-metrics"
)

const defaultMinProtocolPeriod = 200 * time.Millisecond

// Gossip handles the protocol period of the SWIM protocol
type gossip struct {
	node *Node

	stopped bool

	minProtocolPeriod  time.Duration
	numProtocolPeriods int
	lastProtocolPeriod time.Time
	lastProtocolRate   time.Duration

	protocolTiming metrics.Histogram

	l sync.RWMutex // protects stopped
}

// newGossip returns a new gossip SWIM sub-protocol with the given protocol period
func newGossip(node *Node, minProtocolPeriod time.Duration) *gossip {
	gossip := &gossip{
		node:              node,
		stopped:           true,
		minProtocolPeriod: minProtocolPeriod,
		protocolTiming:    metrics.NewHistogram(metrics.NewUniformSample(10)),
	}

	gossip.protocolTiming.Update(int64(gossip.minProtocolPeriod))

	return gossip
}

// returns whether or not the gossip sub-protocol is stopped
func (g *gossip) Stopped() bool {
	g.l.RLock()
	defer g.l.RUnlock()

	return g.stopped
}

// sets the gossip sub-protocol to stopped or not stopped
func (g *gossip) SetStopped(stopped bool) {
	g.l.Lock()
	defer g.l.Unlock()

	g.stopped = stopped
}

// computes a ProtocolDelay for the gossip
func (g *gossip) ProtocolDelay() time.Duration {
	g.l.Lock()
	defer g.l.Unlock()

	if g.numProtocolPeriods != 0 {
		target := g.lastProtocolPeriod.Add(g.lastProtocolRate)
		delay := math.Max(float64(target.Sub(time.Now())), float64(g.minProtocolPeriod))
		return time.Duration(delay)
	}
	// delay for first tick in [0, minProtocolPeriod]ms
	return time.Duration(rand.Intn(int(g.minProtocolPeriod + 1)))
}

// computes a ProtocolRate for the Gossip
func (g *gossip) ProtocolRate() time.Duration {
	g.l.Lock()
	defer g.l.Unlock()

	observed := g.protocolTiming.Percentile(0.5) * 2.0
	return time.Duration(math.Max(observed, float64(g.minProtocolPeriod)))
}

// start the gossip protocol
func (g *gossip) Start() {
	if !g.Stopped() {
		g.node.logger.WithField("local", g.node.Address()).Warn("gossip already started")
		return
	}

	g.SetStopped(false)
	g.RunProtocolRateLoop()
	g.RunProtocolPeriodLoop()

	g.node.logger.WithField("local", g.node.Address()).Debug("started gossip protocol")
}

// stop the gossip protocol
func (g *gossip) Stop() {
	if g.Stopped() {
		g.node.logger.WithField("local", g.node.Address()).Warn("gossip already stopped")
		return
	}

	g.SetStopped(true)

	g.node.logger.WithField("local", g.node.Address()).Debug("stopped gossip protocol")

}

// run the gossip protocol period loop
func (g *gossip) RunProtocolPeriodLoop() {
	go func() {
		startTime := time.Now()
		for !g.Stopped() {
			delay := g.ProtocolDelay()
			time.Sleep(delay)

			g.ProtocolPeriod()
		}

		g.node.logger.WithFields(log.Fields{
			"local": g.node.Address(),
			"start": startTime,
			"end":   time.Now(),
		}).Debug("stopped protocol period loop")
	}()
}

// run a gossip protocol period
func (g *gossip) ProtocolPeriod() {
	g.l.Lock()
	defer g.l.Unlock()

	startTime := time.Now()

	g.node.pingNextMember()

	g.lastProtocolPeriod = time.Now()
	g.numProtocolPeriods++
	g.protocolTiming.Update(int64(time.Now().Sub(startTime)))
}

//
func (g *gossip) RunProtocolRateLoop() {
	go func() {
		for !g.Stopped() {
			time.Sleep(time.Second)
			g.lastProtocolRate = g.ProtocolRate()
		}
	}()
}
