// Design: docs/architecture/plugin/rib-storage-design.md -- live FlowSpec validation
// RFC: rfc/short/rfc8955.md -- Section 12, empty-origin relaxation requires additional policy filtering
// Related: rib_flowspec_validation.go -- flowSpecAuthorized

package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/plugins/rib/storage"
	"github.com/ze-software/ze/internal/core/family"
)

// TestRFC8955EmptyOriginRelaxationRefused verifies that flowSpecAuthorized
// always enforces destination-prefix presence and non-empty external AS_PATH
// matching, never enabling the RFC 8955 Section 12 empty-origin relaxation.
//
// RFC 8955 Section 12: "For a network to utilize this relaxation, the BGP
// policies must support additional filtering since the origin AS field is
// empty."
//
// VALIDATES: an external FlowSpec route with a valid destination and matching
// leftmost AS is authorized, while an external FlowSpec route with an empty
// AS_PATH (empty origin AS) or missing destination prefix is rejected even
// when a covering unicast route exists.
// PREVENTS: silently accepting relaxed empty-origin or destination-less
// FlowSpec routes without the additional filtering Section 12 mandates.
//
// RFC requirement: RFC8955-12-2 positive -- an external FlowSpec route with a
// destination prefix and matching leftmost AS_PATH ASN is authorized against
// its covering unicast route by flowSpecAuthorized.
// RFC requirement: RFC8955-12-2 negative -- an external FlowSpec route with an
// empty AS_PATH (empty origin AS) or without a destination prefix is rejected
// by flowSpecAuthorized even when a covering unicast route is present, proving
// Ze never enables the empty-origin relaxation.
func TestRFC8955EmptyOriginRelaxationRefused(t *testing.T) {
	t.Parallel()

	domain := flowSpecDomain{family: family.IPv4Unicast}
	dest := netip.MustParsePrefix("203.0.113.0/24")
	peer := netip.MustParseAddr("192.0.2.1")

	unicast := []flowSpecUnicast{{
		domain:     domain,
		prefix:     dest,
		originator: peer,
		firstAS:    65001,
		lastAS:     65001,
		candidate:  &Candidate{PeerAddr: peer.String(), PeerIP: peer, PeerASN: 65001, LocalASN: 65000, LocalPref: 100},
	}}

	// Positive: matching external FlowSpec route with valid destination and AS 65001.
	validEntry, _, _, err := storage.ParseRouteEntry(flowValidationAttrs(flowValidationPath(2, 65001), netip.Addr{}, 0, nil))
	require.NoError(t, err)
	t.Cleanup(validEntry.Release)
	validFlow := &flowSpecRoute{
		domain:      domain,
		destination: dest,
		originator:  peer,
		firstAS:     65001,
		localDomain: false,
		validPath:   true,
		matchable:   true,
		candidate:   &Candidate{PeerAddr: peer.String(), PeerIP: peer, PeerASN: 65001, LocalASN: 65000, ASPathLen: 1},
		entry:       validEntry,
	}
	require.True(t, flowSpecAuthorized(validFlow, unicast),
		"normal external FlowSpec with matching destination and leftmost AS must be authorized")

	// Negative 1: external FlowSpec with empty AS_PATH (empty origin AS).
	emptyEntry, _, _, err := storage.ParseRouteEntry(flowValidationAttrs(nil, netip.Addr{}, 0, nil))
	require.NoError(t, err)
	t.Cleanup(emptyEntry.Release)
	emptyOriginFlow := &flowSpecRoute{
		domain:      domain,
		destination: dest,
		originator:  peer,
		firstAS:     0,
		localDomain: true,
		validPath:   true,
		matchable:   true,
		candidate:   &Candidate{PeerAddr: peer.String(), PeerIP: peer, PeerASN: 65001, LocalASN: 65000, ASPathLen: 0},
		entry:       emptyEntry,
	}
	require.False(t, flowSpecAuthorized(emptyOriginFlow, unicast),
		"external FlowSpec with empty origin AS_PATH must be rejected")

	// Negative 2: FlowSpec missing a destination prefix (relaxed rule a).
	noDestFlow := &flowSpecRoute{
		domain:      domain,
		destination: netip.Prefix{},
		originator:  peer,
		firstAS:     65001,
		localDomain: false,
		validPath:   true,
		matchable:   true,
		candidate:   &Candidate{PeerAddr: peer.String(), PeerIP: peer, PeerASN: 65001, LocalASN: 65000, ASPathLen: 1},
		entry:       validEntry,
	}
	require.False(t, flowSpecAuthorized(noDestFlow, unicast),
		"FlowSpec without destination prefix must be rejected")
}
