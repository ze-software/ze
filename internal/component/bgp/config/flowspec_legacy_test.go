package bgpconfig

import (
	"bytes"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestFlowSpecConfigToPlugin verifies that the legacy flow{} reader reconstructs
// NLRI content tokens faithfully: routing a flow{} route config through the
// flowspec plugin parser produces the same NLRI and attributes as the native
// update{} form, including a bracketed multi-value criterion.
//
// VALIDATES: flowSpecConfigToPlugin token reconstruction (the legacy
// flow{ route{ match{} then{} } } form has no .ci coverage).
// PREVENTS: silent wrong NLRI for legacy flow{} configs after the migration
// rerouted them through the plugin (found in /ze-review).
func TestFlowSpecConfigToPlugin(t *testing.T) {
	fr := FlowSpecRouteConfig{
		IsIPv6: false,
		NLRI: map[string][]string{
			"destination": {"192.168.0.1/32"},
			"protocol":    {"=tcp", "=udp"},
		},
		ExtendedCommunity: "discard",
	}
	prc, err := flowSpecConfigToPlugin(fr)
	if err != nil {
		t.Fatalf("flowSpecConfigToPlugin: %v", err)
	}

	// Native update{} equivalent through the same plugin parser.
	parser := registry.ConfigRouteParserByFamily("ipv4/flow")
	if parser == nil {
		t.Fatal("flowspec config route parser not registered")
	}
	ec, err := ParseExtendedCommunity("discard")
	if err != nil {
		t.Fatalf("parse ext-community: %v", err)
	}
	pr, err := parser(registry.ConfigRouteRequest{
		Content:      []string{"add", "destination", "192.168.0.1/32", "protocol", "[", "=tcp", "=udp", "]"},
		ExtCommunity: ec.Bytes,
	})
	if err != nil {
		t.Fatalf("native parse: %v", err)
	}

	if !bytes.Equal(prc.NLRI, pr.NLRI) {
		t.Errorf("legacy flow{} NLRI %x != native update{} NLRI %x", prc.NLRI, pr.NLRI)
	}
	if len(prc.Attrs) != len(pr.Attrs) {
		t.Errorf("attr count: legacy %d != native %d", len(prc.Attrs), len(pr.Attrs))
	}
}

// TestFlowSpecConfigToPluginVPN verifies the VPN variant routes to the flow-vpn
// family and carries the RD in the NLRI (RFC 8955 Section 8).
func TestFlowSpecConfigToPluginVPN(t *testing.T) {
	fr := FlowSpecRouteConfig{
		IsIPv6: false,
		RD:     "65535:65536",
		NLRI:   map[string][]string{"source": {"10.0.0.1/32"}},
	}
	prc, err := flowSpecConfigToPlugin(fr)
	if err != nil {
		t.Fatalf("flowSpecConfigToPlugin VPN: %v", err)
	}
	if prc.Family != "ipv4/flow-vpn" {
		t.Errorf("family = %q, want ipv4/flow-vpn", prc.Family)
	}
	// VPN NLRI = length(1) + RD type-0 65535:65536 (0000ffff00010000) + components.
	if !bytes.Contains(prc.NLRI, []byte{0x00, 0x00, 0xff, 0xff, 0x00, 0x01, 0x00, 0x00}) {
		t.Errorf("VPN NLRI missing RD bytes: %x", prc.NLRI)
	}
}
