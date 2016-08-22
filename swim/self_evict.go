package swim

import (
	"errors"
	"math"
	"sync"
	"time"

	"github.com/uber/ringpop-go/util"

	"github.com/uber-common/bark"
)

var (
	// ErrDuplicateHook is returned when a hook with the given name has already
	// been registered.
	ErrDuplicateHook = errors.New("expected hook name to be unique")

	// ErrEvictionInProgress is returned when ringpop is already in the process
	// of evicting
	ErrEvictionInProgress = errors.New("ringpop is already executing an eviction")
)

// SelfEvict defines the functions that interact with the self eviction of nodes
// from the membership prior to shutting down
type SelfEvict interface {
	RegisterSelfEvictHook(hooks SelfEvictHook) error
	SelfEvict() error
}

// SelfEvictHook is an interface describing a module that can be registered to
// the self eviction hooks
type SelfEvictHook interface {
	// Name returns the name under which the eviction should be regitered
	Name() string

	// PreEvict is the hook that will be called before ringpop evicts itself
	// from the membership
	PreEvict()

	// PostEvict is the hook that will be called after ringpop has evicted
	// itsels from them memership
	PostEvict()
}

type hookFn func(SelfEvictHook)

// SelfEvictOptions configures how self eviction should behave. Applications can
// configure if ringpop should proactively ping members of the network on self
// eviction and what percentage/ratio of the memberlist should be pinged at most
type SelfEvictOptions struct {
	PingDisable bool
	PingRatio   float64
}

func mergeSelfEvictOptions(opt, def SelfEvictOptions) SelfEvictOptions {
	return SelfEvictOptions{
		PingDisable: util.SelectBool(opt.PingDisable, def.PingDisable),
		PingRatio:   util.SelectFloat(opt.PingRatio, def.PingRatio),
	}
}

type evictionPhase int

const (
	preEvict evictionPhase = iota
	evicting
	postEvict
	done
)

type phase struct {
	phase evictionPhase
	start time.Time
	end   time.Time

	// phase specific information
	// phase: evicting
	numberOfPings           int
	numberOfSuccessfulPings int
}

type selfEvict struct {
	node    *Node
	options SelfEvictOptions
	logger  bark.Logger
	phases  []*phase

	hooks map[string]SelfEvictHook
}

func newSelfEvict(node *Node, options SelfEvictOptions) *selfEvict {
	return &selfEvict{
		node:    node,
		options: options,
		logger:  node.logger,

		hooks: make(map[string]SelfEvictHook),
	}
}

// RegisterSelfEvictHook registers a named pre/post eviction hook pair.
func (s *selfEvict) RegisterSelfEvictHook(hook SelfEvictHook) error {
	name := hook.Name()

	_, hasHook := s.hooks[name]
	if hasHook {
		return ErrDuplicateHook
	}

	s.hooks[name] = hook

	return nil
}

func (s *selfEvict) SelfEvict() error {
	if s.currentPhase() != nil {
		return ErrEvictionInProgress
	}

	s.logger.Info("ringpop is initiating self eviction sequence")

	// this starts the shutdown sequence, when it returns the complete sequence
	// has been executed
	s.preEvict()

	return nil
}

func (s *selfEvict) preEvict() {
	s.transitionTo(preEvict)
	s.runHooks(SelfEvictHook.PreEvict)

	// next phase
	s.evict()
}

func (s *selfEvict) evict() {
	phase := s.transitionTo(evicting)
	s.node.memberlist.SetLocalStatus(Faulty)

	if s.options.PingDisable == false {
		numberOfPingableMembers := s.node.memberlist.NumPingableMembers()
		maxNumberOfPings := int(math.Ceil(float64(numberOfPingableMembers) * s.options.PingRatio))

		// final number of members to ping should not exceed any of:
		numberOfPings := min(
			s.node.disseminator.maxP, // the piggyback counter
			numberOfPingableMembers,  // the number of members we can ping
			maxNumberOfPings,         // a configured percentage of members
		)

		phase.numberOfPings = numberOfPings
		phase.numberOfSuccessfulPings = 0

		// select the members we are going to ping
		targets := s.node.memberlist.RandomPingableMembers(numberOfPings, nil)

		s.logger.WithFields(bark.Fields{
			"numberOfPings": phase.numberOfPings,
			"targets":       targets,
		}).Debug("starting proactive gossip on self evict")

		for _, target := range targets {
			// TODO: test parallel pinging
			_, err := sendPing(s.node, target.address(), s.node.pingTimeout)
			if err != nil {
				phase.numberOfSuccessfulPings++
			}
		}

		s.logger.WithFields(bark.Fields{
			"numberOfPings":           phase.numberOfPings,
			"numberOfSuccessfulPings": phase.numberOfSuccessfulPings,
		}).Debug("finished proactive gossip on self evict")
	}

	// next phase
	s.postEvict()
}

func (s *selfEvict) postEvict() {
	s.transitionTo(postEvict)
	s.runHooks(SelfEvictHook.PostEvict)

	// next phase
	s.done()
}

func (s *selfEvict) done() {
	s.transitionTo(done)
	s.logger.Info("ringpop self eviction done")
	//TODO emit total timing
}

func (s *selfEvict) transitionTo(newPhase evictionPhase) *phase {
	p := &phase{
		phase: newPhase,
		start: s.node.clock.Now(),
	}

	previousPhase := s.currentPhase()
	s.phases = append(s.phases, p)

	if previousPhase != nil {
		previousPhase.end = p.start
	}

	s.logger.WithFields(bark.Fields{
		"newPhase": p,
		"oldPhase": previousPhase,
	}).Debug("ringpop self eviction phase-transitioning")

	return p
}

func (s *selfEvict) currentPhase() *phase {
	if len(s.phases) > 0 {
		return s.phases[len(s.phases)-1]
	}
	return nil
}

func (s *selfEvict) runHooks(dispatch hookFn) {
	var wg sync.WaitGroup

	wg.Add(len(s.hooks))
	for name, hook := range s.hooks {
		go func(name string, hook SelfEvictHook) {
			defer wg.Done()

			s.logger.WithField("hook", name).Debug("ringpop self eviction running hook")
			dispatch(hook)
			s.logger.WithField("hook", name).Debug("ringpop self eviction hook done")
		}(name, hook)
	}
	wg.Wait()
	s.logger.Debug("ringpop self eviction done running hooks")
}

// min returns the lowest integer and is defined because golang only has a min
// function for floats and not for ints.
func min(first int, rest ...int) int {
	m := first
	for _, value := range rest {
		if value < m {
			m = value
		}
	}
	return m
}
