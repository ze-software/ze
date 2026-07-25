// Design: ai/rules/feature-gate-registration.md -- ze_isis absent (compile-out) validation
//
//go:build !ze_isis

package hub

// VALIDATES: without the ze_isis build tag (e.g. ze-stripped or a bare ze_core
// build), the IS-IS plugin is NOT registered and its config schema is absent,
// while the rest of the plugin registry is still populated (so the absence is
// real, not a vacuously-empty registry). The binary symbol-drop proof is in
// build_tag_protocols_absent_test.go.
// PREVENTS: a regression where isis leaks into a hardened build via an always-on
// import or an ungated registration/schema import.

import (
	"strings"
	"testing"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	pluginreg "github.com/ze-software/ze/internal/component/plugin/registry"
)

func TestBuildTag_ISIS_Absent(t *testing.T) {
	if len(pluginreg.Names()) == 0 {
		t.Fatal("plugin registry empty; cannot validate isis absence (all.go not linked)")
	}
	if pluginreg.Has("isis") {
		t.Fatal("non-ze_isis build: isis plugin unexpectedly registered (not compiled out)")
	}
}

func TestBuildTag_ISIS_AbsentRejectsISISConfig(t *testing.T) {
	_, err := zeconfig.ParseTreeWithYANG("isis {\n\tnet 49.0001.0000.0000.0001.00;\n}\n", nil)
	if err == nil {
		t.Fatal("non-ze_isis build unexpectedly accepted isis config")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("isis config rejection = %v, want clean unknown-field rejection", err)
	}
}
