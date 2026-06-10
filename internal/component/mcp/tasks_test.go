package mcp

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
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

	id, ctx, _, err := r.Create("alice", "sess1", 0)
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
	id, _, _, err := r.Create("alice", "sess1", 0)
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

	id, _, _, _ := r.Create("alice", "sess1", 0)

	// Bob cannot see Alice's task.
	if _, err := r.Get("bob", id); !errors.Is(err, errTaskNotFound) {
		t.Errorf("cross-identity Get: got %v, want errTaskNotFound", err)
	}
	if _, err := r.Result("bob", id); !errors.Is(err, errTaskNotFound) {
		t.Errorf("cross-identity Result: got %v, want errTaskNotFound", err)
	}
	if _, err := r.Cancel("bob", id); !errors.Is(err, errTaskNotFound) {
		t.Errorf("cross-identity Cancel: got %v, want errTaskNotFound", err)
	}

	// Alice's List shows the task; Bob's does not.
	aliceList := r.List("alice")
	if len(aliceList) != 1 {
		t.Errorf("alice List len = %d, want 1", len(aliceList))
	}
	bobList := r.List("bob")
	if len(bobList) != 0 {
		t.Errorf("bob List len = %d, want 0", len(bobList))
	}
}

func TestTaskRegistry_ConcurrencyCap(t *testing.T) {
	r := newTestTaskRegistry(2, time.Minute)
	defer r.Close()

	_, _, _, err := r.Create("alice", "sess1", 0)
	if err != nil {
		t.Fatalf("first create: %v", err)
	}
	_, _, _, err = r.Create("alice", "sess1", 0)
	if err != nil {
		t.Fatalf("second create: %v", err)
	}
	_, _, _, err = r.Create("alice", "sess1", 0)
	if !errors.Is(err, errTaskConcurrencyCap) {
		t.Fatalf("third create: got %v, want errTaskConcurrencyCap", err)
	}

	// A different identity is not affected.
	_, _, _, err = r.Create("bob", "sess2", 0)
	if err != nil {
		t.Fatalf("bob create: %v", err)
	}
}

func TestTaskRegistry_TransitionAndResult(t *testing.T) {
	r := newTestTaskRegistry(8, time.Minute)
	defer r.Close()

	id, _, _, _ := r.Create("alice", "sess1", 0)

	// Result on non-terminal fails.
	if _, err := r.Result("alice", id); !errors.Is(err, errTaskNotTerminal) {
		t.Fatalf("Result on working: got %v, want errTaskNotTerminal", err)
	}

	// Transition to completed.
	if !r.Transition(id, TaskCompleted) {
		t.Fatal("Transition to completed failed")
	}
	result := map[string]any{"content": []any{map[string]any{"type": "text", "text": "done"}}}
	r.StoreResult(id, result)

	got, err := r.Result("alice", id)
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if got["content"] == nil {
		t.Fatal("Result content is nil")
	}

	// Terminal -> terminal transition rejected.
	if r.Transition(id, TaskFailed) {
		t.Fatal("should reject terminal -> terminal")
	}
}

func TestTaskRegistry_CancelAllForSession(t *testing.T) {
	r := newTestTaskRegistry(8, time.Minute)
	defer r.Close()

	id1, _, _, _ := r.Create("alice", "sess1", 0)
	id2, _, _, _ := r.Create("alice", "sess1", 0)
	id3, _, _, _ := r.Create("alice", "sess2", 0)

	// Complete one so it should not be re-canceled.
	r.Transition(id2, TaskCompleted)

	r.CancelAllForSession("sess1")

	info1, _ := r.Get("alice", id1)
	if info1.State != TaskCancelled {
		t.Errorf("id1 state = %v, want TaskCancelled", info1.State)
	}
	info2, _ := r.Get("alice", id2)
	if info2.State != TaskCompleted {
		t.Errorf("id2 state = %v, want completed (should not be re-canceled)", info2.State)
	}
	info3, _ := r.Get("alice", id3)
	if info3.State != TaskWorking {
		t.Errorf("id3 state = %v, want working (different session)", info3.State)
	}
}

func TestTaskRegistry_NotFoundErrors(t *testing.T) {
	r := newTestTaskRegistry(8, time.Minute)
	defer r.Close()

	if _, err := r.Get("alice", "nonexistent"); !errors.Is(err, errTaskNotFound) {
		t.Errorf("Get nonexistent: %v, want errTaskNotFound", err)
	}
	if _, err := r.Result("alice", "nonexistent"); !errors.Is(err, errTaskNotFound) {
		t.Errorf("Result nonexistent: %v, want errTaskNotFound", err)
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
	wire := info.ToWire()
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
	wire = info.ToWire()
	if wire["error"] != "something broke" {
		t.Errorf("error = %v", wire["error"])
	}
}

func TestTaskWorker_DispatchCompletesAndStores(t *testing.T) {
	r := newTestTaskRegistry(8, time.Minute)
	defer r.Close()

	reg := newSessionRegistry(time.Minute, 0, 10)
	defer reg.Close()
	sess, _ := reg.Create("2025-06-18", Identity{Name: "alice"})

	id, ctx, _, _ := r.Create("alice", sess.ID(), 0)

	done := make(chan struct{})
	work := func(ctx context.Context) (map[string]any, error) {
		defer close(done)
		return map[string]any{"content": []any{map[string]any{"type": "text", "text": "hello"}}}, nil
	}
	runTaskWorker(ctx, r, sess, id, work)
	<-done

	// Give the goroutine a moment to finish transition.
	waitForState(t, r, id, TaskCompleted)

	result, err := r.Result("alice", id)
	if err != nil {
		t.Fatalf("Result: %v", err)
	}
	if result["content"] == nil {
		t.Fatal("result content is nil")
	}
}

func TestTaskWorker_DispatchErrorFails(t *testing.T) {
	r := newTestTaskRegistry(8, time.Minute)
	defer r.Close()

	id, ctx, _, _ := r.Create("alice", "sess1", 0)

	done := make(chan struct{})
	work := func(ctx context.Context) (map[string]any, error) {
		defer close(done)
		return nil, errors.New("dispatch broke")
	}
	runTaskWorker(ctx, r, nil, id, work)
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

	id, ctx, cancel, _ := r.Create("alice", "sess1", 0)

	started := make(chan struct{})
	work := func(ctx context.Context) (map[string]any, error) {
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	runTaskWorker(ctx, r, nil, id, work)
	<-started
	cancel()

	waitForState(t, r, id, TaskCancelled)
}

func TestTaskNotifications_StatusFrameShape(t *testing.T) {
	data, err := buildTaskStatusNotification("task-abc", TaskWorking)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	s := string(data)
	for _, want := range []string{
		`"notifications/tasks/status"`,
		`"task-abc"`,
		`"working"`,
		`"io.modelcontextprotocol/related-task"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in %s", want, s)
		}
	}
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

func TestBuildTaskStatusNotification(t *testing.T) {
	data, err := buildTaskStatusNotification("task-xyz", TaskCompleted)
	if err != nil {
		t.Fatalf("buildTaskStatusNotification: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("empty notification")
	}
	// Verify it contains the key fields.
	s := string(data)
	for _, want := range []string{`"notifications/tasks/status"`, `"task-xyz"`, `"completed"`} {
		if !strings.Contains(s, want) {
			t.Errorf("notification missing %q: %s", want, s)
		}
	}
}
