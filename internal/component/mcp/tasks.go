// Design: docs/architecture/mcp/overview.md -- MCP task registry and worker orchestration

package mcp

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
)

// Task registry errors.
var (
	errTaskNotFound       = errors.New("mcp: task not found")
	errTaskConcurrencyCap = errors.New("mcp: task concurrency cap reached")
	errTaskNotTerminal    = errors.New("mcp: task not terminal")
)

// taskEntry is a single task in the registry. Fields guarded by mu.
type taskEntry struct {
	mu         sync.Mutex
	id         string
	identity   string
	sessionID  string
	state      TaskState
	createdAt  time.Time
	updatedAt  time.Time
	terminalAt time.Time
	result     map[string]any
	errorMsg   string
	cancel     context.CancelFunc
	ttl        time.Duration
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
	now           func() time.Time
	stop          chan struct{}
	stopped       chan struct{}
}

// TaskRegistryConfig holds per-server task limits.
type TaskRegistryConfig struct {
	MaxConcurrent int
	MaxTerminal   int
	TTL           time.Duration
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
	ttl := cfg.TTL
	if ttl <= 0 {
		ttl = defaultTaskTTL
	}
	r := &taskRegistry{
		tasks:         make(map[string]*taskEntry),
		byIdentity:    make(map[string]map[string]struct{}),
		maxConcurrent: maxC,
		maxTerminal:   maxT,
		ttl:           ttl,
		now:           time.Now,
		stop:          make(chan struct{}),
		stopped:       make(chan struct{}),
	}
	go r.runGC()
	return r
}

// Create registers a new task in working state and returns its ID.
// The caller is responsible for launching the worker goroutine with
// the returned cancel func.
func (r *taskRegistry) Create(identity, sessionID string, requestedTTL time.Duration) (string, context.Context, context.CancelFunc, error) {
	ttl := r.clampTTL(requestedTTL)
	now := r.now()

	r.mu.Lock()
	count := r.activeCount(identity)
	if count >= r.maxConcurrent {
		r.mu.Unlock()
		return "", nil, nil, errTaskConcurrencyCap
	}

	id, err := generateSessionID()
	if err != nil {
		r.mu.Unlock()
		return "", nil, nil, err
	}

	ctx, cancel := context.WithCancel(context.Background())

	entry := &taskEntry{
		id:        id,
		identity:  identity,
		sessionID: sessionID,
		state:     TaskWorking,
		createdAt: now,
		updatedAt: now,
		cancel:    cancel,
		ttl:       ttl,
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

// Result returns the stored CallToolResult for a terminal task.
func (r *taskRegistry) Result(identity, taskID string) (map[string]any, error) {
	r.mu.Lock()
	entry, ok := r.tasks[taskID]
	r.mu.Unlock()
	if !ok {
		return nil, errTaskNotFound
	}
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if entry.identity != identity {
		return nil, errTaskNotFound
	}
	if !entry.state.IsTerminal() {
		return nil, errTaskNotTerminal
	}
	return entry.result, nil
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
	defer entry.mu.Unlock()
	if entry.identity != identity {
		return TaskUnspecified, errTaskNotFound
	}
	if entry.state.IsTerminal() {
		return entry.state, nil
	}
	entry.state = TaskCancelled
	entry.updatedAt = r.now()
	entry.terminalAt = entry.updatedAt
	entry.cancel()
	return TaskCancelled, nil
}

// List returns all tasks for the given identity.
func (r *taskRegistry) List(identity string) []TaskInfo {
	r.mu.Lock()
	ids, ok := r.byIdentity[identity]
	if !ok {
		r.mu.Unlock()
		return nil
	}
	idSlice := make([]string, 0, len(ids))
	for id := range ids {
		idSlice = append(idSlice, id)
	}
	r.mu.Unlock()

	result := make([]TaskInfo, 0, len(idSlice))
	for _, id := range idSlice {
		r.mu.Lock()
		entry, ok := r.tasks[id]
		r.mu.Unlock()
		if !ok {
			continue
		}
		entry.mu.Lock()
		result = append(result, taskInfoFromEntry(entry))
		entry.mu.Unlock()
	}
	return result
}

// Transition updates a task's state. Used by workers on completion/failure
// and by the elicitation integration for input_required transitions.
// Rejects invalid transitions (terminal -> any).
func (r *taskRegistry) Transition(taskID string, to TaskState) bool {
	r.mu.Lock()
	entry, ok := r.tasks[taskID]
	r.mu.Unlock()
	if !ok {
		return false
	}
	entry.mu.Lock()
	defer entry.mu.Unlock()
	if entry.state.IsTerminal() {
		return false
	}
	entry.state = to
	entry.updatedAt = r.now()
	if to.IsTerminal() {
		entry.terminalAt = entry.updatedAt
	}
	return true
}

// SetErrorMsg stores the error message for a failed task.
func (r *taskRegistry) SetErrorMsg(taskID, msg string) {
	r.mu.Lock()
	entry, ok := r.tasks[taskID]
	r.mu.Unlock()
	if !ok {
		return
	}
	entry.mu.Lock()
	entry.errorMsg = msg
	entry.mu.Unlock()
}

// StoreResult saves the tool call result on a terminal task.
func (r *taskRegistry) StoreResult(taskID string, result map[string]any) {
	r.mu.Lock()
	entry, ok := r.tasks[taskID]
	r.mu.Unlock()
	if !ok {
		return
	}
	entry.mu.Lock()
	entry.result = result
	entry.mu.Unlock()
}

// CancelAllForSession cancels all in-flight tasks for a session (on
// session close). Terminal tasks are left for TTL GC.
func (r *taskRegistry) CancelAllForSession(sessionID string) {
	r.mu.Lock()
	var toCancel []*taskEntry
	for _, entry := range r.tasks {
		if entry.sessionID == sessionID {
			toCancel = append(toCancel, entry)
		}
	}
	r.mu.Unlock()

	now := r.now()
	for _, entry := range toCancel {
		entry.mu.Lock()
		if !entry.state.IsTerminal() {
			entry.state = TaskCancelled
			entry.updatedAt = now
			entry.terminalAt = now
			entry.cancel()
		}
		entry.mu.Unlock()
	}
}

// Close stops the GC goroutine.
func (r *taskRegistry) Close() {
	close(r.stop)
	<-r.stopped
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

func (r *taskRegistry) clampTTL(requested time.Duration) time.Duration {
	if requested <= 0 {
		return r.ttl
	}
	if requested < minTaskTTL {
		return minTaskTTL
	}
	if requested > maxTaskTTL {
		return maxTaskTTL
	}
	return requested
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

func (r *taskRegistry) sweep() {
	now := r.now()
	r.mu.Lock()
	defer r.mu.Unlock()

	for id, entry := range r.tasks {
		entry.mu.Lock()
		expired := entry.state.IsTerminal() && !entry.terminalAt.IsZero() &&
			now.Sub(entry.terminalAt) > entry.ttl
		identity := entry.identity
		entry.mu.Unlock()
		if expired {
			delete(r.tasks, id)
			if ids, ok := r.byIdentity[identity]; ok {
				delete(ids, id)
				if len(ids) == 0 {
					delete(r.byIdentity, identity)
				}
			}
		}
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
}

// ToWire converts to the MCP camelCase wire format.
func (t TaskInfo) ToWire() map[string]any {
	m := map[string]any{
		"taskId":       t.ID,
		"status":       t.State.String(),
		"createdAt":    t.CreatedAt.UTC().Format(time.RFC3339),
		"lastUpdateAt": t.UpdatedAt.UTC().Format(time.RFC3339),
	}
	if t.ErrorMsg != "" {
		m["error"] = t.ErrorMsg
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
	}
}

// taskWorkerFunc is the function signature a task worker executes.
// ctx is canceled on tasks/cancel or session close.
type taskWorkerFunc func(ctx context.Context) (map[string]any, error)

// runTaskWorker launches a goroutine that executes work, transitions
// the task on completion/failure, stores the result, and emits a
// status notification on the session's GET stream.
func runTaskWorker(reg *taskRegistry, sess *session, taskID string, ctx context.Context, work taskWorkerFunc) {
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

		reg.Transition(taskID, finalState)
		if errMsg != "" {
			reg.SetErrorMsg(taskID, errMsg)
		}
		if result != nil {
			reg.StoreResult(taskID, result)
		}

		frame, fErr := buildTaskStatusNotification(taskID, finalState)
		if fErr == nil && sess != nil {
			_ = sess.Send(frame)
		}
	}()
}

// TaskElicit sends an elicitation request from a task worker context.
// Unlike session.Elicit (which uses the POST reply sink), this sends
// the elicit frame via session.Send (GET SSE stream) because the
// creating POST has already returned by the time a task worker runs.
//
// State transitions: working -> input_required -> (block) -> working.
// On decline/cancel, transitions back to working so the worker can
// decide the terminal state.
func TaskElicit(reg *taskRegistry, sess *session, taskID string, ctx context.Context, message string, schema map[string]any) (map[string]any, error) {
	if !sess.ClientSupportsElicit() {
		return nil, ErrElicitUnsupported
	}
	if err := validateElicitSchema(schema); err != nil {
		return nil, err
	}

	if !reg.Transition(taskID, TaskInputRequired) {
		return nil, errors.New("mcp: task not in a state that allows elicitation")
	}
	if frame, err := buildTaskStatusNotification(taskID, TaskInputRequired); err == nil {
		_ = sess.Send(frame)
	}

	id, ch, err := sess.RegisterElicit()
	if err != nil {
		reg.Transition(taskID, TaskWorking)
		return nil, err
	}
	resolved := false
	defer func() {
		if !resolved {
			sess.CancelElicit(id)
		}
		// Always transition back from input_required (finding #6).
		reg.Transition(taskID, TaskWorking)
		if frame, fErr := buildTaskStatusNotification(taskID, TaskWorking); fErr == nil {
			_ = sess.Send(frame)
		}
	}()

	frame, err := buildElicitFrame(id, message, schema)
	if err != nil {
		return nil, err
	}
	// Send via GET SSE stream, not POST sink (finding #2).
	if err := sess.Send(frame); err != nil {
		return nil, fmt.Errorf("mcp: task elicit: send frame: %w", err)
	}

	select {
	case resp := <-ch:
		resolved = true
		// Defer handles the working transition for all paths.
		return resolveElicitAction(resp)
	case <-ctx.Done():
		return nil, fmt.Errorf("mcp: task elicit: %w", ctx.Err())
	}
}

// buildTaskStatusNotification constructs the notifications/tasks/status
// JSON-RPC notification with _meta.io.modelcontextprotocol/related-task.
func buildTaskStatusNotification(taskID string, state TaskState) ([]byte, error) {
	frame := map[string]any{
		"jsonrpc": "2.0",
		"method":  "notifications/tasks/status",
		"params": map[string]any{
			"taskId": taskID,
			"status": state.String(),
			"_meta": map[string]any{
				"io.modelcontextprotocol/related-task": taskID,
			},
		},
	}
	return json.Marshal(frame)
}
