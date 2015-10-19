// Copyright (c) 2015 Uber Technologies, Inc.
//
// Permission is hereby granted, free of charge, to any person obtaining a copy
// of this software and associated documentation files (the "Software"), to deal
// in the Software without restriction, including without limitation the rights
// to use, copy, modify, merge, publish, distribute, sublicense, and/or sell
// copies of the Software, and to permit persons to whom the Software is
// furnished to do so, subject to the following conditions:
// The above copyright notice and this permission notice shall be included in
// all copies or substantial portions of the Software.
// THE SOFTWARE IS PROVIDED "AS IS", WITHOUT WARRANTY OF ANY KIND, EXPRESS OR
// IMPLIED, INCLUDING BUT NOT LIMITED TO THE WARRANTIES OF MERCHANTABILITY,
// FITNESS FOR A PARTICULAR PURPOSE AND NONINFRINGEMENT. IN NO EVENT SHALL THE
// AUTHORS OR COPYRIGHT HOLDERS BE LIABLE FOR ANY CLAIM, DAMAGES OR OTHER
// LIABILITY, WHETHER IN AN ACTION OF CONTRACT, TORT OR OTHERWISE, ARISING FROM,
// OUT OF OR IN CONNECTION WITH THE SOFTWARE OR THE USE OR OTHER DEALINGS IN
// THE SOFTWARE.

package bark

import (
	"time"

	"github.com/cactus/go-statsd-client/statsd"
)

type barkCactusStatsReporter struct {
	delegate statsd.Statter
}

func newBarkCactusStatsReporter(statter statsd.Statter) StatsReporter {
	return &barkCactusStatsReporter{delegate: statter}
}

func (s *barkCactusStatsReporter) IncCounter(name string, tags Tags, value int64) {
	s.delegate.Inc(name, value, 1.0)
}

func (s *barkCactusStatsReporter) UpdateGauge(name string, tags Tags, value int64) {
	s.delegate.Gauge(name, value, 1.0)
}

func (s *barkCactusStatsReporter) RecordTimer(name string, tags Tags, d time.Duration) {
	s.delegate.TimingDuration(name, d, 1.0)
}
