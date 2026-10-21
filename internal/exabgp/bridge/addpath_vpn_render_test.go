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
// PREVENTS: the sighting during the old-commands sweep that such an UPDATE
// rendered "ipv4 mpls-vpn":{} while the registry decoder read it.
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
