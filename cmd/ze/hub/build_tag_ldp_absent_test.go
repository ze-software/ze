// Design: ai/rules/feature-gate-registration.md -- ze_ldp absent (compile-out) validation
//
//go:build !ze_ldp

package hub

// VALIDATES: without the ze_ldp build tag (e.g. ze-stripped or a bare ze_core
// build), the LDP plugin is NOT registered and its config schema is absent,
// while the rest of the plugin registry is still populated. The binary
// symbol-drop proof is in build_tag_protocols_absent_test.go.
// PREVENTS: a regression where ldp leaks into a hardened build via an always-on
// import or an ungated registration/schema import.

import (
	"strings"
	"testing"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	pluginreg "github.com/ze-software/ze/internal/component/plugin/registry"
)

func TestBuildTag_LDP_Absent(t *testing.T) {
	if len(pluginreg.Names()) == 0 {
		t.Fatal("plugin registry empty; cannot validate ldp absence (all.go not linked)")
	}
	if pluginreg.Has("ldp") {
		t.Fatal("non-ze_ldp build: ldp plugin unexpectedly registered (not compiled out)")
	}
}

func TestBuildTag_LDP_AbsentRejectsLDPConfig(t *testing.T) {
	_, err := zeconfig.ParseTreeWithYANG("ldp {\n\trouter-id 1.1.1.1;\n}\n", nil)
	if err == nil {
		t.Fatal("non-ze_ldp build unexpectedly accepted ldp config")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("ldp config rejection = %v, want clean unknown-field rejection", err)
	}
}
