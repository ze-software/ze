// VALIDATES: labeled unicast (SAFI 4) and MPLS VPN (SAFI 128) routes are stored one
// entry per route, keyed on the route the NLRI names: the prefix behind the label
// stack, and for VPN the Route Distinguisher with it. Every stored route is replayed.
// PREVENTS: label octets (and the RD) read as address octets, which made distinct
// routes share one key, so a later route overwrote an earlier one, a withdrawal
// removed a route it did not name, and the bgp-rs peer-up replay lost routes.

package adj_rib_in

import (
	"encoding/binary"
	"encoding/hex"
	"net/netip"
	"slices"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

var ipv4Labeled = family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMPLSLabel}

// mpAttr frames one MP_REACH_NLRI (code 14) or MP_UNREACH_NLRI (code 15) attribute
// with the extended-length flag, so a test never has to count to 255.
func mpAttr(code byte, value []byte) []byte {
	attr := []byte{0x90, code, 0, 0}
	binary.BigEndian.PutUint16(attr[2:], uint16(len(value)))
	return append(attr, value...)
}

// mpAnnounce builds an UPDATE whose only routes are the given NLRIs in one
// MP_REACH_NLRI, with ORIGIN and an empty AS_PATH beside it.
func mpAnnounce(fam family.Family, nextHop []byte, nlris ...[]byte) []byte {
	value := []byte{0, byte(fam.AFI), byte(fam.SAFI), byte(len(nextHop))}
	value = append(value, nextHop...)
	value = append(value, 0) // Reserved
	for _, n := range nlris {
		value = append(value, n...)
	}
	attrs := []byte{0x40, 0x01, 0x01, 0x00, 0x40, 0x02, 0x00}
	attrs = append(attrs, mpAttr(14, value)...)
	return mpBody(attrs)
}

// mpWithdraw builds an UPDATE whose only attribute is an MP_UNREACH_NLRI.
func mpWithdraw(fam family.Family, nlris ...[]byte) []byte {
	value := []byte{0, byte(fam.AFI), byte(fam.SAFI)}
	for _, n := range nlris {
		value = append(value, n...)
	}
	return mpBody(mpAttr(15, value))
}

func mpBody(attrs []byte) []byte {
	body := []byte{0, 0, 0, 0} // Withdrawn Routes Length 0, Total Path Attribute Length
	binary.BigEndian.PutUint16(body[2:], uint16(len(attrs)))
	return append(body, attrs...)
}

// labelOneBottom is label 1000 with the bottom-of-stack bit set: RFC 8277 Section 2
// carries each label as 20 bits of label, 3 of traffic class, 1 S bit.
var labelOneBottom = []byte{0x00, 0x3e, 0x81}

// compatibility is the RFC 8277 Section 2.4 field a withdrawal carries where the
// announcement carried its label stack.
var compatibility = []byte{0x80, 0x00, 0x00}

func labeledNLRI(label []byte, prefix ...byte) []byte {
	out := []byte{byte(24 + 8*len(prefix))}
	out = append(out, label...)
	return append(out, prefix...)
}

// rd builds a type 0 Route Distinguisher, ASN 65001 with the given assigned number.
func rd(assigned byte) []byte {
	return []byte{0x00, 0x00, 0xfd, 0xe9, 0x00, 0x00, 0x00, assigned}
}

func vpnNLRI(label, distinguisher []byte, prefix ...byte) []byte {
	out := []byte{byte(24 + 64 + 8*len(prefix))}
	out = append(out, label...)
	out = append(out, distinguisher...)
	return append(out, prefix...)
}

func feedStructured(t *testing.T, r *AdjRIBInManager, peer netip.Addr, body []byte) {
	t.Helper()
	feedStructuredContext(t, r, peer, body, bgpctx.EncodingContextForASN4(true))
}

// feedStructuredContext is feedStructured under the given encoding context, so a
// test can negotiate ADD-PATH for a family.
func feedStructuredContext(t *testing.T, r *AdjRIBInManager, peer netip.Addr, body []byte, ctx *bgpctx.EncodingContext) {
	t.Helper()
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)
	wu := wireu.NewWireUpdate(body, ctxID)
	attrs, err := wu.Attrs()
	require.NoError(t, err)
	r.handleReceivedStructured(&rpc.StructuredEvent{
		EventType:   rpc.EventKindUpdate,
		PeerAddress: peer.String(),
		RawMessage: &bgptypes.RawMessage{
			Type:       msgtype.TypeUPDATE,
			WireUpdate: wu,
			AttrsWire:  attrs,
		},
	})
}

// replayedNLRIs returns the NLRI hex of every route the replay to target carries
// from source, sorted.
func replayedNLRIs(r *AdjRIBInManager, target, source netip.Addr) []string {
	routes, _ := r.buildReplayRoutes(target, 0, unboundedReplay())
	var out []string
	for i := range routes {
		if routes[i].SourcePeer == source.String() {
			out = append(out, routes[i].NLRIHex)
		}
	}
	slices.Sort(out)
	return out
}

// TestLabeledUnicastRoutesKeyOnTheirPrefix feeds labeled unicast routes through
// handleReceivedStructured, the bridge path an internal bgp-adj-rib-in takes, and
// reads the store and the replay a late peer is sent.
//
// Two routes behind the same label are two routes, whether they arrive in two
// UPDATEs (the bgp-prefix-sid-malformed-frr order) or in one. A withdrawal of one
// removes that one alone.
func TestLabeledUnicastRoutesKeyOnTheirPrefix(t *testing.T) {
	r := newTestManager(t)
	source := netip.MustParseAddr("192.0.2.1")
	both := netip.MustParseAddr("192.0.2.2")
	target := netip.MustParseAddr("192.0.2.9")
	nextHop := []byte{10, 0, 0, 1}

	first := labeledNLRI(labelOneBottom, 10, 0, 1)
	second := labeledNLRI(labelOneBottom, 10, 0, 4)
	feedStructured(t, r, source, mpAnnounce(ipv4Labeled, nextHop, first))
	feedStructured(t, r, source, mpAnnounce(ipv4Labeled, nextHop, second))
	feedStructured(t, r, both, mpAnnounce(ipv4Labeled, nextHop, first, second))

	require.NotNil(t, r.ribIn[source])
	assert.Equal(t, 2, r.ribIn[source].Len(), "two labeled prefixes in two UPDATEs are two routes")
	require.NotNil(t, r.ribIn[both])
	assert.Equal(t, 2, r.ribIn[both].Len(), "two labeled prefixes in one UPDATE are two routes")

	stored, ok := r.ribIn[source].Get(routeKeyFromStrings(ipv4Labeled, "10.0.1.0/24", 0))
	require.True(t, ok, "the route is keyed on the prefix behind its label")
	assert.Equal(t, hex.EncodeToString(first), stored.NLRIHex, "the stored NLRI keeps its own label")
	assert.Equal(t, rpc.NLRIFramingPrefixOnly, stored.NLRIFraming)

	want := []string{hex.EncodeToString(first), hex.EncodeToString(second)}
	assert.Equal(t, want, replayedNLRIs(r, target, source), "every stored route is replayed")
	assert.Equal(t, want, replayedNLRIs(r, target, both), "every route of a two-NLRI UPDATE is replayed")

	withdrawn := append([]byte{24 + 24}, compatibility...)
	withdrawn = append(withdrawn, 10, 0, 1)
	feedStructured(t, r, source, mpWithdraw(ipv4Labeled, withdrawn))
	assert.Equal(t, []string{hex.EncodeToString(second)}, replayedNLRIs(r, target, source),
		"the withdrawal removes the route it names and no other")
}

// TestVPNRoutesKeyOnDistinguisherAndPrefix is the same proof for VPN, where the
// Route Distinguisher is part of the route's identity: one prefix under two RDs is
// two routes, and two prefixes under one RD are two routes.
func TestVPNRoutesKeyOnDistinguisherAndPrefix(t *testing.T) {
	r := newTestManager(t)
	source := netip.MustParseAddr("192.0.2.1")
	target := netip.MustParseAddr("192.0.2.9")
	nextHop := []byte{0, 0, 0, 0, 0, 0, 0, 0, 10, 0, 0, 1} // RD 0, then the IPv4 address

	control := vpnNLRI(labelOneBottom, rd(1), 10, 0, 3)
	otherRD := vpnNLRI(labelOneBottom, rd(2), 10, 0, 3)
	otherPrefix := vpnNLRI(labelOneBottom, rd(1), 10, 0, 2)
	for _, n := range [][]byte{control, otherRD, otherPrefix} {
		feedStructured(t, r, source, mpAnnounce(ipv4VPN, nextHop, n))
	}

	require.NotNil(t, r.ribIn[source])
	assert.Equal(t, 3, r.ribIn[source].Len(), "RD and prefix together identify a VPN route")
	want := []string{hex.EncodeToString(otherPrefix), hex.EncodeToString(control), hex.EncodeToString(otherRD)}
	slices.Sort(want)
	assert.Equal(t, want, replayedNLRIs(r, target, source), "every stored VPN route is replayed")

	withdrawn := append([]byte{24 + 64 + 24}, compatibility...)
	withdrawn = append(withdrawn, rd(1)...)
	withdrawn = append(withdrawn, 10, 0, 2)
	feedStructured(t, r, source, mpWithdraw(ipv4VPN, withdrawn))
	want = []string{hex.EncodeToString(control), hex.EncodeToString(otherRD)}
	slices.Sort(want)
	assert.Equal(t, want, replayedNLRIs(r, target, source),
		"withdrawing RD 1 10.0.2.0/24 leaves RD 1 and RD 2 10.0.3.0/24")
}
