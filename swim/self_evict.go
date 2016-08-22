package swim

import (
	"errors"
	"sync"
	"time"

	"github.com/uber-common/bark"
)

var (
	// ErrDuplicateHook is returned when a hook with the given name has already
	// been registered.
	ErrDuplicateHook = errors.New("expected hook name to be unique")

	// ErrMissingHook is returned when both the PreEvict and the PostEvict hooks
	// are unset on the hooks struct.
	ErrMissingHook = errors.New("missing both PreEvict and PostEvict on SelfEvictHooks")

	// ErrEvictionInProgress is returned when ringpop is already in the process
	// of evicting
	ErrEvictionInProgress = errors.New("ringpop is already executing an eviction")
)

// SelfEvict defines the functions that interact with the self eviction of nodes
// from the membership prior to shutting down
type SelfEvict interface {
	RegisterSelfEvictHooks(hooks SelfEvictHooks) error
	SelfEvict() error
}

// EvictionHook is a function that can be called during the eviction process of
// ringpop.
type EvictionHook func()

// SelfEvictHooks is used when configuring a named eviction hook. It contains
// the name of the hook and both the hook to be executed pre eviction and post
// eviction.
type SelfEvictHooks struct {
	// Name is the name of the hook to register. Every name can only be
	// registered once.
	Name string

	// PreEvict is the hook that is called before the node is self evicted
	PreEvict EvictionHook

	// PostEvict is the hook that is called after the node is self evicted
	PostEvict EvictionHook
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
}

type selfEvict struct {
	node   *Node
	logger bark.Logger
	phases []*phase

	preHooks  map[string]EvictionHook
	postHooks map[string]EvictionHook
}

func newSelfEvict(node *Node) *selfEvict {
	return &selfEvict{
		node:   node,
		logger: node.logger,

		preHooks:  make(map[string]EvictionHook),
		postHooks: make(map[string]EvictionHook),
	}
}

// RegisterSelfEvictHooks registers a named pre/post eviction hook pair.
func (s *selfEvict) RegisterSelfEvictHooks(hooks SelfEvictHooks) error {

	_, hasPreHook := s.preHooks[hooks.Name]
	_, hasPostHook := s.postHooks[hooks.Name]
	if hasPreHook || hasPostHook {
		return ErrDuplicateHook
	}

	if hooks.PreEvict == nil && hooks.PostEvict == nil {
		return ErrMissingHook
	}

	if hooks.PreEvict != nil {
		s.preHooks[hooks.Name] = hooks.PreEvict
	}

	if hooks.PostEvict != nil {
		s.postHooks[hooks.Name] = hooks.PostEvict
	}

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
	s.runHooks(s.preHooks)

	// next phase
	s.evict()
}

func (s *selfEvict) evict() {
	s.transitionTo(evicting)
	s.node.memberlist.SetLocalStatus(Faulty)

	// next phase
	s.postEvict()
}

func (s *selfEvict) postEvict() {
	s.transitionTo(postEvict)
	s.runHooks(s.postHooks)

	// next phase
	s.done()
}

func (s *selfEvict) done() {
	s.transitionTo(done)
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

func (s *selfEvict) runHooks(hooks map[string]EvictionHook) {
	var wg sync.WaitGroup

	wg.Add(len(hooks))
	for name, hook := range hooks {
		go func(name string, hook EvictionHook) {
			defer wg.Done()

			s.logger.WithField("hook", name).Debug("ringpop self eviction running hook")
			hook()
			s.logger.WithField("hook", name).Debug("ringpop self eviction hook done")
		}(name, hook)
	}
	wg.Wait()
	s.logger.Debug("ringpop self eviction done running hooks")
}
