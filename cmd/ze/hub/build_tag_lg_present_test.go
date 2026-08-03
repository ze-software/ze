// Design: ai/rules/plugins.md -- ze_lg present build validation
//
//go:build ze_lg

package hub

// VALIDATES: with the ze_lg build tag (the default ze / ze-appliance feature
// set), the looking-glass service factory is registered in the construction
// registry.
// PREVENTS: a regression where ze_lg is set but lg is not wired (e.g. the
// register_lg.go init() is dropped or the tag stops reaching the generator).

import "testing"

func TestBuildTag_LG_Present(t *testing.T) {
	if !registeredServiceName("looking-glass") {
		t.Fatal("ze_lg build: looking-glass service factory not registered")
	}
}
