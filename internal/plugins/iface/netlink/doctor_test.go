// Design: docs/features/interfaces.md -- doctor-iface-macvlan capability check tests

package ifacenetlink

import (
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

func ifaceTree(backend string) *config.Tree {
	tree := config.NewTree()
	it := tree.GetOrCreateContainer("interface")
	if backend != "" {
		it.Set("backend", backend)
	}
	return tree
}

// TestDoctorIfaceMacvlanProbe exercises the check's mapping from the capability
// probe result to a diagnostic, and its gating.
//
// VALIDATES: AC-11 -- capable kernel -> OK (no diagnostic); incapable kernel ->
// actionable failure naming CONFIG_MACVLAN; missing CAP_NET_ADMIN -> a graceful
// warning; and the check is a no-op when interface is unconfigured or the
// backend is vpp.
// PREVENTS: a macvlan capability gap surfacing only as a runtime apply failure.
func TestDoctorIfaceMacvlanProbe(t *testing.T) {
	restore := macvlanProbe
	t.Cleanup(func() { macvlanProbe = restore })

	tests := []struct {
		name        string
		tree        *config.Tree
		probe       macvlanProbeResult
		wantCode    string
		wantSev     diagnostic.Severity
		wantNoDiags bool
	}{
		{name: "capable-ok", tree: ifaceTree(""), probe: macvlanProbeOK, wantNoDiags: true},
		{name: "incapable-error", tree: ifaceTree(""), probe: macvlanProbeUnsupported, wantCode: "doctor-iface-macvlan", wantSev: diagnostic.SeverityError},
		{name: "no-privilege-warning", tree: ifaceTree(""), probe: macvlanProbeNoPrivilege, wantCode: "doctor-iface-macvlan", wantSev: diagnostic.SeverityWarning},
		{name: "vpp-backend-skips", tree: ifaceTree("vpp"), probe: macvlanProbeUnsupported, wantNoDiags: true},
		{name: "no-interface-skips", tree: config.NewTree(), probe: macvlanProbeUnsupported, wantNoDiags: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			macvlanProbe = func() macvlanProbeResult { return tt.probe }
			diags := checkIfaceMacvlan(diagnostic.DoctorCheckContext{Tree: tt.tree})
			if tt.wantNoDiags {
				if len(diags) != 0 {
					t.Fatalf("want no diagnostics, got %v", diags)
				}
				return
			}
			if len(diags) != 1 {
				t.Fatalf("want 1 diagnostic, got %d", len(diags))
			}
			if diags[0].Code != tt.wantCode {
				t.Errorf("code = %q, want %q", diags[0].Code, tt.wantCode)
			}
			if diags[0].Severity != tt.wantSev {
				t.Errorf("severity = %q, want %q", diags[0].Severity, tt.wantSev)
			}
		})
	}

	// nil tree is tolerated (no panic, no diagnostic).
	if diags := checkIfaceMacvlan(diagnostic.DoctorCheckContext{Tree: nil}); len(diags) != 0 {
		t.Errorf("nil tree: want no diagnostics, got %v", diags)
	}
}

// TestDoctorIfaceMacvlanCheckRegistered proves the netlink backend registers
// the check via diagnostic.RegisterDoctorCheck (removing the package removes it).
func TestDoctorIfaceMacvlanCheckRegistered(t *testing.T) {
	if !slices.Contains(diagnostic.DoctorCheckNames(), "iface-macvlan") {
		t.Fatal("doctor check iface-macvlan not registered via diagnostic.RegisterDoctorCheck")
	}
}

// TestDoctorIfaceMacvlanCodeRegistered proves the diagnostic code is explainable
// via ze explain (registered in codes.go with title + description).
func TestDoctorIfaceMacvlanCodeRegistered(t *testing.T) {
	diagnostic.RegisterBuiltinCodes()
	meta := diagnostic.Lookup("doctor-iface-macvlan")
	if meta == nil {
		t.Fatal("doctor-iface-macvlan not registered in codes.go")
	}
	if meta.Title == "" || meta.Description == "" {
		t.Error("doctor-iface-macvlan missing title/description")
	}
}
