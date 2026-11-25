// Design: docs/architecture/ddos/cp-survival-5-detect-0-umbrella.md -- separate-process lifecycle for ddos-flowspec
// Related: register.go (CLIHandler, loadBus, localEventBus, runEngine), exit_test.go (engineStub)

package flowspec

import (
	"context"
	"net"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/plugin/registry"
	"github.com/ze-software/ze/internal/core/ddosevent"
	"github.com/ze-software/ze/pkg/plugin/rpc"
	"github.com/ze-software/ze/pkg/ze"
)

// captureCLIStdout invokes fn with os.Stdout redirected to a temporary file
// and returns both the exit code and captured bytes.
func captureCLIStdout(t *testing.T, fn func() int) (int, string) {
	t.Helper()
	outFile, err := os.CreateTemp(t.TempDir(), "cli-stdout-*")
	if err != nil {
		t.Fatalf("create temp stdout file: %v", err)
	}
	origStdout := os.Stdout
	os.Stdout = outFile
	code := fn()
	os.Stdout = origStdout
	if err := outFile.Close(); err != nil {
		t.Fatalf("close temp stdout file: %v", err)
	}
	data, err := os.ReadFile(outFile.Name())
	if err != nil {
		t.Fatalf("read captured stdout: %v", err)
	}
	return code, string(data)
}

// TestCLIHandlerSupportsSeparateProcessEntry proves that the ddos-flowspec
// registration wires CLIHandler through cli.RunPlugin, stays fork-capable,
// and declares its bgp:update send wiring consistently.
//
// VALIDATES: spec-explicit-plugin-and-attach-config S15 / AC-13 -- `ze plugin
// ddos-flowspec` (launched by `process separate;`) dispatches through the
// standard plugin CLI handler rather than exiting 1 immediately.
// PREVENTS: a stub CLIHandler returning 1 that breaks `process separate;`
// before the subprocess can dial the hub or answer `--features`/`--yang`.
func TestCLIHandlerSupportsSeparateProcessEntry(t *testing.T) {
	reg := registry.Lookup(Name)
	if reg == nil {
		t.Fatalf("registry.Lookup(%q) = nil, want registered plugin", Name)
	}
	if reg.RunsInProcessOnly {
		t.Fatalf("registration %q sets RunsInProcessOnly, want false for process separate", Name)
	}
	if reg.CLIHandler == nil {
		t.Fatalf("registration %q has nil CLIHandler", Name)
	}

	code, out := captureCLIStdout(t, func() int {
		return reg.CLIHandler([]string{"--features"})
	})
	if code != 0 {
		t.Fatalf("CLIHandler(--features) exit code = %d, want 0", code)
	}
	if strings.TrimSpace(out) != "yang" {
		t.Errorf("CLIHandler(--features) stdout = %q, want %q", out, "yang")
	}

	code, out = captureCLIStdout(t, func() int {
		return reg.CLIHandler([]string{"--yang"})
	})
	if code != 0 {
		t.Fatalf("CLIHandler(--yang) exit code = %d, want 0", code)
	}
	if !strings.Contains(out, "module ze-ddos-flowspec-conf") {
		t.Errorf("CLIHandler(--yang) stdout missing module header: %q", out)
	}

	foundSend := false
	for _, s := range reg.Sends {
		if s.Message == "bgp:update" {
			foundSend = true
		}
	}
	if !foundSend {
		t.Errorf("registration Sends = %+v, want bgp:update", reg.Sends)
	}
}

// TestLoadBusFallsBackToLocalBusOnlyWhenSeparate proves that loadBus refuses
// a missing daemon EventBus for an in-process run, while falling back to a
// reusable process-local bus when running as a separate OS process.
//
// VALIDATES: separate-process startups (where ConfigureEventBus is never
// called) get a working ze.EventBus without weakening in-process refusal.
// PREVENTS: separate-process OnConfigure failing with "event bus not
// configured", or an in-process wiring bug being masked by a silent fallback.
func TestLoadBusFallsBackToLocalBusOnlyWhenSeparate(t *testing.T) {
	previousBus := eventBusPtr.Load()
	eventBusPtr.Store(nil)
	t.Cleanup(func() { eventBusPtr.Store(previousBus) })

	var fallback ze.EventBus
	if _, err := loadBus(true, &fallback); err == nil {
		t.Fatal("loadBus(isInternal=true) with nil eventBusPtr = nil error, want refusal")
	}
	if fallback != nil {
		t.Fatalf("loadBus(isInternal=true) populated fallback = %T, want nil", fallback)
	}

	first, err := loadBus(false, &fallback)
	if err != nil {
		t.Fatalf("loadBus(isInternal=false) error = %v, want nil", err)
	}
	if first == nil {
		t.Fatal("loadBus(isInternal=false) returned nil bus")
	}
	second, err := loadBus(false, &fallback)
	if err != nil {
		t.Fatalf("second loadBus(isInternal=false) error = %v, want nil", err)
	}
	if second != first {
		t.Fatal("second loadBus(isInternal=false) allocated a different fallback bus")
	}

	evType := ddosevent.Detected.EventType()
	// Nil handler is a no-op per ze.EventBus contract.
	noop := first.Subscribe(ddosevent.Namespace, evType, nil)
	noop()

	var received []any
	unsub := first.Subscribe(ddosevent.Namespace, evType, func(payload any) {
		received = append(received, payload)
	})
	n, err := first.Emit(ddosevent.Namespace, evType, "hit")
	if err != nil {
		t.Fatalf("Emit() error = %v, want nil", err)
	}
	if n != 0 {
		t.Fatalf("Emit() delivered = %d, want 0", n)
	}
	if len(received) != 1 {
		t.Fatalf("received = %v, want [hit]", received)
	}
	if received[0] != "hit" {
		t.Fatalf("received[0] = %v, want hit", received[0])
	}
	unsub()
	unsub() // idempotent per ze.EventBus contract
	if _, err := first.Emit(ddosevent.Namespace, evType, "after-unsub"); err != nil {
		t.Fatalf("Emit after unsubscribe error = %v", err)
	}
	if len(received) != 1 {
		t.Fatalf("received after unsubscribe = %v, want [hit]", received)
	}
}

// TestRunEngineStartsAndReconfiguresInSeparateProcessWithoutEventBus proves
// that runEngine completes startup (OnConfigure, OnStarted state read),
// answers doctor checks, handles config verify/apply reloads, and exits
// cleanly on bye when eventBusPtr is nil on a non-internal connection.
//
// VALIDATES: spec-explicit-plugin-and-attach-config S15 / AC-13 end-to-end
// plugin lifecycle in `process separate;` mode without ConfigureEventBus.
// PREVENTS: handshake failure in OnConfigure or reload failure in
// OnConfigApply when ddos-flowspec runs in a separate OS process.
func TestRunEngineStartsAndReconfiguresInSeparateProcessWithoutEventBus(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	previousBus := eventBusPtr.Load()
	eventBusPtr.Store(nil)
	t.Cleanup(func() { eventBusPtr.Store(previousBus) })

	pluginEnd, engineEnd := net.Pipe()
	e := &engineStub{
		t:     t,
		mux:   rpc.NewMuxConn(rpc.NewConn(engineEnd, engineEnd)),
		store: newStubStore(nil),
		seen:  make(chan string, 256),
		done:  make(chan struct{}),
	}
	go e.run(pluginEnd)
	go e.serve(ctx)
	t.Cleanup(func() {
		_ = e.mux.Close()
		_ = engineEnd.Close()
		_ = pluginEnd.Close()
		select {
		case <-e.done:
		case <-time.After(5 * time.Second):
			t.Error("the separate-process ddos-flowspec engine did not exit after its transport closed")
		}
	})

	e.configure(ctx)
	e.finishStartup(ctx)

	calls := e.stateCalls()
	if len(calls) == 0 {
		t.Fatalf("expected OnStarted state read of %s, got none", announcementRecordKey)
	}
	if calls[0].method != rpc.MethodStateGet {
		t.Fatalf("first state call method = %s, want %s", calls[0].method, rpc.MethodStateGet)
	}
	if calls[0].key != announcementRecordKey {
		t.Fatalf("first state call key = %s, want %s", calls[0].key, announcementRecordKey)
	}

	if diags := e.doctorCheck(ctx, orphanRuntimeCheck); len(diags) != 0 {
		t.Fatalf("doctor check %s with empty store = %+v, want none", orphanRuntimeCheck, diags)
	}

	updated := `{"ddos":{"flowspec":{"response-level":"enforce","action":"discard","hold-down":"600"}}}`
	status, reason := e.configStatus(ctx, "ze-plugin-callback:config-verify", rpc.ConfigVerifyInput{
		Sections: []rpc.ConfigSection{{Root: configRoot, Data: updated}},
	})
	if status != rpc.StatusOK {
		t.Fatalf("config-verify answered %q: %s, want %q", status, reason, rpc.StatusOK)
	}
	status, reason = e.configStatus(ctx, "ze-plugin-callback:config-apply", rpc.ConfigApplyInput{
		Sections: []rpc.ConfigDiffSection{{Root: configRoot}},
	})
	if status != rpc.StatusOK {
		t.Fatalf("config-apply answered %q: %s, want %q", status, reason, rpc.StatusOK)
	}

	e.bye(ctx)
	e.waitExit(ctx)
}
