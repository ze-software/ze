// Design: ai/rules/plugins.md -- ze_bgp absent (compile-out) validation
//
//go:build !ze_bgp

package hub

// VALIDATES: without the ze_bgp build tag (a bare ze_core / hardened build), the
// BGP plugin is NOT registered, no reactor factory is installed, and the bgp{}
// config schema is absent -- while the rest of the plugin registry is still
// populated. The binary symbol-drop proof is in build_tag_protocols_absent_test.go.
// PREVENTS: a regression where bgp leaks into a hardened build via an always-on
// import or an ungated registration/schema import.

import (
	"strings"
	"testing"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	pluginreg "github.com/ze-software/ze/internal/component/plugin/registry"
)

func TestBuildTag_BGP_Absent(t *testing.T) {
	if len(pluginreg.Names()) == 0 {
		t.Fatal("plugin registry empty; cannot validate bgp absence (all.go not linked)")
	}
	if pluginreg.Has("bgp") {
		t.Fatal("non-ze_bgp build: bgp plugin unexpectedly registered (not compiled out)")
	}
	if pluginreg.GetReactorFactory() != nil {
		t.Fatal("non-ze_bgp build: reactor factory unexpectedly registered (bgp/config not compiled out)")
	}
	// The hex-packet decoder seam is filled by internal/component/bgp/cli,
	// which only a ze_bgp build links. The web tool page reads that seam, so a
	// non-nil answer here would be a BGP-less binary still decoding packets.
	if pluginreg.GetPacketDecoder() != nil {
		t.Fatal("non-ze_bgp build: hex-packet decoder unexpectedly registered (bgp/cli not compiled out)")
	}
}

// TestBuildTag_BGP_AbsentRejectsBGPConfig proves the bgp config schema is gone
// too, not just the engine: the whole bgp{} config root lives in the gated
// internal/component/bgp/yang schema package. A bare build must reject a bgp{}
// block as an unknown field rather than silently ignoring it, so an operator who
// pastes BGP config onto a hardened binary sees the error instead of a daemon
// that quietly does nothing (spec R-6, ospf/isis/vrrp absent-config precedent).
func TestBuildTag_BGP_AbsentRejectsBGPConfig(t *testing.T) {
	_, err := zeconfig.ParseTreeWithYANG(bgpAbsenceProbeConfig, nil)
	if err == nil {
		t.Fatal("non-ze_bgp build unexpectedly accepted bgp config")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("bgp config rejection = %v, want clean unknown-field rejection", err)
	}
}
