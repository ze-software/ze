// Design: docs/architecture/update-building.md -- family-aware forwarding next hops.
package reactor

import (
	"bytes"
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/ribevents"
	"github.com/ze-software/ze/internal/core/family"
)

// RFC requirement: RFC8955-4-3 negative -- ignoring a FlowSpec next hop neither changes received observer bytes nor disables the independent legacy-unicast next-hop gate in a mixed UPDATE.
func TestFlowSpecForwardingKeepsLegacyNextHop(t *testing.T) {
	for _, refused := range []bool{false, true} {
		name := "legacy-announce"
		if refused {
			name = "legacy-withdraw"
		}
		t.Run(name, func(t *testing.T) {
			f := newAIGPReplayFixture(t, nil)
			fam := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}
			for _, peer := range []*Peer{f.source, f.destination} {
				peer.negotiated.Store(&NegotiatedCapabilities{ASN4: true, families: map[family.Family]bool{family.IPv4Unicast: true, fam: true}})
			}
			f.destination.settings.PeerAS = 65000
			f.destination.settings.NextHopMode = NextHopUnchanged
			f.destination.refreshForwardFacts()
			raw := mustHex(t, "050118c00002")
			key := ribevents.ValidationRoute{Peer: f.source.Settings().Address, Family: fam, NLRI: string(raw)}
			ribevents.RegisterFlowSpecLookup(func(got ribevents.ValidationRoute, _ uint64) bool { return got == key }, nil, nil, nil)
			t.Cleanup(func() { ribevents.RegisterFlowSpecLookup(nil, nil, nil, nil) })
			hop := netip.MustParseAddr("192.0.2.7").AsSlice()
			if refused {
				hop = f.destination.Settings().Address.AsSlice()
			}
			// The FlowSpec field names the recipient in both controls. Only the
			// genuine unicast sibling's independent field determines its gate.
			body := flowForwardPayload(t, fam, raw, f.destination.Settings().Address.AsSlice())
			body = append(body, 0x40, 3, 4)
			body = append(body, hop...)
			binary.BigEndian.PutUint16(body[2:4], uint16(len(body)-4))
			prefix := []byte{24, 203, 0, 113}
			body = append(body, prefix...)
			original := bytes.Clone(body)
			id := f.receive(t, body)
			f.forward(t, id)
			forwardSocketBarrier(t, f.r)
			bodies := aigpSocketBodies(t, f.conn)
			if len(bodies) != 2 {
				t.Fatalf("mixed recipient UPDATE count=%d, want both sections", len(bodies))
			}
			flow, legacy := false, false
			for _, delivered := range bodies {
				update, err := message.UnpackUpdate(delivered)
				if err != nil {
					t.Fatal(err)
				}
				if _, _, _, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI); found {
					assertFlowForwardWire(t, delivered, fam, raw)
					flow = true
					continue
				}
				legacy = true
				if refused {
					if !bytes.Equal(update.WithdrawnRoutes, prefix) || len(update.NLRI) != 0 {
						t.Errorf("legacy peer-own next hop escaped gate: %x", delivered)
					}
				} else {
					_, _, got, found := attribute.AttrFind(update.PathAttributes, attribute.AttrNextHop)
					if !found || !bytes.Equal(got, hop) || !bytes.Equal(update.NLRI, prefix) || len(update.WithdrawnRoutes) != 0 {
						t.Errorf("legacy next hop or NLRI changed: %x", delivered)
					}
				}
			}
			if !flow || !legacy {
				t.Error("one mixed section disappeared")
			}
			if !bytes.Equal(body, original) {
				t.Errorf("forwarding modified the received observer bytes: %x", body)
			}
		})
	}
}
