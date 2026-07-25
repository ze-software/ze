//go:build !ze_distro

package yang

import (
	"testing"

	configyang "github.com/ze-software/ze/internal/component/config/yang"
)

func TestMinimalBuildRegistersUpdateSchema(t *testing.T) {
	// VALIDATES: minimal (no-tag) build still registers the shared update command tree.
	// PREVENTS: gating firmware implementation code from removing update bgp peer prefix.
	loader := configyang.NewLoader()
	if err := loader.LoadEmbedded(); err != nil {
		t.Fatalf("LoadEmbedded() error = %v", err)
	}
	if err := loader.LoadRegistered(); err != nil {
		t.Fatalf("LoadRegistered() error = %v", err)
	}
	if got := loader.GetModule("ze-cli-update-api"); got == nil {
		t.Fatal("ze-cli-update-api module not registered")
	}
	if got := loader.GetModule("ze-cli-update-cmd"); got == nil {
		t.Fatal("ze-cli-update-cmd module not registered")
	}
}
