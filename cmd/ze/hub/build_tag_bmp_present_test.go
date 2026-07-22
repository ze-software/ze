// Design: ai/rules/feature-gate-registration.md -- ze_bmp present (compile-out) validation
//
//go:build ze_bmp && ze_bgp

package hub

// VALIDATES: with BOTH ze_bmp and ze_bgp (BMP is a DEPENDENT gate -- it monitors
// the BGP RIB and imports the engine, so it exists only in a ze_bgp build) the
// bgp-bmp plugin is registered. The && matches all_ze_bmp.go's own guard
// (//go:build ze_bgp && ze_bmp): a ze_bmp-only build never links BMP, so the
// present assertion must not run there.
// PREVENTS: a regression where ze_bmp is set but BMP is not wired (blank import
// dropped from all_ze_bmp.go, or the manifest tag not reaching the generator).

import (
	"testing"

	pluginreg "codeberg.org/thomas-mangin/ze/internal/component/plugin/registry"
)

func TestBuildTag_BMP_Present(t *testing.T) {
	if !pluginreg.Has("bgp-bmp") {
		t.Fatal("ze_bmp && ze_bgp build: bgp-bmp plugin not registered")
	}
}
