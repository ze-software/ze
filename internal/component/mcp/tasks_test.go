// TestTaskRegistry_CancelAllForSession,
// TestTaskNotifications_StatusFrameShape and TestBuildTaskStatusNotification
// are removed with their subjects. CancelAllForSession canceled a session's
// tasks when the session expired, and MCP 2026-07-28 has no sessions to expire.
//
// Two tests still assert the per-identity cancel path that
// CancelAllForSession shared. TestTaskRegistry_CreateGetCancel covers the happy
// path and the terminal no-op. TestTaskRegistry_IdentityScope covers a
// cross-identity Cancel, which is a not-found.
//
// buildTaskStatusNotification pushed notifications/tasks/status onto the GET SSE
// stream that this phase deletes. The revision defines no server-to-client
// stream to replace that stream. A client observes a task when it polls
// tasks/get. Every cap, the identity index and the TTL clamp keep their tests
// below, unchanged.

package mcp

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
)

func newTestTaskRegistry(maxConcurrent int, ttl time.Duration) *taskRegistry {
	return newTaskRegistry(TaskRegistryConfig{
		MaxConcurrent: maxConcurrent,
		MaxTerminal:   128,
		TTL:           ttl,
	})
}

func TestTaskRegistry_CreateGetCancel(t *testing.T) {
	r := newTestTaskRegistry(8, time.Minute)
	defer r.Close()

	id, ctx, _, err := r.Create("alice")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if id == "" {
		t.Fatal("Create returned empty id")
	}
	if ctx == nil {
		t.Fatal("Create returned nil ctx")
	}

	info, err := r.Get("alice", id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if info.State != TaskWorking {
		t.Errorf("state = %v, want working", info.State)
	}

	state, err := r.Cancel("alice", id)
	if err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if state != TaskCancelled {
		t.Errorf("Cancel state = %v, want TaskCancelled", state)
	}

	info, err = r.Get("alice", id)
	if err != nil {
		t.Fatalf("Get after cancel: %v", err)
	}
	if info.State != TaskCancelled {
		t.Errorf("after cancel: state = %v, want TaskCancelled", info.State)
	}

	// Cancel a terminal task is a no-op (AC-19a).
	state, err = r.Cancel("alice", id)
	if err != nil {
		t.Fatalf("Cancel terminal: %v", err)
	}
	if state != TaskCancelled {
		t.Errorf("Cancel terminal: state = %v, want TaskCancelled", state)
	}
}

func TestTaskRegistry_TTLExpiry(t *testing.T) {
	r := newTestTaskRegistry(8, 2*time.Second)
	defer r.Close()

	fakeNow := time.Now()
	r.now = func() time.Time { return fakeNow }

	// requestedTTL=0 uses the registry default (2s).
	id, _, _, err := r.Create("alice")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	r.Transition(id, TaskCompleted)

	// Not expired yet (only 1s elapsed, TTL is 2s).
	fakeNow = fakeNow.Add(time.Second)
	r.sweep()
	if _, err := r.Get("alice", id); err != nil {
		t.Fatalf("should still exist: %v", err)
	}

	// Advance past TTL (total 3s > 2s).
	fakeNow = fakeNow.Add(2 * time.Second)
	r.sweep()
	if _, err := r.Get("alice", id); !errors.Is(err, errTaskNotFound) {
		t.Fatalf("should be expired, got err=%v", err)
	}
}

func TestTaskRegistry_IdentityScope(t *testing.T) {
	r := newTestTaskRegistry(8, time.Minute)
	defer r.Close()

	id, _, _, _ := r.Create("alice")

	// Bob cannot see Alice's task.
	if _, err := r.Get("bob", id); !errors.Is(err, errTaskNotFound) {
		t.Errorf("cross-identity Get: got %v, want errTaskNotFound", err)
	}
	// registry.Result and registry.List no longer exist. MCP
	// 2026-07-28 changelog Major change 6 removed the tasks/result and
	// tasks/list methods that backed them. The two cross-identity assertions
	// that named those methods therefore cannot be written. The scoping
	// property that those assertions proved is preserved and strengthened
	// below. Get is checked from both directions, and the assertions below
	// state directly that a foreign id and an unknown id are
	// indistinguishable. List proved that property only through an empty list.
	if _, err := r.Cancel("bob", id); !errors.Is(err, errTaskNotFound) {
		t.Errorf("cross-identity Cancel: got %v, want errTaskNotFound", err)
	}

	// Alice can see her own task, so the denials above are a real filter rather
	// than a lookup that fails for everyone.
	if _, err := r.Get("alice", id); err != nil {
		t.Errorf("alice Get on her own task: %v, want it to be visible", err)
	}
	if _, err := r.Cancel("alice", id); err != nil {
		t.Errorf("alice Cancel on her own task: %v, want it to be permitted", err)
	}

	// A foreign id is indistinguishable from an unknown id. Bob's denial and
	// the denial for an id that never existed are the SAME error. A caller
	// therefore cannot use the difference to probe for another principal's
	// task ids.
	_, foreignErr := r.Get("bob", id)
	_, unknownErr := r.Get("bob", "never-minted")
	if foreignErr == nil || unknownErr == nil {
		t.Fatalf("expected both lookups to fail, got foreign=%v unknown=%v", foreignErr, unknownErr)
	}
	if foreignErr.Error() != unknownErr.Error() {
		t.Errorf("foreign id error %q differs from unknown id error %q: existence leaks",
			foreignErr, unknownErr)
	}
}

func TestTaskRegistry_ConcurrencyCap(t *testing.T) {
	r := newTestTaskRegistry(2, time.Minute)
	defer r.Close()

	_, _, _, err := r.Create("alice")
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, _, _, err = r.Create("alice")
	if err != nil {
		t.Fatalf("second create: %v", err)
	}
	_, _, _, err = r.Create("alice")
	if !errors.Is(err, errTaskConcurrencyCap) {
		t.Fatalf("third create: got %v, want errTaskConcurrencyCap", err)
	}

	// A different identity is not affected.
	_, _, _, err = r.Create("bob")
	if err != nil {
		t.Fatalf("bob create: %v", err)
	}
}

// TestTaskConcurrencyCapIsPerPrincipal covers AC-14 and Boundary table row 2.
//
// It is the transport-level counterpart of TestTaskRegistry_ConcurrencyCap
// above. That test calls r.Create("alice", ...) with a literal string, so it
// would pass identically before and after the cutover and proves nothing about
// WHERE the identity comes from. This one drives the full HTTP path under
// auth-mode bearer-list, so the only thing that can scope the cap is the
// per-request authenticator's Identity.
//
// VALIDATES: one authenticated principal reaches exactly maxConcurrent (8)
// concurrent tasks, its next task is refused with the concurrency-cap error,
// and a DIFFERENT authenticated principal independently reaches 8 at the same
// time.
// PREVENTS: a global cap that acts as a per-principal one. Bob would be refused
// while alice holds 8. It also prevents an identity taken from anywhere but the
// credential. A body-supplied or shared identity would collapse the two
// principals into one bucket, and it would refuse bob's first task.
func TestTaskConcurrencyCapIsPerPrincipal(t *testing.T) {
	// The dispatcher blocks, so every created task stays non-terminal and holds
	// a concurrency slot. The workers finish when the deferred close releases
	// that channel.
	release := make(chan struct{})
	defer close(release)

	hs, cleanup := newTestStreamable(t, StreamableConfig{
		AuthMode: AuthBearerList,
		BearerList: []BearerListEntry{
			{Name: "alice", Token: "alice-token"},
			{Name: "bob", Token: "bob-token"},
		},
		// A `required` command is now MANDATORY for this test, not a detail.
		// The dispatcher below blocks until the test ends, which is what keeps
		// each task non-terminal and holding its slot. Against a command that
		// runs SYNCHRONOUSLY that same dispatcher would block the HTTP request
		// itself and the test would hang rather than fail.
		Commands: taskCapableCommands,
		Dispatch: func(ctx context.Context, _ plugin.CallerIdentity, cmd string) (*plugin.Response, error) {
			select {
			case <-release:
			case <-ctx.Done():
				return nil, ctx.Err()
			}
			return plugin.NewResponse(plugin.StatusDone, plugin.Map{"result": "ok", "message": cmd}), nil
		},
	})
	defer cleanup()

	// alice fills her bucket to the cap.
	for i := range defaultMaxConcurrentTasks {
		if taskID, errMsg := createTaskAs(t, hs, "alice-token"); taskID == "" {
			t.Fatalf("alice task %d/%d refused: %s", i+1, defaultMaxConcurrentTasks, errMsg)
		}
	}
	// One more is refused.
	if taskID, errMsg := createTaskAs(t, hs, "alice-token"); taskID != "" {
		t.Fatalf("alice task %d created, want the concurrency cap to refuse it", defaultMaxConcurrentTasks+1)
	} else if !strings.Contains(errMsg, "concurrency cap") {
		t.Fatalf("alice over-cap message = %q, want it to name the concurrency cap", errMsg)
	}

	// bob is a different principal, so his bucket is untouched: he reaches the
	// same cap while alice is still holding hers.
	for i := range defaultMaxConcurrentTasks {
		if taskID, errMsg := createTaskAs(t, hs, "bob-token"); taskID == "" {
			t.Fatalf("bob task %d/%d refused while alice holds %d: %s -- the cap is global, not per principal",
				i+1, defaultMaxConcurrentTasks, defaultMaxConcurrentTasks, errMsg)
		}
	}
	// And bob's cap is real too, not merely absent.
	if taskID, errMsg := createTaskAs(t, hs, "bob-token"); taskID != "" {
		t.Fatalf("bob task %d created, want the concurrency cap to refuse it", defaultMaxConcurrentTasks+1)
	} else if !strings.Contains(errMsg, "concurrency cap") {
		t.Fatalf("bob over-cap message = %q, want it to name the concurrency cap", errMsg)
	}
}

// createTaskAs sends one tools/call as the given bearer token and returns
// either the new task id or the JSON-RPC error message. The params carry no
// task member: the server creates the task from the command's annotation.
func createTaskAs(t *testing.T, hs *httptest.Server, token string) (taskID, errMsg string) {
	t.Helper()
	status, parsed := postMCPAuth(t, hs, token, methodToolsCall, capsTasks,
		`{"name":"ze_slow","arguments":{"action":"cmd"}}`)
	if status != http.StatusOK {
		t.Fatalf("tools/call: status = %d, want 200 (body %v)", status, parsed)
	}
	if rpcErr, isErr := parsed["error"].(map[string]any); isErr {
		msg, _ := rpcErr["message"].(string)
		return "", msg
	}
	id, _ := resultOf(t, parsed)["taskId"].(string)
	if id == "" {
		t.Fatalf("neither a taskId nor an error in %v", parsed)
	}
	return id, ""
}

// TestTasksCancelAcknowledgesWithAnEmptyResult covers AC-9's result shape over
// the real transport.
//
// The shape had NO wire-level test at all: methodTasksCancel appeared in the
// tests only as a row of resultBearingMethods (which asserts envelope
// invariants, not the payload) and in the without-capability table. So the
// handler returning {"taskId":..., "status":...} while AC-9 and the extension
// both say "empty result" went unnoticed through the whole phase.
//
// VALIDATES: a tasks/cancel acknowledgment carries NOTHING beyond the two
// envelope fields that ok() stamps on every result. It carries no taskId and no
// status, for a working task and for one that already reached a terminal state.
// The cancellation still took effect, and the test confirms it through
// tasks/get.
// PREVENTS: a second payload on the acknowledgment. A status reported here is a
// snapshot taken before the client can read it. Cancellation is cooperative, so
// a worker past its last check still completes. A client that treated the
// acknowledgment as final would then record the wrong terminal state.
func TestTasksCancelAcknowledgesWithAnEmptyResult(t *testing.T) {
	// The two envelope fields every result carries. Anything else in a
	// cancellation acknowledgement is a payload the extension does not sanction.
	assertEmptyAck := func(t *testing.T, result map[string]any) {
		t.Helper()
		for key := range result {
			if key != resultTypeKey && key != metaKey {
				t.Errorf("tasks/cancel acknowledgement carries %q = %v; the extension specifies an empty result",
					key, result[key])
			}
		}
		if result[resultTypeKey] != resultTypeComplete {
			t.Errorf("resultType = %v, want %q", result[resultTypeKey], resultTypeComplete)
		}
	}

	t.Run("working task", func(t *testing.T) {
		// The dispatcher blocks until the worker's context is canceled, so the
		// task is genuinely RUNNING when the cancel arrives. A cancel of an
		// already-finished task cannot prove the cancellation path at all.
		hs, cleanup := newTestStreamable(t, StreamableConfig{
			Commands: taskCapableCommands,
			Dispatch: func(ctx context.Context, _ plugin.CallerIdentity, cmd string) (*plugin.Response, error) {
				<-ctx.Done()
				return nil, ctx.Err()
			},
		})
		defer cleanup()

		taskID, errMsg := createTaskAs(t, hs, "")
		if taskID == "" {
			t.Fatalf("create task: %s", errMsg)
		}

		status, parsed := postMCP(t, hs, methodTasksCancel, capsTasks, `{"taskId":"`+taskID+`"}`)
		if status != http.StatusOK {
			t.Fatalf("tasks/cancel: status = %d, want 200 (body %v)", status, parsed)
		}
		assertEmptyAck(t, resultOf(t, parsed))

		// The empty ack is not an empty action: the state moved, and tasks/get
		// is where a client reads it.
		waitTaskTerminal(t, hs, taskID)
		_, got := postMCP(t, hs, methodTasksGet, capsTasks, `{"taskId":"`+taskID+`"}`)
		if state := resultOf(t, got)["status"]; state != TaskCancelled.String() {
			t.Errorf("post-cancel status = %v, want %q", state, TaskCancelled.String())
		}
	})

	t.Run("already terminal task", func(t *testing.T) {
		hs, cleanup := newTestStreamable(t, StreamableConfig{Commands: taskCapableCommands})
		defer cleanup()

		taskID := createTestTask(t, hs)

		status, parsed := postMCP(t, hs, methodTasksCancel, capsTasks, `{"taskId":"`+taskID+`"}`)
		if status != http.StatusOK {
			t.Fatalf("tasks/cancel: status = %d, want 200 (body %v)", status, parsed)
		}
		assertEmptyAck(t, resultOf(t, parsed))

		// A cancel on a terminal task is a no-op. The acknowledgment says nothing
		// about the outcome, so the test confirms the state through tasks/get.
		_, got := postMCP(t, hs, methodTasksGet, capsTasks, `{"taskId":"`+taskID+`"}`)
		if state := resultOf(t, got)["status"]; state != TaskCompleted.String() {
			t.Errorf("post-cancel status = %v, want %q; a cancel rewrote a terminal state", state, TaskCompleted.String())
		}
	})
}

// TestClampTTLBoundaries covers Boundary table row 3.
//
// VALIDATES: clampTaskTTL keeps a configured TTL inside
// [minTaskTTL, maxTaskTTL], silently clamping in both directions, and treats a
// non-positive value as "unspecified" by falling back to defaultTaskTTL.
// PREVENTS: an off-by-one at either bound, and the non-positive sentinel being
// folded into the low clamp -- 0 means "no TTL was configured", which must
// yield the default, not the 1s floor.
//
// the clamp moved from a registry METHOD on the per-create path to
// a package function called once by newTaskRegistry. The client-requested TTL
// branch died with params.task (D-1), so TaskRegistryConfig.TTL is the only TTL
// input left and the bound belongs where it enters. Every case below is kept;
// only the receiver changed.
func TestClampTTLBoundaries(t *testing.T) {
	cases := []struct {
		name      string
		requested time.Duration
		want      time.Duration
	}{
		// Non-positive is the "unspecified" sentinel, not a low-side clamp.
		{"negative falls back to the default", -time.Second, defaultTaskTTL},
		{"zero falls back to the default", 0, defaultTaskTTL},
		// Low bound: anything positive below the floor is raised to it.
		{"one nanosecond clamps up to the floor", time.Nanosecond, minTaskTTL},
		{"just below the floor clamps up", minTaskTTL - time.Millisecond, minTaskTTL},
		{"the floor itself is kept", minTaskTTL, minTaskTTL},
		// High bound: 3600s is the last valid value, 3601s is clamped.
		{"3600s is kept", 3600 * time.Second, maxTaskTTL},
		{"3601s clamps down to the ceiling", 3601 * time.Second, maxTaskTTL},
		{"far above the ceiling clamps down", 24 * time.Hour, maxTaskTTL},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := clampTaskTTL(tc.requested); got != tc.want {
				t.Fatalf("clampTaskTTL(%v) = %v, want %v", tc.requested, got, tc.want)
			}
		})
	}

	// The same table again, driven from the ENTRY POINT that the clamp now sits
	// on rather than from the helper (ai/rules/evidence.md: test a
	// guard where it fires, not the helper alone).
	//
	// A direct call to clampTaskTTL cannot see the regression that matters.
	// newTaskRegistry is the one caller, and retentionHints clamps the value
	// again before it reaches the client. If you delete the constructor's clamp,
	// the CLIENT-facing ttlMs stays correct and r.ttl keeps an unclamped value.
	// Create copies r.ttl onto every entry, and the sweep reads r.ttl as the
	// retention window. A configured TTL of 24h would then retain terminal tasks
	// for 24 hours, and every client would read an expiry of one hour.
	t.Run("entry point", func(t *testing.T) {
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				r := newTaskRegistry(TaskRegistryConfig{TTL: tc.requested})
				defer r.Close()

				if r.ttl != tc.want {
					t.Fatalf("newTaskRegistry(TTL: %v).ttl = %v, want %v -- the sweep window is unclamped",
						tc.requested, r.ttl, tc.want)
				}

				// And the value an entry actually carries: Create copies r.ttl
				// onto the entry, and the sweep compares against entry.ttl.
				id, _, _, err := r.Create("alice")
				if err != nil {
					t.Fatalf("Create: %v", err)
				}
				r.mu.Lock()
				entry := r.tasks[id]
				r.mu.Unlock()
				entry.mu.Lock()
				got := entry.ttl
				entry.mu.Unlock()
				if got != tc.want {
					t.Fatalf("entry ttl = %v, want %v -- the retention window the sweep reads is unclamped", got, tc.want)
				}
			})
		}
	})

	// The bounds are the ones the boundary table names, not whatever the
	// constants drifted to.
	if minTaskTTL != time.Second {
		t.Errorf("minTaskTTL = %v, want 1s", minTaskTTL)
	}
	if maxTaskTTL != time.Hour {
		t.Errorf("maxTaskTTL = %v, want 1h", maxTaskTTL)
	}
}

// TestRetentionHintsInvariant covers Boundary table row 2 (D-6).
//
// VALIDATES: pollIntervalMs is positive and never exceeds half of ttlMs, for
// every legal TTL including both bounds.
// PREVENTS: a client that obeys the poll hint sleeping past a terminal result
// and finding it already swept. At the 1s minimum TTL a fixed 1000ms constant
// would do exactly that.
func TestRetentionHintsInvariant(t *testing.T) {
	for _, ttl := range []time.Duration{
		minTaskTTL, 2 * time.Second, 30 * time.Second,
		defaultTaskTTL, maxTaskTTL,
	} {
		t.Run(ttl.String(), func(t *testing.T) {
			r := newTestTaskRegistry(8, ttl)
			defer r.Close()

			ttlMs, pollMs := r.retentionHints()
			if ttlMs <= 0 {
				t.Fatalf("ttlMs = %d, want positive", ttlMs)
			}
			if pollMs <= 0 {
				t.Fatalf("pollIntervalMs = %d, want positive: 0 makes a conforming client busy-loop", pollMs)
			}
			if pollMs > ttlMs/2 {
				t.Fatalf("pollIntervalMs = %d exceeds half of ttlMs = %d: a client obeying the hint could miss the result",
					pollMs, ttlMs)
			}
		})
	}
}

func TestTaskRegistry_TransitionAndResult(t *testing.T) {
	r := newTestTaskRegistry(8, time.Minute)
	defer r.Close()

	id, _, _, _ := r.Create("alice")

	// registry.Result is gone with tasks/result (changelog Major
	// change 6). The payload now rides on the Get snapshot. registry.Result
	// proved that a task in the working state exposes no result. The assertions
	// below prove that same property directly rather than through a removed
	// method's error.
	//
	// A working task exposes no result. That absence is what lets a polling
	// client tell "still running" from "finished with no output".
	working, err := r.Get("alice", id)
	if err != nil {
		t.Fatalf("Get on working: %v", err)
	}
	if working.Result != nil {
		t.Fatalf("working task exposes a result: %v", working.Result)
	}
	if _, present := working.toWire()["result"]; present {
		t.Fatalf("working task's wire form carries a result: %v", working.toWire())
	}

	// Transition to completed.
	result := map[string]any{"content": []any{map[string]any{"type": "text", "text": "done"}}}
	r.storeResult(id, result)
	if !r.Transition(id, TaskCompleted) {
		t.Fatal("Transition to completed failed")
	}

	got, err := r.Get("alice", id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if got.Result == nil || got.Result["content"] == nil {
		t.Fatalf("terminal task lost its result content: %v", got.Result)
	}
	if got.toWire()["result"] == nil {
		t.Fatalf("terminal task's wire form carries no result: %v", got.toWire())
	}

	// Terminal -> terminal transition rejected.
	if r.Transition(id, TaskFailed) {
		t.Fatal("should reject terminal -> terminal")
	}
}

// TestStuckTaskForcedTerminalAtDeadline covers AC-13 and closes R-3.
//
// VALIDATES: a task whose work never returns is forced to a terminal state at
// its execution deadline. The task releases the concurrency slot that it held,
// and the retention sweep then deletes the task.
// PREVENTS: the hole that MCP 2026-07-28 opened when it removed sessions.
// CancelAllForSession was the only path that forced a NON-terminal task
// terminal. The TTL sweep deletes only entries that already reached a terminal
// state. Without the deadline, a wedged worker would hold one of its
// principal's maxConcurrent slots forever.
//
// The work function here deliberately IGNORES its context, and that is the
// whole point. A cancel of the worker's context is not the guarantee, because a
// wedged dispatch never observes the cancellation. The registry must make the
// ENTRY terminal on its own.
func TestStuckTaskForcedTerminalAtDeadline(t *testing.T) {
	const execDeadline = time.Minute
	r := newTaskRegistry(TaskRegistryConfig{
		MaxConcurrent: 1,
		MaxTerminal:   128,
		TTL:           time.Minute,
		ExecDeadline:  execDeadline,
	})
	defer r.Close()

	// An injected clock: the sweep is driven directly rather than waited on, so
	// the test asserts on state transitions and never on elapsed time
	// (ai/rules/completion.md).
	base := time.Now()
	now := base
	r.now = func() time.Time { return now }

	id, ctx, _, err := r.Create("alice")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// A worker that never returns and never looks at its context.
	release := make(chan struct{})
	started := make(chan struct{})
	runTaskWorker(ctx, r, id, func(_ context.Context) (map[string]any, error) {
		close(started)
		<-release
		return map[string]any{}, nil
	})
	<-started
	defer close(release)

	// The slot is held: the cap is 1, so a second create must be refused while
	// the first task is still working.
	if _, _, _, err := r.Create("alice"); !errors.Is(err, errTaskConcurrencyCap) {
		t.Fatalf("second create while one is in flight: %v, want errTaskConcurrencyCap", err)
	}

	// Before the deadline the sweep leaves it alone.
	now = base.Add(execDeadline / 2)
	r.sweep()
	info, err := r.Get("alice", id)
	if err != nil {
		t.Fatalf("Get before deadline: %v", err)
	}
	if info.State != TaskWorking {
		t.Fatalf("state before deadline = %v, want working: the sweep reaped it early", info.State)
	}

	// Past the deadline the sweep forces it terminal, even though the goroutine
	// is still blocked.
	now = base.Add(execDeadline + time.Second)
	r.sweep()
	info, err = r.Get("alice", id)
	if err != nil {
		t.Fatalf("Get after deadline: %v", err)
	}
	if !info.State.IsTerminal() {
		t.Fatalf("state after deadline = %v, want a terminal state: the wedged task is immortal", info.State)
	}
	if info.ErrorMsg == "" {
		t.Errorf("task forced terminal at its deadline carries no error message")
	}

	// And the slot is released: that is the resource this whole mechanism
	// exists to reclaim.
	if _, _, _, err := r.Create("alice"); err != nil {
		t.Fatalf("create after the stuck task was reaped: %v, want the slot to be free", err)
	}

	// Finally the retention sweep deletes it once its TTL expires.
	now = base.Add(execDeadline + time.Second + 2*time.Minute)
	r.sweep()
	if _, err := r.Get("alice", id); !errors.Is(err, errTaskNotFound) {
		t.Errorf("Get after retention expiry: %v, want errTaskNotFound", err)
	}
}

// TestLateWorkerReturnDoesNotOverwriteTerminalEntry is the regression test for
// the race between the deadline sweep and the worker's own terminal transition.
//
// The sweep (pass 1) makes a past-deadline WORKING entry terminal: state
// TaskFailed, terminalAt, and errTaskExecDeadline as the error message. Only
// Transition refused terminal -> any. storeResult and setErrorMsg had no such
// check, and runTaskWorker calls both BEFORE its Transition. A worker that
// returned late therefore wrote its result and its error over an entry that the
// sweep had already closed.
//
// toWire then emitted `error` AND `result` on one entry. A client that polled
// tasks/get saw status "failed" with a deadline diagnostic beside a complete,
// correct answer. toWire's own godoc says that a terminal task carries one or
// the other.
//
// TestStuckTaskForcedTerminalAtDeadline cannot see this race. That test keeps
// its worker blocked until cleanup, so the late return never happens. This test
// releases the worker after the sweep, and it then asserts on the entry.
//
// VALIDATES: after a deadline-forced failure, a worker's late storeResult and
// setErrorMsg are both dropped. The wire form keeps the deadline error and
// carries no result.
// PREVENTS: removal of the guard as redundant with Transition's guard. The
// guard is not redundant: these are two different writers, and the payload
// writers run first.
func TestLateWorkerReturnDoesNotOverwriteTerminalEntry(t *testing.T) {
	const execDeadline = time.Minute
	r := newTaskRegistry(TaskRegistryConfig{
		MaxConcurrent: 1,
		MaxTerminal:   128,
		TTL:           time.Minute,
		ExecDeadline:  execDeadline,
	})
	defer r.Close()

	base := time.Now()
	now := base
	r.now = func() time.Time { return now }

	id, ctx, _, err := r.Create("alice")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}

	// A worker that finishes successfully, but only after the sweep has already
	// given up on it. It ignores its context, so it produces a real payload:
	// that is what must not land on the closed entry.
	release := make(chan struct{})
	started := make(chan struct{})
	workReturned := make(chan struct{})
	runTaskWorker(ctx, r, id, func(_ context.Context) (map[string]any, error) {
		defer close(workReturned)
		close(started)
		<-release
		return map[string]any{"content": []any{map[string]any{"type": "text", "text": "too late"}}}, nil
	})
	<-started

	// The sweep forces the entry terminal while the worker is still blocked.
	now = base.Add(execDeadline + time.Second)
	r.sweep()

	forced, err := r.Get("alice", id)
	if err != nil {
		t.Fatalf("Get after the sweep: %v", err)
	}
	if forced.State != TaskFailed {
		t.Fatalf("state after the sweep = %v, want failed", forced.State)
	}

	// Now let the worker return. runTaskWorker's goroutine then performs its
	// three registry writes -- storeResult, setErrorMsg, Transition -- against an
	// entry that is already terminal.
	close(release)
	<-workReturned

	// A write that is correctly DROPPED leaves no signal, so there is nothing to
	// wait ON. The join above puts the worker one statement away from its first
	// registry call. The loop below re-reads the entry across many scheduling
	// opportunities, and it fails the instant the overwrite appears. A yield
	// rather than a sleep keeps the test independent of wall-clock time. Any
	// residual imprecision runs in the safe direction, because an unobserved
	// write makes this test pass and never fail. The mutation check proves that
	// the test is sensitive: remove either guard and the test goes red on the
	// first iteration.
	for range lateWriteObservations {
		after, err := r.Get("alice", id)
		if err != nil {
			t.Fatalf("Get after the late return: %v", err)
		}
		if after.State != TaskFailed {
			t.Fatalf("state = %v, want it to stay failed: the late return reopened a terminal task", after.State)
		}
		if after.ErrorMsg != errTaskExecDeadline.Error() {
			t.Fatalf("errorMsg = %q, want the deadline diagnostic to survive the late return", after.ErrorMsg)
		}
		if after.Result != nil {
			t.Fatalf("a task failed at its deadline gained a result from its late worker: %v", after.Result)
		}
		// The wire form is what a client actually reads, and it must never carry
		// both outcomes at once.
		wire := after.toWire()
		if _, hasError := wire["error"]; !hasError {
			t.Fatalf("wire form carries no error for a deadline-failed task: %v", wire)
		}
		if _, hasResult := wire["result"]; hasResult {
			t.Fatalf("wire form carries BOTH error and result: %v", wire)
		}
		runtime.Gosched()
	}

	// setErrorMsg's guard is defense in depth, and the path above cannot reach
	// it. Sweep pass 1 cancels the worker's context, so runTaskWorker always
	// computes finalState TaskCancelled with an empty errMsg. runTaskWorker
	// therefore never calls setErrorMsg. A direct call is the only way to hold
	// all three writers to one rule.
	r.setErrorMsg(id, "a later, wrong explanation")
	if final, getErr := r.Get("alice", id); getErr != nil {
		t.Fatalf("Get after the direct setErrorMsg: %v", getErr)
	} else if final.ErrorMsg != errTaskExecDeadline.Error() {
		t.Errorf("errorMsg = %q, want the terminal entry's diagnostic to be immutable", final.ErrorMsg)
	}
}

// lateWriteObservations is how many times the loop above re-reads an entry that
// must not change. The count is large enough that a runnable goroutine three
// mutex acquisitions from done finishes first. The count is also small enough
// to cost microseconds when the code is correct.
const lateWriteObservations = 2000

// TestCloseCancelsInFlightWorkers is the regression test for a godoc that
// promised a cancellation nothing performed.
//
// taskWorkerFunc's godoc says "ctx is canceled on tasks/cancel and on
// task-registry shutdown". Only the first half was true. Close closed r.stop
// and waited for the GC goroutine, and Close never walked r.tasks. An in-flight
// worker therefore survived Streamable.Close() with a live context, and no
// sweeper was left to force its entry terminal. That is a real leak on
// shutdown, not a stale comment.
//
// VALIDATES: Close cancels the context of every task still working.
// PREVENTS: a second removal of the walk as "the process is exiting anyway".
// The process is not exiting. Streamable.Close runs on a config reload and in
// every test that builds a server. A leaked worker therefore outlives its
// registry inside one process.
func TestCloseCancelsInFlightWorkers(t *testing.T) {
	r := newTestTaskRegistry(8, time.Minute)

	id, ctx, _, err := r.Create("alice")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if ctx.Err() != nil {
		t.Fatalf("fresh task context is already done: %v", ctx.Err())
	}

	r.Close()

	select {
	case <-ctx.Done():
	default:
		t.Fatalf("task %s still has a live context after Close: an in-flight worker outlives its registry", id)
	}
}

// TestTaskGCIndependentOfSessions covers R-3's first half.
//
// VALIDATES: the retention sweep reaps a terminal task with no session
// registry anywhere in the picture.
// PREVENTS: assuming the GC still needs the session lifecycle that MCP
// 2026-07-28 deleted to trigger it.
func TestTaskGCIndependentOfSessions(t *testing.T) {
	r := newTestTaskRegistry(8, minTaskTTL)
	defer r.Close()

	base := time.Now()
	now := base
	r.now = func() time.Time { return now }

	id, _, _, err := r.Create("alice")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	r.storeResult(id, map[string]any{"content": []any{}})
	if !r.Transition(id, TaskCompleted) {
		t.Fatal("Transition to completed failed")
	}

	// Still inside the retention window: a late poller must still get its
	// result.
	r.sweep()
	if _, err := r.Get("alice", id); err != nil {
		t.Fatalf("Get inside the retention window: %v, want the result to still be there", err)
	}

	// Past it, with no session having expired and no session registry existing.
	now = base.Add(minTaskTTL + time.Second)
	r.sweep()
	if _, err := r.Get("alice", id); !errors.Is(err, errTaskNotFound) {
		t.Errorf("Get after the retention window: %v, want errTaskNotFound", err)
	}
	// The identity index self-prunes with its last task, so it cannot outlive
	// the tasks it points at.
	r.mu.Lock()
	_, indexed := r.byIdentity["alice"]
	r.mu.Unlock()
	if indexed {
		t.Errorf("byIdentity still holds alice after her last task was swept")
	}
}

func TestTaskRegistry_NotFoundErrors(t *testing.T) {
	r := newTestTaskRegistry(8, time.Minute)
	defer r.Close()

	if _, err := r.Get("alice", "nonexistent"); !errors.Is(err, errTaskNotFound) {
		t.Errorf("Get nonexistent: %v, want errTaskNotFound", err)
	}
	if _, err := r.Cancel("alice", "nonexistent"); !errors.Is(err, errTaskNotFound) {
		t.Errorf("Cancel nonexistent: %v, want errTaskNotFound", err)
	}
}

func TestTaskInfo_ToWire(t *testing.T) {
	info := TaskInfo{
		ID:        "abc123",
		State:     TaskWorking,
		CreatedAt: time.Date(2026, 5, 11, 10, 0, 0, 0, time.UTC),
		UpdatedAt: time.Date(2026, 5, 11, 10, 0, 1, 0, time.UTC),
		Identity:  "alice",
	}
	wire := info.toWire()
	if wire["taskId"] != "abc123" {
		t.Errorf("taskId = %v", wire["taskId"])
	}
	if wire["status"] != "working" {
		t.Errorf("status = %v", wire["status"])
	}
	if wire["error"] != nil {
		t.Errorf("error should be absent, got %v", wire["error"])
	}

	info.ErrorMsg = "something broke"
	wire = info.toWire()
	if wire["error"] != "something broke" {
		t.Errorf("error = %v", wire["error"])
	}
}

func TestTaskWorker_DispatchCompletesAndStores(t *testing.T) {
	r := newTestTaskRegistry(8, time.Minute)
	defer r.Close()

	id, ctx, _, _ := r.Create("alice")

	done := make(chan struct{})
	work := func(ctx context.Context) (map[string]any, error) {
		defer close(done)
		return map[string]any{"content": []any{map[string]any{"type": "text", "text": "hello"}}}, nil
	}
	runTaskWorker(ctx, r, id, work)
	<-done

	// Give the goroutine a moment to finish transition.
	waitForState(t, r, id, TaskCompleted)

	info, err := r.Get("alice", id)
	if err != nil {
		t.Fatalf("Get: %v", err)
	}
	if info.Result == nil || info.Result["content"] == nil {
		t.Fatalf("result content is nil: %v", info.Result)
	}
}

func TestTaskWorker_DispatchErrorFails(t *testing.T) {
	r := newTestTaskRegistry(8, time.Minute)
	defer r.Close()

	id, ctx, _, _ := r.Create("alice")

	done := make(chan struct{})
	work := func(ctx context.Context) (map[string]any, error) {
		defer close(done)
		return nil, errors.New("dispatch broke")
	}
	runTaskWorker(ctx, r, id, work)
	<-done

	waitForState(t, r, id, TaskFailed)

	info, _ := r.Get("alice", id)
	if info.ErrorMsg != "dispatch broke" {
		t.Errorf("error msg = %q, want %q", info.ErrorMsg, "dispatch broke")
	}
}

func TestTaskWorker_CtxCancelTransitions(t *testing.T) {
	r := newTestTaskRegistry(8, time.Minute)
	defer r.Close()

	id, ctx, cancel, _ := r.Create("alice")

	started := make(chan struct{})
	work := func(ctx context.Context) (map[string]any, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	runTaskWorker(ctx, r, id, work)
	<-started
	cancel()

	waitForState(t, r, id, TaskCancelled)
}

const testIdentity = "alice"

func waitForState(t *testing.T, r *taskRegistry, taskID string, want TaskState) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		info, err := r.Get(testIdentity, taskID)
		if err == nil && info.State == want {
			return
		}
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("task %s did not reach state %v within deadline", taskID, want)
}

// newCappedTaskRegistry builds a registry with an explicit terminal cap so the
// eviction tests can drive it with a handful of tasks instead of 128.
func newCappedTaskRegistry(maxTerminal int) *taskRegistry {
	return newTaskRegistry(TaskRegistryConfig{
		MaxConcurrent: 8,
		MaxTerminal:   maxTerminal,
		TTL:           time.Hour,
	})
}

// terminalTask creates a task for identity and drives it to a terminal state,
// stamping terminalAt from the registry's injected clock. Returns the task id.
func terminalTask(t *testing.T, r *taskRegistry, identity string) string {
	t.Helper()
	id, _, _, err := r.Create(identity)
	if err != nil {
		t.Fatalf("Create(%s): %v", identity, err)
	}
	if !r.Transition(id, TaskCompleted) {
		t.Fatalf("Transition(%s) returned false", id)
	}
	return id
}

// retainedFor returns the ids the registry still holds for identity.
func retainedFor(r *taskRegistry, identity string) map[string]bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make(map[string]bool)
	for id := range r.byIdentity[identity] {
		out[id] = true
	}
	return out
}

// VALIDATES: evictTerminalOverCap bounds retained terminal tasks at the cap and
// drops the OLDEST terminal entries first.
// PREVENTS: unbounded growth of the task registry. The registry is the only
// long-lived per-client structure that survives the deletion of the session
// layer. A completed task frees its concurrency slot immediately, because
// activeCount counts non-terminal entries only. Without this cap, a client can
// cycle tasks to completion and accumulate result maps for the whole
// client-chosen TTL.
func TestTerminalCapEvictsOldestFirst(t *testing.T) {
	const cap = 3
	r := newCappedTaskRegistry(cap)
	defer r.Close()

	base := time.Now()
	fakeNow := base
	r.now = func() time.Time { return fakeNow }

	// Six terminal tasks, each one second newer than the last, so terminalAt
	// ordering is unambiguous and does not depend on map iteration.
	var ids []string
	for i := range 6 {
		fakeNow = base.Add(time.Duration(i) * time.Second)
		ids = append(ids, terminalTask(t, r, "alice"))
	}

	retained := retainedFor(r, "alice")
	if len(retained) != cap {
		t.Fatalf("retained %d terminal tasks, want the cap of %d", len(retained), cap)
	}
	// The three newest survive. The three oldest are gone.
	for _, id := range ids[:3] {
		if retained[id] {
			t.Errorf("oldest task %s survived; eviction is not oldest-first", id)
		}
	}
	for _, id := range ids[3:] {
		if !retained[id] {
			t.Errorf("newest task %s was evicted; eviction is not oldest-first", id)
		}
	}
}

// VALIDATES: a non-terminal task is never evicted, however many terminal tasks
// exist for the same principal.
// PREVENTS: an in-flight task vanishing because other tasks completed, which
// would surface to the client as a task id that stops existing mid-poll.
func TestTerminalCapNeverEvictsInFlight(t *testing.T) {
	const cap = 2
	r := newCappedTaskRegistry(cap)
	defer r.Close()

	base := time.Now()
	fakeNow := base
	r.now = func() time.Time { return fakeNow }

	// An in-flight task created FIRST, so it is the oldest by creation and
	// would be the first casualty if the cap ignored terminal state.
	inFlight, _, _, err := r.Create("alice")
	if err != nil {
		t.Fatalf("Create in-flight: %v", err)
	}

	for i := range 5 {
		fakeNow = base.Add(time.Duration(i+1) * time.Second)
		terminalTask(t, r, "alice")
	}

	if _, err := r.Get("alice", inFlight); err != nil {
		t.Fatalf("in-flight task was evicted: %v", err)
	}
	// The cap bounds terminal entries only, so the in-flight one sits on top.
	if got := len(retainedFor(r, "alice")); got != cap+1 {
		t.Fatalf("retained %d, want %d terminal plus the 1 in-flight", got, cap)
	}
}

// VALIDATES: the terminal cap is scoped per principal, so one principal
// exceeding it leaves another principal's results untouched.
// PREVENTS: the cross-principal denial a global cap would create, where a burst
// of quick tasks from one caller evicts every other caller's results.
func TestTerminalCapIsPerPrincipal(t *testing.T) {
	const cap = 2
	r := newCappedTaskRegistry(cap)
	defer r.Close()

	base := time.Now()
	fakeNow := base
	r.now = func() time.Time { return fakeNow }

	bobIDs := []string{terminalTask(t, r, "bob"), terminalTask(t, r, "bob")}

	// Alice blows well past the cap.
	for i := range 8 {
		fakeNow = base.Add(time.Duration(i+1) * time.Second)
		terminalTask(t, r, "alice")
	}

	if got := len(retainedFor(r, "alice")); got != cap {
		t.Errorf("alice retained %d, want the cap of %d", got, cap)
	}
	bobRetained := retainedFor(r, "bob")
	if len(bobRetained) != len(bobIDs) {
		t.Fatalf("bob retained %d of %d; alice's churn evicted across principals", len(bobRetained), len(bobIDs))
	}
	for _, id := range bobIDs {
		if !bobRetained[id] {
			t.Errorf("bob's task %s was evicted by alice's activity", id)
		}
	}
}

// VALIDATES: eviction keeps byIdentity consistent with tasks, and drops the
// identity key entirely once its last task is evicted.
// PREVENTS: the identity index becoming the unbounded map the eviction exists
// to prevent -- a leaked key is the same growth problem in a different shape.
func TestTerminalCapKeepsIdentityIndexConsistent(t *testing.T) {
	const cap = 2
	r := newCappedTaskRegistry(cap)
	defer r.Close()

	base := time.Now()
	fakeNow := base
	r.now = func() time.Time { return fakeNow }

	for i := range 6 {
		fakeNow = base.Add(time.Duration(i) * time.Second)
		terminalTask(t, r, "alice")
	}

	// Assert that eviction ran, then check the index that eviction maintains.
	// Without this assertion the test passes with eviction disabled, because an
	// index that never loses an entry is consistent for free.
	if got := len(retainedFor(r, "alice")); got != cap {
		t.Fatalf("retained %d, want the cap of %d; eviction did not run so this test would prove nothing", got, cap)
	}

	r.mu.Lock()
	for identity, ids := range r.byIdentity {
		if len(ids) == 0 {
			r.mu.Unlock()
			t.Fatalf("identity %q kept an empty id set instead of being deleted", identity)
		}
		for id := range ids {
			if _, ok := r.tasks[id]; !ok {
				r.mu.Unlock()
				t.Fatalf("byIdentity[%q] holds %s, which is absent from tasks", identity, id)
			}
		}
	}
	r.mu.Unlock()

	// Evicting every one of a principal's tasks removes the key itself.
	r.mu.Lock()
	for id := range r.byIdentity["alice"] {
		r.deleteLocked(id, "alice")
	}
	_, stillPresent := r.byIdentity["alice"]
	r.mu.Unlock()
	if stillPresent {
		t.Fatal("byIdentity kept the alice key after its last task was removed")
	}
}
