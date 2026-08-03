// Design: ai/rules/plugins.md -- ze_gnmi present build validation
//
//go:build ze_gnmi

package hub

// VALIDATES: with the ze_gnmi build tag (the default ze / ze-appliance feature
// set), the gNMI compile-out seam is installed.
// PREVENTS: a regression where ze_gnmi is set but gNMI is not wired through the
// dedicated build/reload seam.

import "testing"

func TestBuildTag_GNMI_Present(t *testing.T) {
	if gnmiBuild == nil || gnmiReloadNotify == nil {
		t.Fatal("ze_gnmi build: gNMI seam not installed (gnmiBuild/gnmiReloadNotify)")
	}
}
