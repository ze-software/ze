// Design: docs/architecture/container/runtime.md -- the supervisor: the desired set, container state and the seams it drives
// Related: config.go -- the config the supervisor is handed
// Related: storeseam.go -- the image store's answers and notices
// Related: participant.go -- the created and ended calls
// Related: library_linux.go -- the runtime library the supervisor drives on Linux

package container

import (
	"context"
	"errors"
	"log/slog"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

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
	StateUnspecified:  textUnspecified,
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

// The bounds of the steps a worker makes. A participant's call is bounded so
// a participant that hangs fails the start rather than holding it. A stop is
// bounded by the container's own stop timeout, then killCleanupTimeout
// (stop.go).
const (
	participantCallTimeout = 10 * time.Second
	startTimeout           = 60 * time.Second
)

// startRequest is what the supervisor hands the runtime library to create
// one container: its config, the store's answer, the file mounts the
// participants contributed and the buffer its output goes to. The library
// takes it by pointer and MUST NOT keep it after create returns; it keeps the
// buffer, which the readers of the container's output fill until its end.
// output MUST NOT be nil.
type startRequest struct {
	instance instanceConfig
	ready    Ready
	mounts   []FileMount
	output   *outputBuffer
}

// runtimeLibrary is the supervisor's view of the patched runtime library.
// The supervisor orders the steps of a start and an end through it, so a
// test proves that order with a library that creates nothing.
type runtimeLibrary interface {
	// create builds the container and starts its init, which blocks before
	// the entrypoint. It MUST NOT keep request after it returns. On an error
	// it MUST leave nothing behind: there is no container to destroy. On
	// success the caller MUST call destroy on the result, also when a later
	// step of the start fails.
	create(ctx context.Context, request *startRequest) (createdContainer, error)
	// sweep removes every container a previous run of the plugin left: it
	// kills the processes of each leftover control group, removes the group,
	// unmounts the root and removes the library's state directory. It answers
	// one entry for each container it found, sorted by name. It MUST be called
	// once, before any create.
	sweep() []leftover
}

// leftover is one container a previous run of the plugin left behind, as the
// sweep found it. err is nil when the sweep removed all of it, and names what
// the kernel refused to remove otherwise.
type leftover struct {
	name string
	err  error
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
	// signal sends sig to the container's init. It returns when the signal is
	// sent; wait answers when init is gone. A container whose init already
	// exited answers an error, which the stop does not act on: its wait
	// decides.
	signal(sig syscall.Signal) error
	// wait blocks until the container's init has exited and answers its exit
	// code: the code it exited with, or 128 plus the signal that killed it.
	// It MUST be called once, after release returned nil, and before destroy.
	wait() (int, error)
	// kill ends every process of the container at once, through its control
	// group. It returns when the kill is sent; wait answers when init is gone.
	kill() error
	// destroy kills what is left of the container and removes its control
	// group and its root. It MUST be called once, after wait returned or in
	// place of release.
	destroy() error
}

// runningContainer is a container whose entrypoint was released and whose
// cleanup has not ended. Its watcher, the one goroutine of its lifetime,
// waits for init to exit, cleans up and closes ended.
type runningContainer struct {
	created     createdContainer
	releaseHold func()
	// stopSignal and stopTimeout are the stop of this run of the container,
	// fixed when it started, so a stop after the config removed the container
	// still has them.
	stopSignal  syscall.Signal
	stopTimeout time.Duration
	// digest is the image this run of the container runs, and since is when
	// its entrypoint was released: what the container listing shows of it.
	digest string
	since  time.Time
	// output is the buffer this run's output goes to, which the watcher
	// quotes when the container ends on its own.
	output *outputBuffer
	// stopRequested is set before a stop signals or kills the container, so
	// its watcher tells an end Ze asked for from one the entrypoint made on
	// its own.
	stopRequested atomic.Bool
	// ended is closed by the watcher once the cleanup ended.
	ended chan struct{}
}

// tunableClaim is the claim on the kernel tunables a container's own network
// namespace needs. The kernel reads them when it creates the namespace, so
// claim MUST answer nil only once they are in force, and every create runs
// after one. release hands them back once no configured container is
// enabled. Both MUST be called from a worker, never from a config callback:
// each one is a call to the engine.
type tunableClaim interface {
	claim(ctx context.Context) error
	release(ctx context.Context) error
}

// supervisor holds the containers the config asks for and their state, and
// drives the store, the participants and the runtime library for each.
//
// install records the desired set and returns: it never waits for a start.
// apply installs, then, once begin was called, wakes the worker of each
// container concerned, which starts or stops it. begin MUST be called once,
// when the engine reports the plugin started: the workers' claims are calls
// to the engine, which it refuses before then. start and stop make one
// sequence each, serialized per container by that container's lifecycle
// lock. close ends every worker and MUST be called once, when the engine
// entry returns. Safe for concurrent use.
type supervisor struct {
	log          *slog.Logger
	library      runtimeLibrary
	store        Store
	participants []Participant
	// siblings holds the siblings' commands and doctor checks the plugin
	// answers. newSupervisor sets an empty set; the engine entry sets the
	// siblings this build registered.
	siblings *siblingRegistry
	// tunables is nil for a supervisor that makes no kernel claim: the guard
	// of a test supervisor, whose library creates no namespace. The engine
	// entry always sets it.
	tunables tunableClaim

	// tunablesMu serializes a claim and the create that relies on it against
	// a release, so a release never withdraws the tunables between the two.
	// It is taken before mu, never after.
	tunablesMu sync.Mutex
	// claimed is whether the tunables are claimed. Guarded by tunablesMu.
	claimed bool

	mu sync.Mutex
	// desired holds the config of every configured container, by name.
	desired map[string]instanceConfig
	// states holds the state of each container that has one. An enabled
	// container no start was made for has no entry.
	states map[string]State
	// waiting holds, for each container in the waiting-image state, the
	// store's absent answer: the reference it said is absent, which the present
	// notice is matched to, and why it is absent, which the listing shows.
	waiting map[string]Absent
	// running holds each container whose entrypoint was released and whose
	// cleanup has not ended.
	running map[string]*runningContainer
	// problems holds what went wrong with a configured container, one entry
	// for each container and kind, with the detail of the kinds that carry
	// one. A container in the failed state holds a problemFailed entry, the
	// step that failed and its fix, for exactly as long as it holds the
	// state, and one stopped after a non-zero exit of its own a
	// problemStopped entry, its status and its last output, the same way. It
	// is what the runtime doctor checks read (doctor.go). It is
	// bounded by the number of configured containers times the number of
	// kinds.
	problems map[problemRecord]problemDetail
	// lifecycles holds the lock that serializes the start and stop sequences
	// of each container. It is bounded by the names configured during this
	// run of the plugin.
	lifecycles map[string]*sync.Mutex
	// workers holds the wake channel of each container's worker. A worker
	// ends when its container left the config and stopped.
	workers map[string]chan struct{}
	// leftovers holds, for each container a previous run left and the sweep
	// could not remove whole, what was left. The configuration does not
	// decide it: a leftover may name a container no longer configured. An
	// entry lasts until that container runs again, or for this run of the
	// plugin. It is bounded by the containers the sweep found.
	leftovers map[string]problemDetail
	// outputs holds the output buffer of each configured container
	// (output.go). install drops the buffer of a container the config no
	// longer holds.
	outputs outputSet
	// started is set by begin; apply wakes no worker before it.
	started bool
	// stopping is set when the daemon's stop reached the plugin, by the drain
	// or by the engine connection closing. No start is made after it.
	stopping bool
	// closed is set by close; no worker starts after it.
	closed bool
}

// problemRecord is one problem of one container.
type problemRecord struct {
	name string
	kind problemKind
}

// newSupervisor returns a supervisor with no container. library is nil in a
// build that cannot run a container. buildStore is nil in a build with no
// image store; otherwise the supervisor builds its store with itself as the
// receiver of the store's notices.
func newSupervisor(log *slog.Logger, library runtimeLibrary, buildStore StoreFactory, participants []Participant) *supervisor {
	s := &supervisor{
		log:          log,
		library:      library,
		participants: participants,
		siblings:     &siblingRegistry{},
		desired:      map[string]instanceConfig{},
		states:       map[string]State{},
		waiting:      map[string]Absent{},
		running:      map[string]*runningContainer{},
		problems:     map[problemRecord]problemDetail{},
		leftovers:    map[string]problemDetail{},
		lifecycles:   map[string]*sync.Mutex{},
		workers:      map[string]chan struct{}{},
	}
	if buildStore != nil {
		s.store = buildStore(s)
	}
	return s
}

// install makes cfg the desired set. It records what the config asks for and
// returns before any container starts or stops. A container the config no
// longer holds leaves the set with its recorded problems, and a config with
// no container empties it. A container that is still running keeps its entry
// in the running set until its worker stopped it.
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
	for record := range s.problems {
		if _, configured := desired[record.name]; !configured {
			delete(s.problems, record)
			continue
		}
		// The failed and the stopped record last as long as their state: a
		// container the config disables leaves both.
		state, followsState := recordState(record.kind)
		if !followsState {
			continue
		}
		if states[record.name] != state {
			delete(s.problems, record)
		}
	}
	for name := range s.waiting {
		instance, configured := desired[name]
		if !configured {
			delete(s.waiting, name)
			continue
		}
		if !instance.enabled {
			delete(s.waiting, name)
			continue
		}
		// A container whose config now names another image waits for the old
		// one no longer: its worker's next converge resolves the new one.
		if instance.image != s.desired[name].image {
			delete(s.waiting, name)
		}
	}
	s.desired, s.states = desired, states
	s.outputs.keepOnly(desired)
	s.log.Debug("container: desired set installed", "containers", len(desired))
	return nil
}

// apply is the config transaction's apply: it installs cfg, then wakes the
// worker of every container the config holds or that is still running, so
// each one is started or stopped by its own worker. It returns before any
// container starts or stops. The boot configure arrives before begin, so it
// only installs: begin wakes the workers once the plugin has started.
func (s *supervisor) apply(cfg containerConfig) error {
	if err := s.install(cfg); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return nil
	}
	if !s.started {
		return nil
	}
	s.wakeAllLocked()
	return nil
}

// begin is the engine's notice that the plugin started: from now on a worker
// may call the engine. It wakes the worker of every configured container, so
// the containers of the boot configure start now.
func (s *supervisor) begin() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.started = true
	if s.closed {
		return
	}
	s.wakeAllLocked()
}

// wakeAllLocked wakes the worker of every container the config holds or that
// is still running. MUST be called with s.mu held, and not after close.
func (s *supervisor) wakeAllLocked() {
	for name := range s.desired {
		s.wakeLocked(name)
	}
	for name := range s.running {
		s.wakeLocked(name)
	}
}

// wakeLocked wakes the worker of name, and starts it when it has none. The
// wake channel holds one pending wake: a converge reads the config when it
// runs, so wakes that arrive while one is pending are one wake. MUST be
// called with s.mu held, and not after close.
func (s *supervisor) wakeLocked(name string) {
	wake, exists := s.workers[name]
	if !exists {
		wake = make(chan struct{}, 1)
		s.workers[name] = wake
		go s.work(name, wake)
	}
	select {
	case wake <- struct{}{}:
	default:
	}
}

// work is the worker of one container, the goroutine of the container's
// life in the config. Each wake converges the container to its config. It
// ends when its container is neither configured nor running, or when close
// closed its channel.
func (s *supervisor) work(name string, wake chan struct{}) {
	for range wake {
		s.converge(name)
		s.releaseTunablesWhenIdle()
		s.mu.Lock()
		_, configured := s.desired[name]
		_, running := s.running[name]
		if !configured && !running && s.workers[name] == wake {
			delete(s.workers, name)
			s.mu.Unlock()
			return
		}
		s.mu.Unlock()
	}
}

// converge makes one step toward what the config asks for the container: a
// start when it is enabled and neither running nor waiting for its image, a
// stop when it is running and the config removed or disabled it.
func (s *supervisor) converge(name string) {
	if s.isStopping() {
		// The drain or the plugin's exit stops what runs; nothing starts or
		// stops here once the daemon's stop began.
		return
	}
	instance, configured := s.instance(name)
	if configured && instance.enabled {
		if s.isRunning(name) {
			return
		}
		if s.isWaiting(name) {
			return
		}
		ctx, cancel := context.WithTimeout(context.Background(), startTimeout)
		defer cancel()
		_, err := s.start(ctx, name)
		if err == nil {
			return
		}
		// A start the daemon's stop refused while this worker waited for the
		// lifecycle lock is no failure of the container.
		if s.isStopping() {
			return
		}
		s.setFailed(name, err)
		return
	}
	if !s.isRunning(name) {
		return
	}
	// The stop bounds itself: the container's stop timeout, then the kill and
	// its cleanup.
	if err := s.stop(context.Background(), name); err != nil {
		s.log.Warn("container: the stop did not end", "name", name, "error", err, "code", codeCleanupIncomplete)
	}
}

// close ends every worker. It stops no container: the drain stops them before
// a daemon stop, and killRunning, called before close, kills what still runs;
// close only ends the goroutines of this run of the plugin. MUST be called
// once, after the last apply.
func (s *supervisor) close() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	for name, wake := range s.workers {
		close(wake)
		delete(s.workers, name)
	}
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

func (s *supervisor) isRunning(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, running := s.running[name]
	return running
}

func (s *supervisor) isStopping() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stopping
}

func (s *supervisor) isWaiting(name string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, waiting := s.waiting[name]
	return waiting
}

// lifecycle answers the lock that serializes the sequences of name.
func (s *supervisor) lifecycle(name string) *sync.Mutex {
	s.mu.Lock()
	defer s.mu.Unlock()
	lock, exists := s.lifecycles[name]
	if !exists {
		lock = &sync.Mutex{}
		s.lifecycles[name] = lock
	}
	return lock
}

// setState records state for a configured container. A container that left
// the config keeps no state. A container that leaves the failed state, by a
// start that went as far as running or waiting for its image, loses its
// failed record with it.
func (s *supervisor) setState(name string, state State) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, configured := s.desired[name]; !configured {
		return
	}
	s.states[name] = state
	if state != StateWaitingImage {
		delete(s.waiting, name)
	}
	if state != StateFailed {
		delete(s.problems, problemRecord{name: name, kind: problemFailed})
	}
	if state != StateStopped {
		delete(s.problems, problemRecord{name: name, kind: problemStopped})
	}
	// A container that runs again was created over what a previous run left,
	// so nothing of that leftover stands in its way any more.
	if state == StateRunning {
		delete(s.leftovers, name)
	}
}

// setFailed records a failed start of a configured container: the failed
// state, and the failed record the doctor check reports, with the step that
// failed, its cause and its fix. A container the config disabled or removed
// meanwhile keeps the state the config gave it.
func (s *supervisor) setFailed(name string, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	instance, configured := s.desired[name]
	if !configured {
		return
	}
	if !instance.enabled {
		return
	}
	detail := failureDetail(name, err)
	s.states[name] = StateFailed
	s.problems[problemRecord{name: name, kind: problemFailed}] = detail
	s.log.Warn("container: the start failed", "name", name, "error", err, "fix", detail.fix, "code", codeFailed)
}

// failureDetail answers the detail of a failed start: the step that failed
// and why, then the fix the refusal names. Every refusal of a start step is
// an operator error, found through any join or wrap around it; an error that
// is none, a Ze defect, is quoted whole and takes the restart command.
func failureDetail(name string, err error) problemDetail {
	var refusal *opererr.Error
	if !errors.As(err, &refusal) {
		return problemDetail{cause: err.Error(), fix: restartCommand(name)}
	}
	cause := refusal.Problem() + "; " + refusal.Why()
	if underlying := refusal.Unwrap(); underlying != nil {
		cause += ": " + underlying.Error()
	}
	return problemDetail{cause: cause, fix: refusal.Fix()}
}

// recordProblem notes that the named container had a problem of kind, for
// the runtime doctor check of that kind to report, with the zero detail: the
// finding takes the check's own text and fix. Recording the same problem
// again changes nothing. A container the config does not hold is refused: its
// record would outlive every config change that could remove it. A failed
// start is recorded by setFailed, which carries its detail.
func (s *supervisor) recordProblem(name string, kind problemKind) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, configured := s.desired[name]; !configured {
		return unknownInstance(name)
	}
	s.problems[problemRecord{name: name, kind: kind}] = problemDetail{}
	return nil
}

// namedProblem is one recorded problem of one container, as a doctor check
// reads it.
type namedProblem struct {
	name   string
	detail problemDetail
}

// problemsOf answers the recorded problems of kind, sorted by container name.
// An incomplete cleanup also answers each leftover of a previous run the sweep
// could not remove whole, configured or not, unless a record of this run
// already names that container.
func (s *supervisor) problemsOf(kind problemKind) []namedProblem {
	s.mu.Lock()
	defer s.mu.Unlock()
	var records []namedProblem
	for record, detail := range s.problems {
		if record.kind == kind {
			records = append(records, namedProblem{name: record.name, detail: detail})
		}
	}
	if kind == problemCleanupIncomplete {
		for name, detail := range s.leftovers {
			if _, recorded := s.problems[problemRecord{name: name, kind: kind}]; recorded {
				continue
			}
			records = append(records, namedProblem{name: name, detail: detail})
		}
	}
	slices.SortFunc(records, func(a, b namedProblem) int { return strings.Compare(a.name, b.name) })
	return records
}

// start makes one start of the named container: resolve, hold, contribute,
// create, the created calls, release. It answers the state the container
// holds after it: running, or waiting-image when the store answered that the
// image is absent, which is no error. A container that already runs is left
// running. The state MUST NOT be read when the error is not nil.
func (s *supervisor) start(ctx context.Context, name string) (State, error) {
	lifecycle := s.lifecycle(name)
	lifecycle.Lock()
	defer lifecycle.Unlock()
	return s.startLocked(ctx, name)
}

// startLocked is start for a caller that holds the lifecycle lock of name.
func (s *supervisor) startLocked(ctx context.Context, name string) (State, error) {
	instance, configured := s.instance(name)
	if !configured {
		return StateUnspecified, unknownInstance(name)
	}
	if s.isRunning(name) {
		return StateRunning, nil
	}
	if s.isStopping() {
		return StateUnspecified, opererr.New(
			"container "+name+" is not started: the daemon is stopping",
			"a daemon stop stops every container, and one started now would outlive it",
			"ze show doctor",
		)
	}
	if s.library == nil {
		return StateUnspecified, noRuntime(name)
	}
	if s.store == nil {
		return StateUnspecified, noImageStore(name)
	}
	answer, err := s.store.Resolve(ctx, ResolveRequest{Instance: name, Reference: instance.image})
	if err != nil {
		return StateUnspecified, opererr.Wrap(err,
			"container "+name+": the image store could not answer",
			"the container's root comes from the store, so no start is made without its answer",
			"ze request container restart name "+name,
		)
	}
	switch resolved := answer.(type) {
	case Absent:
		s.mu.Lock()
		s.waiting[name] = resolved
		s.mu.Unlock()
		s.setState(name, StateWaitingImage)
		return StateWaitingImage, nil
	case Ready:
		return s.run(ctx, instance, resolved)
	}
	// Resolution is sealed to Ready and Absent, so only a nil answer with no
	// error reaches here: a store defect, refused rather than read as absent.
	return StateUnspecified, errors.New("container " + name + ": the image store answered neither ready nor absent")
}

// run starts a container whose image the store holds: the hold, the
// contributions, the create, the created calls and the release. A failure
// after the create destroys the container, makes the ended calls and releases
// the hold, so a failed start leaves nothing behind.
func (s *supervisor) run(ctx context.Context, instance instanceConfig, ready Ready) (State, error) {
	name := instance.name
	signal, err := stopSignalOf(name, instance.stopSignal, ready.StopSignal)
	if err != nil {
		return StateUnspecified, err
	}
	releaseHold := s.store.Hold(name, ready)
	mounts, err := s.contributions(ctx, name)
	if err != nil {
		releaseHold()
		return StateUnspecified, err
	}
	request := &startRequest{instance: instance, ready: ready, mounts: mounts, output: s.outputs.of(name)}
	created, err := s.createWithTunables(ctx, request)
	if err != nil {
		releaseHold()
		return StateUnspecified, err
	}
	if path := created.networkNamespacePath(); path != "" {
		call := CreatedCall{Instance: name, InitProcessID: created.initProcessID(), NetworkNamespacePath: path}
		for _, participant := range s.participants {
			if err := s.created(ctx, participant, call); err != nil {
				s.cleanup(name, created, releaseHold)
				return StateUnspecified, participantFailed(name, participant.Name(), "the created call", err)
			}
		}
	}
	if err := created.release(); err != nil {
		s.cleanup(name, created, releaseHold)
		return StateUnspecified, opererr.Wrap(err,
			"container "+name+": its entrypoint could not be released",
			"init stopped before it executed the entrypoint, so the container was removed",
			"ze request container restart name "+name,
		)
	}
	running := &runningContainer{
		created:     created,
		releaseHold: releaseHold,
		stopSignal:  signal,
		stopTimeout: instance.stopTimeout,
		output:      request.output,
		digest:      ready.Digest,
		since:       time.Now(),
		ended:       make(chan struct{}),
	}
	s.mu.Lock()
	s.running[name] = running
	s.mu.Unlock()
	s.setState(name, StateRunning)
	s.log.Info("container: running", "name", name, "pid", created.initProcessID(), "digest", ready.Digest)
	go s.watch(name, running)
	return StateRunning, nil
}

// createWithTunables claims the kernel tunables, then creates the container,
// under the tunables lock: the kernel reads them when the create makes the
// container's network namespace, so a release MUST NOT fall between the two.
// A claim that fails refuses the create.
func (s *supervisor) createWithTunables(ctx context.Context, request *startRequest) (createdContainer, error) {
	s.tunablesMu.Lock()
	defer s.tunablesMu.Unlock()
	if s.tunables != nil {
		if err := s.tunables.claim(ctx); err != nil {
			return nil, err
		}
		s.claimed = true
	}
	return s.library.create(ctx, request)
}

// releaseTunablesWhenIdle hands the kernel tunables back once they are
// claimed and the config holds no enabled container. A container that is
// enabled keeps them claimed, also while it fails or waits for its image, so
// its next start creates its namespace under them.
func (s *supervisor) releaseTunablesWhenIdle() {
	s.tunablesMu.Lock()
	defer s.tunablesMu.Unlock()
	if !s.claimed {
		return
	}
	if s.anyEnabled() {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), participantCallTimeout)
	defer cancel()
	if err := s.tunables.release(ctx); err != nil {
		// Nothing is wrong for an operator: a tunable left claimed hides the
		// fallback tunnel devices from every new namespace, and the sysctl
		// plugin restores the kernel's value when the daemon stops.
		s.log.Info("container: the kernel tunables stay claimed until the daemon stops", "error", err)
		return
	}
	s.claimed = false
}

// anyEnabled answers whether the config holds an enabled container.
func (s *supervisor) anyEnabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, instance := range s.desired {
		if instance.enabled {
			return true
		}
	}
	return false
}

// contributions asks each participant for its file mounts and judges each one
// (checkFileMount). A participant that fails, or contributes a mount the
// runtime does not admit, fails the start with its name: a mount the
// description cannot hold is never dropped in silence.
func (s *supervisor) contributions(ctx context.Context, name string) ([]FileMount, error) {
	var admitted []FileMount
	for _, participant := range s.participants {
		mounts, err := s.contribute(ctx, participant, name)
		if err != nil {
			return nil, participantFailed(name, participant.Name(), "its description contribution", err)
		}
		for _, mount := range mounts {
			if refusal := checkFileMount(mount, admitted); refusal != nil {
				return nil, participantFailed(name, participant.Name(), "its description contribution", refusal)
			}
			admitted = append(admitted, mount)
		}
	}
	return admitted, nil
}

func (s *supervisor) contribute(ctx context.Context, participant Participant, name string) ([]FileMount, error) {
	callCtx, cancel := context.WithTimeout(ctx, participantCallTimeout)
	defer cancel()
	return participant.Contribute(callCtx, name)
}

func (s *supervisor) created(ctx context.Context, participant Participant, call CreatedCall) error {
	callCtx, cancel := context.WithTimeout(ctx, participantCallTimeout)
	defer cancel()
	return participant.Created(callCtx, call)
}

// watch is the watcher of one running container: it waits for init to exit,
// cleans up and closes ended. A container whose entrypoint ended on its own
// takes the stopped state. One that ended on its own with a status other than
// 0 is also recorded for the stopped check and logged at Warn, with its status
// and the newest rows of its output, so its own explanation of why it ended
// reaches the operator. The rows are read after the cleanup, which returns
// once the readers added every line the container wrote
// (libraryContainer.destroy). The record is written under the same hold of mu
// as the state, so a start that follows cannot be overtaken by it.
func (s *supervisor) watch(name string, running *runningContainer) {
	code, err := running.created.wait()
	if err != nil {
		s.log.Warn("container: its exit could not be read", "name", name, "error", err, "code", codeCleanupIncomplete)
	}
	s.cleanup(name, running.created, running.releaseHold)
	cause := endedCause(code, running.output.newest(endedOutputRows))
	recorded := false
	s.mu.Lock()
	delete(s.running, name)
	if s.states[name] == StateRunning {
		s.states[name] = StateStopped
		recorded = s.recordFailedExitLocked(name, running, err == nil, code, cause)
	}
	s.mu.Unlock()
	switch {
	case recorded:
		s.log.Warn("container: it ended on its own and stays stopped", "name", name, "exit", code, "cause", cause,
			"fix", restartCommand(name), "code", codeStopped)
	case err == nil:
		s.log.Info("container: ended", "name", name, "exit", code)
	}
	close(running.ended)
}

// recordFailedExitLocked records, for the stopped check, a container that
// ended on its own with a status other than 0, with cause as its detail, and
// answers whether it did. An end a stop asked for is not recorded: the
// operator or the daemon ended it. exitRead is whether code was read at all.
// The caller MUST hold mu, and has set the stopped state.
func (s *supervisor) recordFailedExitLocked(name string, running *runningContainer, exitRead bool, code int, cause string) bool {
	if !exitRead {
		return false
	}
	if code == 0 {
		return false
	}
	if running.stopRequested.Load() {
		return false
	}
	s.problems[problemRecord{name: name, kind: problemStopped}] = problemDetail{cause: cause}
	return true
}

// cleanup destroys a created container, makes the ended call of every
// participant and releases the store's hold, in that order: the participants
// learn of the end once the container's processes are gone, and the store
// keeps the image until then. A destroy that fails is recorded as an
// incomplete cleanup.
func (s *supervisor) cleanup(name string, created createdContainer, releaseHold func()) {
	if err := created.destroy(); err != nil {
		s.log.Warn("container: the cleanup did not finish", "name", name, "error", err, "code", codeCleanupIncomplete)
		if recordErr := s.recordProblem(name, problemCleanupIncomplete); recordErr != nil {
			s.log.Debug("container: the problem of a container no longer configured is not recorded", "name", name)
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), participantCallTimeout)
	defer cancel()
	for _, participant := range s.participants {
		participant.Ended(ctx, name)
	}
	releaseHold()
}

// stop ends the named container and returns once its cleanup ended: it sends
// the container its stop signal, waits its stop timeout for it to end, kills
// every process of it through its control group if it has not, then waits for
// the cleanup (destroy, the ended calls, the release of the store's hold). A
// configured container that does not run is left as it is. When ctx ends
// before the stop timeout, the container is killed at once.
func (s *supervisor) stop(ctx context.Context, name string) error {
	lifecycle := s.lifecycle(name)
	lifecycle.Lock()
	defer lifecycle.Unlock()
	return s.stopLocked(ctx, name)
}

// stopLocked is stop for a caller that holds the lifecycle lock of name.
func (s *supervisor) stopLocked(ctx context.Context, name string) error {
	s.mu.Lock()
	running, isRunning := s.running[name]
	_, configured := s.desired[name]
	delete(s.waiting, name)
	s.mu.Unlock()
	if !isRunning {
		if !configured {
			return unknownInstance(name)
		}
		return nil
	}
	return s.stopTogether(ctx, []stopTarget{{name: name, running: running}})
}

// stopTarget is one running container a stop ends.
type stopTarget struct {
	name    string
	running *runningContainer
}

// stopOutcome is how the wait after a container's stop signal ended. Zero is
// reserved and names no outcome.
type stopOutcome uint8

const (
	// stopEnded: the container ended within its stop timeout.
	stopEnded stopOutcome = iota + 1
	// stopTimedOut: the container outlived its stop timeout.
	stopTimedOut
	// stopForced: the caller's context ended first: the daemon's stop was
	// forced, by a second signal or the engine connection closing.
	stopForced
)

// stopTogether stops targets at once and returns when every cleanup ended or
// its bound passed. Every container gets its stop signal first, then each is
// waited for until its own stop timeout, counted from the same moment, so the
// stop of many containers takes the longest timeout among them, never their
// sum: the drain budget holds 30 seconds for the stop signals, whatever the
// number of containers. Each container still running then is killed through
// its control group, and the cleanups are awaited for killCleanupTimeout.
//
// A container killed because it outlived its stop timeout is recorded as
// stop-killed. One killed because ctx ended is not: its stop was cut short,
// it did not ignore its signal. The caller MUST hold the lifecycle lock of
// every target.
func (s *supervisor) stopTogether(ctx context.Context, targets []stopTarget) error {
	signaled := time.Now()
	for _, target := range targets {
		target.running.stopRequested.Store(true)
		if err := target.running.created.signal(target.running.stopSignal); err != nil {
			// An init that already exited cannot take a signal; its watcher
			// is cleaning up, and the wait below sees it end.
			s.log.Debug("container: the stop signal was not delivered", "name", target.name, "error", err)
		}
	}
	killed := make([]stopTarget, 0, len(targets))
	for _, target := range targets {
		outcome := awaitEnd(ctx, target.running, signaled.Add(target.running.stopTimeout))
		switch outcome {
		case stopEnded:
			continue
		case stopTimedOut:
			s.log.Warn("container: it ignored its stop signal and was killed when its stop timeout ended",
				"name", target.name, "signal", target.running.stopSignal.String(),
				"stop-timeout", target.running.stopTimeout.String(), "code", codeStopKilled)
			if err := s.recordProblem(target.name, problemStopKilled); err != nil {
				s.log.Debug("container: the problem of a container no longer configured is not recorded", "name", target.name)
			}
		case stopForced:
			s.log.Info("container: killed at once, because the daemon's stop was forced", "name", target.name)
		}
		killed = append(killed, target)
	}
	for _, target := range killed {
		if err := target.running.created.kill(); err != nil {
			s.log.Warn("container: the kill failed", "name", target.name, "error", err, "code", codeCleanupIncomplete)
		}
	}
	cleanupDeadline := time.Now().Add(killCleanupTimeout)
	var failures []error
	for _, target := range killed {
		if awaitEnd(context.Background(), target.running, cleanupDeadline) == stopEnded {
			continue
		}
		if err := s.recordProblem(target.name, problemCleanupIncomplete); err != nil {
			s.log.Debug("container: the problem of a container no longer configured is not recorded", "name", target.name)
		}
		failures = append(failures, opererr.New(
			"container "+target.name+" did not end after it was killed",
			"its processes or its control group are still there",
			restartCommand(target.name),
		))
	}
	return errors.Join(failures...)
}

// awaitEnd waits for the cleanup of running to end, until deadline or until
// ctx ends. A cleanup that ended answers stopEnded even when the deadline or
// ctx passed at the same moment.
func awaitEnd(ctx context.Context, running *runningContainer, deadline time.Time) stopOutcome {
	timer := time.NewTimer(time.Until(deadline))
	defer timer.Stop()
	select {
	case <-running.ended:
		return stopEnded
	case <-timer.C:
	case <-ctx.Done():
		select {
		case <-running.ended:
			return stopEnded
		default:
		}
		return stopForced
	}
	select {
	case <-running.ended:
		return stopEnded
	default:
	}
	return stopTimedOut
}

// stopThenStart stops the named container when it runs, then starts it, under
// one hold of its lifecycle lock.
func (s *supervisor) stopThenStart(ctx context.Context, name string) (State, error) {
	lifecycle := s.lifecycle(name)
	lifecycle.Lock()
	defer lifecycle.Unlock()
	if err := s.stopLocked(ctx, name); err != nil {
		return StateUnspecified, err
	}
	return s.startLocked(ctx, name)
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
	state, err := s.stopThenStart(ctx, name)
	if err != nil {
		s.setFailed(name, err)
	}
	return state, err
}

// drain stops every running container before the daemon stops. It is the
// plugin's stop drain handler: the one call of a daemon stop that waits for
// the plugin. From its start no container starts. It takes the lifecycle lock
// of every container this run of the plugin knew, in name order, so a start
// in progress ends before the set to stop is read, and stops them together
// (stopTogether): the stop signals within the 30 seconds the drain budget
// holds for them, then the kill and the cleanup within its last 5 seconds.
// When ctx ends first, because a second signal aborted the drain or the
// connection closed, every container still running is killed at once.
func (s *supervisor) drain(ctx context.Context, _ rpc.DrainInput) error {
	s.mu.Lock()
	s.stopping = true
	names := make([]string, 0, len(s.lifecycles))
	for name := range s.lifecycles {
		names = append(names, name)
	}
	s.mu.Unlock()
	slices.Sort(names)
	// The locks are taken in name order, and every other sequence holds one
	// lock at a time, so no two holders wait for each other.
	held := make([]*sync.Mutex, 0, len(names))
	defer func() {
		for _, lifecycle := range held {
			lifecycle.Unlock()
		}
	}()
	for _, name := range names {
		lifecycle := s.lifecycle(name)
		lifecycle.Lock()
		held = append(held, lifecycle)
	}
	return s.stopTogether(ctx, s.runningTargets())
}

// runningTargets answers every running container, sorted by name.
func (s *supervisor) runningTargets() []stopTarget {
	s.mu.Lock()
	defer s.mu.Unlock()
	targets := make([]stopTarget, 0, len(s.running))
	for name, running := range s.running {
		targets = append(targets, stopTarget{name: name, running: running})
	}
	slices.SortFunc(targets, func(a, b stopTarget) int { return strings.Compare(a.name, b.name) })
	return targets
}

// killRunning kills every container still running, at once, and waits for
// their cleanups for killCleanupTimeout. It is the engine entry's last step,
// once the engine connection closed: after a drain nothing runs, and when the
// daemon's stop skipped or cut the drain short, this is the stop. A container
// whose cleanup does not end in time is left to the sweep of the next start.
// It takes no lifecycle lock, so a start that hangs cannot hold the exit, and
// from its start no container starts. MUST be called once, before close.
func (s *supervisor) killRunning() {
	s.mu.Lock()
	s.stopping = true
	s.mu.Unlock()
	targets := s.runningTargets()
	for _, target := range targets {
		s.log.Info("container: killed at once, because the plugin is stopping", "name", target.name)
		target.running.stopRequested.Store(true)
		if err := target.running.created.kill(); err != nil {
			s.log.Warn("container: the kill failed", "name", target.name, "error", err, "code", codeCleanupIncomplete)
		}
	}
	deadline := time.Now().Add(killCleanupTimeout)
	for _, target := range targets {
		if awaitEnd(context.Background(), target.running, deadline) == stopEnded {
			continue
		}
		s.log.Warn("container: its cleanup did not end before the plugin stopped; the next start sweeps what is left",
			"name", target.name, "code", codeCleanupIncomplete)
	}
}

// leftoverFix is the command for a leftover the kernel refused to remove:
// what holds it ends with the boot.
const leftoverFix = "ze signal reboot"

// sweep removes every container a previous run of the plugin left, before any
// container starts: their processes are killed, their control groups and
// mounts removed, and each participant gets the ended call for each, as for
// any container this plugin created. Each leftover is logged by name. One the
// kernel refused to remove whole is kept for the cleanup-incomplete doctor
// check. Containers are never adopted: a configured one is started again as a
// new process once begin wakes its worker. MUST be called once, before begin.
func (s *supervisor) sweep() {
	if s.library == nil {
		return
	}
	for _, found := range s.library.sweep() {
		if found.err != nil {
			s.log.Warn("container: a previous run left this container, and part of it could not be removed",
				"name", found.name, "error", found.err, "fix", leftoverFix, "code", codeCleanupIncomplete)
			s.mu.Lock()
			s.leftovers[found.name] = problemDetail{
				cause: "a previous run of the daemon left it, and the kernel refused part of its removal: " + found.err.Error(),
				fix:   leftoverFix,
			}
			s.mu.Unlock()
		} else {
			s.log.Warn("container: a previous run left this container running; its processes were killed and its control group and root removed",
				"name", found.name, "code", codeCleanupIncomplete)
		}
		ctx, cancel := context.WithTimeout(context.Background(), participantCallTimeout)
		for _, participant := range s.participants {
			participant.Ended(ctx, found.name)
		}
		cancel()
	}
}

// ImagePresent is the store's notice that the image of reference is now
// present. Each container that waited for that reference is started, and the
// names of those now running are answered, sorted.
func (s *supervisor) ImagePresent(ctx context.Context, reference string) []string {
	s.mu.Lock()
	var waiting []string
	for name, awaited := range s.waiting {
		if awaited.Reference == reference {
			waiting = append(waiting, name)
		}
	}
	s.mu.Unlock()
	slices.Sort(waiting)
	var started []string
	for _, name := range waiting {
		lifecycle := s.lifecycle(name)
		lifecycle.Lock()
		s.mu.Lock()
		delete(s.waiting, name)
		s.mu.Unlock()
		state, err := s.startLocked(ctx, name)
		lifecycle.Unlock()
		if err != nil {
			s.setFailed(name, err)
			continue
		}
		if state == StateRunning {
			started = append(started, name)
		}
	}
	return started
}

// RestartOnNewGeneration is the store's request to move the named container
// to its new image: it is stopped, resolved again and started.
func (s *supervisor) RestartOnNewGeneration(ctx context.Context, name string) error {
	_, err := s.stopThenStart(ctx, name)
	return err
}

// StopThenStart is the store's request to stop the named container and start
// it again.
func (s *supervisor) StopThenStart(ctx context.Context, name string) error {
	_, err := s.stopThenStart(ctx, name)
	return err
}

// runningConfigCommand prints the configuration the daemon runs, where the
// configured containers are read.
const runningConfigCommand = "ze show config running"

// unknownInstance refuses a command that names a container the config does
// not hold.
func unknownInstance(name string) error {
	return opererr.New(
		"no container named "+name+" is configured",
		"the command acts on configured containers only",
		runningConfigCommand,
	)
}

// participantFailed is the refusal of a start a participant failed. It names
// the participant and wraps its reason.
func participantFailed(name, participant, step string, err error) error {
	return opererr.Wrap(err,
		"container "+name+": participant "+participant+" failed "+step,
		"a container that lacks what a participant provides, its network among them, must not run, so the container was removed",
		"ze request container restart name "+name,
	)
}

// noRuntime refuses a start on a platform with no runtime library: containers
// run on Linux only.
func noRuntime(name string) error {
	return opererr.New(
		"container "+name+" cannot start: containers run on Linux only",
		"a container is made of Linux namespaces and control groups, which this platform does not have",
		"set container instance "+name+" enabled false",
	)
}

// noImageStore refuses a start in a build that holds no image store.
func noImageStore(name string) error {
	return opererr.New(
		"container "+name+" cannot start: this build of ze holds no image store",
		"a container's root is the image the store holds, so nothing can be started without one",
		"set container instance "+name+" enabled false",
	)
}
