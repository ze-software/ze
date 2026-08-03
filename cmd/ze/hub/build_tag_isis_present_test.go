// Design: ai/rules/plugins.md -- ze_isis present build validation
//
//go:build ze_isis

package hub

// VALIDATES: with the ze_isis build tag (a default ze / ze-appliance feature),
// the IS-IS plugin is registered in the plugin registry.
// PREVENTS: a regression where ze_isis is set but isis is not wired (the
// generated all_ze_isis.go blank import or the dispatch_isis.go companion is
// dropped, or the tag stops reaching the generator).

import (
	"testing"

	pluginreg "github.com/ze-software/ze/internal/component/plugin/registry"
)

func TestBuildTag_ISIS_Present(t *testing.T) {
	if !pluginreg.Has("isis") {
		t.Fatal("ze_isis build: isis plugin not registered")
	}
}
