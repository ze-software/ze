// Design: docs/architecture/api/process-protocol.md -- terminal startup completion
// Related: startup.go -- signalStartupComplete; shutdown_admission_test.go -- gatedShutdownSpawner
package server

import (
	"context"
	"errors"
	"net"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// startupCompletionReactor substitutes only the actual completion callback.
// The callback is installed before Start and remains immutable afterwards.
type startupCompletionReactor struct {
	*mockReactor
	complete func() error
}

func (r *startupCompletionReactor) SignalPluginStartupComplete() error {
	return r.complete()
}

// newStartupPublicationServer holds Server.Start's real first phase inside the
// existing accepted spawner seam. The plugin performs a valid SDK handshake if
// admitted, and reports its real post-startup callback through ready.
func newStartupPublicationServer(t *testing.T, name string, reactor plugin.ReactorLifecycle) (*Server, *gatedShutdownSpawner, <-chan struct{}, func()) {
	t.Helper()
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	ready := make(chan struct{}, 1)
	registerLifecyclePlugin(t, name, nil, func(conn net.Conn) int {
		p := sdk.NewWithConn(name, conn)
		p.OnAllPluginsReady(func() error {
			ready <- struct{}{}
			return nil
		})
		if err := p.Run(context.Background(), &sdk.Registration{}); err != nil {
			return 1
		}
		return 0
	})
	s, err := NewServer(&ServerConfig{
		Plugins:         []plugin.PluginConfig{{Name: name, Internal: true, Encoder: plugin.EncodingJSON}},
		ConfigNeedHolds: holdsForEmptyConfig,
	}, reactor)
	if err != nil {
		t.Fatal(err)
	}
	spawner := &gatedShutdownSpawner{
		ctx: t.Context(), entered: make(chan struct{}), release: make(chan struct{}),
		returned: make(chan error, 1),
	}
	s.SetProcessSpawner(spawner)
	var once sync.Once
	release := func() { once.Do(func() { close(spawner.release) }) }
	t.Cleanup(func() {
		release()
		s.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), failureWait)
		defer cancel()
		// Both success and terminal refusal are expected by callers below.
		_ = s.WaitForStartupComplete(ctx) //nolint:errcheck // each test asserts its startup outcome
		if err := s.Wait(ctx); err != nil {
			t.Errorf("join startup and runtime handlers: %v", err)
		}
	})
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-spawner.entered:
	case <-time.After(failureWait):
		t.Fatal("Server.Start did not enter the actual asynchronous phase")
	}
	return s, spawner, ready, release
}

// TestStoppedStartupDoesNotCallReactor proves completion resumed after a
// finished Stop cannot publish reactor readiness. The completed startup wait
// is the fence for the negative count assertion; no sleep watches an absence.
// MUTATION: remove the terminal guard before completion side effects.
func TestStoppedStartupDoesNotCallReactor(t *testing.T) {
	var callbacks atomic.Int64
	reactor := &startupCompletionReactor{mockReactor: &mockReactor{}, complete: func() error {
		callbacks.Add(1)
		return nil
	}}
	s, spawner, _, release := newStartupPublicationServer(t, "startup-stopped-before-publication", reactor)
	s.Stop()
	release()
	select {
	case err := <-spawner.returned:
		if !errors.Is(err, process.ErrDaemonStopping) {
			t.Fatalf("resumed phase = %v, want ErrDaemonStopping", err)
		}
	case <-time.After(failureWait):
		t.Fatal("sealed spawner did not return")
	}
	ctx, cancel := context.WithTimeout(t.Context(), failureWait)
	defer cancel()
	if err := s.WaitForStartupComplete(ctx); !errors.Is(err, errServerStopped) {
		t.Fatalf("startup after completed Stop = %v, want errServerStopped", err)
	}
	if got := callbacks.Load(); got != 0 {
		t.Fatalf("reactor readiness callbacks after completed Stop = %d, want 0", got)
	}
	if s.postStartupStarted.Load() {
		t.Fatal("stopped startup initiated the success fan-out")
	}
}

// TestStartupStoppedInsideReactorKeepsPreviousStop proves a callback admitted
// while live cannot return later and publish startup success or delete the
// previous stop record. It claims no atomic exclusion of arbitrary overlapping
// callback admission: the callback deliberately enters before Stop begins.
// MUTATION: bypass the terminal checks both after reactor completion and in
// final publication. Removing only one check leaves the other as the fence.
func TestStartupStoppedInsideReactorKeepsPreviousStop(t *testing.T) {
	useLastStopStore(t)
	entered, finish := make(chan struct{}), make(chan struct{})
	var once sync.Once
	releaseCallback := func() { once.Do(func() { close(finish) }) }
	reactor := &startupCompletionReactor{mockReactor: &mockReactor{}, complete: func() error {
		close(entered)
		<-finish
		return nil
	}}
	s, _, _, release := newStartupPublicationServer(t, "startup-stopped-during-reactor", reactor)
	// This cleanup runs before the helper joins startup, including after Fatal.
	t.Cleanup(releaseCallback)
	if err := recordLastStopIn(time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC), LastStopEntry{
		Reason: LastStopCleanupLate, Plugin: "prior-engine", Detail: "previous run cleanup remained unfinished",
	}); err != nil {
		t.Fatal(err)
	}
	previous, found, err := ReadLastStop()
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("previous stop record was not seeded")
	}
	release()
	select {
	case <-entered:
	case <-time.After(failureWait):
		t.Fatal("valid plugin startup did not reach the reactor callback")
	}
	s.Stop()
	releaseCallback()
	ctx, cancel := context.WithTimeout(t.Context(), failureWait)
	defer cancel()
	if err := s.WaitForStartupComplete(ctx); !errors.Is(err, errServerStopped) {
		t.Fatalf("startup whose callback returned after Stop = %v, want errServerStopped", err)
	}
	current, found, err := ReadLastStop()
	if err != nil {
		t.Fatal(err)
	}
	if !found {
		t.Fatal("late completion cleared the previous stop record")
	}
	if !reflect.DeepEqual(current, previous) {
		t.Fatalf("late completion changed previous stop record: got %+v, want %+v", current, previous)
	}
}

// TestStartupReactorCanStopServer proves a real async completion callback can
// reenter Stop without a completion lock deadlock, and that its nil return
// cannot turn the already completed stop into startup success.
// MUTATION: hold lifecycleMu across SignalPluginStartupComplete, or bypass
// both the after-callback and final-publication terminal checks.
func TestStartupReactorCanStopServer(t *testing.T) {
	var s *Server
	reactor := &startupCompletionReactor{mockReactor: &mockReactor{}, complete: func() error {
		s.Stop()
		return nil
	}}
	var release func()
	s, _, _, release = newStartupPublicationServer(t, "startup-reactor-stops-server", reactor)
	release()
	ctx, cancel := context.WithTimeout(t.Context(), failureWait)
	defer cancel()
	if err := s.WaitForStartupComplete(ctx); !errors.Is(err, errServerStopped) {
		t.Fatalf("startup whose reactor stopped the server = %v, want errServerStopped", err)
	}
	if s.Running() {
		t.Fatal("reactor Stop left the server running")
	}
}

// TestLiveStartupSuccessSurvivesLaterStop is the success polarity: the same
// valid gated SDK phase reaches Running, delivers the plugin callback, signals
// the reactor once and clears an older stop record. A subsequent Stop does not
// retroactively change the already-published startup result.
// MUTATION: refuse every completion, skip fan-out/reactor delivery, or check
// stopEntered in WaitForStartupComplete instead of at result publication.
func TestLiveStartupSuccessSurvivesLaterStop(t *testing.T) {
	useLastStopStore(t)
	var callbacks atomic.Int64
	reactor := &startupCompletionReactor{mockReactor: &mockReactor{}, complete: func() error {
		callbacks.Add(1)
		return nil
	}}
	const name = "startup-live-success"
	s, _, ready, release := newStartupPublicationServer(t, name, reactor)
	if err := RecordLastStop(LastStopEntry{Reason: LastStopCleanupLate, Plugin: "old-engine", Detail: "old cleanup"}); err != nil {
		t.Fatal(err)
	}
	release()
	ctx, cancel := context.WithTimeout(t.Context(), failureWait)
	defer cancel()
	if err := s.WaitForStartupComplete(ctx); err != nil {
		t.Fatalf("live startup = %v, want success", err)
	}
	pm := s.procManager.Load()
	if pm == nil {
		t.Fatal("successful startup published no process manager")
	}
	proc := pm.GetProcess(name)
	if proc == nil {
		t.Fatal("successful startup published no configured process")
	}
	if proc.Stage() != plugin.StageRunning {
		t.Fatalf("configured process stage = %v, want Running", proc.Stage())
	}
	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatal("live plugin did not receive OnAllPluginsReady")
	}
	if got := callbacks.Load(); got != 1 {
		t.Fatalf("live reactor readiness callbacks = %d, want 1", got)
	}
	if _, found, err := ReadLastStop(); err != nil {
		t.Fatal(err)
	} else if found {
		t.Fatal("successful startup did not clear the previous stop record")
	}
	s.Stop()
	if err := s.WaitForStartupComplete(ctx); err != nil {
		t.Fatalf("historical successful startup after later Stop = %v, want success", err)
	}
}

// TestEmptyStartupReactorCanStopServer proves the empty-config Start path
// invokes its synchronous completion callback without holding lifecycleMu.
// The server variable is initialized before Start, so its actual callback can
// call Stop and return. A bounded lifecycle helper runs Start so a lock
// regression reports the blocked call rather than hanging the test caller.
// MUTATION: retain StartWithContext's deferred lifecycleMu unlock across its
// empty-config signalStartupComplete call.
func TestEmptyStartupReactorCanStopServer(t *testing.T) {
	var s *Server
	var callbacks atomic.Int64
	reactor := &startupCompletionReactor{mockReactor: &mockReactor{}, complete: func() error {
		callbacks.Add(1)
		s.Stop()
		return nil
	}}
	var err error
	s, err = NewServer(&ServerConfig{}, reactor)
	if err != nil {
		t.Fatal(err)
	}
	started := make(chan error, 1)
	go func() { started <- s.Start() }()
	select {
	case err := <-started:
		if err != nil {
			t.Fatalf("empty-config Start = %v, want an accepted start", err)
		}
	case <-time.After(failureWait):
		t.Fatal("empty-config Start blocked while its synchronous callback called Stop")
	}
	// A successfully returned Start can always be cleaned up. The tested
	// callback already stopped it; this also cleans up a missing-callback cut.
	t.Cleanup(s.Stop)
	if got := callbacks.Load(); got != 1 {
		t.Fatalf("empty-config completion callbacks = %d, want 1", got)
	}
	ctx, cancel := context.WithTimeout(t.Context(), failureWait)
	defer cancel()
	if err := s.WaitForStartupComplete(ctx); !errors.Is(err, errServerStopped) {
		t.Fatalf("empty startup stopped by its callback = %v, want errServerStopped", err)
	}
	if s.Running() {
		t.Fatal("empty startup callback Stop left the server running")
	}
	if s.Context().Err() == nil {
		t.Fatal("empty startup callback Stop left server dispatch live")
	}
}
