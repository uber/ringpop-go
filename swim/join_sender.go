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

	log "github.com/uber-common/bark"
	"github.com/uber/ringpop-go/logging"
	"github.com/uber/ringpop-go/shared"
	"github.com/uber/ringpop-go/util"
	"github.com/uber/tchannel-go/json"
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

var errJoinTimeout = errors.New("join timed out")

// A joinRequest is used to request a join to a remote node
type joinRequest struct {
	App         string            `json:"app"`
	Source      string            `json:"source"`
	Incarnation int64             `json:"incarnationNumber"`
	Timeout     time.Duration     `json:"timeout"`
	Labels      map[string]string `json:"labels,omitempty"`
}

// joinOpts are opts to perform a join with
type joinOpts struct {
	timeout           time.Duration
	size              int
	maxJoinDuration   time.Duration
	parallelismFactor int

	// delayer delays repeated join attempts.
	delayer joinDelayer
}

// A joinSender is used to join an existing cluster of nodes defined in a node's
// bootstrap list
type joinSender struct {
	node    *Node
	timeout time.Duration

	// bootstrapHostsMap is a map of unique hosts each containing a slice of
	// the instances (hostsports) on that particular host.
	bootstrapHostsMap map[string][]string

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

	numTries int

	// delayer delays repeated join attempts.
	delayer joinDelayer

	logger log.Logger
}

// newJoinSender returns a new JoinSender to join a cluster with
func newJoinSender(node *Node, opts *joinOpts) (*joinSender, error) {
	if opts == nil {
		opts = &joinOpts{}
	}

	if node.discoverProvider == nil {
		return nil, errors.New("no discover provider")
	}

	// Resolve/retrieve bootstrap hosts from the provider specified in the
	// join options.
	bootstrapHosts, err := node.discoverProvider.Hosts()
	if err != nil {
		return nil, err
	}

	// Check we're in the bootstrap host list and add ourselves if we're not
	// there. If the host list is empty, this will create a single-node
	// cluster.
	if !util.StringInSlice(bootstrapHosts, node.Address()) {
		bootstrapHosts = append(bootstrapHosts, node.Address())
	}

	js := &joinSender{
		node:   node,
		logger: logging.Logger("join").WithField("local", node.Address()),
	}

	// Parse bootstrap hosts into a map
	js.parseHosts(bootstrapHosts)

	js.potentialNodes = js.CollectPotentialNodes(nil)

	js.timeout = util.SelectDuration(opts.timeout, defaultJoinTimeout)
	js.maxJoinDuration = util.SelectDuration(opts.maxJoinDuration, defaultMaxJoinDuration)
	js.parallelismFactor = util.SelectInt(opts.parallelismFactor, defaultParallelismFactor)
	js.size = util.SelectInt(opts.size, defaultJoinSize)
	js.size = util.Min(js.size, len(js.potentialNodes))
	js.delayer = opts.delayer

	if js.delayer == nil {
		// Create and use exponential delayer as the delay mechanism. Create it
		// with nil opts which uses default delayOpts.
		js.delayer, err = newExponentialDelayer(js.node.address, nil)
		if err != nil {
			return nil, err
		}
	}

	return js, nil
}

// parseHosts populates the bootstrap hosts map from the provided slice of
// hostports.
func (j *joinSender) parseHosts(hostports []string) {
	// Parse bootstrap hosts into a map
	j.bootstrapHostsMap = util.HostPortsByHost(hostports)

	// Perform some sanity checks on the bootstrap hosts
	err := util.CheckLocalMissing(j.node.address, j.bootstrapHostsMap[util.CaptureHost(j.node.address)])
	if err != nil {
		j.logger.Warn(err.Error())
	}

	mismatched, err := util.CheckHostnameIPMismatch(j.node.address, j.bootstrapHostsMap)
	if err != nil {
		j.logger.WithField("mismatched", mismatched).Warn(err.Error())
	}
}

// potential nodes are nodes that can be joined that are not the local node
func (j *joinSender) CollectPotentialNodes(nodesJoined []string) []string {
	if nodesJoined == nil {
		nodesJoined = make([]string, 0)
	}

	var potentialNodes []string

	for _, hostports := range j.bootstrapHostsMap {
		for _, hostport := range hostports {
			if j.node.address != hostport && !util.StringInSlice(nodesJoined, hostport) {
				potentialNodes = append(potentialNodes, hostport)
			}
		}
	}

	return potentialNodes
}

// preferred nodes are nodes that are not on the same host as the local node
func (j *joinSender) CollectPreferredNodes() []string {
	var preferredNodes []string

	for host, hostports := range j.bootstrapHostsMap {
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

	for _, host := range j.bootstrapHostsMap[util.CaptureHost(j.node.address)] {
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

	if util.SingleNodeCluster(j.node.address, j.bootstrapHostsMap) {
		j.logger.Info("got single node cluster to join")
		return nodesJoined, nil
	}

	for {
		if j.node.Destroyed() {
			j.node.EmitEvent(JoinFailedEvent{
				Reason: Destroyed,
				Error:  nil,
			})
			return nil, errors.New("node destroyed while attempting to join cluster")
		}
		// join group of nodes
		successes, failures := j.JoinGroup(nodesJoined)

		nodesJoined = append(nodesJoined, successes...)
		numJoined += len(successes)
		numFailed += len(failures)
		numGroups++

		if numJoined >= j.size {
			j.logger.WithFields(log.Fields{
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
			j.logger.WithFields(log.Fields{
				"joinDuration":    joinDuration,
				"maxJoinDuration": j.maxJoinDuration,
				"numJoined":       numJoined,
				"numFailed":       numFailed,
				"startTime":       startTime,
			}).Warn("max join duration exceeded")

			err := fmt.Errorf("join duration of %v exceeded max %v",
				joinDuration, j.maxJoinDuration)

			j.node.EmitEvent(JoinFailedEvent{
				Reason: Error,
				Error:  err,
			})
			return nodesJoined, err
		}

		j.logger.WithFields(log.Fields{
			"joinSize":  j.size,
			"numJoined": numJoined,
			"numFailed": numFailed,
			"startTime": startTime,
		}).Debug("join not yet complete")

		j.delayer.delay()
	}

	j.node.EmitEvent(JoinCompleteEvent{
		Duration:  time.Now().Sub(startTime),
		NumJoined: numJoined,
		Joined:    nodesJoined,
	})

	return nodesJoined, nil
}

// JoinGroup collects a number of nodes to join and sends join requests to them.
// nodesJoined contains the nodes that are already joined. The method returns
// the nodes that are succesfully joined, and the nodes that failed respond.
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

	j.numTries++
	j.node.EmitEvent(JoinTriesUpdateEvent{j.numTries})

	for _, target := range group {
		wg.Add(1)

		go func(target string) {
			defer wg.Done()

			res, err := sendJoinRequest(j.node, target, j.timeout)

			if err != nil {
				msg := "attempt to join node failed"
				if err == errJoinTimeout {
					msg = "attempt to join node timed out"
				}

				j.logger.WithFields(log.Fields{
					"remote":  target,
					"timeout": j.timeout,
				}).Debug(msg)

				responses.Lock()
				responses.failures = append(responses.failures, target)
				responses.Unlock()
				return
			}

			responses.Lock()
			responses.successes = append(responses.successes, target)
			responses.Unlock()

			start := time.Now()
			j.node.memberlist.AddJoinList(res.Membership)

			j.node.EmitEvent(AddJoinListEvent{
				Duration: time.Now().Sub(start),
			})
		}(target)
	}

	// wait for joins to complete
	wg.Wait()

	// don't need to lock successes/failures since we're finished writing to them
	j.logger.WithFields(log.Fields{
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

// sendJoinRequest sends a join request to the specified target.
func sendJoinRequest(node *Node, target string, timeout time.Duration) (*joinResponse, error) {
	ctx, cancel := shared.NewTChannelContext(timeout)
	defer cancel()

	peer := node.channel.Peers().GetOrAdd(target)

	req := joinRequest{
		App:         node.app,
		Source:      node.address,
		Incarnation: node.Incarnation(),
		Timeout:     timeout,
		Labels:      node.Labels().AsMap(),
	}
	res := &joinResponse{}

	// make request
	errC := make(chan error, 1)
	go func() {
		errC <- json.CallPeer(ctx, peer, node.service, "/protocol/join", req, res)
	}()

	// wait for result or timeout
	var err error
	select {
	case err = <-errC:
	case <-ctx.Done():
		err = errJoinTimeout
	}

	if err != nil {
		logging.Logger("join").WithFields(log.Fields{
			"local": node.Address(),
			"error": err,
		}).Debug("could not complete join")

		return nil, err
	}

	return res, err
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
