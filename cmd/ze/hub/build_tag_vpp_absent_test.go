// Design: ai/rules/feature-gate-registration.md -- ze_vpp absent (compile-out) validation
//
//go:build !ze_vpp

package hub

// VALIDATES: without the ze_vpp build tag the VPP dataplane is gone: the
// fib-vpp plugin is not registered, the `vpp {}` component config root is
// rejected as unknown, and selecting the iface "vpp" backend fails closed with
// "unknown backend" naming the registered set -- while the rest of the plugin
// registry is still populated. The binary symbol-drop proof (all five backend
// packages, the connector component, and vendored govpp) is in
// build_tag_gate12_absent_test.go.
// PREVENTS: a regression where a VPP backend leaks into a hardened build via
// an always-on import (e.g. the static or ike source-tagged files losing
// their ze_vpp constraint).

import (
	"strings"
	"testing"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/iface"
	pluginreg "github.com/ze-software/ze/internal/component/plugin/registry"
)

func TestBuildTag_VPP_Absent(t *testing.T) {
	if len(pluginreg.Names()) == 0 {
		t.Fatal("plugin registry empty; cannot validate vpp absence (all.go not linked)")
	}
	if pluginreg.Has("fib-vpp") {
		t.Fatal("non-ze_vpp build: fib-vpp plugin unexpectedly registered (not compiled out)")
	}
}

// TestBuildTag_VPP_AbsentRejectsVPPConfig proves the vpp component schema is
// gone: a `vpp {}` block must be rejected as an unknown field.
func TestBuildTag_VPP_AbsentRejectsVPPConfig(t *testing.T) {
	const cfg = `vpp {
	cpu {
	}
}
`
	_, err := zeconfig.ParseTreeWithYANG(cfg, nil)
	if err == nil {
		t.Fatal("non-ze_vpp build unexpectedly accepted vpp config")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("vpp config rejection = %v, want clean unknown-field rejection", err)
	}
}

// TestBuildTag_VPP_AbsentIfaceBackendFailsClosed proves an operator selecting
// `interface { backend vpp }` in a vpp-less build gets the fail-closed
// unknown-backend rejection (iface backend.go LoadBackend), never a silent
// no-op. LoadBackend keeps the previous backend on failure, so the probe has
// no side effects.
func TestBuildTag_VPP_AbsentIfaceBackendFailsClosed(t *testing.T) {
	err := iface.LoadBackend("vpp")
	if err == nil {
		t.Fatal("non-ze_vpp build: LoadBackend(vpp) unexpectedly succeeded (backend not compiled out)")
	}
	if !strings.Contains(err.Error(), "unknown backend") {
		t.Fatalf("LoadBackend(vpp) = %v, want fail-closed unknown-backend rejection", err)
	}
}
