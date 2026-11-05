// Design: docs/architecture/api/process-protocol.md -- fatal owner removal outcomes.
package server

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestFatalOwnerRemovalClassifiesActualOutcome drives real SDK Bye and recovery
// Configure callbacks over both transports. Successful removal and a live
// refusal are controls; disconnecting during either callback must stop the
// daemon before recovery replaces the failed fatal generation.
func TestFatalOwnerRemovalClassifiesActualOutcome(t *testing.T) {
	for _, direct := range []bool{false, true} {
		transport := "socket"
		if direct {
			transport = "bridge"
		}
		for _, scenario := range []string{"success", "live-refusal", "bye-disconnect", "recovery-disconnect"} {
			t.Run(transport+"/"+scenario, func(t *testing.T) {
				snapshot := registry.Snapshot()
				registry.Reset()
				t.Cleanup(func() { registry.Restore(snapshot) })
				const name = "fatal-removal-outcome"
				var starts, configured atomic.Int32
				registerLifecyclePlugin(t, name, nil, func(conn net.Conn) int {
					starts.Add(1)
					if !direct {
						conn = struct{ net.Conn }{conn}
					}
					p := sdk.NewWithConn(name, conn)
					defer p.Close() //nolint:errcheck // Test owner cleanup after Run drains callbacks.
					p.OnConfigure(func([]sdk.ConfigSection) error {
						if configured.Add(1) > 1 && scenario == "recovery-disconnect" {
							return errors.Join(errors.New("recovery connection lost"), p.Close())
						}
						return nil
					})
					p.OnBye(func(string) error {
						if scenario == "success" {
							return nil
						}
						if scenario == "bye-disconnect" {
							return errors.Join(errors.New("publication owner disconnected during removal"), p.Close())
						}
						return errors.New("cleanup refused while owner remains live")
					})
					if err := p.Run(context.Background(), &sdk.Registration{FailurePolicy: rpc.FailureFatal}); err != nil {
						return 1
					}
					return 0
				})
				s, spawner := newLifecycleStartupServer(t)
				configs := []plugin.PluginConfig{{Name: name, Internal: true, Encoder: plugin.EncodingJSON}}
				s.config.Plugins = configs
				signaled, stopped := watchDaemonStop(s)
				if err := s.runPluginPhase(configs); err != nil {
					t.Fatal(err)
				}
				original := spawner.pm.GetProcess(name)
				if original == nil {
					t.Fatal("startup did not install the original owner")
				}
				err := s.rollbackStartupProcess(original)
				if scenario == "success" {
					if err != nil {
						t.Fatal(err)
					}
					s.wg.Wait()
					if signaled() || stopped() {
						t.Fatal("acknowledged removal was treated as an unexpected owner failure")
					}
					if got := starts.Load(); got != 1 {
						t.Fatalf("successful removal started %d generations, want 1", got)
					}
					return
				}
				if err == nil {
					t.Fatal("failed removal reported success")
				}
				// Recovery starts before rollback returns. Retain that exact
				// attempt, even if completion removes it from the server map.
				s.removalMu.Lock()
				attempt := s.removals[original]
				s.removalMu.Unlock()
				settled := func() bool {
					s.removalMu.Lock()
					recovering := attempt != nil && attempt.recovering
					current := s.removals[original]
					if current != nil && current.recovering {
						recovering = true
					}
					s.removalMu.Unlock()
					_, transactionDone := s.txLock.inFlight()
					return !recovering && transactionDone == nil
				}
				if scenario == "live-refusal" {
					waitFatalRemovalState(t, func() bool {
						return configured.Load() == 2 && !s.pluginRemovalPending(original) && settled()
					}, "ordinary refusal must restore the same live owner")
					if spawner.pm.GetProcess(name) != original || !original.Running() {
						t.Fatal("live refusal did not retain its running original owner")
					}
					if signaled() || stopped() {
						t.Fatal("live refusal was treated as a fatal owner failure")
					}
					if got := starts.Load(); got != 1 {
						t.Fatalf("live refusal started %d generations, want 1", got)
					}
					return
				}
				waitFatalRemovalState(t, stopped,
					"pending removal must not suppress a disconnected fatal owner's failure")
				waitFatalRemovalState(t, settled,
					"fatal removal recovery must finish before replacement is judged")
				if !signaled() {
					t.Fatal("failure did not reach the daemon shutdown function")
				}
				if got := starts.Load(); got != 1 {
					t.Fatalf("settled fatal removal started %d generations, want 1", got)
				}
				if current := spawner.pm.GetProcess(name); current != nil && current != original {
					t.Fatal("settled fatal removal installed a replacement generation")
				}
			})
		}
	}
}

// waitFatalRemovalState polls the producer's completion state within the same
// lifecycle bound as the existing failure-policy tests; elapsed time is never
// evidence that recovery finished or that a replacement cannot still appear.
func waitFatalRemovalState(t *testing.T, ready func() bool, failure string) {
	t.Helper()
	deadline := time.NewTimer(failureWait)
	defer deadline.Stop()
	tick := time.NewTicker(time.Millisecond)
	defer tick.Stop()
	for !ready() {
		select {
		case <-deadline.C:
			t.Fatal(failure)
		case <-tick.C:
		}
	}
}
