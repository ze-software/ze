// Design: docs/architecture/mcp/overview.md -- MCP task registry and worker orchestration
// Related: task_state.go -- the TaskState enum every entry carries, and the state Ze cannot enter
// Related: streamable_tools.go -- createTask and the tasks/* handlers that drive this registry

package mcp

import (
	"cmp"
	"context"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"slices"
	"sync"
	"time"
)

// Task registry defaults.
const (
	defaultMaxConcurrentTasks = 8
	defaultMaxTerminalTasks   = 128
	defaultTaskTTL            = 5 * time.Minute
	minTaskTTL                = time.Second
	maxTaskTTL                = time.Hour
	taskGCInterval            = 30 * time.Second
	// defaultTaskExecDeadline bounds how long a worker can run before the
	// registry forces its entry terminal (D-3).
	//
	// This is a LIVENESS bound, not a retention one, and it exists because MCP
	// 2026-07-28 removed sessions. Under the previous revision
	// CancelAllForSession was wired to session expiry. It was the only path that
	// forced a NON-terminal task terminal. The TTL sweep cannot do it,
	// because it deletes only entries that ALREADY reached a terminal state.
	//
	// The removal of sessions therefore removed a liveness guarantee, not just a
	// cleanup hook. A worker whose dispatch never returns would hold one of its
	// principal's maxConcurrent slots forever, and no sweep would reach it.
	//
	// 10 minutes is deliberately far longer than any annotated command takes. It
	// is a backstop against a wedged dispatch, not a timeout operators are
	// expected to tune around.
	defaultTaskExecDeadline = 10 * time.Minute
	// defaultPollInterval is the ceiling on the pollIntervalMs hint. The value
	// actually sent is min(this, ttl/2) -- see retentionHints.
	defaultPollInterval = time.Second
	// taskIDRawBytes is the entropy behind a task id: 128 bits, base64url
	// encoded. A task id is a capability-free handle, because the registry
	// checks the authenticated principal on every lookup. But the id is still
	// unguessable, so a task cannot be probed for by enumeration.
	taskIDRawBytes = 16
)

// generateTaskID returns a fresh base64url-encoded 128-bit task identifier.
func generateTaskID() (string, error) {
	var buf [taskIDRawBytes]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(buf[:]), nil
}

// Task registry errors.
var (
	errTaskNotFound       = errors.New("mcp: task not found")
	errTaskConcurrencyCap = errors.New("mcp: task concurrency cap reached")
	// errTaskExecDeadline is the error text stored on a task the sweep forced
	// terminal for outrunning its execution deadline (D-3). A client that polls
	// tasks/get reads it as the task's `error`. It therefore says what happened
	// and what to do, rather than only that the task failed.
	errTaskExecDeadline = errors.New("mcp: task exceeded its server-side execution deadline and was terminated; re-run the command, and if it legitimately needs longer, raise TaskRegistryConfig.ExecDeadline")
)

// taskEntry is a single task in the registry. Fields guarded by mu.
type taskEntry struct {
	mu         sync.Mutex
	id         string
	identity   string
	state      TaskState
	createdAt  time.Time
	updatedAt  time.Time
	terminalAt time.Time
	result     map[string]any
	errorMsg   string
	cancel     context.CancelFunc
	ttl        time.Duration
	// deadline is the instant past which a still-working task is forced
	// terminal by the sweep (D-3). Zero means no deadline, which no path
	// through Create can produce.
	deadline time.Time
}

// taskRegistry holds all in-flight and recently-terminal tasks.
// Scoped to the Streamable server instance. Thread-safe.
type taskRegistry struct {
	mu            sync.Mutex
	tasks         map[string]*taskEntry
	byIdentity    map[string]map[string]struct{}
	maxConcurrent int
	maxTerminal   int
	ttl           time.Duration
	execDeadline  time.Duration
	now           func() time.Time
	stop          chan struct{}
	stopped       chan struct{}
}

// TaskRegistryConfig holds per-server task limits.
type TaskRegistryConfig struct {
	MaxConcurrent int
	MaxTerminal   int
	TTL           time.Duration
	// ExecDeadline bounds a single worker's run before the sweep forces its
	// entry terminal. Non-positive selects defaultTaskExecDeadline.
	ExecDeadline time.Duration
}

func newTaskRegistry(cfg TaskRegistryConfig) *taskRegistry {
	maxC := cfg.MaxConcurrent
	if maxC <= 0 {
		maxC = defaultMaxConcurrentTasks
	}
	maxT := cfg.MaxTerminal
	if maxT <= 0 {
		maxT = defaultMaxTerminalTasks
	}
	deadline := cfg.ExecDeadline
	if deadline <= 0 {
		deadline = defaultTaskExecDeadline
	}
	r := &taskRegistry{
		tasks:         make(map[string]*taskEntry),
		byIdentity:    make(map[string]map[string]struct{}),
		maxConcurrent: maxC,
		maxTerminal:   maxT,
		// The TTL is clamped ONCE, here. It used to be clamped per Create,
		// because the client requested a TTL on each task-augmented call.
		// With the client-directed opt-in gone (D-1), the config field is the
		// only TTL input that exists. The bound therefore belongs at the single
		// point it enters the registry, and not on a path taken per task.
		ttl:          clampTaskTTL(cfg.TTL),
		execDeadline: deadline,
		now:          time.Now,
		stop:         make(chan struct{}),
		stopped:      make(chan struct{}),
	}
	go r.runGC()
	return r
}

// clampTaskTTL bounds a configured retention window to [minTaskTTL, maxTaskTTL],
// reading a non-positive value as "use the default" rather than as "zero".
func clampTaskTTL(requested time.Duration) time.Duration {
	switch {
	case requested <= 0:
		return defaultTaskTTL
	case requested < minTaskTTL:
		return minTaskTTL
	case requested > maxTaskTTL:
		return maxTaskTTL
	default:
		return requested
	}
}

// retentionHints returns the two client-facing timing numbers a
// CreateTaskResult carries, in milliseconds. The first is how long a terminal
// result is retained. The second is how often the client polls.
//
// pollIntervalMs is DERIVED, never configured (D-6). A fixed constant can
// exceed the retention window. At the 1 s minimum TTL, a 1 s poll hint would
// let a conforming client sleep exactly past the result and find it swept. The
// hint is therefore capped at half the TTL. That keeps one invariant true for
// every legal TTL: a client that obeys the hint polls at least twice inside
// the window.
func (r *taskRegistry) retentionHints() (ttlMs, pollMs int64) {
	ttl := clampTaskTTL(r.ttl)
	poll := min(defaultPollInterval, ttl/2)
	return ttl.Milliseconds(), poll.Milliseconds()
}

// Create registers a new task in working state and returns its ID.
// The caller is responsible for launching the worker goroutine with
// the returned cancel func.
//
// identity is the authenticated principal the task belongs to. It comes from
// the per-request authenticator, never from a request body field. And it is
// the only key the registry indexes by. The concurrency cap, every lookup and
// the visibility rule are all per principal.
func (r *taskRegistry) Create(identity string) (string, context.Context, context.CancelFunc, error) {
	now := r.now()
	deadline := now.Add(r.execDeadline)

	r.mu.Lock()
	count := r.activeCount(identity)
	if count >= r.maxConcurrent {
		r.mu.Unlock()
		return "", nil, nil, errTaskConcurrencyCap
	}

	id, err := generateTaskID()
	if err != nil {
		r.mu.Unlock()
		return "", nil, nil, err
	}

	// The worker context carries the deadline so a well-behaved dispatch that
	// selects on ctx.Done() unwinds on its own. That reaches the dispatcher
	// through server.context (tools.go), which is the ONE reader of the runner's
	// ctx field; both dispatch sites used to hard-code context.Background(), so
	// this half of the mechanism was inert and the claim below was the only one
	// that held. It is still NOT the guarantee: a wedged dispatch that never
	// looks at its context would keep the goroutine and the concurrency slot
	// regardless. The registry-side backstop is in sweep, which forces the ENTRY
	// terminal at the same instant whether or not the goroutine ever returns
	// (D-3).
	ctx, cancel := context.WithDeadline(context.Background(), deadline)

	entry := &taskEntry{
		id:        id,
		identity:  identity,
		state:     TaskWorking,
		createdAt: now,
		updatedAt: now,
		cancel:    cancel,
		ttl:       r.ttl,
		deadline:  deadline,
	}
	r.tasks[id] = entry
	if r.byIdentity[identity] == nil {
		r.byIdentity[identity] = make(map[string]struct{})
	}
	r.byIdentity[identity][id] = struct{}{}
	r.mu.Unlock()

	return id, ctx, cancel, nil
}

// Get returns the task state visible to the given identity.
// Returns errTaskNotFound for unknown IDs or cross-identity lookups.
func (r *taskRegistry) Get(identity, taskID string) (TaskInfo, error) {
	r.mu.Lock()
	entry, ok := r.tasks[taskID]
	r.mu.Unlock()
	if !ok {
		return TaskInfo{}, errTaskNotFound
	}
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if entry.identity != identity {
		return TaskInfo{}, errTaskNotFound
	}
	return taskInfoFromEntry(entry), nil
}

// Cancel requests cancellation of a working task. Terminal tasks are
// left as-is (idempotent no-op per AC-19a).
func (r *taskRegistry) Cancel(identity, taskID string) (TaskState, error) {
	r.mu.Lock()
	entry, ok := r.tasks[taskID]
	r.mu.Unlock()
	if !ok {
		return TaskUnspecified, errTaskNotFound
	}
	entry.mu.Lock()
	if entry.identity != identity {
		entry.mu.Unlock()
		return TaskUnspecified, errTaskNotFound
	}
	if entry.state.IsTerminal() {
		state := entry.state
		entry.mu.Unlock()
		return state, nil
	}
	entry.state = TaskCancelled
	entry.updatedAt = r.now()
	entry.terminalAt = entry.updatedAt
	entry.cancel()
	entry.mu.Unlock()

	// A cancel is the other way a task becomes terminal, so the retention cap
	// is enforced here too. entry.mu is released first: the registry lock order
	// is r.mu before entry.mu.
	r.evictTerminalOverCap(identity)
	return TaskCancelled, nil
}

// Transition updates a task's state. Used by workers on completion/failure.
// Rejects invalid transitions (terminal -> any).
func (r *taskRegistry) Transition(taskID string, to TaskState) bool {
	r.mu.Lock()
	entry, ok := r.tasks[taskID]
	r.mu.Unlock()
	if !ok {
		return false
	}
	entry.mu.Lock()
	if entry.state.IsTerminal() {
		entry.mu.Unlock()
		return false
	}
	entry.state = to
	entry.updatedAt = r.now()
	becameTerminal := to.IsTerminal()
	if becameTerminal {
		entry.terminalAt = entry.updatedAt
	}
	identity := entry.identity
	entry.mu.Unlock()

	// Enforce the retention cap here, not only on the GC ticker. This is the
	// moment the terminal set grows, and a burst can cross the cap many times
	// between two taskGCInterval ticks. The invariant would otherwise not hold
	// at the points a caller can observe it. entry.mu is released first,
	// because the registry lock order is r.mu before entry.mu (see sweep and
	// activeCount).
	if becameTerminal {
		r.evictTerminalOverCap(identity)
	}
	return true
}

// setErrorMsg stores the error message for a task that has not yet finished.
//
// Terminal entries are immutable. Read the note on storeResult. A worker that
// returns after the deadline sweep already failed its entry must not overwrite
// the diagnostic that explains why the task ended.
func (r *taskRegistry) setErrorMsg(taskID, msg string) {
	r.mu.Lock()
	entry, ok := r.tasks[taskID]
	r.mu.Unlock()
	if !ok {
		return
	}
	entry.mu.Lock()
	if !entry.state.IsTerminal() {
		entry.errorMsg = msg
	}
	entry.mu.Unlock()
}

// storeResult saves the tool call result on a task that has not yet finished.
//
// # A terminal entry is immutable, and that is the whole point
//
// Three writers touch an entry after creation: Transition, setErrorMsg and this
// one. Transition has always refused terminal -> any; the other two did not,
// and the gap was reachable. The deadline sweep (pass 1) sets state, terminalAt
// and errorMsg on a WORKING task whose worker has not returned. When that worker
// eventually does return, runTaskWorker calls storeResult and setErrorMsg before
// its (correctly refused) Transition -- so the entry ended up carrying the
// sweep's deadline error AND the worker's completed result, and toWire emitted
// both `error` and `result`. A client polling tasks/get read status "failed"
// with a deadline diagnostic beside a complete, correct answer, which toWire's
// own godoc says cannot happen.
//
// The guard is the same predicate Transition uses, so all three writers now
// agree on one rule: the first terminal state wins, and everything after it is
// dropped. Ordering inside runTaskWorker is unaffected -- on the normal path the
// entry is still working when the payload is stored, which is the property that
// keeps a polling client from seeing a terminal status with an empty payload.
func (r *taskRegistry) storeResult(taskID string, result map[string]any) {
	r.mu.Lock()
	entry, ok := r.tasks[taskID]
	r.mu.Unlock()
	if !ok {
		return
	}
	entry.mu.Lock()
	if !entry.state.IsTerminal() {
		entry.result = result
	}
	entry.mu.Unlock()
}

// Close stops the GC goroutine and cancels every task still in flight.
//
// The cancellation is not housekeeping. taskWorkerFunc's godoc promises "ctx is
// canceled on tasks/cancel and on task-registry shutdown", and only the first
// half was true: Close stopped the sweeper and returned, leaving every working
// entry's context live. A worker blocked in a dispatch therefore outlived
// Streamable.Close() with nothing left to reap it -- the sweep that would have
// forced its entry terminal is the goroutine Close just stopped.
//
// Only the context is canceled; no state is written. The registry is being torn
// down, so a worker that unwinds here finds its entry via the normal path and
// transitions it, and one that ignores its context leaks no more than it
// already did. Lock order is r.mu before entry.mu, as in sweep.
func (r *taskRegistry) Close() {
	close(r.stop)
	<-r.stopped

	r.mu.Lock()
	defer r.mu.Unlock()
	for _, entry := range r.tasks {
		entry.mu.Lock()
		if !entry.state.IsTerminal() && entry.cancel != nil {
			entry.cancel()
		}
		entry.mu.Unlock()
	}
}

// activeCount returns the number of non-terminal tasks for identity.
// Caller must hold r.mu.
// activeCount returns the number of non-terminal tasks for identity.
// Caller must hold r.mu. Acquires entry.mu per entry to read state
// safely (Transition/Cancel write state under entry.mu, not r.mu).
func (r *taskRegistry) activeCount(identity string) int {
	ids, ok := r.byIdentity[identity]
	if !ok {
		return 0
	}
	count := 0
	for id := range ids {
		entry, ok := r.tasks[id]
		if !ok {
			continue
		}
		entry.mu.Lock()
		terminal := entry.state.IsTerminal()
		entry.mu.Unlock()
		if !terminal {
			count++
		}
	}
	return count
}

// terminalCap returns the retained-terminal-task cap for one principal.
//
// newTaskRegistry normalizes a non-positive MaxTerminal to the default, exactly
// as it does for MaxConcurrent and the TTL. A registry built through the
// constructor therefore always carries a positive value. This helper repeats
// that one convention for any registry that was not.
//
// A zero can therefore never be read as "unlimited". A zero-valued cap that
// silently means "no cap" is the fail-open shape
// ai/rules/fail-closed-guards.md exists to stop. There is deliberately no third
// convention, and nothing in this registry spells "uncapped".
func (r *taskRegistry) terminalCap() int {
	if r.maxTerminal <= 0 {
		return defaultMaxTerminalTasks
	}
	return r.maxTerminal
}

// deleteLocked removes one task and prunes the identity index. It drops the
// identity key once its last id goes. Caller must hold r.mu.
//
// Both reapers -- the TTL sweep and the retention cap -- go through here. A
// leaked byIdentity key is the same unbounded map growth in a different shape,
// so the index-consistent removal is written once rather than copied.
func (r *taskRegistry) deleteLocked(id, identity string) {
	delete(r.tasks, id)
	ids, ok := r.byIdentity[identity]
	if !ok {
		return
	}
	delete(ids, id)
	if len(ids) == 0 {
		delete(r.byIdentity, identity)
	}
}

// evictTerminalOverCap bounds the terminal tasks retained for one principal.
// It drops the oldest terminal task first, until at most terminalCap remain.
//
// # Why this cap exists
//
// The registry is the only long-lived per-client structure on this transport.
// A terminal entry retains a full result map (storeResult) until its TTL
// expires. That window is client-chosen and can be as long as an hour
// (clampTTL).
//
// The concurrency cap counts only non-terminal tasks (activeCount). A completed
// task therefore frees its slot immediately, and a client can cycle tasks to
// completion indefinitely. Without this cap the map grows without bound.
//
// # Why the cap is per principal and not global
//
// A GLOBAL cap would bound the map at one flat number. But it would also give
// every authenticated caller a cross-principal denial. A burst of quick tasks
// would evict every other principal's results. That is one principal that
// destroys another's data, which is a worse failure than the growth it
// prevents.
//
// PER PRINCIPAL bounds the map at principals x cap, and no principal can reach
// another's entries. The multiplier is set by the auth mode, not by the caller:
//
//   - none and bearer both authenticate to the anonymous identity. Every
//     caller therefore collapses onto one key, and the bound is a hard cap.
//     This is the default for `ze --mcp` and for ze-chaos, and it is the
//     deployment where the unbounded growth was reachable.
//
//   - bearer-list is bounded by the operator's configured identity list.
//
//   - oauth is bounded by the subjects the trusted issuer has signed tokens
//     for. Admission there belongs to the authorization server, which Ze
//     already trusts for authentication itself. A smaller multiplier bought
//     with an eviction primitive that every tenant holds against every other
//     tenant is the worse trade.
//
// It also keeps the sibling caps coherent. maxConcurrent is already per
// principal, so a global maxTerminal would be a second and contradictory
// scoping convention inside one registry.
//
// A non-terminal task is never evicted. An in-flight task must not vanish
// because other tasks completed.
func (r *taskRegistry) evictTerminalOverCap(identity string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.evictTerminalOverCapLocked(identity)
}

// evictTerminalOverCapLocked is evictTerminalOverCap with r.mu already held.
// It takes entry.mu per entry, which is the registry's lock order (r.mu before
// entry.mu, as in sweep and activeCount). A caller MUST NOT hold an entry lock
// when it calls this function.
func (r *taskRegistry) evictTerminalOverCapLocked(identity string) {
	ids, ok := r.byIdentity[identity]
	if !ok {
		return
	}
	limit := r.terminalCap()
	// len(ids) counts terminal and non-terminal alike, so it is an upper bound
	// on the terminal count. A length at or under the cap here means there is
	// nothing to evict.
	if len(ids) <= limit {
		return
	}

	type terminalRef struct {
		id string
		at time.Time
	}
	refs := make([]terminalRef, 0, len(ids))
	for id := range ids {
		entry, ok := r.tasks[id]
		if !ok {
			continue
		}
		entry.mu.Lock()
		terminal := entry.state.IsTerminal()
		at := entry.terminalAt
		entry.mu.Unlock()
		if terminal {
			refs = append(refs, terminalRef{id: id, at: at})
		}
	}
	excess := len(refs) - limit
	if excess <= 0 {
		return
	}

	// Oldest terminal first. The id breaks ties, so eviction is deterministic
	// rather than dependent on map iteration order. That matters when a coarse
	// or injected clock stamps several tasks with one instant.
	slices.SortFunc(refs, func(a, b terminalRef) int {
		return cmp.Or(a.at.Compare(b.at), cmp.Compare(a.id, b.id))
	})
	for _, ref := range refs[:excess] {
		r.deleteLocked(ref.id, identity)
	}
}

func (r *taskRegistry) runGC() {
	defer close(r.stopped)
	ticker := time.NewTicker(taskGCInterval)
	defer ticker.Stop()
	for {
		select {
		case <-r.stop:
			return
		case <-ticker.C:
			r.sweep()
		}
	}
}

// sweep runs both reapers, in the order they depend on.
//
// Pass 1 forces a past-deadline WORKING task terminal (D-3). Pass 2 deletes a
// terminal task whose retention TTL has expired. The order matters for one
// reason. A task forced terminal here starts its retention window now, and pass
// 2 does not delete it in the same tick. A client that polls once more
// therefore learns how its task ended, rather than finds it gone.
func (r *taskRegistry) sweep() {
	now := r.now()
	r.mu.Lock()
	defer r.mu.Unlock()

	// Pass 1: the liveness backstop the deleted session reaper used to provide.
	//
	// A canceled worker context is not enough on its own. The goroutine can be
	// blocked somewhere that never observes cancellation. And until the ENTRY
	// is terminal it keeps counting against activeCount, so it holds one of its
	// principal's maxConcurrent slots. The entry is therefore transitioned
	// here, whether or not the goroutine ever returns.
	//
	// The worker's own later Transition is a no-op, because Transition refuses
	// terminal -> any.
	for _, entry := range r.tasks {
		entry.mu.Lock()
		overdue := !entry.state.IsTerminal() && !entry.deadline.IsZero() &&
			now.After(entry.deadline)
		if overdue {
			entry.state = TaskFailed
			entry.updatedAt = now
			entry.terminalAt = now
			entry.errorMsg = errTaskExecDeadline.Error()
			if entry.cancel != nil {
				entry.cancel()
			}
		}
		entry.mu.Unlock()
	}

	// Pass 2: retention.
	for id, entry := range r.tasks {
		entry.mu.Lock()
		expired := entry.state.IsTerminal() && !entry.terminalAt.IsZero() &&
			now.Sub(entry.terminalAt) > entry.ttl
		identity := entry.identity
		entry.mu.Unlock()
		if expired {
			r.deleteLocked(id, identity)
		}
	}

	// Backstop for the retention cap. Transition and Cancel already enforce it
	// at the moment a task becomes terminal, so this normally finds nothing; it
	// runs anyway so a future path that makes a task terminal without going
	// through them cannot quietly unbound the map. Deleting the current key
	// while ranging is safe, and evictTerminalOverCapLocked touches no other.
	for identity := range r.byIdentity {
		r.evictTerminalOverCapLocked(identity)
	}
}

// TaskInfo is the read-only snapshot returned by Get/List.
// Wire serialization uses map[string]any (MCP camelCase exemption).
type TaskInfo struct {
	ID        string
	State     TaskState
	CreatedAt time.Time
	UpdatedAt time.Time
	Identity  string
	ErrorMsg  string
	// Result is the tool output, populated only once the task is terminal.
	// It is what tasks/get hands back now that tasks/result is gone.
	Result map[string]any
}

// toWire converts to the MCP camelCase wire format.
//
// The terminal payload rule is the whole reason tasks/result was deleted. MCP
// 2026-07-28 changelog Major change 6: the extension "replaces the blocking
// tasks/result method with polling via tasks/get".
//
// A polling client must be able to learn the OUTCOME from the same call that
// reports the status. A terminal task therefore carries `result` when it
// completed, and `error` when it failed. A non-terminal task carries neither,
// which is what makes "still working" and "finished with nothing to say"
// distinguishable.
func (t TaskInfo) toWire() map[string]any {
	m := map[string]any{
		"taskId":       t.ID,
		"status":       t.State.String(),
		"createdAt":    t.CreatedAt.UTC().Format(time.RFC3339),
		"lastUpdateAt": t.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if t.ErrorMsg != "" {
		m["error"] = t.ErrorMsg
	}
	if t.State.IsTerminal() && t.Result != nil {
		m["result"] = t.Result
	}
	return m
}

func taskInfoFromEntry(e *taskEntry) TaskInfo {
	return TaskInfo{
		ID:        e.id,
		State:     e.state,
		CreatedAt: e.createdAt,
		UpdatedAt: e.updatedAt,
		Identity:  e.identity,
		ErrorMsg:  e.errorMsg,
		Result:    e.result,
	}
}

// taskWorkerFunc is the function signature a task worker executes.
// ctx is canceled on tasks/cancel and on task-registry shutdown.
type taskWorkerFunc func(ctx context.Context) (map[string]any, error)

// runTaskWorker launches a goroutine that executes work, transitions the task
// on completion/failure, and stores the result.
//
// Nothing is pushed to the client: MCP 2026-07-28 has no server-to-client
// stream on this transport, so a client observes a task by polling tasks/get.
func runTaskWorker(ctx context.Context, reg *taskRegistry, taskID string, work taskWorkerFunc) {
	go func() {
		result, err := work(ctx)

		var finalState TaskState
		var errMsg string
		switch {
		case ctx.Err() != nil:
			finalState = TaskCancelled
		case err != nil:
			finalState = TaskFailed
			errMsg = err.Error()
		default:
			finalState = TaskCompleted
		}

		// The payload is stored BEFORE the state goes terminal, and the order is
		// load-bearing now that tasks/get is the only way to collect a result.
		//
		// A client polls on an interval it was given. The instant it sees a
		// terminal status, it reads `result`/`error` from that SAME response
		// and stops polling.
		//
		// A transition first would open a window where the task reads terminal
		// with an empty payload. The client would then correctly conclude that
		// the task finished with no output, and it would never look again.
		// Under the old blocking tasks/result the window was invisible, because
		// the client made a second call that arrived after the store.
		if result != nil {
			reg.storeResult(taskID, result)
		}
		if errMsg != "" {
			reg.setErrorMsg(taskID, errMsg)
		}
		reg.Transition(taskID, finalState)
	}()
}
