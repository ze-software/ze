// Design: ai/rules/feature-gate-registration.md -- ze_ike present (compile-out) validation
//
//go:build ze_ike

package hub

// VALIDATES: with the ze_ike build tag (a default ze / ze-appliance feature)
// the IKE engine plugin is registered, so a `vpn { ipsec {} }` config is
// reachable and the hub's register_ike.go blank imports fired.
// PREVENTS: a regression where ze_ike is set but IKE is not wired -- the
// generated all_ze_ike.go group file or the hub register_ike.go dropped.

import (
	"testing"

	pluginreg "github.com/ze-software/ze/internal/component/plugin/registry"
)

func TestBuildTag_IKE_Present(t *testing.T) {
	if !pluginreg.Has("ike") {
		t.Fatal("ze_ike build: ike plugin not registered")
	}
}
