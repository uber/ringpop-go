// Copyright (c) 2015 Uber Technologies, Inc.

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

package tchannel

import "math"

// ScoreCalculator defines the interface to calculate the score.
type ScoreCalculator interface {
	GetScore(p *Peer) uint64
}

// ScoreCalculatorFunc is an adapter that allows functions to be used as ScoreCalculator
type ScoreCalculatorFunc func(p *Peer) uint64

// GetScore calls the underlying function.
func (f ScoreCalculatorFunc) GetScore(p *Peer) uint64 {
	return f(p)
}

type zeroCalculator struct{}

func (zeroCalculator) GetScore(p *Peer) uint64 {
	return 0
}

func newZeroCalculator() zeroCalculator {
	return zeroCalculator{}
}

type preferIncomingCalculator struct{}

func (preferIncomingCalculator) GetScore(p *Peer) uint64 {
	if p.NumInbound() <= 0 {
		return math.MaxUint64
	}

	return uint64(p.NumPendingOutbound())
}

// newPreferIncomingCalculator calculates the score for peers. It prefers
// peers who have incoming connections and choose based on the least pending
// outbound calls. The less number of outbound calls, the smaller score the peer has.
func newPreferIncomingCalculator() preferIncomingCalculator {
	return preferIncomingCalculator{}
}
