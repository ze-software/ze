//go:build ze_bgp

// Design: docs/architecture/exabgp-bridge.md -- the ExaBGP JSON a process receives
// RFC: rfc/short/rfc7911.md -- RFC7911-3-1, the Path Identifier ahead of a VPN NLRI

package bridge

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// TestAddPathVPNAnnounceRenders renders an ipv4 mpls-vpn MP_REACH_NLRI whose
// NLRI carries a Path Identifier, under a session that negotiated ADD-PATH for
// the family, as the ExaBGP JSON a process receives.
//
// VALIDATES: the announce member for "ipv4 mpls-vpn" names 10.0.0.0/24 with
// its route distinguisher and path identifier.
// PREVENTS: a well-formed ADD-PATH VPN announce rendering "ipv4 mpls-vpn":{}.
// The old-commands sweep saw that output, but from malformed input whose NLRI
// did not split, not from an UPDATE like this one. That split failure is
// TestSplitErrorFamilyIsNotRendered.
func TestAddPathVPNAnnounceRenders(t *testing.T) {
	t.Parallel()

	vpn := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIVPN}
	caps := &capability.EncodingCaps{
		ASN4:        true,
		Families:    []capability.Family{vpn},
		AddPathMode: map[capability.Family]capability.AddPathMode{vpn: capability.AddPathBoth},
	}
	facts := SessionFacts{
		Local: oldLocalAddress, Peer: oldLocalAddress,
		LocalAS: 65533, PeerAS: 65533, RouterID: 0x0a000002,
		Encoding: caps,
	}

	// RFC 7911 Section 3: Path Identifier 1, then RFC 4364 Section 4.3.4:
	// Length 112, label 100 with S set, RD 0:65001:100, prefix 10.0.0.0/24.
	nlri := []byte{0, 0, 0, 1, 112, 0x00, 0x06, 0x41, 0, 0, 0xFD, 0xE9, 0, 0, 0, 100, 10, 0, 0}
	// AFI 1, SAFI 128, next hop length 12: RD zero then 192.0.2.1, reserved.
	reach := []byte{0, 1, 128, 12, 0, 0, 0, 0, 0, 0, 0, 0, 192, 0, 2, 1, 0}
	reach = append(reach, nlri...)
	attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 6, 2, 1, 0, 0, 0xFD, 0xE9}
	attrs = append(attrs, 0x80, 14, byte(len(reach)))
	attrs = append(attrs, reach...)
	body := append([]byte{0, 0, 0, byte(len(attrs))}, attrs...)

	document, err := WireUpdateToExabgpJSON(body, facts, rpc.DirectionReceived)
	if err != nil {
		t.Fatalf("render: %v", err)
	}
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	text := string(encoded)
	if strings.Contains(text, `"ipv4 mpls-vpn":{}`) {
		t.Fatalf("the VPN announce rendered empty:\n%s", text)
	}
	for _, want := range []string{`"ipv4 mpls-vpn"`, `10.0.0.0/24`, `"path-information":"0.0.0.1"`, `"rd":"65001:100"`} {
		if !strings.Contains(text, want) {
			t.Fatalf("the document lacks %s:\n%s", want, text)
		}
	}
}

// TestSplitErrorFamilyIsNotRendered renders an UPDATE whose ipv6 unicast
// MP_REACH_NLRI, then one whose MP_UNREACH_NLRI, names a 64-bit prefix and
// carries two of its octets, so the field does not split into routes and the
// ze event carries the error member in place of an nlri list.
//
// Each UPDATE carries one of the two attributes, so each case isolates one
// error arm. RFC 7606 Section 5.1 binds both ends of the session. The sender:
// "An UPDATE message MUST NOT contain more than one of the following:
// non-empty Withdrawn Routes field, non-empty Network Layer Reachability
// Information field, MP_REACH_NLRI attribute, and MP_UNREACH_NLRI attribute."
// The receiver: "Since older BGP speakers may not implement these
// restrictions, an implementation MUST still be prepared to receive these
// fields in any position or combination." Ze meets the receive half by
// stating each family once with every operation under it
// (TestOneFamilyKeyPerUpdate in internal/component/bgp/format).
//
// VALIDATES: the ExaBGP JSON names no "ipv6 unicast" family under announce or
// withdraw: ExaBGP never hands a script an NLRI it could not parse.
// PREVENTS: convertUpdateIPC2 creating the announce family before reading any
// route, so the error operation rendered "ipv6 unicast":{}, a family with no
// next hop and no route.
func TestSplitErrorFamilyIsNotRendered(t *testing.T) {
	t.Parallel()

	facts := SessionFacts{
		Local: oldLocalAddress, Peer: oldLocalAddress,
		LocalAS: 65533, PeerAS: 65533, RouterID: 0x0a000002,
		Encoding: &capability.EncodingCaps{
			ASN4:     true,
			Families: []capability.Family{{AFI: family.AFIIPv6, SAFI: family.SAFIUnicast}},
		},
	}
	cases := []struct {
		name  string
		code  byte
		value []byte
	}{
		// AFI 2, SAFI 1, next hop length 16 (2001:db8::1), reserved, then
		// NLRI [64][FD AB]: a length the two octets cannot hold.
		{"mp_reach", 14, []byte{0, 2, 1, 16, 0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1, 0, 64, 0xfd, 0xab}},
		// AFI 2, SAFI 1, withdrawn [64][FD AB].
		{"mp_unreach", 15, []byte{0, 2, 1, 64, 0xfd, 0xab}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			attrs := []byte{0x40, 1, 1, 0, 0x40, 2, 0, 0x80, tc.code, byte(len(tc.value))}
			attrs = append(attrs, tc.value...)
			body := append([]byte{0, 0, 0, byte(len(attrs))}, attrs...)

			document, err := WireUpdateToExabgpJSON(body, facts, rpc.DirectionReceived)
			if err != nil {
				t.Fatalf("render: %v", err)
			}
			encoded, err := json.Marshal(document)
			if err != nil {
				t.Fatalf("encode: %v", err)
			}
			if text := string(encoded); strings.Contains(text, `"ipv6 unicast"`) {
				t.Fatalf("a family whose NLRI did not split reached the script:\n%s", text)
			}
		})
	}
}
