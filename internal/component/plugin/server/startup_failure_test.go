package server

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// VALIDATES: startupFailureError wraps the recorded cause, so the error ze
// prints on stderr before exiting names WHY the plugin failed, not just the
// stage it stopped at.
// PREVENTS: the regression this function exists to fix -- an unprivileged ze
// with an interface{} block exited 1 with only "plugin interface failed during
// startup at stage Config", giving the operator no offending object, no reason
// and no corrective action (ai/rules/cli.md).
func TestStartupFailureErrorWrapsCause(t *testing.T) {
	proc := process.NewProcess(plugin.PluginConfig{Name: "interface"})
	proc.SetStage(plugin.StageConfig)

	cause := errors.New(`interface config: dummy zdiag0 create: iface: create dummy "zdiag0": operation not permitted (interface configuration needs CAP_NET_ADMIN: run ze as root, or grant the binary the capability with ` + "`setcap cap_net_admin+ep <path-to-ze>`" + `)`)
	proc.SetStartupError(cause)

	err := startupFailureError(proc)
	if err == nil {
		t.Fatal("startupFailureError = nil, want an error")
	}

	// The chain must survive for errors.Is/errors.As, not only the text.
	if !errors.Is(err, cause) {
		t.Fatalf("startupFailureError = %v, want it to wrap the cause", err)
	}

	got := err.Error()
	for _, want := range []string{
		"plugin interface",        // which plugin
		"stage Config",            // where it stopped
		"operation not permitted", // WHY -- the evidence
		"zdiag0",                  // the offending object
		"needs CAP_NET_ADMIN",     // what to do next
	} {
		if !strings.Contains(got, want) {
			t.Errorf("startupFailureError text missing %q\ngot: %s", want, got)
		}
	}
}

// VALIDATES: with no cause recorded, startupFailureError still reports the
// failure, naming the plugin and the stage.
// PREVENTS: fail-OPEN behavior -- a missing cause must never turn a failed
// startup into a nil error, because the absence of a diagnosis is not evidence
// that startup succeeded (ai/rules/evidence.md).
func TestStartupFailureErrorWithoutCauseStillFails(t *testing.T) {
	proc := process.NewProcess(plugin.PluginConfig{Name: "vrrp"})
	proc.SetStage(plugin.StageInit)

	err := startupFailureError(proc)
	if err == nil {
		t.Fatal("startupFailureError = nil with no recorded cause, want an error (fail closed)")
	}

	got := err.Error()
	if !strings.Contains(got, "plugin vrrp") || !strings.Contains(got, "stage Init") {
		t.Errorf("startupFailureError = %q, want it to name the plugin and stage", got)
	}
}

// TestPreferDiagnosedErrorYieldsBarrierAbortToRealCause pins which failure a
// phase reports when a tier ends with one real diagnosis and many bystanders.
//
// VALIDATES: a barrier abort already held is replaced by a non-abort cause.
// PREVENTS: reporting the alphabetically-first failing plugin. The tier is
// walked in sorted name order, so keeping the first failure buried the real
// cause behind a bystander's "startup barrier aborted" -- reproduced live with a
// config carrying both `bgp` and `interface`.
func TestPreferDiagnosedErrorYieldsBarrierAbortToRealCause(t *testing.T) {
	abort := fmt.Errorf("plugin bgp-filter-aspath failed during startup at stage Config: %w", errStartupBarrierAborted)
	real := fmt.Errorf("plugin interface failed during startup at stage Config: %w", errNoBackendForTest)

	if got := preferDiagnosedError(nil, abort); !errors.Is(got, errStartupBarrierAborted) {
		t.Fatalf("nil current must take the candidate, got %v", got)
	}
	if got := preferDiagnosedError(abort, real); !errors.Is(got, errNoBackendForTest) {
		t.Fatalf("a barrier abort must yield to a real cause, got %v", got)
	}
	// Order is otherwise preserved: the first real diagnosis wins.
	second := fmt.Errorf("plugin other failed during startup at stage Config: %w", errOtherForTest)
	if got := preferDiagnosedError(real, second); !errors.Is(got, errNoBackendForTest) {
		t.Fatalf("the first real diagnosis must win, got %v", got)
	}
	// An all-aborts tier still reports something rather than nothing.
	if got := preferDiagnosedError(abort, abort); got == nil {
		t.Fatal("an all-abort tier must still report a failure")
	}
}

var (
	errNoBackendForTest = errors.New("interface: no backend configured and no OS default available")
	errOtherForTest     = errors.New("some other cause")
)

// startupPublicationReactor observes the real engine's callback transport at
// the instant startup releases runtime work, before the final ready response.
type startupPublicationReactor struct {
	*mockReactor
	onReady func()
}

func (r *startupPublicationReactor) SignalAPIReady() { r.onReady() }

// TestStartupBridgePublicationBoundary runs a real SDK handshake and observes
// publication at the runtime signal. Closing IPC there injects a final-OK write
// failure, which must roll back rather than leave a published but stranded bridge.
func TestStartupBridgePublicationBoundary(t *testing.T) {
	for _, failReady := range []bool{false, true} {
		name := "success"
		if failReady {
			name = "final-ok-failure"
		}
		t.Run(name, func(t *testing.T) {
			snap := registry.Snapshot()
			registry.Reset()
			t.Cleanup(func() { registry.Restore(snap) })
			const pluginName = "startup-publication"
			const commandName = "show startup publication"
			started := make(chan struct{})
			registerLifecyclePlugin(t, pluginName, nil, func(conn net.Conn) int {
				p := sdk.NewWithConn(pluginName, conn)
				p.OnStarted(func(context.Context) error {
					close(started)
					return nil
				})
				if err := p.Run(t.Context(), sdk.Registration{
					Commands: []sdk.CommandDecl{{Name: commandName}},
				}); err != nil {
					return 1
				}
				return 0
			})
			s, spawner := newLifecycleStartupServer(t)
			var observed *process.Process
			var publicationErr error
			s.reactor = &startupPublicationReactor{
				mockReactor: &mockReactor{},
				onReady: func() {
					observed = spawner.pm.GetProcess(pluginName)
					if !observed.Conn().HasBridge() {
						publicationErr = errors.New("runtime signaled before bridge publication")
					} else if err := observed.Conn().Err(); err != nil {
						publicationErr = fmt.Errorf("unactivated published bridge: %w", err)
					}
					if failReady {
						if err := observed.Conn().Close(); err != nil {
							publicationErr = fmt.Errorf("close before final OK: %w", err)
						}
					}
				},
			}
			err := s.runPluginPhase([]plugin.PluginConfig{{
				Name: pluginName, Internal: true, Encoder: plugin.EncodingJSON,
			}})
			if publicationErr != nil {
				t.Error(publicationErr)
			}
			if observed == nil {
				t.Fatalf("startup never reached runtime boundary: %v", err)
			}
			if failReady {
				if err == nil {
					t.Fatal("failed final OK was reported as successful startup")
				}
				if observed.StartupError() == nil {
					t.Fatal("failed final OK lost its startup error")
				}
				if observed.Stage() >= plugin.StageRunning {
					t.Errorf("failed final OK left stage %v", observed.Stage())
				}
				if spawner.pm.GetProcess(pluginName) != nil {
					t.Error("failed final OK left process registered")
				}
				if owner := s.registry.LookupCommand(commandName); owner != "" {
					t.Errorf("failed final OK retained command owner %q", owner)
				}
				callbackCtx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
				defer cancel()
				if _, err := observed.Bridge().SendCallback(callbackCtx, "test", nil); !errors.Is(err, rpc.ErrBridgeClosed) {
					t.Errorf("failed startup bridge callback: %v, want ErrBridgeClosed", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("startup: %v", err)
			}
			select {
			case <-started:
			case <-time.After(5 * time.Second):
				t.Fatal("SDK did not activate the published bridge")
			}
			if !observed.Bridge().Activated() {
				t.Fatal("SDK startup did not activate the negotiated bridge")
			}
			if err := observed.Conn().Err(); err != nil {
				t.Errorf("activated bridge liveness: %v", err)
			}
		})
	}
}

// stoppingStartupSink reproduces cancellation after the final barrier, when
// the driver owns a transport that Process.Stop has removed from the process.
type stoppingStartupSink struct {
	*engineStartupSink
	stopped bool
}

func (s *stoppingStartupSink) transition(from, to plugin.PluginStage) bool {
	if !s.engineStartupSink.transition(from, to) {
		return false
	}
	if to == plugin.StageRunning {
		s.proc.Stop()
		s.stopped = true
	}
	return true
}

// TestStartupStopBeforeBridgePublication drives the actual SDK and engine
// handshake through a deterministic Stop at the final barrier. Publication must
// use the driver's transport, not re-read the now-cleared process connection.
// The final response must still fail; cancellation must not become success.
func TestStartupStopBeforeBridgePublication(t *testing.T) {
	snap := registry.Snapshot()
	registry.Reset()
	t.Cleanup(func() { registry.Restore(snap) })
	const pluginName = "startup-stop-publication"
	started := make(chan struct{})
	sdkDone := make(chan struct{})
	registerLifecyclePlugin(t, pluginName, nil, func(conn net.Conn) int {
		p := sdk.NewWithConn(pluginName, conn)
		p.OnStarted(func(context.Context) error {
			close(started)
			return nil
		})
		err := p.Run(t.Context(), sdk.Registration{})
		close(sdkDone)
		if err != nil {
			return 1
		}
		return 0
	})
	s, spawner := newLifecycleStartupServer(t)
	if err := spawner.SpawnMore([]plugin.PluginConfig{{
		Name: pluginName, Internal: true, Encoder: plugin.EncodingJSON,
	}}); err != nil {
		t.Fatal(err)
	}
	s.procManager.Store(spawner.pm)
	proc := spawner.pm.GetProcess(pluginName)
	if err := proc.InitConns(); err != nil {
		t.Fatal(err)
	}
	conn := proc.Conn()
	sink := &stoppingStartupSink{engineStartupSink: &engineStartupSink{s: s, proc: proc}}
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	err := runStartupHandshake(ctx, sink)
	if !sink.stopped {
		t.Fatalf("handshake never reached the stop boundary: %v", err)
	}
	if proc.Conn() != nil {
		t.Fatal("Stop did not clear the process connection")
	}
	if !conn.HasBridge() {
		t.Fatal("publication skipped the driver's captured transport")
	}
	if err == nil || !strings.Contains(err.Error(), "stage 5 respond:") {
		t.Fatalf("stopped handshake must fail the final response, got %v", err)
	}
	// The SDK treats a stage-5 connection close as graceful shutdown, so its
	// return value is not a readiness receipt. Runtime entry is the boundary.
	select {
	case <-sdkDone:
	case <-ctx.Done():
		t.Fatal("SDK did not exit after startup Stop")
	}
	select {
	case <-started:
		t.Fatal("SDK entered runtime after startup Stop")
	default:
	}
	if proc.Bridge().Activated() {
		t.Fatal("SDK activated the bridge after startup Stop")
	}
	if _, err := proc.Bridge().SendCallback(ctx, "test", nil); !errors.Is(err, rpc.ErrBridgeClosed) {
		t.Fatalf("stopped bridge callback: %v, want ErrBridgeClosed", err)
	}
}
