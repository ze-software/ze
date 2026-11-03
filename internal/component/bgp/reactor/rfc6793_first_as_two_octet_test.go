// Design: docs/architecture/edge-cases/as4.md — the first-AS check on a two-octet session
// Related: session_validation.go — firstASMismatch, firstASOnWire
// Related: session_validation_receive.go — prepareRFC7606FirstAS

package reactor

import (
	"bytes"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
)

// TestFirstASTwoOctetASTransPeer checks the first-AS rule on a two-octet
// session whose peer is configured as AS_TRANS, the way Ze peers with a
// four-octet AS it does not negotiate ASN4 with.
//
// Method: the peer sends AS_PATH [AS_TRANS 65010] with AS4_PATH
// [4200000000 65010], as a NEW speaker in old mode does (RFC 6793 Section
// 4.2.2). Ze cannot name 4200000000 in its config, so the reconstructed
// first AS can never equal the configured AS_TRANS; RFC 4271 Section 6.3
// compares "the leftmost ... AS in the AS_PATH attribute", which is AS_TRANS.
// The route must stay announced with the reconstructed path, on both the
// IPv4 body (the early check) and MP_REACH_NLRI (the late check). A wire
// AS_PATH that starts with another AS is withdrawn, so the check still runs.
func TestFirstASTwoOctetASTransPeer(t *testing.T) {
	const peerReal = uint32(4200000000)
	ipv6 := &capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast}
	prefix6 := []byte{32, 0x20, 0x01, 0x0d, 0xb8}
	reach := append([]byte{0, 2, 1, 16}, make([]byte, 16)...)
	reach[19] = 1
	reach = append(reach, 0)
	reach = append(reach, prefix6...)
	prefix4 := []byte{24, 203, 0, 113}

	for _, family := range []string{"ipv4-body", "ipv6-mp"} {
		for _, wireFirst := range []uint32{attribute.ASTrans, 65003} {
			name := family + "/as-trans"
			if wireFirst != attribute.ASTrans {
				name = family + "/other"
			}
			t.Run(name, func(t *testing.T) {
				settings := NewPeerSettings(netip.MustParseAddr("192.0.2.1"), 65001, attribute.ASTrans, 0x01020301)
				var caps []capability.Capability
				if family == "ipv6-mp" {
					caps = []capability.Capability{ipv6}
					settings.Capabilities = caps
				}
				s, client := firstASSession(t, settings, caps, nil)
				if s.Negotiated().ASN4 {
					t.Fatal("the session negotiated ASN4, want a two-octet session")
				}
				attrs := firstASAttrs(2, wireFirst, 65010)
				attrs = append(attrs, collapseAttr(0xC0, byte(attribute.AttrAS4Path),
					collapseASPathValue(4, peerReal, 65010))...)
				var nlri []byte
				if family == "ipv6-mp" {
					attrs = append(attrs, collapseAttr(0x80, byte(attribute.AttrMPReachNLRI), reach)...)
				} else {
					nlri = prefix4
				}
				wu := firstASReceive(t, s, client, makeUpdateBody(nil, attrs, nlri))

				if wireFirst != attribute.ASTrans {
					want := makeUpdateBody(prefix4, nil, nil)
					if family == "ipv6-mp" {
						unreach := append([]byte{0, 2, 1}, prefix6...)
						want = makeUpdateBody(nil, collapseAttr(0x80, byte(attribute.AttrMPUnreachNLRI), unreach), nil)
					}
					if !bytes.Equal(wu.Payload(), want) {
						t.Fatalf("wire first AS %d delivered %x, want withdrawal %x", wireFirst, wu.Payload(), want)
					}
					return
				}
				if family == "ipv6-mp" {
					mp, err := wu.MPReach()
					if err != nil || !bytes.Equal(mp.NLRIBytes(), prefix6) {
						t.Fatalf("AS_TRANS first AS lost MP reachability: %x, %v", wu.Payload(), err)
					}
				} else {
					got, err := wu.NLRI()
					if err != nil || !bytes.Equal(got, prefix4) {
						t.Fatalf("AS_TRANS first AS lost the route: %x, %v", wu.Payload(), err)
					}
				}
				path, present := collapseAttrValue(t, wu, attribute.AttrASPath)
				if !present || !bytes.Equal(path, collapseASPathValue(4, peerReal, 65010)) {
					t.Fatalf("canonical path is %x, want the reconstructed [%d 65010]", path, peerReal)
				}
			})
		}
	}
}
