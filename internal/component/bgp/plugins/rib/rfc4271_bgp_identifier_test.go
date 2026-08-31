// Design: docs/architecture/plugin/rib-storage-design.md -- best-path selection
// RFC: rfc/short/rfc4271.md -- Section 9.1.2.2, the ordered tie-breaking criteria
// Related: rib_commands.go -- extractCandidate, which reads the identifier
// Related: bestpath.go -- comparePair, step f) and step g)
package rib

import (
	"net/netip"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/bgp/wireu"
	bgpctx "github.com/ze-software/ze/internal/core/bgp/context"
	"github.com/ze-software/ze/internal/core/bgp/msgtype"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/pkg/plugin/rpc"
)

// feedReceivedWithIdentifier drives the received-UPDATE entry point with a peer
// whose OPEN carried remoteRouterID.
//
// It is the entry point on purpose. The comparison in comparePair reads
// Candidate.OriginatorIP and was always correct about it; the defect this file
// pins was in what FILLED that field, so a test that builds Candidates by hand
// passes either way and proves nothing (ai/rules/evidence.md).
func feedReceivedWithIdentifier(t *testing.T, r *RIBManager, peer netip.Addr, remoteRouterID uint32, ctxID bgpctx.ContextID, body []byte) {
	t.Helper()
	wu := wireu.NewWireUpdate(body, ctxID)
	attrs, _ := wu.Attrs()
	r.handleReceivedStructured(&rpc.StructuredEvent{
		EventType:      rpc.EventKindUpdate,
		PeerAddress:    peer.String(),
		PeerAS:         65001,
		LocalAS:        65000,
		RouterID:       identifierFor(t, "192.0.2.254"), // this speaker's own, shared by every peer
		RemoteRouterID: remoteRouterID,
		RawMessage: &types.RawMessage{
			Type:       msgtype.TypeUPDATE,
			RawBytes:   body,
			WireUpdate: wu,
			AttrsWire:  attrs,
		},
	})
}

// identifierFor spells a BGP Identifier the way RFC 4271 Section 4.2 defines
// it: a 32-bit unsigned integer whose octets read as an IPv4 address. The tests
// below name identifiers as addresses because that is how an operator sets one
// and how the decision process reports one.
func identifierFor(t *testing.T, address string) uint32 {
	t.Helper()
	octets := netip.MustParseAddr(address).As4()
	return uint32(octets[0])<<24 | uint32(octets[1])<<16 | uint32(octets[2])<<8 | uint32(octets[3])
}

// identifierTestUpdate announces 10.0.0.0/8 with the well-known mandatory
// attributes and nothing else, so two of these differ in no comparison before
// step f): ORIGIN is IGP on both, AS_PATH is empty on both (which also keeps
// the MED step from running, since it needs a first AS on each side), NEXT_HOP
// is the same address on both, and neither carries LOCAL_PREF.
func identifierTestUpdate() []byte {
	return []byte{
		0x00, 0x00, // Withdrawn Routes length 0
		0x00, 0x0e, // Total Path Attribute Length 14
		0x40, 0x01, 0x01, 0x00, // ORIGIN = IGP
		0x40, 0x02, 0x00, // AS_PATH = empty
		0x40, 0x03, 0x04, 0x0a, 0x00, 0x00, 0x01, // NEXT_HOP = 10.0.0.1
		0x08, 0x0a, // NLRI 10.0.0.0/8
	}
}

// VALIDATES: RFC 4271 Section 9.1.2.2 step f) reaches the decision process with
// the PEER's BGP Identifier, and yields to step g) only when two peers report
// the same one. Boundary: the two candidates are identical through every
// earlier criterion, and the peer-address order is the REVERSE of the
// identifier order, so the two steps name different winners and the test can
// tell which one decided.
// PREVENTS: the defect measured on 2026-08-31, where every candidate carried
// THIS speaker's identifier (plugin.PeerInfo.RouterID) instead of the peer's.
// Step f) then compared one number against itself on every pair, tied on every
// comparison, and selection silently fell through to step g). Ze was not
// implementing step f) at all, and no test could see it because the value was
// uniformly wrong rather than absent.
//
// RFC requirement: RFC4271-9.1.2.2-1 positive -- "The tie-breaking criteria MUST
// be applied in the order specified" (Section 9.1.2.2). Step f) is reached and
// decides: the peer whose BGP Identifier is lower wins, even though step g)
// would have chosen the other one.
func TestBestPathStepFComparesThePeerBGPIdentifier(t *testing.T) {
	r := newTestRIBManager(t)
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	cidr := []byte{8, 10}

	// The LOWER peer address carries the HIGHER BGP Identifier, so step f) and
	// step g) disagree and the winner names the step that ran.
	lowAddress := netip.MustParseAddr("192.0.2.1")
	highAddress := netip.MustParseAddr("192.0.2.2")
	feedReceivedWithIdentifier(t, r, lowAddress, identifierFor(t, "10.0.0.9"), ctxID, identifierTestUpdate())
	feedReceivedWithIdentifier(t, r, highAddress, identifierFor(t, "10.0.0.1"), ctxID, identifierTestUpdate())

	candidates := r.gatherCandidates(family.IPv4Unicast, cidr)
	require.Len(t, candidates, 2, "both peers announced the prefix, so both are candidates")

	explanation := SelectBestExplain(candidates)
	require.NotNil(t, explanation)
	require.Len(t, explanation.Steps, 1, "two candidates are one pairwise comparison")
	assert.Equal(t, BestStepRouterID, explanation.Steps[0].Step,
		"the BGP Identifier step must decide; reaching the peer-address step means step f) could not discriminate")
	assert.Equal(t, highAddress.String(), explanation.Winner.PeerAddr,
		"10.0.0.1 is the lower BGP Identifier, so its route wins even from the higher peer address")
}

// RFC requirement: RFC4271-9.1.2.2-1 negative -- the order is a sequence, not a
// single step: a criterion that TIES must hand the decision to the next one.
// Two peers reporting the same BGP Identifier leave step f) undecided, and step
// g) then picks the lower peer address. Without this half, a step f) hard-wired
// to any single answer would pass the positive case.
func TestBestPathEqualBGPIdentifiersFallThroughToPeerAddress(t *testing.T) {
	r := newTestRIBManager(t)
	ctxID, _ := bgpctx.Registry.Register(bgpctx.EncodingContextForASN4(true))
	cidr := []byte{8, 10}

	shared := identifierFor(t, "10.0.0.7")
	lowAddress := netip.MustParseAddr("192.0.2.1")
	highAddress := netip.MustParseAddr("192.0.2.2")
	feedReceivedWithIdentifier(t, r, lowAddress, shared, ctxID, identifierTestUpdate())
	feedReceivedWithIdentifier(t, r, highAddress, shared, ctxID, identifierTestUpdate())

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
