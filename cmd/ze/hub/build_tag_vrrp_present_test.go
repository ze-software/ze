// Design: ai/rules/feature-gate-registration.md -- ze_vrrp present build validation
//
//go:build ze_vrrp

package hub

// VALIDATES: with the ze_vrrp build tag (a default ze / ze-appliance feature),
// the VRRP plugin is registered in the plugin registry.
// PREVENTS: a regression where ze_vrrp is set but vrrp is not wired (the
// generated all_ze_vrrp.go blank import is dropped, or the tag stops reaching
// the generator).

import (
	"testing"

	pluginreg "github.com/ze-software/ze/internal/component/plugin/registry"
)

func TestBuildTag_VRRP_Present(t *testing.T) {
	if !pluginreg.Has("vrrp") {
		t.Fatal("ze_vrrp build: vrrp plugin not registered")
	}
}
