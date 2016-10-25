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
	"testing"
	"time"

	"github.com/stretchr/testify/suite"
)

type StatsTestSuite struct {
	suite.Suite

	testNode *testNode

	cluster *swimCluster
}

func (s *StatsTestSuite) SetupTest() {
	// Create a test node, on its own. Tests can join this to the cluster for
	// testing, if they want.
	s.testNode = newChannelNode(s.T())

	// Create a cluster for testing. Join these guys to each other.
	s.cluster = newSwimCluster(4)
	s.cluster.Bootstrap()
}

func (s *StatsTestSuite) TearDownTest() {
	if s.cluster != nil {
		s.cluster.Destroy()
	}
}

func (s *StatsTestSuite) TestUptime() {
	s.cluster.Add(s.testNode.node)
	s.True(s.testNode.node.Uptime() > 0, "expected uptime to be greater than zero")
}

// TestProtocolStats tests that the returned struct has non-zero values for all
// fields.
func (s *StatsTestSuite) TestProtocolStats() {
	if testing.Short() {
		s.T().Skip("skipping protocol stats test in short mode")
	}

	s.cluster.Add(s.testNode.node)

	// We need to sleep for at least 5 seconds, as this is the tick period for
	// the metrics.Meter and it cannot be easily changed.
	time.Sleep(6 * time.Second)

	stats := s.testNode.node.ProtocolStats()

	s.NotEmpty(stats.Timing.Type)
	s.NotZero(stats.Timing.Min)
	s.NotZero(stats.Timing.Max)
	s.NotZero(stats.Timing.Sum)
	s.NotZero(stats.Timing.Variance)
	s.NotZero(stats.Timing.Mean)
	s.NotZero(stats.Timing.StdDev)
	s.NotZero(stats.Timing.Count)
	s.NotZero(stats.Timing.Median)
	s.NotZero(stats.Timing.P75)
	s.NotZero(stats.Timing.P95)
	s.NotZero(stats.Timing.P99)
	s.NotZero(stats.Timing.P999)

	s.NotZero(stats.ServerRate)
	s.NotZero(stats.TotalRate)
	// TODO: Fix ClientRate, which is currently always 0
	//s.NotZero(stats.ClientRate)
}

func (s *StatsTestSuite) TestMemberStats() {
	s.cluster.WaitForConvergence(s.T(), 100)
	stats := s.cluster.Nodes()[0].MemberStats()

	// Extract addresses from the member list
	var memberAddresses []string
	for _, member := range stats.Members {
		memberAddresses = append(memberAddresses, member.Address)
	}

	sort.Strings(memberAddresses)

	clusterAddresses := s.cluster.Addresses()
	sort.Strings(clusterAddresses)

	s.Equal(clusterAddresses, memberAddresses, "member addresses should match cluster hosts")
	s.NotZero(stats.Checksum, "checksum should be non-zero")
}

func TestStatsTestSuite(t *testing.T) {
	suite.Run(t, new(StatsTestSuite))
}
