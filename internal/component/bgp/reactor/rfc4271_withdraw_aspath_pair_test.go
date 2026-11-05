// Design: docs/architecture/bgp/egress-attribute-rules.md -- no prepend on a withdrawal.
// Related: rfc4456_forward_build_withdraw_shape_test.go -- owned full-dispatch fixture.
// RFC 4271 Section 5.1.2 -- see rfc/short/rfc4271.md.
package reactor

import (
	"net/netip"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
)

// TestForwardWithdrawalDoesNotCreateASPath drives the real relay dispatch to an
// external ASN4 destination. Next-hop-self reaches the independent attribute
// creation gate even when the AS-path dispatcher records no prepend.
// RFC 4271 Section 5.1.2: "When a given BGP speaker advertises the route to an
// external peer, the advertising speaker updates the AS_PATH attribute as
// follows:"
// MUTATION: Bypass Record's no-advertisement branch and planAttr's creation
// refusal for AS_PATH only. Each guard alone protects the withdrawal; together
// the breaks create an attribute. The advertisement control forbids blanket
// suppression of AS_PATH or next-hop rewriting.
// RFC requirement: RFC4271-5.1.2-3 negative -- an attribute-free IPv4 withdrawal
// relayed to an external ASN4 peer gains no AS_PATH and stays byte-identical,
// while the advertised control gains exactly the local AS before the source AS.
func TestForwardWithdrawalDoesNotCreateASPath(t *testing.T) {
	destination := &PeerSettings{
		Connection:   ConnectionBoth,
		Address:      netip.MustParseAddr("10.0.0.2"),
		LocalAS:      65000,
		PeerAS:       65002,
		RouterID:     0x01020301,
		NextHopMode:  NextHopSelf,
		LocalAddress: netip.MustParseAddr("192.0.2.1"),
	}
	t.Run("withdrawal", func(t *testing.T) {
		body := withdrawOnlyBody()
		original := slices.Clone(body)
		// RFC 4271 Section 5.1.2(b): prepend belongs to advertisement.
		got := forwardOneBody(t, 65001, destination, body)
		if _, present := rebuiltAttrs(t, got)[byte(attribute.AttrASPath)]; present {
			t.Error("the relayed withdrawal gained AS_PATH")
		}
		if !slices.Equal(got, original) {
			t.Errorf("withdrawal changed: got %x, want %x", got, original)
		}
		if !slices.Equal(body, original) {
			t.Error("forwarding changed the received withdrawal")
		}
	})
	t.Run("advertisement", func(t *testing.T) {
		// UPDATE body: withdrawn-len[0:2], attr-len[2:4], ordered
		// ORIGIN/AS_PATH/NEXT_HOP, then IPv4 NLRI 10.0.0.0/24.
		// AS_SEQUENCE has count1 and the four-octet source ASN65001.
		attrs := makeAttr(0x40, byte(attribute.AttrOrigin), []byte{0})
		attrs = append(attrs, makeAttr(0x40, byte(attribute.AttrASPath), []byte{2, 1, 0, 0, 0xfd, 0xe9})...)
		attrs = append(attrs, makeAttr(0x40, byte(attribute.AttrNextHop), []byte{192, 0, 2, 254})...)
		body := buildModTestPayload(attrs, []byte{24, 10, 0, 0})
		original := slices.Clone(body)
		// RFC 4271 Section 5.1.2(b): the advertised path is prepended.
		got := forwardOneBody(t, 65001, destination, body)
		wantAttrs := makeAttr(0x40, byte(attribute.AttrOrigin), []byte{0})
		wantAttrs = append(wantAttrs, makeAttr(0x40, byte(attribute.AttrASPath), []byte{2, 2, 0, 0, 0xfd, 0xe8, 0, 0, 0xfd, 0xe9})...)
		wantAttrs = append(wantAttrs, makeAttr(0x40, byte(attribute.AttrNextHop), []byte{192, 0, 2, 1})...)
		want := buildModTestPayload(wantAttrs, []byte{24, 10, 0, 0})
		if !slices.Equal(got, want) {
			t.Errorf("advertised UPDATE: got %x, want %x", got, want)
		}
		if !slices.Equal(body, original) {
			t.Error("forwarding changed the received advertisement")
		}
	})
}
