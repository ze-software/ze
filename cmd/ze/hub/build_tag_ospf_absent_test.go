// Design: ai/rules/plugins.md -- ze_ospf absent (compile-out) validation
//
//go:build !ze_ospf

package hub

// VALIDATES: without the ze_ospf build tag (e.g. ze-stripped or a bare ze_core
// build), the OSPF plugin is NOT registered and its config schema is absent,
// while the rest of the plugin registry is still populated. The binary
// symbol-drop proof is in build_tag_protocols_absent_test.go.
// PREVENTS: a regression where ospf leaks into a hardened build via an always-on
// import or a missed composition root (ze_core_dispatch.go still links ospf/cli).

import (
	"strings"
	"testing"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	pluginreg "github.com/ze-software/ze/internal/component/plugin/registry"
)

func TestBuildTag_OSPF_Absent(t *testing.T) {
	if len(pluginreg.Names()) == 0 {
		t.Fatal("plugin registry empty; cannot validate ospf absence (all.go not linked)")
	}
	if pluginreg.Has("ospf") {
		t.Fatal("non-ze_ospf build: ospf plugin unexpectedly registered (not compiled out)")
	}
}

func TestBuildTag_OSPF_AbsentRejectsOSPFConfig(t *testing.T) {
	_, err := zeconfig.ParseTreeWithYANG("ospf {\n\trouter-id 1.1.1.1;\n}\n", nil)
	if err == nil {
		t.Fatal("non-ze_ospf build unexpectedly accepted ospf config")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("ospf config rejection = %v, want clean unknown-field rejection", err)
	}
}
