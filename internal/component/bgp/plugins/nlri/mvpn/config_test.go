package mvpn

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestMVPNConfigRouteParserRegistered verifies the MVPN plugin registers a config
// route parser for both MVPN families.
//
// VALIDATES: registry.ConfigRouteParserByFamily returns the MVPN parser.
// PREVENTS: MVPN config routes silently failing after the migration off the
// hardcoded central switch.
func TestMVPNConfigRouteParserRegistered(t *testing.T) {
	for _, fam := range []string{"ipv4/mvpn", "ipv6/mvpn"} {
		if registry.ConfigRouteParserByFamily(fam) == nil {
			t.Errorf("no config route parser registered for %s", fam)
		}
	}
}

// TestParseConfigRoute_MVPN verifies the MVPN config parser builds the RFC 6514
// MCAST-VPN NLRI from the content tokens and marks the route groupable.
//
// VALIDATES: NLRI byte layout for a shared-join (type 6) route, and that ORIGIN /
// NEXT_HOP / EXT_COMMUNITIES are assembled with Group=true.
// PREVENTS: MVPN wire regression vs the old BuildMVPN / grouping path.
func TestParseConfigRoute_MVPN(t *testing.T) {
	req := registry.ConfigRouteRequest{
		Content:      strings.Fields("shared-join rp 10.99.199.1 group 239.251.255.228 rd 65000:99999 source-as 65000"),
		NextHop:      "10.10.6.3",
		ExtCommunity: []byte{0x01, 0x02, 0xC0, 0xA8, 0x5E, 0x0C, 0x00, 0x05},
	}
	pr, err := parseConfigRoute(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !pr.Group {
		t.Error("MVPN route must set Group=true for UPDATE packing")
	}

	// NLRI: 06 16 RD(0000FDE80001869F) SourceAS(0000FDE8) 20 0A63C701 20 EFFBFFE4
	want := "06160000fde80001869f0000fde8200a63c70120effbffe4"
	if got := hex.EncodeToString(pr.NLRI); got != want {
		t.Errorf("NLRI = %s, want %s", got, want)
	}

	got := map[uint8]bool{}
	for _, a := range pr.Attrs {
		got[a.Code] = true
	}
	for _, code := range []uint8{attrCodeOrigin, attrCodeNextHop, attrCodeExtComm} {
		if !got[code] {
			t.Errorf("missing attribute code %d", code)
		}
	}
}

// TestParseConfigRoute_MVPNInvalidInput verifies the parser is uniformly
// fail-loud: a malformed source-as, rd, source, or group is rejected rather than
// silently defaulting (regression for the source-as error-drop found in /ze-review).
//
// VALIDATES: every per-field parse error propagates out of parseConfigRoute.
// PREVENTS: a typo'd MVPN field producing a wrong-but-sent NLRI.
func TestParseConfigRoute_MVPNInvalidInput(t *testing.T) {
	base := "shared-join rp 10.99.199.1 group 239.251.255.228 rd 65000:99999 source-as 65000"
	cases := map[string]string{
		"bad source-as": strings.Replace(base, "source-as 65000", "source-as nope", 1),
		"bad rd":        strings.Replace(base, "rd 65000:99999", "rd not-an-rd", 1),
		"bad source":    strings.Replace(base, "rp 10.99.199.1", "rp not-an-ip", 1),
		"bad group":     strings.Replace(base, "group 239.251.255.228", "group not-an-ip", 1),
		"missing rd":    "shared-join rp 10.99.199.1 group 239.251.255.228 source-as 65000",
	}
	for name, content := range cases {
		if _, err := parseConfigRoute(registry.ConfigRouteRequest{Content: strings.Fields(content)}); err == nil {
			t.Errorf("%s: expected error, got nil", name)
		}
	}
}

// TestMVPNConfigRejectsUnknownAndDanglingToken verifies MVPN config route content
// rejects unrecognized keys and keys missing a value.
//
// VALIDATES: MVPN config parser uses strict key/value token handling for family
// fields.
// PREVENTS: static MVPN config silently dropping an operator typo before sending
// a different route.
func TestMVPNConfigRejectsUnknownAndDanglingToken(t *testing.T) {
	cases := map[string]struct {
		content string
		want    string
	}{
		"unknown": {
			content: "shared-join rp 10.99.199.1 group 239.251.255.228 rd 65000:99999 source-as 65000 bogus value",
			want:    "bogus",
		},
		"dangling": {
			content: "shared-join rp 10.99.199.1 group",
			want:    "group",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := parseConfigRoute(registry.ConfigRouteRequest{Content: strings.Fields(tc.content)})
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("parseConfigRoute error = %v, want token %q", err, tc.want)
			}
		})
	}
}

// TestRDStringToBytes verifies RFC 4364 Route Distinguisher string encoding.
//
// VALIDATES: Type 0 (ASN<=65535:NN) and Type 1 (IP:NN) wire forms.
// PREVENTS: Wrong RD bytes in the MVPN NLRI.
func TestRDStringToBytes(t *testing.T) {
	tests := []struct {
		in   string
		want string
	}{
		{"65000:99999", "0000fde80001869f"},       // Type 0
		{"192.168.201.1:123", "0001c0a8c901007b"}, // Type 1
	}
	for _, tt := range tests {
		rd, err := rdStringToBytes(tt.in)
		if err != nil {
			t.Fatalf("rdStringToBytes(%q): %v", tt.in, err)
		}
		if got := hex.EncodeToString(rd[:]); got != tt.want {
			t.Errorf("rd(%q) = %s, want %s", tt.in, got, tt.want)
		}
	}
}
