package server

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/component/plugin/process"
)

// VALIDATES: startupFailureError wraps the recorded cause, so the error ze
// prints on stderr before exiting names WHY the plugin failed, not just the
// stage it stopped at.
// PREVENTS: the regression this function exists to fix -- an unprivileged ze
// with an interface{} block exited 1 with only "plugin interface failed during
// startup at stage Config", giving the operator no offending object, no reason
// and no corrective action (ai/rules/error-messages.md).
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
// that startup succeeded (ai/rules/fail-closed-guards.md).
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
