package server

import (
	"context"
	"errors"
	"net"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/commandtest"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// startRestartTarget starts one internal plugin through the real startup
// phase and answers the server. The test's cleanups restore the registry and
// wait for every goroutine the server started, after its teardown.
func startRestartTarget(t *testing.T, name string, policy rpc.FailurePolicy, onEvent sdk.EventHandler) *Server {
	t.Helper()
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })

	err := registry.Register(registry.Registration{
		Name:        name,
		Description: "restart of a running plugin",
		RunEngine: func(conn net.Conn) int {
			p := sdk.NewWithConn(name, conn)
			if onEvent != nil {
				p.OnEvent(onEvent)
			}
			if err := p.Run(context.Background(), &sdk.Registration{FailurePolicy: policy}); err != nil {
				return 1
			}
			return 0
		},
		CLIHandler: func([]string) int { return 0 },
	})
	if err != nil {
		t.Fatalf("register: %v", err)
	}

	// Registered before the server's own cleanup, so it runs after it.
	var s *Server
	t.Cleanup(func() {
		if s != nil {
			s.wg.Wait()
		}
	})
	s, _ = newLifecycleStartupServer(t)
	if err := s.runPluginPhase([]plugin.PluginConfig{{Name: name, Internal: true, Encoder: plugin.EncodingJSON}}); err != nil {
		t.Fatalf("startup: %v", err)
	}
	s.dispatcher.Registry().Freeze()
	if proc := s.procManager.Load().GetProcess(name); proc == nil || !proc.Running() {
		t.Fatalf("plugin %s is not running after startup", name)
	}
	return s
}

// restartByCommand runs `request plugin restart <name>` through its handler.
func restartByCommand(t *testing.T, ctx *CommandContext, args command.ValidatedArgs) *plugin.Response {
	t.Helper()
	resp, err := handlePluginRestart(ctx, args)
	if err != nil {
		t.Fatalf("handlePluginRestart: %v", err)
	}
	return resp
}

// TestRestartCommandReplacesARunningPlugin restarts a running plugin by the
// operator command.
//
// VALIDATES: the command accepts a running plugin, replaces its process and
// says it stopped a running one.
// PREVENTS: every doctor row naming `ze request plugin restart` for a running
// plugin naming a command the daemon refused.
func TestRestartCommandReplacesARunningPlugin(t *testing.T) {
	const name = "restart-running"
	s := startRestartTarget(t, name, rpc.FailureUnspecified, nil)
	before := s.procManager.Load().GetProcess(name)

	resp := restartByCommand(t, &CommandContext{Server: s}, commandtest.Args("name", name))
	if resp.Status != plugin.StatusDone {
		t.Fatalf("the restart of a running plugin was refused: %+v", resp)
	}
	if after := s.procManager.Load().GetProcess(name); after == nil || after == before {
		t.Fatalf("the process was not replaced: %p, was %p", after, before)
	}
	data, ok := resp.Data.(plugin.Map)
	if !ok || data[fieldRestartWasRunning] != true {
		t.Fatalf("the answer does not say a running plugin was stopped: %+v", resp.Data)
	}
}

// TestRestartCommandClearsDoctorRecords sets one doctor record on a running
// plugin per subtest and runs the command each row names as its fix.
//
// VALIDATES: after the command the plugin's doctor state holds none of the
// AC-22, AC-17, AC-14, AC-21 (registration) and AC-18 (projection) records.
// PREVENTS: a doctor row whose fix leaves the row in place.
func TestRestartCommandClearsDoctorRecords(t *testing.T) {
	failingEvents := func(string) error { return errors.New("projection broke") }
	cases := []struct {
		name    string
		onEvent sdk.EventHandler
		set     func(t *testing.T, name string, proc *process.Process)
		cleared func(state *PluginDoctorState) bool
	}{
		{
			name: "AC-22 config unapplied",
			set: func(_ *testing.T, _ string, proc *process.Process) {
				proc.RecordConfigCommit("tx-missed", errors.New("config-committed timed out"))
			},
			cleared: func(state *PluginDoctorState) bool { return state.Records.ConfigCommit.Error == "" },
		},
		{
			name: "AC-17 post-startup undelivered",
			set: func(_ *testing.T, _ string, proc *process.Process) {
				proc.SetPostStartup(process.PostStartupUndelivered, errors.New("connection closed"))
			},
			cleared: func(state *PluginDoctorState) bool {
				return state.Records.PostStartup != process.PostStartupUndelivered
			},
		},
		{
			name: "AC-14 subscription dropped",
			set: func(_ *testing.T, _ string, proc *process.Process) {
				proc.RecordSubscriptionDrop(process.SubscriptionDrop{
					Cause: process.SubscriptionDropUnknownNamespace, Namespace: "no-such-namespace",
				})
			},
			cleared: func(state *PluginDoctorState) bool { return len(state.Records.SubscriptionDrops) == 0 },
		},
		{
			name: "AC-21 enricher registration",
			set: func(_ *testing.T, name string, _ *process.Process) {
				recordEnricherFailure(name, EnricherFailure{
					Command: "show restart-enricher", Key: "extra", Registration: true, Error: "duplicate key",
				})
			},
			cleared: func(state *PluginDoctorState) bool { return len(state.Enrichers) == 0 },
		},
		{
			name:    "AC-18 projection error",
			onEvent: failingEvents,
			set: func(t *testing.T, _ string, proc *process.Process) {
				t.Helper()
				result := make(chan process.EventResult, 1)
				if !proc.Deliver(process.EventDelivery{Output: `{"type":"restart-probe"}`, Result: result}) {
					t.Fatal("the running process refused the delivery")
				}
				select {
				case res := <-result:
					if res.Err == nil {
						t.Fatal("the plugin applied the event, so no projection error exists to clear")
					}
				case <-time.After(10 * time.Second):
					t.Fatal("the delivery was never answered")
				}
			},
			cleared: func(state *PluginDoctorState) bool { return state.Records.ProjectionError == "" },
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			const name = "restart-records"
			t.Cleanup(func() { forgetEnricherFailures(name) })
			s := startRestartTarget(t, name, rpc.FailureUnspecified, tc.onEvent)
			tc.set(t, name, s.procManager.Load().GetProcess(name))
			if state := s.pluginDoctorState(name, true); tc.cleared(&state) {
				t.Fatalf("the record is not in the doctor state before the restart: %+v", state)
			}

			resp := restartByCommand(t, &CommandContext{Server: s}, commandtest.Args("name", name))
			if resp.Status != plugin.StatusDone {
				t.Fatalf("restart refused: %+v", resp)
			}
			if state := s.pluginDoctorState(name, true); !tc.cleared(&state) {
				t.Fatalf("the restart left the record in the doctor state: %+v", state)
			}
		})
	}
}

// TestRestartCommandOfRunningFatalPluginKeepsDaemonUp restarts a running
// plugin whose failure policy stops ze.
//
// VALIDATES: the replaced process's failure policy, as its supervisor applies
// it once the old engine ends, neither stops the daemon nor records the
// plugin down: the name already holds the replacement.
// PREVENTS: an operator restart of a `fatal` plugin shutting ze down.
func TestRestartCommandOfRunningFatalPluginKeepsDaemonUp(t *testing.T) {
	const name = "restart-fatal"
	s := startRestartTarget(t, name, rpc.FailureFatal, nil)
	var stopped atomic.Bool
	s.shutdownFunc = func() { stopped.Store(true) }
	before := s.procManager.Load().GetProcess(name)

	resp := restartByCommand(t, &CommandContext{Server: s}, commandtest.Args("name", name))
	if resp.Status != plugin.StatusDone {
		t.Fatalf("restart refused: %+v", resp)
	}
	// The call the old supervisor makes, made here so its outcome is known
	// before the assertions rather than raced by them.
	s.applyFailurePolicy(before)

	if stopped.Load() {
		t.Fatal("restarting a running fatal plugin stopped the daemon")
	}
	select {
	case <-s.shutdownRequested:
		t.Fatal("restarting a running fatal plugin requested shutdown")
	default:
	}
	if _, down := s.procManager.Load().Stopped(name); down {
		t.Fatal("restarting a running plugin recorded it down")
	}
}

// TestRestartCommandRefusals runs the command where it cannot restart.
//
// VALIDATES: an unknown name, a missing name, several names, no server, no
// process manager and a config transaction in flight are refused, and the
// refusal during a transaction leaves the running process in place.
// PREVENTS: a restart that replaces the process a reload is talking to.
func TestRestartCommandRefusals(t *testing.T) {
	configured := &Server{}
	configured.procManager.Store(process.NewProcessManager([]plugin.PluginConfig{{Name: "alpha"}}))
	refused := []struct {
		name string
		ctx  *CommandContext
		args command.ValidatedArgs
	}{
		{"unknown name", &CommandContext{Server: configured}, commandtest.Args("name", "nope")},
		{"missing name", &CommandContext{Server: configured}, commandtest.Args()},
		{"several names", &CommandContext{Server: configured}, commandtest.Args("alpha", "beta")},
		{"no server", &CommandContext{}, commandtest.Args("name", "alpha")},
		{"no process manager", &CommandContext{Server: &Server{}}, commandtest.Args("name", "alpha")},
	}
	for _, tc := range refused {
		t.Run(tc.name, func(t *testing.T) {
			if resp := restartByCommand(t, tc.ctx, tc.args); resp.Status == plugin.StatusDone {
				t.Fatalf("accepted: %+v", resp)
			}
		})
	}

	t.Run("transaction in flight", func(t *testing.T) {
		const name = "restart-in-tx"
		s := startRestartTarget(t, name, rpc.FailureUnspecified, nil)
		before := s.procManager.Load().GetProcess(name)
		if !s.txLock.tryAcquire() {
			t.Fatal("the transaction lock is already held")
		}
		resp := restartByCommand(t, &CommandContext{Server: s}, commandtest.Args("name", name))
		s.txLock.release()
		if resp.Status == plugin.StatusDone {
			t.Fatalf("a restart during a config transaction was accepted: %+v", resp)
		}
		if after := s.procManager.Load().GetProcess(name); after != before {
			t.Fatal("the refused restart replaced the process")
		}
	})
}

// TestRestartCompletionOffersRunningPlugin asks the completer of a server
// whose one plugin runs.
//
// VALIDATES: the running plugin is offered, marked running.
// PREVENTS: Tab hiding the name the doctor row tells the operator to type.
func TestRestartCompletionOffersRunningPlugin(t *testing.T) {
	const name = "restart-complete"
	s := startRestartTarget(t, name, rpc.FailureUnspecified, nil)
	for _, candidate := range completePluginRestart(s) {
		if candidate.Value == name {
			if candidate.ShortHelp != completionRunning {
				t.Fatalf("the running plugin is offered as %q", candidate.ShortHelp)
			}
			return
		}
	}
	t.Fatalf("the running plugin %s is not offered", name)
}
