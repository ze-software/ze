// Design: ai/rules/feature-gate-registration.md -- ze_bmp absent (compile-out) validation
//
//go:build !ze_bmp

package hub

// VALIDATES: without the ze_bmp build tag the bgp-bmp plugin is NOT registered
// (compiled out), while the rest of the plugin registry -- including the rest of
// the BGP subsystem when ze_bgp is on -- is still populated. The binary
// symbol-drop proof is in build_tag_gate11_absent_test.go.
// PREVENTS: a regression where BMP leaks into a build with ze_bmp off via an
// always-on or same-ze_bgp import path (BMP must be droppable independently of
// the rest of the BGP engine).

import (
	"testing"

	pluginreg "github.com/ze-software/ze/internal/component/plugin/registry"
)

func TestBuildTag_BMP_Absent(t *testing.T) {
	if len(pluginreg.Names()) == 0 {
		t.Fatal("plugin registry empty; cannot validate bmp absence (all.go not linked)")
	}
	if pluginreg.Has("bgp-bmp") {
		t.Fatal("non-ze_bmp build: bgp-bmp plugin unexpectedly registered (not compiled out)")
	}
}
