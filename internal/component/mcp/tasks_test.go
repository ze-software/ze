// test-relax: TestTaskRegistry_CancelAllForSession,
// TestTaskNotifications_StatusFrameShape and TestBuildTaskStatusNotification
// are removed with their subjects. CancelAllForSession existed only to cancel a
// session's tasks on session expiry, and MCP 2026-07-28 has no sessions to
// expire; the per-identity cancel path it shared is still asserted by
// TestTaskRegistry_CreateGetCancel (the happy path plus the terminal no-op) and
// TestTaskRegistry_IdentityScope (a cross-identity Cancel is a not-found).
// buildTaskStatusNotification pushed notifications/tasks/status onto the GET SSE
// stream this phase deletes, and the revision defines no server-to-client stream
// to replace it -- a client observes a task by polling tasks/get. Every cap, the
// identity index and the TTL clamp keep their tests below, unchanged.

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
	// test-relax: registry.Result and registry.List no longer exist -- MCP
	// 2026-07-28 changelog Major change 6 removed the tasks/result and
	// tasks/list methods they backed, so the two cross-identity assertions
	// naming them cannot be written. The scoping property they proved is
	// preserved and strengthened below: Get is checked from both directions,
	// and the foreign/unknown indistinguishability that List's emptiness only
	// implied is now asserted directly.
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

	// A foreign id is indistinguishable from an unknown one: bob's denial and
	// the denial for an id that never existed are the SAME error, so a caller
	// cannot use the difference to probe for another principal's task ids.
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
// PREVENTS: a global cap masquerading as a per-principal one (bob would be
// refused while alice holds 8), and the identity being taken from anywhere but
// the credential -- a body-supplied or shared identity would collapse the two
// principals into one bucket and refuse bob's very first task.
func TestTaskConcurrencyCapIsPerPrincipal(t *testing.T) {
	// The dispatcher blocks so every created task stays non-terminal and keeps
	// occupying a concurrency slot; releasing it lets the workers finish.
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
			return plugin.NewResponse(plugin.StatusDone, plugin.RawJSON("ok: "+cmd)), nil
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
// VALIDATES: a tasks/cancel acknowledgement carries NOTHING beyond the two
// envelope fields ok() stamps on every result -- no taskId, no status -- for a
// working task and for one that already reached a terminal state; and the
// cancellation still took effect, read back through tasks/get.
// PREVENTS: the acknowledgement growing a payload again. A status reported here
// is a snapshot taken before the client can read it (cancellation is
// cooperative, so a worker past its last check still completes), and a client
// that treated the ack as final would record the wrong terminal state.
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

		// Canceling a terminal task is a no-op, and the ack says nothing about
		// that either way -- which is why the state has to be read back.
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
// test-relax: the clamp moved from a registry METHOD on the per-create path to
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

	// The same table again, driven from the ENTRY POINT the clamp now sits on
	// rather than from the helper (ai/rules/fail-closed-guards.md: test a guard
	// where it fires, not the helper alone).
	//
	// Calling clampTaskTTL directly cannot see the regression that matters.
	// newTaskRegistry is the one caller, and retentionHints re-clamps on the way
	// out, so deleting the constructor's clamp leaves the CLIENT-facing ttlMs
	// correct while r.ttl -- copied onto every entry at Create and read by the
	// sweep as the retention window -- keeps an unclamped value. A 24h configured
	// TTL would then retain terminal tasks for 24 hours while telling every
	// client they expire in one.
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

	// test-relax: registry.Result is gone with tasks/result (changelog Major
	// change 6). The payload now rides on the Get snapshot, and the
	// "not readable while working" property it proved is asserted directly
	// below rather than through a removed method's error.
	//
	// A working task exposes no result: that is what lets a polling client tell
	// "still running" from "finished with no output".
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
// its execution deadline, releasing the concurrency slot it held, and is then
// deleted by the retention sweep.
// PREVENTS: the hole MCP 2026-07-28 opened by removing sessions.
// CancelAllForSession was the only path that could force a NON-terminal task
// terminal; the TTL sweep deletes only entries that already reached a terminal
// state, so without the deadline a wedged worker would hold one of its
// principal's maxConcurrent slots forever.
//
// The work function here deliberately IGNORES its context, which is the whole
// point: canceling the worker's context is not the guarantee, because a
// genuinely wedged dispatch never observes cancellation. The registry must make
// the ENTRY terminal on its own.
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
	// (ai/rules/fix-dont-record.md).
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
// the deadline sweep racing the worker's own terminal transition.
//
// The sweep (pass 1) makes a past-deadline WORKING entry terminal: state
// TaskFailed, terminalAt, and errTaskExecDeadline as the error message. Only
// Transition refused terminal -> any. storeResult and setErrorMsg had no such
// check, and runTaskWorker calls both BEFORE its Transition -- so a worker that
// eventually returned wrote its result and its error over an entry the sweep had
// already closed. toWire then emitted `error` AND `result` on one entry: a
// client polling tasks/get saw status "failed" with a deadline diagnostic beside
// a complete, correct answer. toWire's own godoc says a terminal task carries
// one or the other.
//
// TestStuckTaskForcedTerminalAtDeadline cannot see this: it keeps its worker
// blocked until cleanup, so the late return never happens. This test releases
// the worker after the sweep and asserts on what the entry looks like
// afterwards.
//
// VALIDATES: after a deadline-forced failure, a worker's late storeResult and
// setErrorMsg are both dropped; the wire form keeps the deadline error and
// carries no result.
// PREVENTS: the guard being removed as redundant with Transition's. It is not:
// these are two different writers, and the payload writers run first.
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
	// wait ON: the join above puts the worker one statement away from its first
	// registry call, and the loop below re-reads the entry across many scheduling
	// opportunities, failing the instant the overwrite appears. Yielding rather
	// than sleeping keeps it off the clock entirely. The direction of any
	// residual imprecision is the safe one -- an unobserved write makes this test
	// pass, never fail -- and the mutation check (remove either guard, watch it
	// go red on the first iteration) is what proves it is sensitive.
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

	// setErrorMsg's guard is defense in depth and cannot be reached through the
	// path above: sweep pass 1 cancels the worker's context, so runTaskWorker
	// always computes finalState TaskCancelled with an empty errMsg and never
	// calls it. Driving it directly is the only way to hold all three writers to
	// one rule.
	r.setErrorMsg(id, "a later, wrong explanation")
	if final, getErr := r.Get("alice", id); getErr != nil {
		t.Fatalf("Get after the direct setErrorMsg: %v", getErr)
	} else if final.ErrorMsg != errTaskExecDeadline.Error() {
		t.Errorf("errorMsg = %q, want the terminal entry's diagnostic to be immutable", final.ErrorMsg)
	}
}

// lateWriteObservations is how many times the loop above re-reads an entry that
// must not change. Large enough that a runnable goroutine three mutex
// acquisitions from done will have finished, cheap enough to cost microseconds
// when nothing is wrong.
const lateWriteObservations = 2000

// TestCloseCancelsInFlightWorkers is the regression test for a godoc that
// promised a cancellation nothing performed.
//
// taskWorkerFunc's godoc says "ctx is canceled on tasks/cancel and on
// task-registry shutdown". Only the first half was true: Close closed r.stop and
// waited for the GC goroutine, and never walked r.tasks. An in-flight worker
// therefore survived Streamable.Close() with a live context and no sweeper left
// to force its entry terminal -- a real leak on shutdown, not just a stale
// comment.
//
// VALIDATES: Close cancels the context of every task still working.
// PREVENTS: the walk being dropped again as "the process is exiting anyway". It
// is not: Streamable.Close runs on a config reload and in every test that builds
// a server, so a leaked worker outlives its registry inside one process.
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

	// test-relax: registry.Result is gone with tasks/result; the completed
	// task's payload is read from the Get snapshot instead. Same assertion,
	// surviving accessor.
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
// PREVENTS: unbounded growth of the task registry, which after the session
// layer's deletion is the only long-lived per-client structure. A completed
// task frees its concurrency slot immediately (activeCount counts non-terminal
// only), so without this cap a client can cycle tasks to completion and
// accumulate result maps for the whole client-chosen TTL.
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
	// The three newest survive; the three oldest are gone.
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

	// Assert eviction actually ran before checking the index it maintains.
	// Without this the test passes with eviction disabled -- the index is
	// trivially consistent when nothing is ever removed from it.
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
