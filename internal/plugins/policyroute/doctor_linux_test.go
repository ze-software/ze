//go:build linux

// Design: docs/architecture/policyroute/policy-routing.md -- the readiness check this plugin owns
// Detail: doctor_linux.go -- checkPolicyRouteNetlink and its registration

package policyroute

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// closedHandle counts how many times the check closed the handle it opened.
type closedHandle struct{ closes int }

func (h *closedHandle) Close() { h.closes++ }

// withRouteNetlink stands in a kernel for one test: err is what opening the
// route netlink handle answers, and the returned handle records its close.
func withRouteNetlink(t *testing.T, err error) *closedHandle {
	t.Helper()
	previous := openRouteNetlink
	handle := &closedHandle{}
	openRouteNetlink = func() (routeNetlinkHandle, error) {
		if err != nil {
			return nil, err
		}
		return handle, nil
	}
	t.Cleanup(func() { openRouteNetlink = previous })
	return handle
}

// policyRouteTree builds the smallest config that installs one policy route.
func policyRouteTree() *config.Tree {
	tree := config.NewTree()
	route := config.NewTree()
	route.AddListEntry("rule", "r1", config.NewTree())
	tree.GetOrCreateContainer(configRoot).AddListEntry("route", "pbr", route)
	return tree
}

// TestCheckPolicyRouteNetlinkReportsAnUnopenableHandle drives the check over a
// policy route on a kernel that refuses the netlink handle.
//
// VALIDATES: doctor-policyroute-netlink at warning severity, carrying the
// refusal.
// PREVENTS: policy route netlink dependency gaps being missed during readiness
// checks.
func TestCheckPolicyRouteNetlinkReportsAnUnopenableHandle(t *testing.T) {
	withRouteNetlink(t, errors.New("netlink unavailable"))

	diags := checkPolicyRouteNetlink(diagnostic.DoctorCheckContext{Tree: policyRouteTree()})
	if len(diags) != 1 {
		t.Fatalf("diagnostics = %d, want 1: %+v", len(diags), diags)
	}
	if diags[0].Code != codePolicyRouteNetlink || diags[0].Severity != diagnostic.SeverityWarning {
		t.Fatalf("diagnostic = %s/%s, want %s at warning", diags[0].Code, diags[0].Severity, codePolicyRouteNetlink)
	}
	if !strings.Contains(diags[0].Message, "netlink unavailable") {
		t.Fatalf("message = %q, want the refusal carried", diags[0].Message)
	}
}

// TestCheckPolicyRouteNetlinkClosesTheHandleItOpened is the positive half: a
// handle that opens is closed and nothing is reported.
//
// VALIDATES: no diagnostic, and exactly one close of the probe handle.
// PREVENTS: a leaked netlink socket on every doctor run.
func TestCheckPolicyRouteNetlinkClosesTheHandleItOpened(t *testing.T) {
	handle := withRouteNetlink(t, nil)

	if diags := checkPolicyRouteNetlink(diagnostic.DoctorCheckContext{Tree: policyRouteTree()}); len(diags) != 0 {
		t.Fatalf("diagnostics = %d, want 0: %+v", len(diags), diags)
	}
	if handle.closes != 1 {
		t.Fatalf("handle closed %d times, want 1", handle.closes)
	}
}

// TestCheckPolicyRouteNetlinkIsSilentWithoutRoutes is the negative half: no
// policy block, an empty one, and no config each open no handle.
//
// VALIDATES: no diagnostic and no probe in each case.
// PREVENTS: a netlink probe on every box that never configured a policy route.
func TestCheckPolicyRouteNetlinkIsSilentWithoutRoutes(t *testing.T) {
	opened := false
	previous := openRouteNetlink
	openRouteNetlink = func() (routeNetlinkHandle, error) { opened = true; return nil, errors.New("must not open") }
	t.Cleanup(func() { openRouteNetlink = previous })

	if diags := checkPolicyRouteNetlink(diagnostic.DoctorCheckContext{Tree: config.NewTree()}); len(diags) != 0 {
		t.Fatalf("no block: diagnostics = %d, want 0", len(diags))
	}
	empty := config.NewTree()
	empty.GetOrCreateContainer(configRoot)
	if diags := checkPolicyRouteNetlink(diagnostic.DoctorCheckContext{Tree: empty}); len(diags) != 0 {
		t.Fatalf("empty block: diagnostics = %d, want 0", len(diags))
	}
	if diags := checkPolicyRouteNetlink(diagnostic.DoctorCheckContext{}); len(diags) != 0 {
		t.Fatalf("nil tree: diagnostics = %d, want 0", len(diags))
	}
	if opened {
		t.Fatal("a skipped check opened a netlink handle")
	}
}

// TestPolicyRouteDoctorCheckRegistered asks the registry the doctor runner
// reads whether it holds this plugin's check, at the phase and order the
// declaration states, and whether the code it emits resolves for `ze explain`.
//
// VALIDATES: the init() in register_linux.go installed the check and the
// registry accepted it.
// PREVENTS: a check that `ze doctor` stopped running while its unit tests
// stayed green.
func TestPolicyRouteDoctorCheckRegistered(t *testing.T) {
	want := policyRouteDoctorCheck
	var found *diagnostic.DoctorCheck
	checks := diagnostic.DoctorChecksForPhase(want.Phase)
	for i := range checks {
		if checks[i].Name == want.Name {
			found = &checks[i]
			break
		}
	}
	if found == nil {
		t.Fatalf("doctor check %q is not registered for phase %q", want.Name, want.Phase)
	}
	if found.Order != want.Order {
		t.Fatalf("order = %d, want %d", found.Order, want.Order)
	}
	if found.Component != want.Component {
		t.Fatalf("component = %q, want %q", found.Component, want.Component)
	}

	diagnostic.RegisterBuiltinCodes()
	if diagnostic.Lookup(codePolicyRouteNetlink) == nil {
		t.Fatalf("diagnostic code %q is not registered", codePolicyRouteNetlink)
	}
}
