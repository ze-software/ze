// Design: docs/architecture/api/process-protocol.md -- whole-daemon shutdown
// Related: reload.go -- terminal transaction admission and delayed cancel publication
package server

import (
	"context"
	"errors"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/config/transaction"
	"github.com/ze-software/ze/internal/component/plugin"
	pluginmanager "github.com/ze-software/ze/internal/component/plugin/manager"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// A terminal server refuses reuse through the existing API and keeps the
// context canceled. This is a future guard, not a claimed original red.
func TestServerStartAfterStopIsRefused(t *testing.T) {
	s, err := NewServer(&ServerConfig{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Stop)
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	s.Stop()
	stopped := s.Context()
	if err := s.StartWithContext(t.Context()); !errors.Is(err, errServerStopped) {
		t.Fatalf("StartWithContext after Stop = %v, want errServerStopped", err)
	}
	if s.Context() != stopped {
		t.Fatal("refused restart replaced the server context")
	}
	if s.running.Load() {
		t.Fatal("refused restart marked the server running")
	}
	if stopped.Err() == nil {
		t.Fatal("Stop left the server context live")
	}
}

// Stop seals the transaction lock even when no transaction was in flight.
// Both direct reload APIs must refuse while ordered cleanup keeps dispatch live.
func TestServerStopRefusesDirectReloads(t *testing.T) {
	s, err := NewServer(&ServerConfig{}, &mockReactor{})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Stop)
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	s.SetConfigLoader(func() (map[string]any, []plugin.PluginConfig, error) { return map[string]any{}, nil, nil })
	s.Stop()
	if err := s.ReloadConfig(t.Context(), map[string]any{}); !errors.Is(err, transaction.ErrShutdown) {
		t.Fatalf("ReloadConfig after Stop = %v, want ErrShutdown", err)
	}
	if err := s.ReloadFromDisk(t.Context()); !errors.Is(err, transaction.ErrShutdown) {
		t.Fatalf("ReloadFromDisk after Stop = %v, want ErrShutdown", err)
	}
	if err := s.txLock.acquire(t.Context()); !errors.Is(err, transaction.ErrShutdown) {
		t.Fatalf("compensation admission after Stop = %v, want ErrShutdown", err)
	}
}

// A transaction can acquire just before Stop captures it and publish its
// cancel handle afterwards. That exact owner must still receive ErrShutdown.
func TestStopCancelsTransactionPublishedAfterCapture(t *testing.T) {
	s, err := NewServer(&ServerConfig{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !s.txLock.tryAcquire() {
		t.Fatal("initial transaction was refused")
	}
	_, done := s.txLock.closeAdmission()
	if done == nil {
		t.Fatal("Stop did not capture the acquired transaction")
	}
	ctx, cancel := context.WithCancelCause(t.Context())
	defer cancel(nil)
	s.txLock.setCancel(cancel, nil)
	if !errors.Is(context.Cause(ctx), transaction.ErrShutdown) {
		t.Fatalf("late cancel cause = %v, want ErrShutdown", context.Cause(ctx))
	}
	s.txLock.release()
	select {
	case <-done:
	default:
		t.Fatal("captured transaction did not finish")
	}
	if s.txLock.tryAcquire() {
		t.Fatal("release reopened terminal admission")
	}
}

// Concurrent Start and Stop must leave one terminal lifecycle. Start may win
// admission or be refused; neither schedule may leave a live context afterwards.
func TestConcurrentServerStartAndStopRemainTerminal(t *testing.T) {
	s, err := NewServer(&ServerConfig{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Stop)
	start := make(chan error, 1)
	go func() { start <- s.StartWithContext(t.Context()) }()
	s.Stop()
	select {
	case err := <-start:
		if err != nil && !errors.Is(err, errServerStopped) {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("Start did not return")
	}
	if s.running.Load() {
		t.Fatal("Start raced past terminal Stop")
	}
	if ctx := s.Context(); ctx != nil && ctx.Err() == nil {
		t.Fatal("Start published a live context after Stop")
	}
}

// OnStop still observes canceled dispatch and finishes before Stop returns,
// even though provider cleanup now precedes the server cancellation.
func TestOrderedServerStopCancelsContextBeforeHooks(t *testing.T) {
	s, err := NewServer(&ServerConfig{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Stop)
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	ran := false
	if _, err := s.OnStop(func() error {
		if s.Context().Err() == nil {
			t.Error("OnStop ran with live server dispatch")
		}
		ran = true
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	s.Stop()
	if !ran {
		t.Fatal("Stop returned before its OnStop hook")
	}
}

// The real Manager can exist before its first async startup phase publishes a
// ProcessManager. Server.Stop must seal that creator, so a later phase cannot
// create a process owner the completed stop never saw.
func TestServerStopSealsUnpublishedManager(t *testing.T) {
	m := pluginmanager.NewManager()
	if err := m.StartAll(t.Context(), nil, nil); err != nil {
		t.Fatal(err)
	}
	s, err := NewServer(&ServerConfig{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	s.SetProcessSpawner(m)
	t.Cleanup(s.Stop)
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	s.Stop()
	if err := m.SpawnMore([]plugin.PluginConfig{{Name: "late-server-phase", Internal: true}}); !errors.Is(err, process.ErrDaemonStopping) {
		t.Fatalf("late startup phase = %v, want ErrDaemonStopping", err)
	}
	if pm := m.GetProcessManager(); pm != nil {
		t.Fatal("late phase created an unjoined process owner")
	}
}

// Stop seals hook registration before it waits for an acquired transaction.
// The gate proves refusal while the context and dependency dispatch remain live.
func TestServerStopRefusesHooksBeforeContextCancellation(t *testing.T) {
	s, err := NewServer(&ServerConfig{}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(s.Stop)
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	if !s.txLock.tryAcquire() {
		t.Fatal("initial transaction was refused")
	}
	entered, stopped := make(chan struct{}), make(chan struct{})
	s.txLock.setCancel(func(error) { close(entered) }, nil)
	go func() { s.Stop(); close(stopped) }()
	<-entered
	if s.Context().Err() != nil {
		t.Error("transaction gate lost live dependency dispatch")
	}
	withdraw, err := s.OnStop(func() error {
		t.Error("late hook ran")
		return nil
	})
	if !errors.Is(err, errServerStopped) {
		t.Errorf("OnStop during Stop = %v, want errServerStopped", err)
	}
	if withdraw != nil {
		t.Error("refused hook returned a withdrawal")
	}
	s.txLock.release()
	select {
	case <-stopped:
	case <-time.After(time.Second):
		t.Fatal("Stop did not finish after transaction release")
	}
}

// This alternate accepted spawner holds its first phase before publication.
// The shared admission lock protects only the seal/publication decision, not
// its preparation gate or process Start. No concrete Manager is involved.
type gatedShutdownSpawner struct {
	mu       sync.Mutex
	closed   bool
	pm       *process.ProcessManager
	ctx      context.Context
	entered  chan struct{}
	release  chan struct{}
	returned chan error
}

func (s *gatedShutdownSpawner) SpawnMore(configs []plugin.PluginConfig) (err error) {
	defer func() { s.returned <- err }()
	close(s.entered)
	<-s.release
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return process.ErrDaemonStopping
	}
	s.pm = process.NewProcessManager(configs)
	pm := s.pm
	s.mu.Unlock()
	return pm.StartWithContext(s.ctx)
}
func (s *gatedShutdownSpawner) GetProcessManager() any {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.pm
}
func (s *gatedShutdownSpawner) SealProcessAdmission() func() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.closed = true
	return func() {}
}

// Server.Start's actual async phase is already inside SpawnMore when Stop
// seals it. The phase later resumes and must refuse publication after Stop.
// DISCRIMINATES: remove the required seal call from Server.Stop; the resumed
// alternate spawner then publishes a ProcessManager that Stop never joined.
func TestServerStopSealsAlternateSpawnerInsideFirstPhase(t *testing.T) {
	snapshot := registry.Snapshot()
	t.Cleanup(func() { registry.Restore(snapshot) })
	registerLifecyclePlugin(t, "gated-alternate", nil, func(conn net.Conn) int {
		if err := conn.Close(); err != nil {
			return 1
		}
		return 0
	})
	s, err := NewServer(&ServerConfig{Plugins: []plugin.PluginConfig{{Name: "gated-alternate", Internal: true}}}, nil)
	if err != nil {
		t.Fatal(err)
	}
	spawner := &gatedShutdownSpawner{ctx: t.Context(), entered: make(chan struct{}), release: make(chan struct{}), returned: make(chan error, 1)}
	s.SetProcessSpawner(spawner)
	var release sync.Once
	t.Cleanup(func() {
		release.Do(func() { close(spawner.release) })
		s.Stop()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()
		_ = s.WaitForStartupComplete(ctx) //nolint:errcheck // the main assertion checks the startup outcome
		if pm, ok := spawner.GetProcessManager().(*process.ProcessManager); ok {
			if pm != nil {
				pm.StopInOrder(shutdownPluginTiers(pm))
			}
		}
	})
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-spawner.entered:
	case <-time.After(time.Second):
		t.Fatal("Server.Start did not enter the alternate spawner")
	}
	s.Stop()
	release.Do(func() { close(spawner.release) })
	select {
	case err := <-spawner.returned:
		if !errors.Is(err, process.ErrDaemonStopping) {
			t.Fatalf("released phase = %v, want ErrDaemonStopping", err)
		}
	case <-time.After(time.Second):
		t.Fatal("alternate phase did not finish after release")
	}
	if pm := spawner.GetProcessManager(); pm != (*process.ProcessManager)(nil) {
		t.Fatal("phase published an unjoined process manager after Stop")
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := s.WaitForStartupComplete(ctx); err == nil {
		t.Fatal("sealed async startup reported success")
	}
}
