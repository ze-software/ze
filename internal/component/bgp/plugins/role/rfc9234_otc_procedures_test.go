package role

import (
	"encoding/binary"
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/filterapi"
)

// rfc9234AllRoles is the widest export set an operator can write: every
// destination role, so no RFC 9234 Section 5 outcome below comes from an
// export set that happens to exclude the destination.
var rfc9234AllRoles = []string{roleProvider, roleCustomer, rolePeer, roleRS, roleRSClient}

// rfc9234ResetFilterState clears the package filter state a test installed.
func rfc9234ResetFilterState() {
	setFilterState(nil, nil)
	filterMu.Lock()
	filterRemoteRoles = nil
	filterMu.Unlock()
}

// rfc9234EgressStamp runs one route announcing 10.2.2.0/24, from a source that
// is our Customer, to a destination whose remote role is destRole, with local
// AS 65000 for that session. It answers the accept decision and the queued
// attribute operations.
func rfc9234EgressStamp(t *testing.T, destRole string, otcASN uint32) (bool, []filterapi.AttrOp) {
	t.Helper()
	setFilterState(map[string]*peerRoleConfig{"10.0.0.1": {role: roleProvider}}, nil)
	setFilterRemoteRole("10.0.0.1", "", roleCustomer)
	setFilterRemoteRole("10.0.0.5", "", destRole)
	defer rfc9234ResetFilterState()

	payload := buildTestPayload(buildTestAttrs(otcASN), []byte{24, 10, 2, 2})
	source := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1")}
	target := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.5"), LocalAS: 65000}
	meta := map[string]any{"src-role": roleProvider}

	var mods filterapi.ModAccumulator
	accept := OTCEgressFilter(source, target, payload, meta, &mods)
	return accept, mods.Ops()
}

// RFC requirement: RFC9234-5-4 positive -- a route announcing 10.2.2.0/24 with
// no OTC, from a Customer, advertised to a Customer, to a Peer and to an
// RS-Client (Ze as the RS), is accepted and gains exactly one OTC set operation
// (attribute 35) whose 4-octet value is the session's local AS, 65000.
func TestRFC9234OTCStampedToEveryDownstreamDestination(t *testing.T) {
	for _, destRole := range []string{roleCustomer, rolePeer, roleRSClient} {
		t.Run(destRole, func(t *testing.T) {
			accept, ops := rfc9234EgressStamp(t, destRole, 0)
			require.True(t, accept, "a route toward %s must be advertised", destRole)
			require.Len(t, ops, 1, "exactly one OTC operation toward %s", destRole)
			assert.Equal(t, otcAttrCode, ops[0].Code)
			assert.Equal(t, filterapi.AttrModSet, ops[0].Action)
			require.Len(t, ops[0].Buf, 4)
			assert.Equal(t, uint32(65000), binary.BigEndian.Uint32(ops[0].Buf),
				"the OTC value must be the local AS")
		})
	}
}

// RFC requirement: RFC9234-5-4 negative -- the same route, which announces
// NLRI, is advertised to a Provider and to an RS with no OTC operation, so the
// destination-role scope, and not the absence of NLRI, is what withholds the
// stamp. A route that already carries OTC 64999 toward a Customer is accepted
// with no operation either: the stamp is added only when OTC is not present.
func TestRFC9234OTCNotStampedOutsideTheDownstreamScope(t *testing.T) {
	for _, destRole := range []string{roleProvider, roleRS} {
		t.Run(destRole, func(t *testing.T) {
			accept, ops := rfc9234EgressStamp(t, destRole, 0)
			require.True(t, accept, "a Customer route toward %s may be advertised", destRole)
			assert.Empty(t, ops, "no OTC may be added toward %s", destRole)
		})
	}
	t.Run("otc-present", func(t *testing.T) {
		accept, ops := rfc9234EgressStamp(t, roleCustomer, 64999)
		require.True(t, accept, "a route carrying OTC may go to a Customer")
		assert.Empty(t, ops, "an OTC already present must not be stamped again")
	})
}

// RFC requirement: RFC9234-5-11 positive -- every operator setting the role
// configuration offers is set to its most permissive value (strict mode on, an
// export set naming every role), and the Section 5 procedures still run: a
// route without OTC from a Provider gains OTC 64500 (the Provider's AS) on
// ingress, a route carrying OTC from a Customer is refused on ingress, a route
// carrying OTC is still withheld from a Provider on egress, and a route without
// OTC toward a Customer is still stamped with the local AS 65000.
func TestRFC9234OTCProceduresSurviveOperatorSettings(t *testing.T) {
	setFilterState(map[string]*peerRoleConfig{
		"10.0.0.1": {role: roleCustomer, strict: true, export: rfc9234AllRoles, resolvedExport: rfc9234AllRoles},
		"10.0.0.2": {role: roleProvider, strict: true, export: rfc9234AllRoles, resolvedExport: rfc9234AllRoles},
	}, nil)
	setFilterRemoteRole("10.0.0.1", "", roleProvider)
	setFilterRemoteRole("10.0.0.2", "", roleCustomer)
	setFilterRemoteRole("10.0.0.5", "", roleProvider)
	setFilterRemoteRole("10.0.0.6", "", roleCustomer)
	defer rfc9234ResetFilterState()

	nlri := []byte{24, 10, 2, 2}
	fromProvider := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.1"), PeerAS: 64500}
	fromCustomer := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.2"), PeerAS: 64501}

	accept, modified := OTCIngressFilter(fromProvider, buildTestPayload(buildTestAttrs(0), nlri), map[string]any{})
	require.True(t, accept, "a Provider route is accepted")
	require.NotNil(t, modified, "ingress must add OTC to a Provider route")
	otcASN, found, malformed := findOTC(extractAttrsFromPayload(modified))
	require.True(t, found)
	require.False(t, malformed)
	assert.Equal(t, uint32(64500), otcASN, "the ingress OTC value must be the Provider's AS")

	accept, _ = OTCIngressFilter(fromCustomer, buildTestPayload(buildTestAttrs(64999), nlri), map[string]any{})
	assert.False(t, accept, "a Customer route carrying OTC is a leak whatever the export set")

	toProvider := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.5"), LocalAS: 65000}
	withOTC := buildTestPayload(buildTestAttrs(64999), nlri)
	assert.False(t, OTCEgressFilter(fromCustomer, toProvider, withOTC, map[string]any{"src-role": roleProvider}, nil),
		"a route carrying OTC must not reach a Provider, though the export set names provider")

	toCustomer := filterapi.PeerFilterInfo{Address: netip.MustParseAddr("10.0.0.6"), LocalAS: 65000}
	var mods filterapi.ModAccumulator
	require.True(t, OTCEgressFilter(fromCustomer, toCustomer, buildTestPayload(buildTestAttrs(0), nlri),
		map[string]any{"src-role": roleProvider}, &mods))
	ops := mods.Ops()
	require.Len(t, ops, 1, "the egress stamp still runs")
	assert.Equal(t, otcAttrCode, ops[0].Code)
	assert.Equal(t, uint32(65000), binary.BigEndian.Uint32(ops[0].Buf))
}
