// Design: ai/rules/repo-maintenance.md -- doctor checks owned by the plugin that
// owns the runtime dependency
// Related: doctor_api.go -- checkVPPAPISocket and checkVPPVersion under test

package ifacevpp

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// vppBackendTree builds the smallest config that selects this backend, with
// the api-socket leaf when path is not empty.
func vppBackendTree(path string) *config.Tree {
	tree := config.NewTree()
	tree.GetOrCreateContainer("interface").Set("backend", backendVPP)
	if path != "" {
		tree.GetOrCreateContainer("vpp").Set("api-socket", path)
	}
	return tree
}

// closeFails is a connection whose Close reports an error.
type closeFails struct{ err error }

func (c closeFails) Close() error { return c.err }

// installAPISocketDial points checkVPPAPISocket at dial for one test and
// restores the real probe afterwards. It records the path dialed so a test can
// assert which socket the check chose.
func installAPISocketDial(t *testing.T, dial func(string) (io.Closer, error)) *string {
	t.Helper()
	saved := vppAPISocketDial
	t.Cleanup(func() { vppAPISocketDial = saved })
	var dialed string
	if dial == nil {
		vppAPISocketDial = nil
		return &dialed
	}
	vppAPISocketDial = func(_ context.Context, path string) (io.Closer, error) {
		dialed = path
		return dial(path)
	}
	return &dialed
}

// installVersionProbe points checkVPPVersion at probe for one test.
func installVersionProbe(t *testing.T, probe func() (string, error)) {
	t.Helper()
	saved := vppVersionProbe
	t.Cleanup(func() { vppVersionProbe = saved })
	if probe == nil {
		vppVersionProbe = nil
		return
	}
	vppVersionProbe = func(context.Context) (string, error) { return probe() }
}

func requireOneDiag(t *testing.T, diags []diagnostic.Diagnostic, code string, severity diagnostic.Severity) diagnostic.Diagnostic {
	t.Helper()
	if len(diags) != 1 {
		t.Fatalf("diagnostics = %d, want 1: %+v", len(diags), diags)
	}
	if diags[0].Code != code {
		t.Fatalf("code = %q, want %q", diags[0].Code, code)
	}
	if diags[0].Severity != severity {
		t.Fatalf("severity = %q, want %q", diags[0].Severity, severity)
	}
	return diags[0]
}

// TestCheckVPPAPISocketReportsARefusedDial drives the check over a config that
// names a socket nothing listens on.
//
// VALIDATES: the diagnostic carries doctor-vpp-unreachable at error severity,
// names the configured path, and the probe dialed that path rather than the
// default.
// PREVENTS: the backend's first apply being where an operator learns VPP is
// not running.
func TestCheckVPPAPISocketReportsARefusedDial(t *testing.T) {
	dialed := installAPISocketDial(t, func(string) (io.Closer, error) { return nil, errors.New("connection refused") })

	diags := checkVPPAPISocket(diagnostic.DoctorCheckContext{Tree: vppBackendTree("/run/custom/vpp.sock")})
	diag := requireOneDiag(t, diags, codeVPPUnreachable, diagnostic.SeverityError)
	if diag.Path != "/run/custom/vpp.sock" || *dialed != "/run/custom/vpp.sock" {
		t.Fatalf("path = %q, dialed %q, want the configured socket", diag.Path, *dialed)
	}
	if !strings.Contains(diag.Message, "unreachable") {
		t.Fatalf("message = %q, want the unreachable wording", diag.Message)
	}
}

// TestCheckVPPAPISocketDialsTheDefaultPath pins the socket the check dials
// when the config names none: the path the vpp component applies by default.
//
// VALIDATES: an absent `vpp { api-socket }` probes vppSocketPath.
// PREVENTS: a probe of an empty path, which every dial refuses, reporting an
// unreachable VPP on a host whose VPP listens on the default socket.
func TestCheckVPPAPISocketDialsTheDefaultPath(t *testing.T) {
	dialed := installAPISocketDial(t, func(string) (io.Closer, error) { return closeFails{}, nil })

	if diags := checkVPPAPISocket(diagnostic.DoctorCheckContext{Tree: vppBackendTree("")}); len(diags) != 0 {
		t.Fatalf("diagnostics = %d, want 0 for a socket that answers", len(diags))
	}
	if *dialed != vppSocketPath {
		t.Fatalf("dialed %q, want %q", *dialed, vppSocketPath)
	}
}

// TestCheckVPPAPISocketWarnsWhenTheConnectionWillNotClose covers the second
// verdict: the socket accepted the dial and the close failed.
//
// VALIDATES: doctor-vpp-unreachable at warning severity with the close wording.
// PREVENTS: a close failure being reported as a refused dial, at error
// severity, when the backend can in fact reach VPP.
func TestCheckVPPAPISocketWarnsWhenTheConnectionWillNotClose(t *testing.T) {
	installAPISocketDial(t, func(string) (io.Closer, error) { return closeFails{err: errors.New("bad file descriptor")}, nil })

	diags := checkVPPAPISocket(diagnostic.DoctorCheckContext{Tree: vppBackendTree("")})
	diag := requireOneDiag(t, diags, codeVPPUnreachable, diagnostic.SeverityWarning)
	if !strings.Contains(diag.Message, "close") {
		t.Fatalf("message = %q, want the close wording", diag.Message)
	}
}

// TestCheckVPPAPISocketIsSilentOffTheBackend is the negative half: no probe
// opens for a config on another backend, for no config, or where VPP does not
// run.
//
// VALIDATES: no diagnostic and no dial for a netlink backend, a nil tree, and a
// nil probe.
// PREVENTS: a VPP diagnostic on every box that never selected VPP.
func TestCheckVPPAPISocketIsSilentOffTheBackend(t *testing.T) {
	dialed := installAPISocketDial(t, func(string) (io.Closer, error) { return nil, errors.New("must not dial") })

	netlinkTree := config.NewTree()
	netlinkTree.GetOrCreateContainer("interface").Set("backend", "netlink")
	if diags := checkVPPAPISocket(diagnostic.DoctorCheckContext{Tree: netlinkTree}); len(diags) != 0 {
		t.Fatalf("netlink backend: diagnostics = %d, want 0", len(diags))
	}
	if diags := checkVPPAPISocket(diagnostic.DoctorCheckContext{}); len(diags) != 0 {
		t.Fatalf("nil tree: diagnostics = %d, want 0", len(diags))
	}
	if *dialed != "" {
		t.Fatalf("a skipped check dialed %q", *dialed)
	}

	installAPISocketDial(t, nil)
	if diags := checkVPPAPISocket(diagnostic.DoctorCheckContext{Tree: vppBackendTree("")}); len(diags) != 0 {
		t.Fatalf("nil probe: diagnostics = %d, want 0", len(diags))
	}
}

// TestCheckVPPVersionReadsVppctl drives the version probe through its three
// answers: a version line, a failed probe, and output without a version.
//
// VALIDATES: a `vpp v` line is silent; a failed probe and unrecognized output
// each warn under doctor-vpp-version with their own wording.
// PREVENTS: an absent or wedged VPP reported at error severity on the strength
// of a version the probe could not read.
func TestCheckVPPVersionReadsVppctl(t *testing.T) {
	tree := vppBackendTree("")

	installVersionProbe(t, func() (string, error) {
		return "vpp v25.02-release built by root on buildkitsandbox at 2025-02-26T14:32:29\n", nil
	})
	if diags := checkVPPVersion(diagnostic.DoctorCheckContext{Tree: tree}); len(diags) != 0 {
		t.Fatalf("version line: diagnostics = %d, want 0", len(diags))
	}

	installVersionProbe(t, func() (string, error) { return "", errors.New("exec: \"vppctl\": executable file not found in $PATH") })
	diag := requireOneDiag(t, checkVPPVersion(diagnostic.DoctorCheckContext{Tree: tree}), codeVPPVersion, diagnostic.SeverityWarning)
	if !strings.Contains(diag.Message, "cannot determine VPP version") {
		t.Fatalf("message = %q, want the probe-failure wording", diag.Message)
	}

	installVersionProbe(t, func() (string, error) { return "clib_socket_init: connect (fd 3, '/run/vpp/cli.sock')\n", nil })
	diag = requireOneDiag(t, checkVPPVersion(diagnostic.DoctorCheckContext{Tree: tree}), codeVPPVersion, diagnostic.SeverityWarning)
	if !strings.Contains(diag.Message, "unexpected VPP version output") {
		t.Fatalf("message = %q, want the unexpected-output wording", diag.Message)
	}
}

// TestCheckVPPVersionIsSilentOffTheBackend is the negative half of the version
// probe.
//
// VALIDATES: no diagnostic for another backend, a nil tree, and a nil probe.
// PREVENTS: vppctl being run on a box that never selected VPP.
func TestCheckVPPVersionIsSilentOffTheBackend(t *testing.T) {
	calls := 0
	installVersionProbe(t, func() (string, error) { calls++; return "", errors.New("must not run") })

	if diags := checkVPPVersion(diagnostic.DoctorCheckContext{Tree: config.NewTree()}); len(diags) != 0 {
		t.Fatalf("no interface block: diagnostics = %d, want 0", len(diags))
	}
	if diags := checkVPPVersion(diagnostic.DoctorCheckContext{}); len(diags) != 0 {
		t.Fatalf("nil tree: diagnostics = %d, want 0", len(diags))
	}
	if calls != 0 {
		t.Fatalf("a skipped check ran vppctl %d times", calls)
	}

	installVersionProbe(t, nil)
	if diags := checkVPPVersion(diagnostic.DoctorCheckContext{Tree: vppBackendTree("")}); len(diags) != 0 {
		t.Fatalf("nil probe: diagnostics = %d, want 0", len(diags))
	}
}

// TestVPPBackendDoctorChecksRegistered asks the registry the doctor runner
// reads whether it holds every check this backend declares, at the phase and
// order the table states.
//
// VALIDATES: the init() in register.go installed the table and the registry
// accepted it.
// PREVENTS: a check that `ze doctor` stopped running while its unit tests
// stayed green.
func TestVPPBackendDoctorChecksRegistered(t *testing.T) {
	if len(vppDoctorChecks) == 0 {
		t.Fatal("this backend declares no check: the loop below would pass over an empty table")
	}
	for i := range vppDoctorChecks {
		want := vppDoctorChecks[i]
		var found *diagnostic.DoctorCheck
		checks := diagnostic.DoctorChecksForPhase(want.Phase)
		for j := range checks {
			if checks[j].Name == want.Name {
				found = &checks[j]
				break
			}
		}
		if found == nil {
			t.Errorf("doctor check %q is not registered for phase %q", want.Name, want.Phase)
			continue
		}
		if found.Order != want.Order {
			t.Errorf("check %q: order = %d, want %d", want.Name, found.Order, want.Order)
		}
		if found.Component != want.Component {
			t.Errorf("check %q: component = %q, want %q", want.Name, found.Component, want.Component)
		}
	}
}
