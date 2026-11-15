// Design: docs/architecture/update-building.md — final writer family and ownership admission
// Related: rfc7607_egress_test.go — AS-zero output matrix and socket fixture
// RFC: rfc/short/rfc7607.md — AS-zero refusal must preserve real withdrawals

package reactor

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/capability"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
)

// TestRFC7607ASZeroReplacementWithdrawsMixedFamilies seeds the recipient with
// clean IPv4 and IPv6 routes, then offers an AS-zero replacement containing
// both new announcements and explicit withdrawals. Every held route must leave
// as a real withdrawal, with no mixed-field or AS-zero output sent to the peer.
// RFC requirement: RFC7607-2-1 positive -- an AS-zero replacement withdraws previously advertised IPv4 and MP IPv6 routes while preserving explicit withdrawals; every output UPDATE is free of AS zero and has at most one NLRI-bearing field.
func TestRFC7607ASZeroReplacementWithdrawsMixedFamilies(t *testing.T) {
	f := rfc7607EgressFixture(t, true)
	caps := []capability.Capability{
		&capability.ASN4{ASN: 65000},
		&capability.Multiprotocol{AFI: capability.AFIIPv4, SAFI: capability.SAFIUnicast},
		&capability.Multiprotocol{AFI: capability.AFIIPv6, SAFI: capability.SAFIUnicast},
	}
	negotiated := capability.Negotiate(caps, caps, capability.PeerIdentity{
		LocalASN: 65000, PeerASN: 65001,
	})
	f.destination.session.negotiated = negotiated
	f.destination.negotiated.Store(NewNegotiatedCapabilities(negotiated))
	session := f.destination.session
	v4First := []byte{24, 203, 0, 113}
	v4Second := []byte{24, 198, 51, 100}
	v6First := []byte{32, 0x20, 0x01, 0x0d, 0xb8}
	v6Second := []byte{32, 0x20, 0x01, 0x0d, 0xb9}
	clean := rfc7607EgressAttrs(rfc7607EgressCase{code: attribute.AttrASPath, asn4: true},
		65011, f.source.Settings().Address)
	require.NoError(t, session.sendRawUpdateBody(makeUpdateBody(nil, clean, append(append([]byte{}, v4First...), v4Second...))))
	cleanMP := append(append([]byte{}, clean...), rfc7607BoundaryMPReach(append(append([]byte{}, v6First...), v6Second...))...)
	require.NoError(t, session.sendRawUpdateBody(makeUpdateBody(nil, cleanMP, nil)))
	before := len(f.conn.written())
	require.NotZero(t, before, "the clean routes must have reached the recipient first")

	bad := rfc7607EgressAttrs(rfc7607EgressCase{code: attribute.AttrASPath, asn4: true},
		0, f.source.Settings().Address)
	bad = append(bad, rfc7607BoundaryMPReach(v6First)...)
	bad = append(bad, collapseAttr(0x80, 15, append([]byte{0, 2, 1}, v6Second...))...)
	require.NoError(t, session.sendRawUpdateBody(makeUpdateBody(v4Second, bad, v4First)))

	var v4Withdrawn, v6Withdrawn []string
	bodies := rfc9774SentBodies(t, f.conn.written()[before:])
	require.NotEmpty(t, bodies, "refusal must remove the clean routes the recipient held")
	for _, body := range bodies {
		update, err := message.UnpackUpdate(body)
		require.NoError(t, err)
		assert.Empty(t, update.NLRI, "the bad replacement must announce nothing")
		assert.False(t, update.MixesNLRIFields(), "outbound RFC 7606 framing remains compliant")
		v4Withdrawn = append(v4Withdrawn, rfc7607BoundaryPrefixes(t, update.WithdrawnRoutes, family.IPv4Unicast)...)
		it := attribute.NewAttrIterator(update.PathAttributes)
		for {
			code, _, value, ok := it.Next()
			if !ok {
				break
			}
			assert.NotEqual(t, attribute.AttrMPReachNLRI, code, "no MP announcement survives")
			for _, asn := range rfc7607ReadEgressAS(t, code, value, true) {
				assert.NotZero(t, asn)
			}
			if code == attribute.AttrMPUnreachNLRI {
				require.GreaterOrEqual(t, len(value), 3)
				require.Equal(t, []byte{0, 2, 1}, value[:3])
				v6Withdrawn = append(v6Withdrawn, rfc7607BoundaryPrefixes(t, value[3:], family.IPv6Unicast)...)
			}
		}
	}
	assert.ElementsMatch(t, []string{"203.0.113.0/24", "198.51.100.0/24"}, v4Withdrawn)
	assert.ElementsMatch(t, []string{"2001:db8::/32", "2001:db9::/32"}, v6Withdrawn)
}

// TestRFC7607EgressPreservesEmptyEORAndZeroOctetPrefixes checks the guard's
// negative space: zero bytes in protocol framing or an IP prefix are not ASN 0.
func TestRFC7607EgressPreservesEmptyEORAndZeroOctetPrefixes(t *testing.T) {
	f := rfc7607EgressFixture(t, true)
	require.NoError(t, f.destination.session.sendRawUpdateBody([]byte{0, 0, 0, 0}))
	bodies := aigpSocketBodies(t, f.conn)
	require.Len(t, bodies, 1)
	assert.Equal(t, []byte{0, 0, 0, 0}, bodies[0], "empty EOR remains on the wire")
	attrs := rfc7607EgressAttrs(rfc7607EgressCase{code: attribute.AttrASPath, asn4: true},
		65536, f.source.Settings().Address)
	require.NoError(t, f.destination.session.sendRawUpdateBody(makeUpdateBody(nil, attrs, []byte{8, 0})))
	bodies = aigpSocketBodies(t, f.conn)
	require.Len(t, bodies, 2)
	update, err := message.UnpackUpdate(bodies[1])
	require.NoError(t, err)
	assert.Equal(t, []byte{8, 0}, update.NLRI, "0.0.0.0/8 is not an AS-zero attribute")
}

// rfc7607BoundaryMPReach carries a normal global IPv6 next hop and wire NLRIs.
func rfc7607BoundaryMPReach(prefixes []byte) []byte {
	value := []byte{0, 2, 1, 16,
		0x20, 0x01, 0x0d, 0xb8, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 1,
		0}
	return collapseAttr(0x80, 14, append(value, prefixes...))
}

func rfc7607BoundaryPrefixes(t *testing.T, encoded []byte, fam family.Family) []string {
	t.Helper()
	var prefixes []string
	it := nlri.NewNLRIIterator(encoded, false)
	for {
		wirePrefix, _, ok := it.Next()
		if !ok {
			return prefixes
		}
		prefix, valid := nlri.WirePrefixToKey(wirePrefix, fam)
		require.True(t, valid)
		prefixes = append(prefixes, prefix.String())
	}
}
