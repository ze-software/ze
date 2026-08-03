// Design: ai/rules/plugins.md -- ze_ntp present (compile-out) validation
//
//go:build ze_ntp

package hub

// VALIDATES: with the ze_ntp build tag (a default ze / ze-appliance feature) the
// NTP plugin is registered in the plugin registry, so an `environment { ntp {} }`
// config is reachable and `show system` can read sync state through the nil-safe
// registry.GetNTPSyncInfo seam the plugin populates at init.
// PREVENTS: a regression where ze_ntp is set but NTP is not wired -- the generated
// all_ze_ntp.go blank import dropped, or the manifest tag not reaching the generator.

import (
	"testing"

	pluginreg "github.com/ze-software/ze/internal/component/plugin/registry"
)

func TestBuildTag_NTP_Present(t *testing.T) {
	if !pluginreg.Has("ntp") {
		t.Fatal("ze_ntp build: ntp plugin not registered")
	}
}
