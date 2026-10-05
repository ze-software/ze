// Design: docs/architecture/update-building.md -- empty FlowSpec next hops.
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

// These are independent wire fixtures, not output from an originating builder.
var flowForwardWireCases = []struct {
	name string
	fam  family.Family
	nlri string
}{
	{"ipv4-flow", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpec}, "050118c00002"},
	{"ipv4-flow-vpn", family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIFlowSpecVPN}, "0d0000ffff000100000118c00002"},
	{"ipv6-flow", family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIFlowSpec}, "0701200020010db8"},
	{"ipv6-flow-vpn", family.Family{AFI: family.AFIIPv6, SAFI: family.SAFIFlowSpecVPN}, "0f0000ffff0001000001200020010db8"},
}

// RFC 8955 Section 4: "When advertising Flow Specifications, the Length of the
// Next-Hop Network Address MUST be set to 0. The Network Address of the Next-Hop
// field MUST be ignored."
// RFC requirement: RFC8955-4-3 positive -- cached forwarding delivers each IPv4/IPv6 flow and flow-vpn rule with its exact NLRI, RD and traffic-rate action and an empty next hop at the recipient.
// RFC requirement: RFC8955-4-3 negative -- self/explicit destination rewrites and received nonzero next hops cannot leak a forwarding address or suppress the rule.
// RFC requirement: RFC5575-4-8 positive -- recipient wire preserves the native FlowSpec rule and action while advertising zero next-hop length.
// RFC requirement: RFC5575-4-8 negative -- received next-hop bytes have no effect on delivery, even when they name the destination or an unnegotiated IPv6 address.
// MUTATION: bypass the common FlowSpec next-hop normalization or permit its
// MP_REACH attribute handler to honor a nonempty replacement address.
func TestFlowSpecForwardingOmitsNextHop(t *testing.T) {
	for _, tc := range flowForwardWireCases {
		for _, mode := range []string{"unchanged", "self", "explicit4", "explicit6"} {
			for _, received := range []string{"empty", "peer-address", "ipv6"} {
				t.Run(tc.name+"/"+mode+"/"+received, func(t *testing.T) {
					f := newAIGPReplayFixture(t, nil)
					for _, peer := range []*Peer{f.source, f.destination} {
						peer.negotiated.Store(&NegotiatedCapabilities{ASN4: true, families: map[family.Family]bool{tc.fam: true}})
					}
					f.destination.settings.PeerAS = 65000
					flowForwardNextHopSettings(f.destination.settings, mode)
					f.destination.refreshForwardFacts()
					raw := mustHex(t, tc.nlri)
					key := ribevents.ValidationRoute{Peer: f.source.Settings().Address, Family: tc.fam, NLRI: string(raw)}
					ribevents.RegisterFlowSpecLookup(func(got ribevents.ValidationRoute, _ uint64) bool { return got == key }, nil, nil, nil)
					t.Cleanup(func() { ribevents.RegisterFlowSpecLookup(nil, nil, nil, nil) })
					var hop []byte
					switch received {
					case "peer-address":
						hop = f.destination.Settings().Address.AsSlice()
					case "ipv6":
						hop = netip.MustParseAddr("2001:db8::dead").AsSlice()
					}
					id := f.receive(t, flowForwardPayload(t, tc.fam, raw, hop))
					f.forward(t, id)
					forwardSocketBarrier(t, f.r)
					bodies := aigpSocketBodies(t, f.conn)
					if len(bodies) != 1 {
						t.Fatalf("recipient UPDATE count=%d, want one rule", len(bodies))
					}
					assertFlowForwardWire(t, bodies[0], tc.fam, raw)
				})
			}
		}
	}
}

func flowForwardNextHopSettings(settings *PeerSettings, mode string) {
	switch mode {
	case "unchanged":
		settings.NextHopMode = NextHopUnchanged
	case "self":
		settings.NextHopMode = NextHopSelf
		settings.LocalAddress = netip.MustParseAddr("192.0.2.254")
	case "explicit4":
		settings.NextHopMode = NextHopExplicit
		settings.NextHopAddress = netip.MustParseAddr("192.0.2.99")
	case "explicit6":
		settings.NextHopMode = NextHopExplicit
		settings.NextHopAddress = netip.MustParseAddr("2001:db8::99")
	}
}

func flowForwardPayload(t *testing.T, fam family.Family, raw, hop []byte) []byte {
	t.Helper()
	attrs := mustHex(t, "4001010040020602010000fdeac010088006000046160000")
	mp := []byte{byte(fam.AFI >> 8), byte(fam.AFI), byte(fam.SAFI), byte(len(hop))}
	mp = append(mp, hop...)
	mp = append(mp, 0)
	mp = append(mp, raw...)
	attrs = append(attrs, 0x80, 14, byte(len(mp)))
	attrs = append(attrs, mp...)
	body := binary.BigEndian.AppendUint16([]byte{0, 0}, uint16(len(attrs)))
	return append(body, attrs...)
}

func assertFlowForwardWire(t *testing.T, body []byte, fam family.Family, raw []byte) {
	t.Helper()
	update, err := message.UnpackUpdate(body)
	if err != nil {
		t.Fatal(err)
	}
	want := []byte{byte(fam.AFI >> 8), byte(fam.AFI), byte(fam.SAFI), 0, 0}
	want = append(want, raw...)
	_, _, mp, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPReachNLRI)
	if !found || !bytes.Equal(mp, want) {
		t.Errorf("recipient MP_REACH=%x present=%v, want exact zero-hop rule %x", mp, found, want)
	}
	_, _, actions, found := attribute.AttrFind(update.PathAttributes, attribute.AttrExtCommunity)
	if !found || !bytes.Equal(actions, mustHex(t, "8006000046160000")) {
		t.Errorf("recipient traffic-rate action=%x present=%v", actions, found)
	}
	if len(update.NLRI) != 0 || len(update.WithdrawnRoutes) != 0 {
		t.Errorf("unexpected legacy announcements=%x withdrawals=%x", update.NLRI, update.WithdrawnRoutes)
	}
	if _, _, _, found := attribute.AttrFind(update.PathAttributes, attribute.AttrMPUnreachNLRI); found {
		t.Error("ignored next hop caused a withdrawal")
	}
}
