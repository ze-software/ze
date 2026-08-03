// Design: ai/rules/plugins.md -- ze_vrrp absent (compile-out) validation
//
//go:build !ze_vrrp

package hub

// VALIDATES: without the ze_vrrp build tag (e.g. ze-stripped or a bare ze_core
// build), the VRRP plugin is NOT registered and its config schema (the vrrp
// augment under interface units) is absent, while the rest of the plugin
// registry is still populated. The binary symbol-drop proof is in
// build_tag_protocols_absent_test.go.
// PREVENTS: a regression where vrrp leaks into a hardened build via an always-on
// import or an ungated registration/schema import.

import (
	"strings"
	"testing"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	pluginreg "github.com/ze-software/ze/internal/component/plugin/registry"
)

func TestBuildTag_VRRP_Absent(t *testing.T) {
	if len(pluginreg.Names()) == 0 {
		t.Fatal("plugin registry empty; cannot validate vrrp absence (all.go not linked)")
	}
	if pluginreg.Has("vrrp") {
		t.Fatal("non-ze_vrrp build: vrrp plugin unexpectedly registered (not compiled out)")
	}
}

// TestBuildTag_VRRP_AbsentRejectsVRRPConfig proves the vrrp config schema is gone
// too, not just the engine. VRRP config is a YANG augment under interface units
// (/iface:interface/.../ipv4), so the snippet is a minimal-but-valid interface
// tree (proven by test/vrrp/vrrp-doctor-quiet.ci when vrrp is present) whose only
// token the gated-out schema removes is `vrrp`. A bare build must reject it as an
// unknown field rather than silently accept or crash.
func TestBuildTag_VRRP_AbsentRejectsVRRPConfig(t *testing.T) {
	const cfg = `interface {
	backend netlink;
	ethernet eth0 {
		unit 0 {
			ipv4 {
				address [ 192.0.2.251/24 ];
				vrrp {
					group uplink {
						vrid 10;
						virtual-address [ 192.0.2.1 ];
					}
				}
			}
		}
	}
}
`
	_, err := zeconfig.ParseTreeWithYANG(cfg, nil)
	if err == nil {
		t.Fatal("non-ze_vrrp build unexpectedly accepted vrrp config")
	}
	if !strings.Contains(err.Error(), "unknown") {
		t.Fatalf("vrrp config rejection = %v, want clean unknown-field rejection", err)
	}
}
