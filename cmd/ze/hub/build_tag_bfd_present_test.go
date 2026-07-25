// Design: ai/rules/feature-gate-registration.md -- ze_bfd present (compile-out) validation
//
//go:build ze_bfd

package hub

// VALIDATES: with the ze_bfd build tag (a default ze / ze-appliance feature)
// the BFD plugin is registered in the plugin registry, so a `bfd {}` config is
// reachable and the engine publishes its Service through the bfd/api seam when
// it starts.
// PREVENTS: a regression where ze_bfd is set but BFD is not wired -- the
// generated all_ze_bfd.go blank import dropped, or the manifest tag not
// reaching the generator.

import (
	"testing"

	pluginreg "github.com/ze-software/ze/internal/component/plugin/registry"
)

func TestBuildTag_BFD_Present(t *testing.T) {
	if !pluginreg.Has("bfd") {
		t.Fatal("ze_bfd build: bfd plugin not registered")
	}
}
