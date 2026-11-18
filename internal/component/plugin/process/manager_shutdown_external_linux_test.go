// Design: docs/architecture/api/process-protocol.md -- whole-daemon shutdown
// Related: process_exit_grace_test.go -- real external child and stderr relay
//go:build linux

package process

import (
	"context"
	"log/slog"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
)

// shutdownRelayGate holds a real relay Handle call, after the child's line has
// reached the daemon. Running(false) is deliberately insufficient: monitorCmd
// sets it before this handler and the stderr relay have finished.
// All workers are joined before the package logger is restored.
type shutdownRelayGate struct {
	line      string
	entered   chan struct{}
	release   chan struct{}
	finished  atomic.Bool
	once      sync.Once
	closeOnce sync.Once
}

func holdShutdownRelay(t *testing.T, line string) *shutdownRelayGate {
	t.Helper()
	gate := &shutdownRelayGate{line: line, entered: make(chan struct{}), release: make(chan struct{})}
	original := stderrLogger
	stderrLogger = func() *slog.Logger { return slog.New(gate) }
	t.Cleanup(func() { stderrLogger = original })
	t.Cleanup(gate.open)
	return gate
}
func (g *shutdownRelayGate) open()                                    { g.closeOnce.Do(func() { close(g.release) }) }
func (g *shutdownRelayGate) Enabled(context.Context, slog.Level) bool { return true }
func (g *shutdownRelayGate) WithAttrs([]slog.Attr) slog.Handler       { return g }
func (g *shutdownRelayGate) WithGroup(string) slog.Handler            { return g }

//nolint:gocritic // hugeParam: slog.Handler fixes this signature; a pointer receiver for the record is not allowed.
func (g *shutdownRelayGate) Handle(_ context.Context, record slog.Record) error {
	if strings.Contains(record.Message, g.line) {
		g.once.Do(func() { close(g.entered) })
		<-g.release
		g.finished.Store(true)
	}
	return nil
}

// A real external consumer's last line is held in the daemon's stderr relay.
// The provider must remain callable until that physical finalizer is released.
// DISCRIMINATES: replace the external Wait in waitShutdownProcess with WaitEngine;
// providerStopped then closes while the relay gate is still held. This Linux
// cut is owed to the main thread; no source-only review claims it was run.
func TestOrderedStopKeepsProviderUntilExternalConsumerExits(t *testing.T) {
	gate := holdShutdownRelay(t, exitChildLastLine)
	consumer, _ := startExitGraceChild(t, exitChildFlushes)
	providerStopped := make(chan struct{})
	var relayFinishedBeforeProvider atomic.Bool
	const provider = "test-ordered-external-provider"
	registerRunEngine(t, provider, func(conn net.Conn) int {
		drainUntilClosed(conn)
		relayFinishedBeforeProvider.Store(gate.finished.Load())
		close(providerStopped)
		return 0
	})
	pm := NewProcessManager([]plugin.PluginConfig{{Name: provider, Internal: true, Encoder: "json"}})
	if err := pm.Start(); err != nil {
		t.Fatal(err)
	}
	if err := pm.AddProcess(consumer.Name(), consumer); err != nil {
		t.Fatal(err)
	}
	stopped := make(chan struct{})
	t.Cleanup(func() {
		gate.open()
		pm.Stop()
		consumer.Stop()
		waitStopped(t, consumer)
		select {
		case <-stopped:
		case <-time.After(pluginStopGrace + time.Second):
			t.Error("outer ordered stop did not finish during cleanup")
		}
	})
	go func() {
		pm.StopInOrder(func([]plugin.PluginConfig) [][]string { return [][]string{{provider}, {consumer.Name()}} })
		close(stopped)
	}()
	select {
	case <-gate.entered:
	case <-time.After(time.Second):
		t.Fatal("external consumer never reached its real stderr finalizer")
	}
	// This timer bounds the assertion that a held finalizer prevents completion;
	// it never waits for a child to die or repairs a missed production join.
	select {
	case <-providerStopped:
		t.Error("provider stopped while the external stderr finalizer was held")
	case <-stopped:
		t.Error("ordered stop returned while the external stderr finalizer was held")
	case <-time.After(time.Second):
	}
	gate.open()
	select {
	case <-stopped:
	case <-time.After(pluginStopGrace + time.Second):
		t.Fatal("ordered stop did not finish after releasing stderr")
	}
	if !relayFinishedBeforeProvider.Load() {
		t.Error("provider stopped before the external consumer's final stderr was relayed")
	}
	state := consumer.Cmd().ProcessState
	if state == nil {
		t.Fatal("external consumer was not reaped")
	}
	if !state.Exited() {
		t.Fatal("external consumer was killed instead of finishing exit")
	}
	if state.ExitCode() != 0 {
		t.Fatalf("external consumer exit = %d", state.ExitCode())
	}
	if late := pm.CleanupLate(); len(late) != 0 {
		t.Fatalf("clean external shutdown reported late: %v", late)
	}
}

// StartMore's internal failure path calls reusable Stop. The terminal fence
// must already own shutdown when acceptor.Stop wakes that failed start. A real
// stderr worker remains held after the child is killed, so dropping a failure
// owner's Wait cannot be hidden by a childAlive poll inside tier resolution.
// Respawn uses the same admitted-owner path and must retain its failed child.
// DISCRIMINATES: move acceptor.Stop ahead of stop ownership, omit failed-start
// Wait, or start Respawn's replacement outside admitted ownership. The provider
// or resolver then reaches its boundary with this exact relay still unfinished.
func TestOrderedStopJoinsFailedExternalSpawnOwners(t *testing.T) {
	for _, scenario := range []struct{ respawn, failureFirst bool }{{}, {respawn: true}, {failureFirst: true}, {respawn: true, failureFirst: true}} {
		respawn := scenario.respawn
		name := "StartMore"
		if respawn {
			name = "Respawn"
		}
		if scenario.failureFirst {
			name += "/failure-before-terminal-fence"
		}
		t.Run(name, func(t *testing.T) {
			const marker = "panic: held-unpublished-shutdown-owner"
			gate := holdShutdownRelay(t, marker)
			acceptor := startTestHubAcceptor(t)
			providerStopped := make(chan struct{})
			var finishedBeforeProvider atomic.Bool
			const provider = "test-failed-spawn-provider"
			registerRunEngine(t, provider, func(conn net.Conn) int {
				drainUntilClosed(conn)
				finishedBeforeProvider.Store(gate.finished.Load())
				close(providerStopped)
				return 0
			})
			pm := NewProcessManager([]plugin.PluginConfig{{Name: provider, Internal: true, Encoder: "json"}})
			pm.SetAcceptor(acceptor)
			if err := pm.Start(); err != nil {
				t.Fatal(err)
			}
			pidPath := filepath.Join(t.TempDir(), "unpublished.pid")
			cfg := plugin.PluginConfig{Name: "held-unpublished-owner", Encoder: "json", Run: "echo $$ > " + pidPath + "; echo '" + marker + "' >&2; exec sleep 30"}
			if respawn {
				pm.mu.Lock()
				pm.configs = append(pm.configs, cfg)
				pm.mu.Unlock()
			}
			started := make(chan error, 1)
			spawnReturned := make(chan struct{})
			var stopped chan struct{}
			t.Cleanup(func() {
				gate.open()
				acceptor.Stop()
				pm.Stop()
				select {
				case <-spawnReturned:
				case <-time.After(pluginStopGrace + time.Second):
					t.Error("spawn owner did not return during cleanup")
				}
				if stopped != nil {
					select {
					case <-stopped:
					case <-time.After(pluginStopGrace + time.Second):
						t.Error("ordered owner did not finish")
					}
				}
			})
			go func() {
				defer close(spawnReturned)
				if respawn {
					_, err := pm.Respawn(cfg.Name)
					started <- err
					return
				}
				started <- pm.StartMore([]plugin.PluginConfig{cfg})
			}()
			_ = waitForPluginPid(t, pidPath)
			select {
			case <-gate.entered:
			case <-time.After(time.Second):
				t.Fatal("failed-start child never reached the controlled stderr relay")
			}
			pm.mu.RLock()
			var unpublished *Process
			for proc := range pm.starting {
				if proc.Name() == cfg.Name {
					unpublished = proc
				}
			}
			pm.mu.RUnlock()
			if unpublished == nil {
				t.Fatal("forked replacement has no admitted owner")
			}
			if scenario.failureFirst {
				// With no terminal pointer yet, failure still owes a real Wait. This
				// gate catches the previous conditional failure-join branch directly.
				acceptor.Stop()
				select {
				case <-providerStopped:
					t.Error("failed spawn stopped its provider before joining its relay")
				case <-spawnReturned:
					t.Error("failed spawn discarded ownership before a terminal fence existed")
				case <-time.After(time.Second):
				}
			}
			stopped = make(chan struct{})
			go func() {
				pm.StopInOrder(func(configs []plugin.PluginConfig) [][]string {
					if !gate.finished.Load() {
						t.Error("tier resolver ran before the unpublished relay joined")
					}
					if !shutdownExitRecorded(unpublished) {
						t.Error("tier resolver ran before the unpublished child was reaped")
					}
					return shutdownTestTiers(configs)
				})
				close(stopped)
			}()
			select {
			case <-providerStopped:
				t.Error("provider stopped while failed spawn cleanup was held")
			case <-spawnReturned:
				t.Error("failed spawn returned before joining its relay")
			case <-stopped:
				t.Error("ordered stop returned with a held failed spawn")
			case <-time.After(time.Second):
			}
			gate.open()
			select {
			case <-stopped:
			case <-time.After(pluginStopGrace + time.Second):
				t.Fatal("ordered stop did not finish after releasing the failed owner")
			}
			if !finishedBeforeProvider.Load() {
				t.Error("provider stopped before failed spawn finalizers")
			}
			select {
			case err := <-started:
				if err == nil {
					t.Error("nonconnecting child start succeeded")
				}
			case <-time.After(time.Second):
				t.Fatal("failed spawn did not return")
			}
			if late := pm.CleanupLate(); len(late) != 0 {
				t.Errorf("joined failed owner reported late: %v", late)
			}
		})
	}
}

// The real external child is held inside its daemon-side stderr Handler. That
// makes Process.Wait take the already-expired shared deadline deterministically.
// Its error boundary then releases and observes the actual monitor completion
// before returning the error: a finished physical owner must not be named late.
// MUTATION: remove the externalDone observation after Process.Wait's error;
// this completed child is then the sole false late owner, without random selects.
func TestOrderedStopRecognizesExternalCompletionAtExpiredDeadline(t *testing.T) {
	gate := holdShutdownRelay(t, exitChildLastLine)
	consumer, _ := startExitGraceChild(t, exitChildFlushes)
	t.Cleanup(func() {
		gate.open()
		consumer.Stop()
		waitStopped(t, consumer)
	})
	if consumer.externalDone == nil {
		t.Fatal("started external child has no physical completion boundary")
	}
	consumer.Stop()
	select {
	case <-gate.entered:
	case <-time.After(processStopBudget):
		t.Fatal("external child never reached its stderr finalizer")
	}
	select {
	case <-consumer.externalDone:
		t.Fatal("external child reported physical completion while its stderr finalizer was held")
	default:
	}
	expired, cancel := context.WithDeadline(t.Context(), time.Time{})
	defer cancel()
	var waitErrorObserved atomic.Bool
	completed := shutdownCompletedContext{Context: expired, complete: func() {
		waitErrorObserved.Store(true)
		gate.open()
		select {
		case <-consumer.externalDone:
		case <-time.After(processStopBudget):
			t.Error("external monitor did not finish after releasing stderr")
		}
		joinCtx, joinCancel := context.WithTimeout(t.Context(), processStopBudget)
		defer joinCancel()
		if err := consumer.Wait(joinCtx); err != nil {
			t.Errorf("completed external child's goroutines did not join before the expired wait returned: %v", err)
		}
	}}
	pm := NewProcessManager(nil)
	late := pm.stopEngines(completed, []*Process{consumer}, shutdownTestTiers, true)
	if !waitErrorObserved.Load() {
		t.Fatal("the real Process.Wait did not reach its expired-context error boundary")
	}
	if len(late) != 0 {
		t.Fatalf("physically completed external child reported late: %v", late)
	}
	if !gate.finished.Load() {
		t.Fatal("completion bypassed the real stderr Handler")
	}
	state := consumer.Cmd().ProcessState
	if state == nil {
		t.Fatal("physically completed child was not reaped")
	}
	if !state.Exited() {
		t.Fatal("external child was killed instead of exiting normally")
	}
	if state.ExitCode() != 0 {
		t.Fatalf("external child exit = %d", state.ExitCode())
	}
}

// The same expired shared deadline must still name a genuinely held physical
// owner. A reaped child, canceled context, or Running(false) cannot stand in for
// completion while the actual stderr Handler is still on the monitor's path.
// MUTATION: move close(externalDone) immediately before exit.record in the
// monitor. The recorded-reap gate then sees the early signal while stderr is
// still held, and the owner vanishes from the exact late result.
func TestOrderedStopReportsExternalStderrHeldAtExpiredDeadline(t *testing.T) {
	gate := holdShutdownRelay(t, exitChildLastLine)
	consumer, _ := startExitGraceChild(t, exitChildFlushes)
	t.Cleanup(func() {
		gate.open()
		consumer.Stop()
		waitStopped(t, consumer)
	})
	if consumer.externalDone == nil {
		t.Fatal("started external child has no physical completion boundary")
	}
	consumer.Stop()
	select {
	case <-gate.entered:
	case <-time.After(processStopBudget):
		t.Fatal("external child never reached its stderr finalizer")
	}
	// This observes the real reap only to establish the negative: reaping
	// alone is insufficient while stderr is held. It never stands for the
	// physical completion asserted below after the release.
	reapCtx, reapCancel := context.WithTimeout(t.Context(), processStopBudget)
	defer reapCancel()
	if _, recorded := consumer.exit.wait(reapCtx); !recorded {
		t.Fatal("held-stderr child was not reaped")
	}
	expired, cancel := context.WithDeadline(t.Context(), time.Time{})
	defer cancel()
	pm := NewProcessManager(nil)
	late := pm.stopEngines(expired, []*Process{consumer}, shutdownTestTiers, true)
	if len(late) != 1 {
		t.Fatalf("late owners = %v, want only held child %s", late, consumer.Name())
	}
	if late[0] != consumer.Name() {
		t.Fatalf("late owner = %s, want held child %s", late[0], consumer.Name())
	}
	if gate.finished.Load() {
		t.Fatal("held stderr Handler finished before its release")
	}
	select {
	case <-consumer.externalDone:
		t.Fatal("held stderr was incorrectly reported physically complete")
	default:
	}
	gate.open()
	select {
	case <-consumer.externalDone:
	case <-time.After(processStopBudget):
		t.Fatal("released stderr did not complete the actual external monitor")
	}
	if !gate.finished.Load() {
		t.Fatal("monitor completion did not include the released stderr Handler")
	}
}
