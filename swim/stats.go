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
	"sort"
	"time"
)

type members []Member

// These methods exist to satisfy the sort.Interface for sorting.
func (s *members) Len() int           { return len(*s) }
func (s *members) Less(i, j int) bool { return (*s)[i].Address < (*s)[j].Address }
func (s *members) Swap(i, j int)      { (*s)[i], (*s)[j] = (*s)[j], (*s)[i] }

// MemberStats contains members in a memberlist and the checksum of those members
type MemberStats struct {
	Checksum uint32   `json:"checksum"`
	Members  []Member `json:"members"`
}

// GetChecksum returns the current checksum of the node's memberlist.
func (n *Node) GetChecksum() uint32 {
	return n.memberlist.Checksum()
}

// MemberStats returns the current checksum of the node's memberlist and a slice
// of the members in the memberlist in lexographically sorted order by address
func (n *Node) MemberStats() MemberStats {
	members := members(n.memberlist.GetMembers())
	sort.Sort(&members)
	return MemberStats{n.memberlist.Checksum(), members}
}

// ProtocolStats contains stats about the SWIM Protocol for the node
type ProtocolStats struct {
	Timing     Timing        `json:"timing"`
	Rate       time.Duration `json:"protocolRate"`
	ClientRate float64       `json:"clientRate"`
	ServerRate float64       `json:"serverRate"`
	TotalRate  float64       `json:"totalRate"`
}

// Timing contains timing information for the SWIM protocol for the node
type Timing struct {
	Type     string  `json:"type"`
	Min      int64   `json:"min"`
	Max      int64   `json:"max"`
	Sum      int64   `json:"sum"`
	Variance float64 `json:"variance"`
	Mean     float64 `json:"mean"`
	StdDev   float64 `json:"std_dev"`
	Count    int64   `json:"count"`
	Median   float64 `json:"median"`
	P75      float64 `json:"p75"`
	P95      float64 `json:"p95"`
	P99      float64 `json:"p99"`
	P999     float64 `json:"p999"`
}

// ProtocolStats returns stats about the node's SWIM protocol.
func (n *Node) ProtocolStats() ProtocolStats {
	timing := n.gossip.ProtocolTiming()
	return ProtocolStats{
		Timing{
			Type:     "histogram",
			Min:      timing.Min(),
			Max:      timing.Max(),
			Sum:      timing.Sum(),
			Variance: timing.Variance(),
			Mean:     timing.Mean(),
			StdDev:   timing.StdDev(),
			Count:    timing.Count(),
			Median:   timing.Percentile(0.5),
			P75:      timing.Percentile(0.75),
			P95:      timing.Percentile(0.95),
			P99:      timing.Percentile(0.99),
			P999:     timing.Percentile(0.999),
		},
		n.gossip.ProtocolRate(),
		n.clientRate.Rate1(),
		n.serverRate.Rate1(),
		n.totalRate.Rate1(),
	}
}

// Uptime returns the amount of time the node has been running for
func (n *Node) Uptime() time.Duration {
	return time.Now().Sub(n.startTime)
}
