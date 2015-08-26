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
	"errors"
	"fmt"
	"math/rand"
	"sync"
	"time"

	log "github.com/uber/bark"
	"github.com/uber/ringpop-go/swim/util"
	"github.com/uber/tchannel/golang/json"
)

const (
	// If a node cannot complete a join within defaultMaxJoinDuration
	// there is likely something very wrong. The aim is for the join
	// operation to take no more than 1s, under normal conditions.
	//
	// The duration assigned below is very high for the following
	// purposes:
	//   - Gives an application developer some time to diagnose
	//   what could be wrong.
	//   - Gives an operator some time to bootstrap a newly
	//   provisioned cluster
	//   - Trying forever is futile
	defaultMaxJoinDuration   = 120 * time.Second
	defaultJoinTimeout       = time.Second
	defaultJoinSize          = 3
	defaultParallelismFactor = 2
)

// A joinRequest is used to request a join to a remote node
type joinRequest struct {
	App         string        `json:"app"`
	Source      string        `json:"source"`
	Incarnation int64         `json:"incarnation"`
	Timeout     time.Duration `json:"timeout"`
}

// joinOpts are opts to perform a join with
type joinOpts struct {
	timeout           time.Duration
	size              int
	maxJoinDuration   time.Duration
	parallelismFactor int
}

// A joinSender is used to join an existing cluster of nodes defined in a node's
// bootstrap list
type joinSender struct {
	node    *Node
	timeout time.Duration

	// This is used as a multiple of the required nodes left to satisfy
	// `joinSize`. Additional parallelism can be applied in order for
	// `joinSize` to be satisfied faster.
	parallelismFactor int

	maxJoinDuration time.Duration

	potentialNodes    []string
	preferredNodes    []string
	nonPreferredNodes []string

	// We either join the number of nodes defined by size or limit it to the number
	// of `potentialNodes` as we can't join more than there are to join in the first place.
	size int

	// A round is a complete cycle through all potential join targets. When a round
	// is completed we start all over again, though full cycles should be very rare.
	// We try to join nodes until `joinSize` is reached or `maxJoinDuration` is exceeded.
	roundPotentialNodes    []string
	roundPreferredNodes    []string
	roundNonPreferredNodes []string
}

// newJoinSender returns a new JoinSender to join a cluster with
func newJoinSender(node *Node, opts *joinOpts) (*joinSender, error) {
	if opts == nil {
		opts = &joinOpts{}
	}

	if len(node.bootstrapHosts) == 0 {
		return nil, errors.New("bootstrap hosts cannot be empty")
	}

	js := &joinSender{
		node: node,
	}

	js.potentialNodes = js.CollectPotentialNodes(nil)

	js.timeout = util.SelectDuration(opts.timeout, defaultJoinTimeout)
	js.maxJoinDuration = util.SelectDuration(opts.maxJoinDuration, defaultMaxJoinDuration)
	js.parallelismFactor = util.SelectInt(opts.parallelismFactor, defaultParallelismFactor)
	js.size = util.SelectInt(opts.size, defaultJoinSize)
	js.size = util.Min(js.size, len(js.potentialNodes))

	return js, nil
}

// potential nodes are nodes that can be joined that are not the local node
func (j *joinSender) CollectPotentialNodes(nodesJoined []string) []string {
	if nodesJoined == nil {
		nodesJoined = make([]string, 0)
	}

	var potentialNodes []string

	for _, hostports := range j.node.bootstrapHosts {
		for _, hostport := range hostports {
			if j.node.address != hostport && util.IndexOf(nodesJoined, hostport) == -1 {
				potentialNodes = append(potentialNodes, hostport)
			}
		}
	}

	return potentialNodes
}

// preferred nodes are nodes that are not on the same host as the local node
func (j *joinSender) CollectPreferredNodes() []string {
	var preferredNodes []string

	for host, hostports := range j.node.bootstrapHosts {
		if host != util.CaptureHost(j.node.address) {
			preferredNodes = append(preferredNodes, hostports...)
		}
	}

	return preferredNodes
}

// non-preferred nodes are everyone else
func (j *joinSender) CollectNonPreferredNodes() []string {
	if len(j.preferredNodes) == 0 {
		return j.potentialNodes
	}

	var nonPreferredNodes []string

	for _, host := range j.node.bootstrapHosts[util.CaptureHost(j.node.address)] {
		if host != j.node.address {
			nonPreferredNodes = append(nonPreferredNodes, host)
		}
	}
	return nonPreferredNodes
}

func (j *joinSender) Init(nodesJoined []string) {
	rand.Seed(time.Now().UnixNano())

	j.potentialNodes = j.CollectPotentialNodes(nodesJoined)
	j.preferredNodes = j.CollectPreferredNodes()
	j.nonPreferredNodes = j.CollectNonPreferredNodes()

	j.roundPotentialNodes = append(j.roundPotentialNodes, j.potentialNodes...)
	j.roundPreferredNodes = append(j.roundPreferredNodes, j.preferredNodes...)
	j.roundNonPreferredNodes = append(j.roundNonPreferredNodes, j.nonPreferredNodes...)
}

// selects a group of nodes
func (j *joinSender) SelectGroup(nodesJoined []string) []string {
	var group []string
	// if fully exhausted or first round, initialize this round's nodes
	if len(j.roundPreferredNodes) == 0 && len(j.roundNonPreferredNodes) == 0 {
		j.Init(nodesJoined)
	}

	numNodesLeft := j.size - len(nodesJoined)

	cont := func() bool {
		if len(group) == numNodesLeft*j.parallelismFactor {
			return false
		}

		nodesAvailable := len(j.roundPreferredNodes) + len(j.roundNonPreferredNodes)
		if nodesAvailable == 0 {
			return false
		}

		return true
	}

	for cont() {
		if len(j.roundPreferredNodes) > 0 {
			group = append(group, util.TakeNode(&j.roundPreferredNodes, -1))
		} else if len(j.roundNonPreferredNodes) > 0 {
			group = append(group, util.TakeNode(&j.roundNonPreferredNodes, -1))
		}
	}

	return group
}

func (j *joinSender) JoinCluster() ([]string, error) {
	var nodesJoined []string
	var numGroups = 0
	var numJoined = 0
	var numFailed = 0
	var startTime = time.Now()

	if util.SingleNodeCluster(j.node.address, j.node.bootstrapHosts) {
		j.node.logger.WithField("local", j.node.address).Info("got single node cluster to join")
		return nodesJoined, nil
	}

	for {
		if j.node.Destroyed() {
			return nil, errors.New("node destroyed while attempting to join cluster")
		}
		// join group of nodes
		successes, failures := j.JoinGroup(nodesJoined)

		nodesJoined = append(nodesJoined, successes...)
		numJoined += len(successes)
		numFailed += len(failures)
		numGroups++

		if numJoined >= j.size {
			j.node.logger.WithFields(log.Fields{
				"local":     j.node.address,
				"joinSize":  j.size,
				"joinTime":  time.Now().Sub(startTime),
				"numJoined": numJoined,
				"numFailed": numFailed,
				"numGroups": numGroups,
			}).Debug("join complete")

			break
		}

		joinDuration := time.Now().Sub(startTime)

		if joinDuration > j.maxJoinDuration {
			j.node.logger.WithFields(log.Fields{
				"local":           j.node.address,
				"joinDuration":    joinDuration,
				"maxJoinDuration": j.maxJoinDuration,
				"numJoined":       numJoined,
				"numFailed":       numFailed,
				"startTime":       startTime,
			}).Warn("max join duration exceeded")

			err := fmt.Errorf("join duration of %v exceeded max %v",
				joinDuration, j.maxJoinDuration)
			return nodesJoined, err
		}

		j.node.logger.WithFields(log.Fields{
			"local":     j.node.address,
			"joinSize":  j.size,
			"numJoined": numJoined,
			"numFailed": numFailed,
			"startTime": startTime,
		}).Debug("join not yet complete")
	}

	j.node.emit(JoinCompleteEvent{
		Duration:  time.Now().Sub(startTime),
		NumJoined: numJoined,
		Joined:    nodesJoined,
	})

	return nodesJoined, nil
}

func (j *joinSender) JoinGroup(nodesJoined []string) ([]string, []string) {
	group := j.SelectGroup(nodesJoined)

	var responses struct {
		successes []string
		failures  []string
		sync.Mutex
	}

	var numNodesLeft = j.size - len(nodesJoined)
	var startTime = time.Now()

	var wg sync.WaitGroup

	for _, node := range group {
		wg.Add(1)
		go func(n string) {
			ctx, cancel := json.NewContext(j.timeout)
			defer cancel()

			var res joinResponse
			var failed bool

			select {
			case err := <-j.MakeCall(ctx, n, &res):
				if err != nil {
					j.node.logger.WithFields(log.Fields{
						"local":   j.node.address,
						"remote":  n,
						"timeout": j.timeout,
					}).Debug("attempt to join node failed")
					failed = true
					break
				}
				j.node.memberlist.Update(res.Changes)

			case <-ctx.Done():
				j.node.logger.WithFields(log.Fields{
					"local":   j.node.address,
					"remote":  n,
					"timeout": j.timeout,
				}).Debug("attempt to join node timed out")
				failed = true
			}

			if !failed {
				responses.Lock()
				responses.successes = append(responses.successes, n)
				responses.Unlock()
			} else {
				responses.Lock()
				responses.failures = append(responses.failures, n)
				responses.Unlock()
			}

			wg.Done()
		}(node)
	}
	// wait for joins to complete
	wg.Wait()

	// don't need to lock successes/failures since we're finished writing to them
	j.node.logger.WithFields(log.Fields{
		"local":        j.node.address,
		"groupSize":    len(group),
		"joinSize":     j.size,
		"joinTime":     time.Now().Sub(startTime),
		"numNodesLeft": numNodesLeft,
		"numFailures":  len(responses.failures),
		"failures":     responses.failures,
		"numSuccesses": len(responses.successes),
		"successes":    responses.successes,
	}).Debug("join group complete")

	return responses.successes, responses.failures
}

func (j *joinSender) MakeCall(ctx json.Context, node string, res *joinResponse) <-chan error {
	errC := make(chan error)

	go func() {
		defer close(errC)

		peer := j.node.channel.Peers().GetOrAdd(node)

		req := joinRequest{
			App:         j.node.app,
			Source:      j.node.address,
			Incarnation: j.node.Incarnation(),
			Timeout:     j.timeout,
		}

		err := json.CallPeer(ctx, peer, j.node.service, "/protocol/join", req, res)
		if err != nil {
			j.node.logger.WithFields(log.Fields{
				"local": j.node.address,
				"call":  "join-send",
				"error": err,
			}).Debug("could not complete join")
			errC <- err
			return
		}

		errC <- nil
	}()

	return errC
}

// SendJoin creates a new JoinSender and attempts to join the cluster defined by
// the nodes bootstrap hosts
func sendJoin(node *Node, opts *joinOpts) ([]string, error) {
	joiner, err := newJoinSender(node, opts)
	if err != nil {
		return nil, err
	}
	return joiner.JoinCluster()
}
