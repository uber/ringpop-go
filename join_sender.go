package ringpop

import (
	"errors"
	"math"

	"golang.org/x/net/context"

	"math/rand"
	"time"

	log "github.com/Sirupsen/logrus"
	"github.com/uber/tchannel/golang"
)

// A joiner joins a ringpop to a (existing?) cluster

const (
	// If a node cannot complete a join within _MAX_JOIN_DURATION_
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
	defaultMaxJoinDuration   = 120000 * time.Millisecond
	defaultJoinTimeout       = 1000 * time.Millisecond
	defaultJoinSize          = 3
	defaultParallelismFactor = 2
)

func isSingleNodeCluster(ringpop *Ringpop) bool {
	_, ok := ringpop.bootstrapHosts[captureHost(ringpop.HostPort)]

	return ok && len(ringpop.bootstrapHosts) == 1
}

func takeNode(hosts []string) ([]string, string) {
	if len(hosts) == 0 {
		return hosts, ""
	}

	i := rand.Intn(len(hosts))
	host := hosts[i]
	hosts = append(hosts[:i], hosts[i+1:]...)

	return hosts, host
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	JOINER
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

type joinBody struct {
	App         string        `json:"app"`
	Source      string        `json:"source"`
	Incarnation int64         `json:"incarnation"`
	Timeout     time.Duration `json:"timeout"`
}

type joinerOptions struct {
	timeout           time.Duration
	maxJoinDuration   time.Duration
	parallelismFactor int
	joinSize          int
}

type joiner struct {
	ringpop *Ringpop
	host    string
	timeout time.Duration

	// This is used as a multiple of the required nodes left
	// to join to satisfy `joinSize`. Additional parallelism
	// can be applied in order for `joinSize` to be satisified
	// faster.
	parallelismFactor int

	// We eventually want to give up if the join process cannot
	// succeed. `maxJoinDuration` is used to restrict that process
	// to a certain time limit.
	maxJoinDuration time.Duration

	// Potential nodes are nodes in the ringpop bootstrap
	// list that can be joined. Upon instantiation, this step
	// simply filters out a node from attempting to join itself.
	potentialNodes    []string
	preferredNodes    []string
	nonPreferredNodes []string

	// We either join the number of nodes defined by `joinSize`
	// or limit it to the number of `potentialNodes`. After all,
	// we can't join more than there are to join in the first place.
	joinSize int

	// A round is defined as a complete cycle through all
	// potential join targets. Once a round is completed,
	// we start all over again. A full-cycle should be pretty
	// darned rare. We will try and try to join other nodes
	// until `joinSize` is reached or `maxJoinDuration` is
	// exceeded.
	roundPotentialNodes    []string
	roundPreferredNodes    []string
	roundNonPreferredNodes []string
}

func newJoiner(ringpop *Ringpop, opts *joinerOptions) *joiner {
	if ringpop == nil {
		return nil
	}

	joiner := &joiner{
		ringpop:             ringpop,
		host:                captureHost(ringpop.HostPort),
		preferredNodes:      make([]string, 0),
		nonPreferredNodes:   make([]string, 0),
		roundPotentialNodes: make([]string, 0),
		roundPreferredNodes: make([]string, 0),
	}

	joiner.potentialNodes = joiner.collectPotentialNodes([]string{})

	joiner.timeout = selectDurationOrDefault(opts.timeout, defaultJoinTimeout)
	joiner.parallelismFactor = selectNumOrDefault(opts.parallelismFactor, defaultParallelismFactor)
	joiner.maxJoinDuration = selectDurationOrDefault(opts.maxJoinDuration, defaultMaxJoinDuration)

	joinSize := selectNumOrDefault(opts.joinSize, defaultJoinSize)
	joiner.joinSize = int(math.Min(float64(joinSize), float64(len(joiner.potentialNodes))))

	return joiner
}

func (j *joiner) init(nodesJoined []string) {
	j.potentialNodes = j.collectPotentialNodes(nodesJoined)
	j.preferredNodes = j.collectPreferredNodes()
	j.nonPreferredNodes = j.collectNonPreferredNodes()

	j.roundPotentialNodes = append(j.roundPotentialNodes, j.potentialNodes...)
	j.roundPreferredNodes = append(j.roundPreferredNodes, j.preferredNodes...)

}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	METHODS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

// potential nodes are nodes that are not this instance of ringpop
func (j *joiner) collectPotentialNodes(nodesJoined []string) []string {
	if nodesJoined == nil {
		nodesJoined = make([]string, 0)
	}

	var potentialNodes []string

	for _, hostports := range j.ringpop.bootstrapHosts {
		for _, hostport := range hostports {
			if j.ringpop.HostPort != hostport && indexOf(nodesJoined, hostport) == -1 {
				potentialNodes = append(potentialNodes, hostport)
			}
		}
	}

	return potentialNodes
}

// preferred nodes are nodes that are not on the same host as this isntance of ringpop
func (j *joiner) collectPreferredNodes() []string {
	var preferredNodes []string

	for host, hostports := range j.ringpop.bootstrapHosts {
		if host != captureHost(j.ringpop.HostPort) {
			preferredNodes = append(preferredNodes, hostports...)
		}
	}

	return preferredNodes
}

// non-preferred nodes are everyone else
func (j *joiner) collectNonPreferredNodes() []string {
	if len(j.preferredNodes) == 0 {
		return j.potentialNodes
	}

	var nonPreferredNodes []string

	for _, host := range j.ringpop.bootstrapHosts[captureHost(j.ringpop.HostPort)] {
		if host != j.ringpop.HostPort {
			nonPreferredNodes = append(nonPreferredNodes, host)
		}
	}
	return nonPreferredNodes
}

func (j *joiner) selectGroup(nodesJoined []string) []string {
	var group []string
	var host string

	preferredNodes := j.roundPreferredNodes
	nonPreferredNodes := j.nonPreferredNodes
	numNodesLeft := j.joinSize - len(nodesJoined)

	cont := func() bool {
		numNodesSelected := len(group)
		if numNodesSelected == numNodesLeft*j.parallelismFactor {
			return false
		}

		numNodesAvailable := len(preferredNodes) + len(nonPreferredNodes)
		if numNodesAvailable == 0 {
			return false
		}

		return true
	}

	for cont() {
		if len(preferredNodes) > 0 {
			preferredNodes, host = takeNode(preferredNodes)
			group = append(group, host)
		} else if len(nonPreferredNodes) > 0 {
			nonPreferredNodes, host = takeNode(nonPreferredNodes)
			group = append(group, host)
		}
	}

	return group
}

// join attempts to join the ringpop to a cluster
func (j *joiner) join() ([]string, error) {
	if j.ringpop.destroyed {
		return nil, errors.New("joiner was destroyed")
	}

	if isSingleNodeCluster(j.ringpop) {
		j.ringpop.logger.WithField("local", j.ringpop.WhoAmI()).
			Info("ringpop received a single node cluster to join")

		return nil, nil
	}

	var nodesJoined []string
	// var numGroups = 0
	// var numJoined = 0
	// var jumFailed = 0
	// var startTime = time.Now()

	j.joinGroup(nodesJoined)

	return nodesJoined, nil
}

func (j *joiner) joinGroup(totalNodesJoined []string) ([]string, []string) {
	group := j.selectGroup(totalNodesJoined)

	var nodesJoined []string
	var nodesFailed []string
	var numNodesLeft = j.joinSize - len(totalNodesJoined)
	var startTime = time.Now()

	for _, node := range group {
		err := j.joinNode(node)
		if err != nil {
			nodesFailed = append(nodesFailed, node)
		} else {
			nodesJoined = append(nodesJoined, node)
		}

		numCompleted := len(nodesJoined) + len(nodesFailed)

		// Finished when either all joins have completed or enough to satisfy requirement
		// specified by `joinSize`
		if len(nodesJoined) >= numNodesLeft || numCompleted >= len(group) {
			break
		}
	}

	j.ringpop.logger.WithFields(log.Fields{
		"local":        j.ringpop.WhoAmI(),
		"groupSize":    len(group),
		"joinSize":     j.joinSize,
		"joinTime":     time.Now().Sub(startTime),
		"numFailures":  len(nodesFailed),
		"numSuccesses": len(nodesJoined),
		"numNodesLeft": numNodesLeft,
		"failures":     nodesFailed,
		"successes":    nodesJoined,
	})

	return nodesJoined, nodesFailed
}

func (j *joiner) joinNode(node string) error {
	ctx, cancel := context.WithTimeout(context.Background(), j.timeout)
	defer cancel()

	// begin call
	call, err := j.ringpop.channel.BeginCall(ctx, node, "ringpop", "/protocol/join", nil)
	if err != nil {
		j.ringpop.logger.WithFields(log.Fields{
			"local":  j.ringpop.WhoAmI(),
			"remote": node,
		}).Warnf("could not begin call to remote member: %v", err)
		return err
	}

	// send request
	var reqHeaders headers
	if err := tchannel.NewArgWriter(call.Arg2Writer()).WriteJSON(reqHeaders); err != nil {

		return err
	}

	reqBody := joinBody{
		App:         j.ringpop.App,
		Source:      j.ringpop.WhoAmI(),
		Incarnation: j.ringpop.membership.localmember.Incarnation,
		Timeout:     j.timeout,
	}
	if err := tchannel.NewArgWriter(call.Arg3Writer()).WriteJSON(reqBody); err != nil {
		return err
	}

	// get response
	var resHeaders headers
	if err := tchannel.NewArgReader(call.Response().Arg2Reader()).ReadJSON(&resHeaders); err != nil {
		return err
	}

	var resBody joinResBody
	if err := tchannel.NewArgReader(call.Response().Arg3Reader()).ReadJSON(&resBody); err != nil {
		return err
	}

	return nil
}

func sendJoin(ringpop *Ringpop) ([]string, error) {
	joiner := newJoiner(ringpop, nil)
	joiner.collectPotentialNodes(nil)
	return joiner.join()
}
