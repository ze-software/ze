// Design: docs/architecture/update-building.md — originated and relayed UPDATE output
// Related: rfc7311_aigp_readvertise_test.go — actual Session writer and forwarding FIFO
// Related: rfc7607_update_test.go — receive-side error handling is a separate boundary
// RFC: rfc/short/rfc7607.md — Section 2 forbids originating or propagating AS zero

package reactor

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"
)

// TestRFC7607ASZeroNeverLeavesOriginatedOrRelayedRoutes injects the forbidden
// value after the receive boundary, through the real announce API and both
// relay rails. A receive-only validator cannot satisfy these socket assertions.
// RFC requirement: RFC7607-2-1 positive -- originated API routes and both general and native route-server relay output never carry AS zero in AS_PATH, AS4_PATH, AGGREGATOR or AS4_AGGREGATOR; mandatory-path failures cannot remain announcements, while optional attribute discard or withdrawal is permitted.
func TestRFC7607ASZeroNeverLeavesOriginatedOrRelayedRoutes(t *testing.T) {
	rfc7607EgressMatrix(t, true)
}

// TestRFC7607RealASLeavesOriginatedAndRelayedRoutes changes only the attribute's
// AS value. Its actual final wire must still announce the route and preserve the
// real AS, so suppression of every route or attribute cannot satisfy the guard.
// RFC requirement: RFC7607-2-1 negative -- the same originated and relayed routes with real AS numbers remain advertised, preserving their nonzero AS value at the negotiated width, including legitimate AS4 attributes toward OLD speakers.
func TestRFC7607RealASLeavesOriginatedAndRelayedRoutes(t *testing.T) {
	rfc7607EgressMatrix(t, false)
}

// rfc7607EgressCase chooses the destination width and the one AS-bearing field
// changed by a case. AS4 attributes have a legitimate preservation control only
// toward OLD speakers; RFC 6793 forbids them between NEW speakers.
type rfc7607EgressCase struct {
	name string
	code attribute.AttributeCode
	asn4 bool
	real uint32
}

func rfc7607EgressMatrix(t *testing.T, zero bool) {
	t.Helper()
	cases := []rfc7607EgressCase{
		{"AS_PATH OLD", attribute.AttrASPath, false, 65011},
		{"AS_PATH NEW", attribute.AttrASPath, true, 65536},
		{"AGGREGATOR OLD", attribute.AttrAggregator, false, 65011},
		{"AGGREGATOR NEW", attribute.AttrAggregator, true, 65536},
		{"AS4_PATH OLD", attribute.AttrAS4Path, false, 65536},
		{"AS4_AGGREGATOR OLD", attribute.AttrAS4Aggregator, false, 65536},
	}
	for _, rail := range []string{"originate", "raw", "relay", "route-server"} {
		for _, tc := range cases {
			t.Run(rail+"/"+tc.name, func(t *testing.T) {
				f := rfc7607EgressFixture(t, tc.asn4)
				asn := tc.real
				if zero {
					asn = 0
				}
				attrs := rfc7607EgressAttrs(tc, asn, f.source.Settings().Address)
				rfc7607SendEgress(t, f, rail, attrs)
				bodies := aigpSocketBodies(t, f.conn)
				advertised := false
				preserved := false
				for _, body := range bodies {
					update, err := message.UnpackUpdate(body)
					require.NoError(t, err)
					if len(update.NLRI) != 0 {
						advertised = true
						assert.Equal(t, []byte{24, 203, 0, 113}, update.NLRI)
						_, _, _, hasPath := attribute.AttrFind(update.PathAttributes, attribute.AttrASPath)
						assert.True(t, hasPath, "discarding a mandatory AS_PATH cannot leave an announcement")
						if zero && tc.code == attribute.AttrASPath {
							t.Error("an AS-zero AS_PATH must not remain an announcement")
						}
					}
					it := attribute.NewAttrIterator(update.PathAttributes)
					for {
						code, _, value, ok := it.Next()
						if !ok {
							break
						}
						for _, emitted := range rfc7607ReadEgressAS(t, code, value, tc.asn4) {
							assert.NotZero(t, emitted, "AS zero reached the final wire in attribute %d", code)
							if code == tc.code && emitted == tc.real {
								preserved = true
							}
						}
					}
				}
				if !zero {
					assert.True(t, advertised, "the valid twin must actually reach the peer")
					assert.True(t, preserved, "the valid AS value must survive in its attribute")
				}
			})
		}
	}
}

// rfc7607EgressFixture retains the existing socket writer and FIFO ownership,
// while negotiating either a two-octet or four-octet session consistently.
func rfc7607EgressFixture(t *testing.T, asn4 bool) *aigpReplayFixture {
	t.Helper()
	f := newAIGPReplayFixture(t, nil)
	f.source.settings.PeerAS = 65010
	ctx := bgpctx.EncodingContextForASN4(asn4)
	ctxID, err := bgpctx.Registry.Register(ctx)
	require.NoError(t, err)
	f.ctxID = ctxID
	for _, peer := range []*Peer{f.source, f.destination} {
		caps := []capability.Capability{
			&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		}
		if asn4 {
			caps = append(caps, &capability.ASN4{ASN: peer.settings.LocalAS})
		}
		negotiated := capability.Negotiate(caps, caps, capability.PeerIdentity{
			LocalASN: peer.settings.LocalAS, PeerASN: peer.settings.PeerAS,
			Internal: peer.settings.IsIBGP(),
		})
		peer.session.negotiated = negotiated
		peer.negotiated.Store(NewNegotiatedCapabilities(negotiated))
		peer.sendCtx.Store(ctx)
		peer.sendCtxID, peer.recvCtxID = ctxID, ctxID
		peer.session.sendCtxID, peer.session.recvCtxID = ctxID, ctxID
		peer.refreshForwardFacts()
	}
	return f
}

// rfc7607SendEgress drives public origination or the real cached/native relay.
// The cache receives a candidate directly: the egress prohibition must still
// hold when an API producer or export transformation bypasses receive checking.
func rfc7607SendEgress(t *testing.T, f *aigpReplayFixture, rail string, attrs []byte) {
	t.Helper()
	if rail == "raw" {
		body := makeUpdateBody(nil, attrs, []byte{24, 203, 0, 113})
		require.NoError(t, (&reactorAPIAdapter{r: f.r}).SendRawMessage(
			f.destination.Settings().Address, uint8(msgtype.TypeUPDATE), body, plugin.OperatorSender()))
		return
	}
	if rail == "originate" {
		batch := bgptypes.NLRIBatch{
			Family: family.IPv4Unicast,
			NLRIs: []nlri.NLRI{nlri.NewINET(family.IPv4Unicast,
				netip.MustParsePrefix("203.0.113.0/24"), 0, false)},
			NextHop: bgptypes.NewNextHopExplicit(f.source.Settings().Address),
			Wire:    attribute.NewAttributesWire(attrs, f.ctxID),
		}
		require.NoError(t, (&reactorAPIAdapter{r: f.r}).AnnounceNLRIBatch(t.Context(),
			selector.Addr(f.destination.Settings().Address), batch, plugin.OperatorSender()))
		return
	}
	body := makeUpdateBody(nil, attrs, []byte{24, 203, 0, 113})
	rfc4271ProofRelay(t, f, body, rail == "route-server")
}

// rfc7607EgressAttrs builds an otherwise valid attribute block. A zero byte in
// the prefix or next-hop address is deliberately ordinary data, not an AS zero.
func rfc7607EgressAttrs(tc rfc7607EgressCase, asn uint32, nextHop netip.Addr) []byte {
	width := 2
	if tc.asn4 {
		width = 4
	}
	path := rfc7607EgressPath(width, 65010)
	if tc.code == attribute.AttrASPath {
		path = rfc7607EgressPath(width, 65010, asn)
	}
	if tc.code == attribute.AttrAS4Path {
		path = rfc7607EgressPath(2, 65010, attribute.ASTrans)
	}
	attrs := collapseAttr(0x40, 1, []byte{0})
	attrs = append(attrs, collapseAttr(0x40, 2, path)...)
	attrs = append(attrs, collapseAttr(0x40, 3, nextHop.AsSlice())...)
	if tc.code == attribute.AttrAS4Path {
		return append(attrs, collapseAttr(0xc0, 17, rfc7607EgressPath(4, 65010, asn))...)
	}
	if tc.code == attribute.AttrAggregator {
		return append(attrs, collapseAttr(0xc0, 7, rfc7607EgressAggregator(width, asn))...)
	}
	if tc.code == attribute.AttrAS4Aggregator {
		attrs = append(attrs, collapseAttr(0xc0, 7, rfc7607EgressAggregator(2, attribute.ASTrans))...)
		return append(attrs, collapseAttr(0xc0, 18, rfc7607EgressAggregator(4, asn))...)
	}
	return attrs
}

func rfc7607EgressPath(width int, asns ...uint32) []byte {
	path := make([]byte, 2+width*len(asns))
	path[0], path[1] = 2, byte(len(asns)) // fixed test cases contain at most two ASNs
	for i, asn := range asns {
		if width == 4 {
			binary.BigEndian.PutUint32(path[2+i*width:], asn)
		} else {
			binary.BigEndian.PutUint16(path[2+i*width:], uint16(asn)) //nolint:gosec // two-octet fixture cases are bounded above
		}
	}
	return path
}

func rfc7607EgressAggregator(width int, asn uint32) []byte {
	value := make([]byte, width+4)
	if width == 4 {
		binary.BigEndian.PutUint32(value, asn)
	} else {
		binary.BigEndian.PutUint16(value, uint16(asn)) //nolint:gosec // two-octet fixture cases are bounded above
	}
	copy(value[width:], []byte{192, 0, 2, 0})
	return value
}

// rfc7607ReadEgressAS uses independent codecs at the recipient's negotiated
// width and examines every occurrence, including optional attributes migrated
// into another AS-bearing form by the forward rewrite.
func rfc7607ReadEgressAS(t *testing.T, code attribute.AttributeCode, value []byte, asn4 bool) []uint32 {
	t.Helper()
	var segments []attribute.ASPathSegment
	// Only the four attribute codes named in RFC 7607 carry the AS being judged.
	//exhaustive:ignore
	switch code {
	case attribute.AttrASPath:
		path, err := attribute.ParseASPath(value, asn4)
		require.NoError(t, err)
		segments = path.Segments
	case attribute.AttrAS4Path:
		path, err := attribute.ParseAS4Path(value)
		require.NoError(t, err)
		segments = path.Segments
	case attribute.AttrAggregator:
		aggregator, err := attribute.ParseAggregator(value, asn4)
		require.NoError(t, err)
		return []uint32{aggregator.ASN}
	case attribute.AttrAS4Aggregator:
		aggregator, err := attribute.ParseAS4Aggregator(value)
		require.NoError(t, err)
		return []uint32{aggregator.ASN}
	}
	var asns []uint32
	for _, segment := range segments {
		asns = append(asns, segment.ASNs...)
	}
	return asns
}
