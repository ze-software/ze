package vpls

import (
	"encoding/hex"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/plugin/registry"
)

// TestVPLSConfigRouteParserRegistered verifies the VPLS plugin registers a config
// route parser for the l2vpn/vpls family.
//
// VALIDATES: registry.ConfigRouteParserByFamily returns the VPLS parser.
// PREVENTS: VPLS config routes silently failing after the migration off the
// hardcoded central switch.
func TestVPLSConfigRouteParserRegistered(t *testing.T) {
	if registry.ConfigRouteParserByFamily(familyVPLS) == nil {
		t.Fatalf("no config route parser registered for %s", familyVPLS)
	}
}

// TestParseConfigRoute_VPLS verifies the VPLS config parser builds the RFC 4761
// NLRI from "rd RD add ve-id ... ve-block-offset ... ve-block-size ... label-base ..."
// and assembles the generic path attributes.
//
// VALIDATES: NLRI byte layout (length 0x11, RD, ve-id, offset, size, label-base)
// and that ORIGIN/MED/COMMUNITY/ORIGINATOR_ID/CLUSTER_LIST/EXT_COMMUNITIES plus
// typed ASPath/LocalPreference are carried.
// PREVENTS: VPLS wire regression vs the old BuildVPLS path.
func TestParseConfigRoute_VPLS(t *testing.T) {
	req := registry.ConfigRouteRequest{
		Content:         strings.Fields("rd 192.168.201.1:123 add ve-id 5 ve-block-offset 1 ve-block-size 8 label-base 10702"),
		NextHop:         "192.168.201.1",
		MED:             2000,
		LocalPreference: 1,
		ASPath:          []uint32{30740, 30740},
		Community:       []uint32{0xD53F007B},
		OriginatorID:    0xC0A81601,
		ClusterList:     []uint32{0x03030303, 0xC0A8C901},
		ExtCommunity:    []byte{0x00, 0x02, 0xD5, 0x3F, 0x00, 0x00, 0x00, 0x06},
	}
	pr, err := parseConfigRoute(req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// NLRI: 0011 0001C0A8C901007B 0005 0001 0008 029CE1
	wantNLRI := "00110001c0a8c901007b0005000100080" + "29ce1"
	if got := hex.EncodeToString(pr.NLRI); got != strings.ReplaceAll(wantNLRI, " ", "") {
		t.Errorf("NLRI = %s, want %s", got, wantNLRI)
	}

	// ASPath / LocalPreference carried through typed.
	if len(pr.ASPath) != 2 || pr.LocalPreference != 1 {
		t.Errorf("ASPath=%v LocalPreference=%d, want [30740 30740] / 1", pr.ASPath, pr.LocalPreference)
	}

	// Generic attributes present with expected codes.
	got := map[uint8]bool{}
	for _, a := range pr.Attrs {
		got[a.Code] = true
	}
	for _, code := range []uint8{attrCodeOrigin, attrCodeMED, attrCodeCommunity, attrCodeOriginatorID, attrCodeClusterList, attrCodeExtComm} {
		if !got[code] {
			t.Errorf("missing attribute code %d", code)
		}
	}
}

// TestEncodeVPLSRejectsUnknownAndDanglingToken verifies the in-process VPLS NLRI
// encoder rejects unrecognized keys and keys missing a value.
//
// VALIDATES: VPLS encode-nlri input is exact-or-reject for unknown and dangling
// family tokens.
// PREVENTS: RPC/update-text VPLS NLRI encoding silently dropping operator typos.
func TestEncodeVPLSRejectsUnknownAndDanglingToken(t *testing.T) {
	cases := map[string]struct {
		args []string
		want string
	}{
		"unknown": {
			args: strings.Fields("rd 1:1 ve-id 1 ve-block-offset 0 ve-block-size 10 label 100 bogus value"),
			want: "bogus",
		},
		"dangling": {
			args: strings.Fields("rd 1:1 ve-id 1 ve-block-offset 0 ve-block-size 10 label"),
			want: "label",
		},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			_, err := EncodeNLRIHex(familyVPLS, tc.args)
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("EncodeNLRIHex error = %v, want token %q", err, tc.want)
			}
		})
	}
}

// TestVPLSRouteParserRejectsDanglingToken verifies the owner route-command parser
// does not ignore a final key without a value.
//
// VALIDATES: VPLS canonical route encoding parser rejects dangling route tokens.
// PREVENTS: `ze bgp encode -f l2vpn/vpls` silently dropping a final typo.
func TestVPLSRouteParserRejectsDanglingToken(t *testing.T) {
	_, err := parseVPLSArgs(strings.Fields("rd 1:1 ve-block-offset 0 ve-block-size 10 label"))
	if err == nil || !strings.Contains(err.Error(), "label") {
		t.Fatalf("parseVPLSArgs error = %v, want label", err)
	}
}
