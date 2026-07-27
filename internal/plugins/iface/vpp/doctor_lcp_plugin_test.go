// Design: ai/rules/doctor-checks.md -- doctor checks owned by the plugin that
// owns the runtime dependency
// Related: doctor.go -- checkVPPLCPPlugin and lcpEnabled under test

package ifacevpp

import (
	"runtime"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/core/diagnostic"
)

// lcpTree builds a config tree with a vpp/lcp container, optionally setting the
// `enabled` leaf. Passing "" omits the leaf, which is the YANG-default (on) case.
func lcpTree(enabled string) *config.Tree {
	tree := config.NewTree()
	vpp := config.NewTree()
	lcp := config.NewTree()
	if enabled != "" {
		lcp.Set("enabled", enabled)
	}
	vpp.SetContainer("lcp", lcp)
	tree.SetContainer("vpp", vpp)
	return tree
}

// TestLCPEnabledTreatsAbsentLeafAsOn pins the gate the whole check hangs on.
//
// VALIDATES: fixit-vpp-lcp-reachability AC-11 -- the check skips when LCP is
// off, and engages when it is on.
//
// PREVENTS: reading a missing `enabled` leaf as "off". The YANG default is on,
// so `vpp { lcp { } }` DOES load linux_cp_plugin.so via startup.conf
// (component/vpp/startupconf.go); treating it as off would silently skip the
// diagnostic for the very config shape most likely to be written by hand.
func TestLCPEnabledTreatsAbsentLeafAsOn(t *testing.T) {
	for _, tt := range []struct {
		name string
		tree *config.Tree
		want bool
	}{
		{"no vpp container at all", config.NewTree(), false},
		{"lcp present, enabled omitted (YANG default on)", lcpTree(""), true},
		{"lcp explicitly enabled", lcpTree("true"), true},
		{"lcp explicitly disabled", lcpTree("false"), false},
	} {
		t.Run(tt.name, func(t *testing.T) {
			if got := lcpEnabled(tt.tree); got != tt.want {
				t.Fatalf("lcpEnabled() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestCheckVPPLCPPluginSkipsWhenNotApplicable covers the two no-probe paths.
//
// VALIDATES: AC-11 -- with LCP disabled, or on a non-Linux host, the check
// returns nothing and opens no probe.
//
// PREVENTS: an unconditional vppctl exec. Doctor runs on every `ze doctor`, and
// shelling out on a host that cannot run VPP would produce a warning about a
// dependency the operator never asked for.
func TestCheckVPPLCPPluginSkipsWhenNotApplicable(t *testing.T) {
	t.Run("lcp disabled yields nothing", func(t *testing.T) {
		got := checkVPPLCPPlugin(diagnostic.DoctorCheckContext{Tree: lcpTree("false")})
		if len(got) != 0 {
			t.Fatalf("expected no diagnostics with lcp disabled, got %v", got)
		}
	})

	t.Run("nil tree yields nothing", func(t *testing.T) {
		if got := checkVPPLCPPlugin(diagnostic.DoctorCheckContext{}); len(got) != 0 {
			t.Fatalf("expected no diagnostics for a nil tree, got %v", got)
		}
	})

	if runtime.GOOS != "linux" {
		t.Run("non-linux host opens no probe", func(t *testing.T) {
			got := checkVPPLCPPlugin(diagnostic.DoctorCheckContext{Tree: lcpTree("true")})
			if len(got) != 0 {
				t.Fatalf("VPP is Linux-only; expected no diagnostics here, got %v", got)
			}
		})
	}
}

// TestVPPLCPPluginCheckIsRegistered proves the check reaches the registry with
// the metadata `ze doctor` filters on.
//
// VALIDATES: AC-7 -- the check is registered and its diagnostic code is known.
//
// PREVENTS: the shape ai/rules/doctor-checks.md exists to stop: a check that is
// implemented and unit-tested but never runs, because nothing registered it.
func TestVPPLCPPluginCheckIsRegistered(t *testing.T) {
	// Built-in codes are registered by the binary entry point, not by init(),
	// so a package-level test has to ask for them.
	diagnostic.RegisterBuiltinCodes()

	meta := diagnostic.Lookup("doctor-vpp-lcp-plugin")
	if meta == nil {
		t.Fatal("diagnostic code doctor-vpp-lcp-plugin is not registered; ze explain would not resolve it")
	}
	if meta.Description == "" {
		t.Fatal("registered code has no description; ze explain would print nothing useful")
	}
}
