// Design: ai/rules/plugins.md -- MRT absent without ze_bgp (compile-out) validation
//
//go:build !ze_bgp

package hub

// VALIDATES: without the ze_bgp build tag the MRT plugin is NOT registered
// (compiled out with the BGP engine whose data it records), while the rest of the
// plugin registry is still populated. The binary symbol-drop proof (MRT +
// internal/mrt format library) is in build_tag_gate11_absent_test.go.
// PREVENTS: a regression where MRT leaks into a BGP-free build via an always-on
// import -- e.g. cmd/ze/hub re-importing internal/plugins/mrt for the reactor
// bridges instead of reading them through the registry seam.

import (
	"testing"

	pluginreg "github.com/ze-software/ze/internal/component/plugin/registry"
)

func TestBuildTag_MRT_Absent(t *testing.T) {
	if len(pluginreg.Names()) == 0 {
		t.Fatal("plugin registry empty; cannot validate mrt absence (all.go not linked)")
	}
	if pluginreg.Has("mrt") {
		t.Fatal("non-ze_bgp build: mrt plugin unexpectedly registered (not compiled out)")
	}
}
