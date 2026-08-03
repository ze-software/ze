// Design: ai/rules/plugins.md -- ze_ike absent (compile-out) validation
//
//go:build !ze_ike

package hub

// VALIDATES: without the ze_ike build tag the IKE engine plugin is NOT
// registered and its config schema (`vpn { ipsec {} }`) is absent, while the
// rest of the plugin registry is still populated. The shared XFRM seam
// (internal/component/ike/dataplane) deliberately stays linked for OSPF's
// RFC 4552 use; the engine/crypto/eap/wire/transport symbol-drop proof is in
// build_tag_gate12_absent_test.go.
// PREVENTS: a regression where the IKE engine leaks into a hardened build via
// an always-on import (e.g. the hub blank imports returning to main.go, or the
// web VPN page losing its ze_ike constraint).

import (
	"strings"
	"testing"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	pluginreg "github.com/ze-software/ze/internal/component/plugin/registry"
)

func TestBuildTag_IKE_Absent(t *testing.T) {
	if len(pluginreg.Names()) == 0 {
		t.Fatal("plugin registry empty; cannot validate ike absence (all.go not linked)")
	}
	if pluginreg.Has("ike") {
		t.Fatal("non-ze_ike build: ike plugin unexpectedly registered (not compiled out)")
	}
}

// TestBuildTag_IKE_AbsentRejectsIPsecConfig proves the IPsec config schema is
// gone too, not just the engine. A bare build must reject a `vpn { ipsec {} }`
// block as an unknown field rather than silently accept or crash.
func TestBuildTag_IKE_AbsentRejectsIPsecConfig(t *testing.T) {
	const cfg = `vpn {
	ipsec {
	}
}
`
	_, err := zeconfig.ParseTreeWithYANG(cfg, nil)
	if err == nil {
		t.Fatal("non-ze_ike build unexpectedly accepted vpn ipsec config")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("vpn ipsec config rejection = %v, want clean unknown-field rejection", err)
	}
}
