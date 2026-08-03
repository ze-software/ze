// Design: ai/rules/plugins.md -- ze_mrt present (compile-out) validation
//
//go:build ze_mrt

package hub

// VALIDATES: with the ze_mrt build tag (a default ze / ze-appliance feature) the
// MRT plugin (RFC 6396 routing-information export) is registered in the plugin
// registry, so an mrt{} config is reachable and the reactor bridges are wired via
// the registry seam (registry.SetMRTMessageCallback / SetMRTPeerCallback).
// PREVENTS: a regression where ze_mrt is set but MRT is not wired -- the generated
// all_ze_mrt.go blank import dropped, or the manifest tag not reaching the generator.

import (
	"testing"

	pluginreg "github.com/ze-software/ze/internal/component/plugin/registry"
)

func TestBuildTag_MRT_Present(t *testing.T) {
	if !pluginreg.Has("mrt") {
		t.Fatal("ze_mrt build: mrt plugin not registered")
	}
}
