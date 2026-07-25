// Design: ai/rules/feature-gate-registration.md -- ze_ldp present build validation
//
//go:build ze_ldp

package hub

// VALIDATES: with the ze_ldp build tag (a default ze / ze-appliance feature),
// the LDP plugin is registered in the plugin registry.
// PREVENTS: a regression where ze_ldp is set but ldp is not wired (the generated
// all_ze_ldp.go blank import is dropped, or the tag stops reaching the generator).

import (
	"testing"

	pluginreg "github.com/ze-software/ze/internal/component/plugin/registry"
)

func TestBuildTag_LDP_Present(t *testing.T) {
	if !pluginreg.Has("ldp") {
		t.Fatal("ze_ldp build: ldp plugin not registered")
	}
}
