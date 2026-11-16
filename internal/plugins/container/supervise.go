// Design: docs/architecture/container/runtime.md -- the supervisor: the desired set, container state and the seams it drives
// Related: config.go -- the config the supervisor is handed
// Related: storeseam.go -- the image store's answers and notices
// Related: participant.go -- the created and ended calls

package container

import (
	"context"
	"errors"
	"log/slog"
	"sort"
	"sync"

	"github.com/ze-software/ze/internal/core/opererr"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// State is a container's state as an operator sees it. The six named values
// are the only ones a listing shows. Zero is StateUnspecified, which no
// container holds.
type State uint8

const (
	StateUnspecified State = iota
	// StateWaitingImage: the store answered that the image is absent.
	StateWaitingImage
	// StateRunning: the entrypoint was released.
	StateRunning
	// StateRestarting: the entrypoint ended, the policy restarts it, and the
	// delay is running.
	StateRestarting
	// StateStopped: the entrypoint ended and the policy keeps it stopped.
	StateStopped
	// StateDisabled: the config's enabled leaf is false.
	StateDisabled
	// StateFailed: a start step failed five times in a row.
	StateFailed
)

// stateNames holds the word of each state, indexed by its value.
var stateNames = [...]string{
	StateUnspecified:  "unspecified",
	StateWaitingImage: "waiting-image",
	StateRunning:      "running",
	StateRestarting:   "restarting",
	StateStopped:      "stopped",
	StateDisabled:     "disabled",
	StateFailed:       "failed",
}

// String answers the word an operator reads for the state. It is for a
// listing and a log line: compare states by value.
func (s State) String() string {
	if int(s) < len(stateNames) {
		return stateNames[s]
	}
	return stateNames[StateUnspecified]
}

// startRequest is what the supervisor hands the runtime library to create
// one container: its config, the store's answer and the file mounts the
// participants contributed.
type startRequest struct {
	instance instanceConfig
	image    Ready
	mounts   []FileMount
}

// runtimeLibrary is the supervisor's view of the patched runtime library.
// The supervisor orders the steps of a start and an end through it, so a
// test proves that order with a library that creates nothing.
type runtimeLibrary interface {
	// create builds the container and starts its init, which blocks before
	// the entrypoint. The caller MUST call destroy on the result, also when
	// a later step of the start fails.
	create(ctx context.Context, request startRequest) (createdContainer, error)
}

// createdContainer is one container the library created and has not
// destroyed.
type createdContainer interface {
	// initProcessID is the process id of the container's init, on the host.
	initProcessID() int
	// networkNamespacePath names the container's own network namespace. It
	// is empty for a container that shares the host's.
	networkNamespacePath() string
	// release lets init execute the entrypoint. It MUST be called after
	// every created call returned nil, and at most once.
	release() error
	// destroy kills what is left of the container and removes its control
	// group. It MUST be called once, after release or in its place.
	destroy() error
}

// supervisor holds the containers the config asks for and their state, and
// drives the store, the participants and the runtime library for each.
//
// install records the desired set and returns: it never waits for a start.
// Safe for concurrent use.
type supervisor struct {
	log          *slog.Logger
	library      runtimeLibrary
	store        Store
	participants []Participant

	mu sync.Mutex
	// desired holds the config of every configured container, by name.
	desired map[string]instanceConfig
	// states holds the state of each container that has one. An enabled
	// container no start was made for has no entry.
	states map[string]State
}

// newSupervisor returns a supervisor with no container. library is nil until
// a start can be made. buildStore is nil in a build with no image store;
// otherwise the supervisor builds its store with itself as the receiver of
// the store's notices.
func newSupervisor(log *slog.Logger, library runtimeLibrary, buildStore StoreFactory, participants []Participant) *supervisor {
	s := &supervisor{
		log:          log,
		library:      library,
		participants: participants,
		desired:      map[string]instanceConfig{},
		states:       map[string]State{},
	}
	if buildStore != nil {
		s.store = buildStore(s)
	}
	return s
}

// install makes cfg the desired set. It records what the config asks for and
// returns before any container starts or stops. A container the config no
// longer holds leaves the set, and a config with no container empties it.
func (s *supervisor) install(cfg containerConfig) error {
	desired := make(map[string]instanceConfig, len(cfg.instances))
	states := make(map[string]State, len(cfg.instances))

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, instance := range cfg.instances {
		desired[instance.name] = instance
		if !instance.enabled {
			states[instance.name] = StateDisabled
			continue
		}
		previous, known := s.states[instance.name]
		if !known {
			continue
		}
		// A container the config enables again has left the disabled state
		// and holds no other until a start is made for it.
		if previous == StateDisabled {
			continue
		}
		states[instance.name] = previous
	}
	s.desired, s.states = desired, states
	s.log.Debug("container: desired set installed", "containers", len(desired))
	return nil
}

// instance answers the config of the named container, and false when the
// config holds no container of that name.
func (s *supervisor) instance(name string) (instanceConfig, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	instance, configured := s.desired[name]
	return instance, configured
}

// state answers the state of the named container. The second result is false
// when the container is not configured, and when it is configured and no
// start was made for it yet: the first result MUST NOT be read then.
func (s *supervisor) state(name string) (State, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	state, known := s.states[name]
	return state, known
}

// namesIn answers the names of the containers in state, sorted.
func (s *supervisor) namesIn(state State) []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	var names []string
	for name, held := range s.states {
		if held == state {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

// start makes one start of the named container: resolve, contribute, create,
// the created calls, release. It answers the state the container holds after
// it: running, or waiting-image when the store answered that the image is
// absent, which is no error. The state MUST NOT be read when the error is
// not nil.
//
// The sequence is not built. It refuses with the refusal the rule set gives
// for the same container, so no caller reads a start that did not happen as
// one that did.
func (s *supervisor) start(_ context.Context, name string) (State, error) {
	if _, configured := s.instance(name); !configured {
		return StateUnspecified, unknownInstance(name)
	}
	return StateUnspecified, runtimeNotBuilt(name)
}

// stop ends the named container: its stop signal, the kill at its stop
// timeout, the cleanup and the ended calls.
//
// The sequence is not built, and it refuses as start does.
func (s *supervisor) stop(_ context.Context, name string) error {
	if _, configured := s.instance(name); !configured {
		return unknownInstance(name)
	}
	return runtimeNotBuilt(name)
}

// restart answers the restart command for the named container, with the
// state the container holds after it. A disabled container is refused with
// the statement that enables it: a restart would start what the
// configuration says to keep stopped. The state MUST NOT be read when the
// error is not nil.
func (s *supervisor) restart(ctx context.Context, name string) (State, error) {
	instance, configured := s.instance(name)
	if !configured {
		return StateUnspecified, unknownInstance(name)
	}
	if !instance.enabled {
		return StateUnspecified, opererr.New(
			"container "+name+" is disabled in the configuration",
			"a restart would start what the configuration says to keep stopped",
			"set container instance "+name+" enabled true",
		)
	}
	return s.start(ctx, name)
}

// drain stops every running container before the daemon stops. It is the
// plugin's stop drain handler: the one call of a daemon stop that waits for
// the plugin. It returns when every stop ended or ctx did.
func (s *supervisor) drain(ctx context.Context, _ rpc.DrainInput) error {
	var failures []error
	for _, name := range s.namesIn(StateRunning) {
		if err := s.stop(ctx, name); err != nil {
			failures = append(failures, err)
		}
	}
	return errors.Join(failures...)
}

// ImagePresent is the store's notice that the image of reference is now
// present. The start sequence is not built, so no container waits for an
// image and none is started.
func (s *supervisor) ImagePresent(_ context.Context, reference string) []string {
	s.log.Debug("container: image present", "reference", reference, "waiting", len(s.namesIn(StateWaitingImage)))
	return nil
}

// RestartOnNewGeneration is the store's request to move the named container
// to its new image. It refuses as start does.
func (s *supervisor) RestartOnNewGeneration(ctx context.Context, name string) error {
	_, err := s.start(ctx, name)
	return err
}

// StopThenStart is the store's request to stop the named container and start
// it again. It refuses as start does.
func (s *supervisor) StopThenStart(ctx context.Context, name string) error {
	_, err := s.start(ctx, name)
	return err
}

// unknownInstance refuses a command that names a container the config does
// not hold.
func unknownInstance(name string) error {
	return opererr.New(
		"no container named "+name+" is configured",
		"the command acts on configured containers only",
		"ze show config",
	)
}
