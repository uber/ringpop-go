package ringpop

import (
	"math"
	"time"
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

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	MEMBERSHIP ITERATOR
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

type joinerOpts struct {
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
	roundPotentialNodes []string
	roundPreferredNodes []string
}

func newJoiner(ringpop *Ringpop, opts *joinerOpts) *joiner {
	if ringpop == nil {
		return nil
	}

	joiner := &joiner{
		ringpop:           ringpop,
		host:              captureHost(ringpop.HostPort),
		preferredNodes:    make([]string, 0),
		nonPreferredNodes: make([]string, 0),
	}

	joiner.potentialNodes = joiner.collectPotentialNodes([]string{})

	joinSize := defaultJoinSize
	if opts.joinSize != 0 {
		joinSize = opts.joinSize
	}
	joiner.joinSize = int(math.Min(float64(joinSize), float64(len(joiner.potentialNodes))))

	return joiner
}

//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =
//
//	METHODS
//
//= = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = = =

func (j *joiner) collectPotentialNodes(nodesJoined []string) []string {
	return []string{}
}

func (j *joiner) collectPreferredNodes() []string {
	return []string{}
}

func (j *joiner) collectNonPrefferedNodes() []string {
	if len(j.preferredNodes) == 0 {
		return j.potentialNodes
	}

	return []string{}
}

// join attempts to join the ringpop to a cluster
func (j *joiner) join() (int, error) {

	return 0, nil
}

func sendJoin(ringpop *Ringpop) (int, error) {
	joiner := newJoiner(ringpop, nil)
	joiner.join()
	return 0, nil
}
