// Design: docs/architecture/config/transaction-protocol.md -- removal result ownership.
// Related: failure_policy.go -- the declaration that governs an ended generation.
package server

import (
	"context"
	"errors"
	"net"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// TestFatalRemovalFailurePrecedesGenerationRetirement sends an actual Bye over
// each transport. A disconnected fatal owner must stop the daemon while that
// generation still owns its manager slot; an acknowledged removal must not.
// Joining the server workers settles both the supervisor and removal recovery,
// so the negative assertion does not depend on a quiet interval.
func TestFatalRemovalFailurePrecedesGenerationRetirement(t *testing.T) {
	for _, direct := range []bool{false, true} {
		for _, disconnected := range []bool{false, true} {
			t.Run("direct="+strconv.FormatBool(direct)+"/disconnected="+strconv.FormatBool(disconnected), func(t *testing.T) {
				snapshot := registry.Snapshot()
				registry.Reset()
				t.Cleanup(func() { registry.Restore(snapshot) })
				const name = "fatal-removal-owner"
				var callbacks atomic.Int32
				registerLifecyclePlugin(t, name, nil, func(conn net.Conn) int {
					if !direct {
						conn = struct{ net.Conn }{conn}
					}
					p := sdk.NewWithConn(name, conn)
					p.OnBye(func(string) error {
						callbacks.Add(1)
						if disconnected {
							return errors.Join(p.Close(), errors.New("cleanup was not acknowledged before disconnect"))
						}
						return nil
					})
					err := p.Run(context.Background(), &sdk.Registration{FailurePolicy: rpc.FailureFatal})
					if closeErr := p.Close(); err == nil {
						err = closeErr
					}
					if err != nil {
						return 1
					}
					return 0
				})
				s, spawner := newLifecycleStartupServer(t)
				var original *process.Process
				shutdown := make(chan bool, 2)
				s.SetShutdownFunc(func() {
					shutdown <- spawner.pm.GetProcess(name) == original
				})
				if err := s.runPluginPhase([]plugin.PluginConfig{{
					Name: name, Internal: true, Encoder: plugin.EncodingJSON, Respawn: plugin.RespawnDeclined,
				}}); err != nil {
					t.Fatal(err)
				}
				original = spawner.pm.GetProcess(name)
				if original == nil {
					t.Fatal("startup did not publish the fatal owner")
				}
				err := s.notifyPluginRemoval(original)
				if (err != nil) != disconnected {
					t.Fatalf("removal error=%v, disconnected=%t", err, disconnected)
				}
				s.wg.Wait()
				if got := callbacks.Load(); got != 1 {
					t.Fatalf("Bye callback ran %d times, want one", got)
				}
				if spawner.pm.GetProcess(name) != nil {
					t.Fatal("removal did not retire its settled generation")
				}
				select {
				case owned := <-shutdown:
					if !disconnected {
						t.Fatal("acknowledged Bye was classified as a fatal failure")
					}
					if !owned {
						t.Fatal("fatal removal failure was classified after its generation lost ownership")
					}
				default:
					if disconnected {
						t.Fatal("disconnected fatal owner was retired without stopping the daemon")
					}
				}
			})
		}
	}
}

// TestFatalRemovalRefusalThenConfigureDisconnect keeps the Bye refusal live,
// then disconnects during the actual recovery Configure. That later transport
// failure must classify the original owner, not disappear behind the earlier
// ordinary refusal or create a replacement generation.
func TestFatalRemovalRefusalThenConfigureDisconnect(t *testing.T) {
	for _, direct := range []bool{false, true} {
		t.Run("direct="+strconv.FormatBool(direct), func(t *testing.T) {
			snapshot := registry.Snapshot()
			registry.Reset()
			t.Cleanup(func() { registry.Restore(snapshot) })
			const name = "fatal-removal-configure-owner"
			var starts, configured atomic.Int32
			registerLifecyclePlugin(t, name, nil, func(conn net.Conn) int {
				generation := starts.Add(1)
				if !direct {
					conn = struct{ net.Conn }{conn}
				}
				p := sdk.NewWithConn(name, conn)
				p.OnConfigure(func([]sdk.ConfigSection) error {
					if generation != 1 {
						return errors.New("a fatal owner must not be replaced")
					}
					if configured.Add(1) == 1 {
						return nil
					}
					return errors.Join(p.Close(), errors.New("recovery Configure disconnected"))
				})
				p.OnBye(func(string) error {
					return errors.New("cleanup refused while the connection is still usable")
				})
				err := p.Run(context.Background(), &sdk.Registration{FailurePolicy: rpc.FailureFatal})
				if closeErr := p.Close(); err == nil {
					err = closeErr
				}
				if err != nil {
					return 1
				}
				return 0
			})
			s, spawner := newLifecycleStartupServer(t)
			s.reactor = &mockReloadReactor{tree: map[string]any{}}
			s.config.Plugins = []plugin.PluginConfig{{Name: name, Internal: true, Encoder: plugin.EncodingJSON}}
			var original *process.Process
			shutdown := make(chan bool, 2)
			s.SetShutdownFunc(func() { shutdown <- spawner.pm.GetProcess(name) == original })
			if err := s.runPluginPhase(s.config.Plugins); err != nil {
				t.Fatal(err)
			}
			original = spawner.pm.GetProcess(name)
			if original == nil {
				t.Fatal("startup did not publish the fatal owner")
			}
			if err := s.notifyPluginRemoval(original); err == nil {
				t.Fatal("Bye refusal was acknowledged as successful removal")
			}
			s.wg.Wait()
			if got := configured.Load(); got != 2 {
				t.Fatalf("Configure calls=%d, want initial and actual recovery", got)
			}
			if got := starts.Load(); got != 1 {
				t.Fatalf("fatal owner was replaced: %d engine generations", got)
			}
			select {
			case owned := <-shutdown:
				if !owned {
					t.Fatal("Configure disconnect was classified after the fatal owner lost its manager slot")
				}
			default:
				t.Fatal("fatal Configure disconnect did not stop the daemon")
			}
		})
	}
}
