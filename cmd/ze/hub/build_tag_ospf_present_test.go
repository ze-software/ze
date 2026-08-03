// Design: ai/rules/plugins.md -- ze_ospf present build validation
//
//go:build ze_ospf

package hub

// VALIDATES: with the ze_ospf build tag (a default ze / ze-appliance feature),
// the OSPF plugin is registered in the plugin registry.
// PREVENTS: a regression where ze_ospf is set but ospf is not wired (the
// generated all_ze_ospf.go blank import or the dispatch_ospf.go companion is
// dropped, or the tag stops reaching the generator).

import (
	"testing"

	pluginreg "github.com/ze-software/ze/internal/component/plugin/registry"
)

func TestBuildTag_OSPF_Present(t *testing.T) {
	if !pluginreg.Has("ospf") {
		t.Fatal("ze_ospf build: ospf plugin not registered")
	}
}
