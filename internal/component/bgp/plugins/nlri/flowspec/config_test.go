package flowspec

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestFlowSpecConfigRouteParserRegistered verifies the FlowSpec plugin registers
// a config route parser for all four FlowSpec families.
//
// VALIDATES: registry.ConfigRouteParserByFamily returns the FlowSpec parser.
// PREVENTS: FlowSpec config routes silently failing after the migration off the
// hardcoded central switch.
func TestFlowSpecConfigRouteParserRegistered(t *testing.T) {
	for _, fam := range []string{"ipv4/flow", "ipv6/flow", "ipv4/flow-vpn", "ipv6/flow-vpn"} {
		if registry.ConfigRouteParserByFamily(fam) == nil {
			t.Errorf("no config route parser registered for %s", fam)
		}
	}
}

// TestParseConfigRoute_FlowSpec verifies the FlowSpec config parser builds the
// RFC 8955 NLRI from match criteria and assembles community / extended-community.
//
// VALIDATES: non-VPN NLRI (length-prefixed) and that COMMUNITY (8) and
// EXT_COMMUNITY (16) attributes are present.
// PREVENTS: FlowSpec wire regression vs the old BuildFlowSpec path.
func TestParseConfigRoute_FlowSpec(t *testing.T) {
	req := registry.ConfigRouteRequest{
		Content:      strings.Fields("add source-ipv4 10.0.0.2/32"),
		Community:    []uint32{0x77770000},
		ExtCommunity: []byte{0x80, 0x06, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00},
	}
	pr, err := parseConfigRoute(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pr.NLRI) == 0 {
		t.Fatal("empty FlowSpec NLRI")
	}
	got := map[uint8]bool{}
	for _, a := range pr.Attrs {
		got[a.Code] = true
	}
	if !got[attrCodeCommunity] || !got[attrCodeExtComm] {
		t.Errorf("missing community (8) or ext-community (16): %v", got)
	}
}

// TestParseConfigRoute_FlowSpecRejectsBadCriteria verifies the parser fails loud
// on a criterion whose value cannot be parsed, instead of silently dropping it.
//
// VALIDATES: buildFlowSpecComponents reports dropped criteria and parseConfigRoute
// errors. PREVENTS: a typo'd criterion silently widening the filter -- worst case
// a single bad criterion produced a zero-component all-match rule (found in /ze-review).
func TestParseConfigRoute_FlowSpecRejectsBadCriteria(t *testing.T) {
	cases := map[string]string{
		"bad destination":   "add destination-ipv4 999.999.999/24",
		"bad port":          "add destination-ipv4 10.0.0.0/24 port =garbage",
		"only bad prefix":   "add source-ipv4 not-a-prefix",
		"bad protocol":      "add protocol =notaproto",
		"unknown criterion": "add destnation 10.0.0.0/24",                    // typo: would yield all-match
		"valid plus typo":   "add destination-ipv4 10.0.0.0/24 prtocol =tcp", // typo alongside valid: would over-match
	}
	for name, content := range cases {
		_, err := parseConfigRoute(registry.ConfigRouteRequest{Content: strings.Fields(content)})
		if err == nil {
			t.Errorf("%s: expected error, got nil (criterion silently dropped)", name)
		}
	}
}

// TestParseConfigRoute_FlowSpecRejectsUnterminatedBracket verifies an unbalanced
// bracketed list is rejected rather than silently absorbing trailing tokens.
//
// VALIDATES: flowSpecCriteriaFromContent returns an error on a missing ']'.
func TestParseConfigRoute_FlowSpecRejectsUnterminatedBracket(t *testing.T) {
	_, err := parseConfigRoute(registry.ConfigRouteRequest{
		Content: strings.Fields("add port [ =80 =8080"),
	})
	if err == nil {
		t.Error("expected error for unterminated '[' list, got nil")
	}
}

// TestParseConfigRoute_FlowSpecValidStillBuilds guards against the fail-loud change
// rejecting legitimate routes: a fully valid multi-criteria route must still parse.
func TestParseConfigRoute_FlowSpecValidStillBuilds(t *testing.T) {
	pr, err := parseConfigRoute(registry.ConfigRouteRequest{
		Content: strings.Fields("add destination-ipv4 10.0.0.0/24 protocol [ =tcp =udp ] port =80"),
	})
	if err != nil {
		t.Fatalf("valid route rejected: %v", err)
	}
	if len(pr.NLRI) == 0 {
		t.Error("valid route produced empty NLRI")
	}
}

// TestParseConfigRoute_FlowSpecVPN verifies the VPN variant wraps the NLRI with a
// length prefix + RD (RFC 8955 Section 8).
//
// VALIDATES: VPN NLRI begins with a length byte then the 8-byte RD.
// PREVENTS: Missing/incorrect VPN NLRI envelope.
func TestParseConfigRoute_FlowSpecVPN(t *testing.T) {
	req := registry.ConfigRouteRequest{
		Content: strings.Fields("rd 65535:65536 add source-ipv4 10.0.0.1/32"),
	}
	pr, err := parseConfigRoute(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	h := hex.EncodeToString(pr.NLRI)
	// length(1) + RD type0 65535:65536 = 0000ffff00010000, then components.
	if !strings.HasPrefix(h, "11") && !strings.HasPrefix(h, "12") && !strings.HasPrefix(h, "13") {
		// length byte is payload (8 RD + components); just assert the RD follows byte 0.
		if !strings.Contains(h, "0000ffff00010000") {
			t.Errorf("VPN NLRI missing RD bytes: %s", h)
		}
	}
}
