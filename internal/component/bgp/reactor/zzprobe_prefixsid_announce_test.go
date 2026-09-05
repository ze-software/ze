// VALIDATES: RFC 8669 Section 8 on the API/readvertise ANNOUNCE rail -- attribute
// 40 does not cross an AS boundary toward an external peer the operator has not
// declared to be inside ze's SR domain.
// PREVENTS: the boundary holding on four rails and leaking on the fifth. The two
// forward rails and the two origination rails ask prefixSIDAllowedTo
// (forward_prefix_sid.go); buildBatchAnnounceUpdate (reactor_api_batch.go) asks
// nobody, and copies the caller's attribute block verbatim.
//
// THIS TEST IS RED AT HEAD, DELIBERATELY. It states the requirement, not the
// behavior: pinning the leak would be pinning non-conformance
// (ai/rules/rfc-compliance.md). It carries no `RFC requirement:` tag, because a
// tag is a claim that a test DEMONSTRATES something and this one demonstrates the
// gap.
//
// Making it green needs a per-destination bool on buildBatchAnnounceUpdate and on
// announceBuildKey, which mechanically edits
// TestAnnounceStripsLocalPrefTowardExternalPeer (reactor_api_origin_test.go). That
// test carries `RFC requirement: RFC4271-5.1.5-1/-2`, and only the owner approves
// an edit to a tagged test, as a row in test/rfc-changed.md. The fix waits on that
// answer.
package reactor

import (
	"encoding/hex"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/core/bgp/attribute"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"

	"github.com/stretchr/testify/require"
)

func TestAnnounceRailKeepsPrefixSIDInsideTheSRDomain(t *testing.T) {
	// ORIGIN igp, AS_PATH [65001], PREFIX_SID carrying a Label-Index TLV
	// (RFC 8669 Section 3.1: type 1, length 7, RESERVED 0, Flags 0, index 100).
	// No LOCAL_PREF: RFC 4271 Section 5.1.5 forbids one toward an external peer,
	// and the sibling strip would remove it, which would confuse the reading.
	packed, err := hex.DecodeString(strings.ReplaceAll(
		"400101 00 4002 06 02010000fde9 c0280a 01000700000000000064", " ", ""))
	require.NoError(t, err)

	route := nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("192.0.2.0/24"), 0)
	adapter := &reactorAPIAdapter{r: &Reactor{config: &Config{LocalAS: 65000}}}

	update, buildErr := adapter.buildBatchAnnounceUpdate(
		make([]byte, message.MaxMsgLen), make([]byte, message.MaxMsgLen),
		bgptypes.NLRIBatch{
			Family:  family.IPv4Unicast,
			NLRIs:   []nlri.NLRI{route},
			NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("10.0.0.1")),
			Wire:    attribute.NewAttributesWire(packed, bgpctx.APIContextID),
		},
		netip.MustParseAddr("10.0.0.1"),
		false /*isIBGP: an EXTERNAL destination*/, false /*rsClient*/, true /*asn4*/, false /*addPath*/, 65000)
	require.NoError(t, buildErr)
	require.NotNil(t, update)

	_, _, _, found := attribute.AttrFind(update.PathAttributes, attribute.AttrPrefixSID)
	require.False(t, found,
		"RFC 8669 Section 8: \"The propagation to other ASes MUST be explicitly configured.\" "+
			"No leaf was set, so attribute 40 must not be in %x", update.PathAttributes)
}
