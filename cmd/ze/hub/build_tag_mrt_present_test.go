// Design: ai/rules/plugins.md -- MRT present in a ze_bgp build (compile-out) validation
//
//go:build ze_bgp

package hub

// VALIDATES: MRT (RFC 6396 routing-information export) is a BGP plugin gated
// under ze_bgp with no tag of its own, so a ze_bgp build registers it: an mrt{}
// config is reachable and the reactor bridges are wired via the registry seam
// (registry.SetMRTMessageCallback / SetMRTPeerCallback).
// PREVENTS: a regression where ze_bgp is set but MRT is not wired -- its blank
// import dropped from the generated all_ze_bgp.go, or its feature-gates.txt line
// not reaching the generator.

import (
	"testing"

	pluginreg "github.com/ze-software/ze/internal/component/plugin/registry"
)

func TestBuildTag_MRT_Present(t *testing.T) {
	if !pluginreg.Has("mrt") {
		t.Fatal("ze_bgp build: mrt plugin not registered")
	}
}
