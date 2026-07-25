package mup

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestMUPConfigRouteParserRegistered verifies the MUP plugin registers a config
// route parser for both MUP families.
//
// VALIDATES: registry.ConfigRouteParserByFamily returns the MUP parser.
// PREVENTS: MUP config routes silently failing because the generic dispatch
// cannot find a parser after the migration off the hardcoded switch.
func TestMUPConfigRouteParserRegistered(t *testing.T) {
	for _, fam := range []string{"ipv4/mup", "ipv6/mup"} {
		if registry.ConfigRouteParserByFamily(fam) == nil {
			t.Errorf("no config route parser registered for %s", fam)
		}
	}
}

// TestParseConfigRoute_T1STSource verifies T1ST source-field NLRI encoding via the
// plugin config route parser (moved from config/loader_test.go).
//
// VALIDATES: MUP T1ST routes correctly encode the optional source field
// (source_len + source_addr) and fail loudly on invalid input.
// PREVENTS: Silent failures / missing source encoding in NLRI output.
func TestParseConfigRoute_T1STSource(t *testing.T) {
	tests := []struct {
		name        string
		content     string
		isIPv6      bool
		wantHex     string // expected lowercase hex substring in NLRI
		wantErr     bool
		wantErrText string
	}{
		{
			name:    "IPv4 T1ST with source",
			content: "mup-t1st 192.168.0.2/32 rd 100:100 teid 12345 qfi 9 endpoint 10.0.0.1 source 10.0.1.1",
			wantHex: "200a000101", // source: len=32 (0x20), addr=10.0.1.1
		},
		{
			name:    "IPv6 T1ST with source",
			content: "mup-t1st 2001:db8:1:1::2/128 rd 100:100 teid 12345 qfi 9 endpoint 2001::1 source 2002::2",
			isIPv6:  true,
			wantHex: "8020020000000000000000000000000002", // source: len=128 (0x80), addr=2002::2
		},
		{
			name:    "T1ST without source (optional)",
			content: "mup-t1st 192.168.0.2/32 rd 100:100 teid 12345 qfi 9 endpoint 10.0.0.1",
			wantHex: "200a000001", // endpoint last: len=32, addr=10.0.0.1, no source after
		},
		{
			name:        "T1ST with invalid source fails loudly",
			content:     "mup-t1st 192.168.0.2/32 rd 100:100 teid 12345 qfi 9 endpoint 10.0.0.1 source not-an-ip",
			wantErr:     true,
			wantErrText: "invalid T1ST source",
		},
		{
			name:        "T1ST with invalid endpoint fails loudly",
			content:     "mup-t1st 192.168.0.2/32 rd 100:100 teid 12345 qfi 9 endpoint bad-endpoint source 10.0.1.1",
			wantErr:     true,
			wantErrText: "invalid T1ST endpoint",
		},
		{
			name:        "T1ST with invalid prefix fails loudly",
			content:     "mup-t1st not-a-prefix rd 100:100",
			wantErr:     true,
			wantErrText: "invalid T1ST prefix",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := registry.ConfigRouteRequest{
				Content: strings.Fields(tt.content),
				IsIPv6:  tt.isIPv6,
			}
			pr, err := parseConfigRoute(req)

			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error containing %q, got nil", tt.wantErrText)
				}
				if !strings.Contains(err.Error(), tt.wantErrText) {
					t.Errorf("error %q does not contain %q", err.Error(), tt.wantErrText)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			nlriHex := strings.ToLower(hex.EncodeToString(pr.NLRI))
			if !strings.Contains(nlriHex, tt.wantHex) {
				t.Errorf("NLRI should contain %s, got %s", tt.wantHex, nlriHex)
			}
		})
	}
}

// TestParseConfigRoute_Attrs verifies the MUP config parser assembles the
// MUP-specific path attributes from the pre-parsed attribute block.
//
// VALIDATES: IPv4 MUP with an IPv4 next-hop emits NEXT_HOP (code 3); extended
// communities (code 16) and Prefix-SID (code 40) are carried through.
// PREVENTS: Wire regression vs the old BuildMUP path (NEXT_HOP code-3 quirk).
func TestParseConfigRoute_Attrs(t *testing.T) {
	req := registry.ConfigRouteRequest{
		Content:      strings.Fields("mup-isd 10.0.1.0/24 rd 100:100"),
		NextHop:      "10.0.0.2",
		ExtCommunity: []byte{0x00, 0x02, 0x00, 0x0a, 0x00, 0x00, 0x00, 0x0a},
		PrefixSID:    []byte{0x05, 0x00, 0x01, 0x00},
	}
	pr, err := parseConfigRoute(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got := map[uint8]bool{}
	for _, a := range pr.Attrs {
		got[a.Code] = true
	}
	for _, code := range []uint8{attrCodeNextHop, attrCodeExtComm, attrCodePrefixSID} {
		if !got[code] {
			t.Errorf("missing attribute code %d", code)
		}
	}

	// IPv6 MUP must NOT emit a legacy NEXT_HOP (code 3).
	req6 := registry.ConfigRouteRequest{
		Content: strings.Fields("mup-isd 2001::/64 rd 100:100"),
		NextHop: "2001::1",
		IsIPv6:  true,
	}
	pr6, err := parseConfigRoute(req6)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, a := range pr6.Attrs {
		if a.Code == attrCodeNextHop {
			t.Error("IPv6 MUP should not emit NEXT_HOP code-3")
		}
	}
}

// TestEncodeMUPRejectsUnknownAndDanglingToken verifies the in-process MUP NLRI
// encoder rejects unrecognized keys and keys missing a value.
//
// VALIDATES: MUP encode-nlri input is exact-or-reject for unknown and dangling
// tokens.
// PREVENTS: RPC/update-text MUP NLRI encoding silently dropping operator typos.
func TestEncodeMUPRejectsUnknownAndDanglingToken(t *testing.T) {
	cases := map[string]struct {
		args []string
		want string
	}{
		"unknown": {
			args: strings.Fields("route-type mup-isd rd 100:100 prefix 10.0.0.0/24 bogus value"),
			want: "bogus",
		},
		"dangling": {
			args: strings.Fields("route-type mup-isd rd 100:100 prefix 10.0.0.0/24 endpoint"),
			want: "endpoint",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := EncodeNLRIHex("ipv4/mup", tc.args)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("EncodeNLRIHex error = %v, want token %q", err, tc.want)
			}
		})
	}
}

// TestMUPConfigRejectsUnknownAndDanglingToken verifies the config route parser
// applies the same strict token-pair semantics before building NLRI bytes.
//
// VALIDATES: MUP config route content rejects unknown and dangling family tokens.
// PREVENTS: static MUP config silently advertising a route after dropping a typo.
func TestMUPConfigRejectsUnknownAndDanglingToken(t *testing.T) {
	cases := map[string]struct {
		content string
		want    string
	}{
		"unknown":  {content: "mup-isd 10.0.0.0/24 rd 100:100 bogus value", want: "bogus"},
		"dangling": {content: "mup-isd 10.0.0.0/24 rd", want: "rd"},
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
