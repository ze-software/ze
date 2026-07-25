// Design: ai/rules/feature-gate-registration.md -- spec-feature-gate-12 Group A absent validation
//
//go:build !ze_flowexport && !ze_ddos && !ze_anomaly && !ze_as112 && !ze_geodns && !ze_dhcpserver && !ze_pxe && !ze_trafficusage && !ze_policyroute && !ze_cos && !ze_copp && !ze_mpls

package hub

// VALIDATES: without the spec-feature-gate-12 Group A build tags (the bare
// ze_core lane) none of the gated plugins are registered, their config schemas
// are gone (each representative config block is rejected as unknown rather than
// silently accepted), and the MPLS show-forwarding RPC is absent -- while the
// rest of the plugin registry is still populated. The binary symbol-drop proof
// is in build_tag_gate12_absent_test.go.
// PREVENTS: a regression where a Group A feature leaks into a hardened build
// via an always-on import or an ungated registration/schema import.

import (
	"strings"
	"testing"

	zeconfig "github.com/ze-software/ze/internal/component/config"
	pluginreg "github.com/ze-software/ze/internal/component/plugin/registry"
	pluginserver "github.com/ze-software/ze/internal/component/plugin/server"
)

func TestBuildTag_Gate12GroupA_Absent(t *testing.T) {
	if len(pluginreg.Names()) == 0 {
		t.Fatal("plugin registry empty; cannot validate absence (all.go not linked)")
	}
	names := []string{
		"flow-export",
		"ddos-detect", "ddos-flowspec", "ddos-flowtriq", "ddos-local", "ddos-observe",
		"anomaly-detect", "anomaly-shape",
		"as112", "geodns", "dhcpserver",
		"tftpserver", "imageserver",
		"traffic-usage", "policy-routes", "cos", "copp",
	}
	for _, name := range names {
		if pluginreg.Has(name) {
			t.Errorf("bare build: plugin %q unexpectedly registered (not compiled out)", name)
		}
	}
}

// TestBuildTag_Gate12GroupA_AbsentRejectsConfig proves each gated feature's
// config schema is gone too, not just the engine: every representative snippet
// must be rejected as an unknown field. In the bare lane some snippets fail on
// their shared parent container (`service`, gone when every service provider is
// off) rather than the leaf token; both are correct rejections.
func TestBuildTag_Gate12GroupA_AbsentRejectsConfig(t *testing.T) {
	cases := map[string]string{
		"flowexport":   "flow-export {\n\tsampling {\n\t\trate 100;\n\t}\n}\n",
		"ddos":         "ddos {\n\tdetect {\n\t\tenabled true;\n\t}\n}\n",
		"anomaly":      "anomaly {\n\tdetect {\n\t\tenabled true;\n\t}\n}\n",
		"as112":        "service {\n\tas112 {\n\t\tenabled true;\n\t}\n}\n",
		"geodns":       "service {\n\tgeodns {\n\t\tenabled true;\n\t}\n}\n",
		"dhcpserver":   "service {\n\tdhcp-server {\n\t\tenabled true;\n\t}\n}\n",
		"pxe-tftp":     "service {\n\ttftp-server {\n\t\tenabled true;\n\t}\n}\n",
		"pxe-image":    "service {\n\timage-server {\n\t\tenabled true;\n\t}\n}\n",
		"trafficusage": "traffic {\n\tusage {\n\t\tenabled true;\n\t}\n}\n",
		"policyroute":  "policy {\n\troute example {\n\t}\n}\n",
		"cos":          "class-of-service {\n\tprofile basic {\n\t}\n}\n",
		"copp":         "control-plane-protection {\n\tbgp {\n\t}\n}\n",
	}
	for name, cfg := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := zeconfig.ParseTreeWithYANG(cfg, nil)
			if err == nil {
				t.Fatalf("bare build unexpectedly accepted %s config", name)
			}
			if !strings.Contains(err.Error(), "unknown") {
				t.Fatalf("%s config rejection = %v, want clean unknown-field rejection", name, err)
			}
		})
	}
}

// TestBuildTag_Gate12GroupA_MPLSAbsent covers ze_mpls, which registers an RPC
// (not a plugin-registry entry): the show-forwarding wire method must be gone.
func TestBuildTag_Gate12GroupA_MPLSAbsent(t *testing.T) {
	for _, reg := range pluginserver.AllBuiltinRPCs() {
		if reg.WireMethod == "ze-show:mpls-forwarding" {
			t.Fatal("bare build: ze-show:mpls-forwarding RPC unexpectedly registered (not compiled out)")
		}
	}
}
