// Design: ai/rules/feature-gate-registration.md -- ze_bfd absent (compile-out) validation
//
//go:build !ze_bfd

package hub

// VALIDATES: without the ze_bfd build tag the BFD plugin is NOT registered and
// its config schema (`bfd {}`) is absent, while the rest of the plugin
// registry is still populated. BGP/OSPF/static keep working: their only BFD
// coupling is the always-on nil-able bfd/api seam (GetService returns nil,
// every consumer warns and degrades). The binary symbol-drop proof -- engine
// dropped, api/packet contract deliberately retained -- is in
// build_tag_gate12_absent_test.go.
// PREVENTS: a regression where the bfd engine leaks into a hardened build via
// an always-on import (e.g. the analysis-tree schema pin returning to the
// ungated tree.go).

import (
	"strings"
	"testing"

	zeconfig "codeberg.org/thomas-mangin/ze/internal/component/config"
	pluginreg "codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

func TestBuildTag_BFD_Absent(t *testing.T) {
	if len(pluginreg.Names()) == 0 {
		t.Fatal("plugin registry empty; cannot validate bfd absence (all.go not linked)")
	}
	if pluginreg.Has("bfd") {
		t.Fatal("non-ze_bfd build: bfd plugin unexpectedly registered (not compiled out)")
	}
}

// TestBuildTag_BFD_AbsentRejectsBFDConfig proves the bfd config schema is gone
// too, not just the engine. A bare build must reject a `bfd {}` block as an
// unknown field rather than silently accept or crash.
func TestBuildTag_BFD_AbsentRejectsBFDConfig(t *testing.T) {
	const cfg = `bfd {
	echo {
	}
}
`
	_, err := zeconfig.ParseTreeWithYANG(cfg, nil)
	if err == nil {
		t.Fatal("non-ze_bfd build unexpectedly accepted bfd config")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("bfd config rejection = %v, want clean unknown-field rejection", err)
	}
}
