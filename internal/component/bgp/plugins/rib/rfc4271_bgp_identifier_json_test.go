// Design: docs/architecture/plugin/rib-storage-design.md -- best-path selection
// RFC: rfc/short/rfc4271.md -- Section 9.1.2.2, the ordered tie-breaking criteria
// Related: rfc4271_bgp_identifier_test.go -- the same requirement on the structured rail
// Related: format/text.go -- appendPeerJSON, which writes remote.router-id
// Related: rib.go -- updatePeerMetadata, which reads it back
package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/format"
	"github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	pluginapi "github.com/ze-software/ze/internal/component/plugin"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
)

// feedReceivedJSONWithIdentifier drives the JSON rail from end to end: it asks
// the shipped producer (format.AppendMessage) for the event text an external
// plugin process receives, then hands that text to the rib's own parser and
// dispatcher, which is the body of the OnEvent handler runRIBPlugin registers.
//
// Nothing here builds a peerMetadata or a Candidate. The defect this file pins
// lives between the two rails, so a test that starts after the parse cannot
// see it.
func feedReceivedJSONWithIdentifier(t *testing.T, r *RIBManager, peer netip.Addr, remoteRouterID uint32, ctxID bgpctx.ContextID, body []byte) {
	t.Helper()
	wu := wireu.NewWireUpdate(body, ctxID)
	attrs, err := wu.Attrs()
	require.NoError(t, err, "the test UPDATE body must parse")

	peerInfo := pluginapi.PeerInfo{
		Address:    peer,
		AddressStr: peer.String(),
		PeerAS:     65001,
		LocalAS:    65000,
		RouterID:   identifierFor(t, "192.0.2.254"), // this speaker's own, shared by every peer
		// The peer's own, which the JSON contract carries as remote.router-id.
		RemoteRouterID: remoteRouterID,
	}
	msg := types.RawMessage{
		Type:       msgtype.TypeUPDATE,
		RawBytes:   body,
		WireUpdate: wu,
		AttrsWire:  attrs,
	}
	// format=full is what the rib requires of an event (handleReceived refuses
	// one without raw fields), and json is the encoding an external plugin
	// process reads.
	line := format.AppendMessage(nil, &peerInfo, msg, types.ContentConfig{
		Encoding: pluginapi.EncodingJSON,
		Format:   pluginapi.FormatFull,
	})

	event, err := parseEvent(line)
	require.NoError(t, err, "the produced event must parse: %s", line)
	r.dispatch(event)
}

// VALIDATES: RFC 4271 Section 9.1.2.2 step f) reaches the decision process with
// the PEER's BGP Identifier when the events arrive as JSON, which is how every
// out-of-process plugin receives them. Boundary: the two candidates are
// identical through every earlier criterion, and the peer-address order is the
// REVERSE of the identifier order, so step f) and step g) name different
// winners and the test can tell which one decided.
// PREVENTS: the half-fix measured on 2026-08-31. The identifier was threaded to
// peerMetadata on the structured rail only. On the JSON rail the contract
// carried no peer identifier at all, updatePeerMetadata left the field zero,
// step f) tied on every comparison, and selection fell through to step g).
// One peer's best route then depended on how its events were delivered.
//
// RFC requirement: RFC4271-9.1.2.2-1 positive -- "The tie-breaking criteria MUST
// be applied in the order specified" (Section 9.1.2.2). Step f) is reached and
// decides: the peer whose BGP Identifier is lower wins, even though step g)
// would have chosen the other one.
func TestBestPathStepFComparesThePeerBGPIdentifierOnTheJSONRail(t *testing.T) {
	r := newTestRIBManager(t)
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	cidr := []byte{8, 10}

	// The LOWER peer address carries the HIGHER BGP Identifier, so step f) and
	// step g) disagree and the winner names the step that ran.
	lowAddress := netip.MustParseAddr("192.0.2.1")
	highAddress := netip.MustParseAddr("192.0.2.2")
	feedReceivedJSONWithIdentifier(t, r, lowAddress, identifierFor(t, "10.0.0.9"), ctxID, identifierTestUpdate())
	feedReceivedJSONWithIdentifier(t, r, highAddress, identifierFor(t, "10.0.0.1"), ctxID, identifierTestUpdate())

	candidates := r.gatherCandidates(family.IPv4Unicast, cidr)
	require.Len(t, candidates, 2, "both peers announced the prefix, so both are candidates")

	explanation := SelectBestExplain(candidates)
	require.NotNil(t, explanation)
	require.Len(t, explanation.Steps, 1, "two candidates are one pairwise comparison")
	assert.Equal(t, BestStepRouterID, explanation.Steps[0].Step,
		"the BGP Identifier step must decide; reaching the peer-address step means the JSON rail lost the identifier")
	assert.Equal(t, highAddress.String(), explanation.Winner.PeerAddr,
		"10.0.0.1 is the lower BGP Identifier, so its route wins even from the higher peer address")
}

// RFC requirement: RFC4271-9.1.2.2-1 negative -- the order is a sequence, not a
// single step: a criterion that TIES must hand the decision to the next one.
// Two peers reporting the same BGP Identifier over the JSON rail leave step f)
// undecided, and step g) then picks the lower peer address. Without this half,
// a JSON rail that hard-wired step f) to any single answer would pass the
// positive case.
func TestBestPathEqualBGPIdentifiersOnTheJSONRailFallThroughToPeerAddress(t *testing.T) {
	r := newTestRIBManager(t)
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	cidr := []byte{8, 10}

	shared := identifierFor(t, "10.0.0.7")
	lowAddress := netip.MustParseAddr("192.0.2.1")
	highAddress := netip.MustParseAddr("192.0.2.2")
	feedReceivedJSONWithIdentifier(t, r, lowAddress, shared, ctxID, identifierTestUpdate())
	feedReceivedJSONWithIdentifier(t, r, highAddress, shared, ctxID, identifierTestUpdate())

	candidates := r.gatherCandidates(family.IPv4Unicast, cidr)
	require.Len(t, candidates, 2)

	explanation := SelectBestExplain(candidates)
	require.NotNil(t, explanation)
	require.Len(t, explanation.Steps, 1)
	assert.Equal(t, BestStepPeerAddr, explanation.Steps[0].Step,
		"equal BGP Identifiers must not decide; the next criterion in the order does")
	assert.Equal(t, lowAddress.String(), explanation.Winner.PeerAddr,
		"step g) prefers the lower peer address")
}

// VALIDATES: the peer's BGP Identifier and this speaker's own are two different
// facts on one event, and the JSON contract keeps them apart. Boundary: both
// are present and they differ, which is the case a single shared `router-id`
// key cannot express.
// PREVENTS: a repair that reused the existing top-level router-id key, which
// names this speaker's identifier. Every candidate would again carry one number
// and step f) would again tie on every comparison.
func TestPeerEventCarriesTheRemoteAndLocalIdentifiersSeparately(t *testing.T) {
	r := newTestRIBManager(t)
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	peer := netip.MustParseAddr("192.0.2.1")

	feedReceivedJSONWithIdentifier(t, r, peer, identifierFor(t, "10.0.0.9"), ctxID, identifierTestUpdate())

	r.peerMu.RLock()
	meta := r.peerMeta[peer]
	r.peerMu.RUnlock()
	require.NotNil(t, meta, "a received UPDATE records the peer")
	assert.Equal(t, identifierFor(t, "10.0.0.9"), meta.RemoteRouterID,
		"the stored identifier is the peer's, not the 192.0.2.254 this speaker announced")
}
