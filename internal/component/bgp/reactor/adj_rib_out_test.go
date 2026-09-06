// Design: docs/architecture/update-building.md -- the API origination rail these tests drive
// Related: adj_rib_out.go -- the Adj-RIB-Out under test
// Related: reactor_api_batch.go -- announceBatchToPeers and withdrawBatchFromPeers
// Overview: group_updates_framing_test.go -- the peer, reactor and frame-reading harness these borrow
//
// RFC 4271 Section 9.2: "A BGP speaker SHOULD NOT advertise a given feasible BGP
// route from its Adj-RIB-Out if it would produce an UPDATE message containing
// the same BGP route as was previously advertised."
//
// Until 2026-09-06 the API rails had no Adj-RIB-Out at all, so `announce route
// X` twice put two identical UPDATEs on the wire and four ported ExaBGP
// compatibility cases failed on the duplicate. These tests count the MESSAGES a
// peer's connection received, because the number of frames is the whole
// behavior: a test that counted a call inside the table would pass against a
// rail that suppresses the record and sends anyway.
package reactor

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/message"
	"github.com/ze-software/ze/internal/component/bgp/route"
	bgptypes "github.com/ze-software/ze/internal/component/bgp/types"
	"github.com/ze-software/ze/internal/component/plugin"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
	"github.com/ze-software/ze/internal/core/selector"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// adjOutBatch is a one-prefix IPv4 unicast batch toward the next hop given.
// One prefix is what an ExaBGP `announce route` line becomes, and it is the
// shape every failing compatibility case sends.
func adjOutBatch(prefix, nextHop string) bgptypes.NLRIBatch {
	return bgptypes.NLRIBatch{
		Family:  family.IPv4Unicast,
		NLRIs:   []nlri.NLRI{nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix(prefix), 0)},
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr(nextHop)),
	}
}

// TestAnnounceTwiceSendsOneUpdate is the defect's own case, and it is the shape
// test/exabgp-compat/etc/run/api-nexthop-self.run sends: the same
// `announce route 2.2.0.1/32 next-hop self` line twice in a row.
//
// VALIDATES: the second announce of a route already in the peer's Adj-RIB-Out,
// with the same attributes, puts nothing on the wire (RFC 4271 Section 9.2).
// PREVENTS: the duplicate UPDATE that failed api-nexthop-self, api-fast,
// api-api and api-reload against the ExaBGP fixtures.
func TestAnnounceTwiceSendsOneUpdate(t *testing.T) {
	peer, conn := newGroupUpdatesPeer(t, "10.0.0.2", "absent")
	adapter := groupUpdatesReactor([]*Peer{peer}, false)

	require.NoError(t, adapter.AnnounceNLRIBatch(selector.All(), adjOutBatch("2.2.0.1/32", "1.1.1.1"), plugin.OperatorSender()))
	require.Len(t, framesOf(t, conn, false), 1, "the first announce reaches the wire")

	require.NoError(t, adapter.AnnounceNLRIBatch(selector.All(), adjOutBatch("2.2.0.1/32", "1.1.1.1"), plugin.OperatorSender()))

	assert.Len(t, framesOf(t, conn, false), 1, "the second announce of an unchanged route sends nothing")
	assert.Equal(t, uint64(1), peer.adjOut.suppressedCount(),
		"a suppression is counted, so an operator can tell it from a dropped route")
}

// TestAnnounceTwiceReportsSuccess pins the CALLER's side of the suppression.
//
// A peer that already holds the route accepted the announce. Reporting
// ErrNoPeersAcceptedFamily would name a cause that is untrue, and
// DispatchNLRIGroups downgrades that error to a warning on the strength of it,
// so an operator's `announce` would answer "no peers accepted" for a route the
// peer has.
//
// VALIDATES: a fully suppressed announce returns nil.
// PREVENTS: the suppression turning every repeat announce into a warning.
func TestAnnounceTwiceReportsSuccess(t *testing.T) {
	peer, _ := newGroupUpdatesPeer(t, "10.0.0.2", "absent")
	adapter := groupUpdatesReactor([]*Peer{peer}, false)

	require.NoError(t, adapter.AnnounceNLRIBatch(selector.All(), adjOutBatch("2.2.0.1/32", "1.1.1.1"), plugin.OperatorSender()))
	assert.NoError(t, adapter.AnnounceNLRIBatch(selector.All(), adjOutBatch("2.2.0.1/32", "1.1.1.1"), plugin.OperatorSender()))
}

// TestAnnounceChangedNextHopSendsAgain is the other polarity, and it is the
// reason the comparison runs on the bytes rather than on the prefix.
//
// VALIDATES: a second announce of the same prefix with a different next hop
// reaches the wire.
// PREVENTS: a suppression keyed on the NLRI alone, which would blackhole every
// attribute change an operator makes to a route the peer already holds.
func TestAnnounceChangedNextHopSendsAgain(t *testing.T) {
	peer, conn := newGroupUpdatesPeer(t, "10.0.0.2", "absent")
	adapter := groupUpdatesReactor([]*Peer{peer}, false)

	require.NoError(t, adapter.AnnounceNLRIBatch(selector.All(), adjOutBatch("2.2.0.1/32", "1.1.1.1"), plugin.OperatorSender()))
	require.NoError(t, adapter.AnnounceNLRIBatch(selector.All(), adjOutBatch("2.2.0.1/32", "2.2.2.2"), plugin.OperatorSender()))

	assert.Len(t, framesOf(t, conn, false), 2, "a changed next hop is a changed route")
	assert.Equal(t, uint64(0), peer.adjOut.suppressedCount())
}

// TestWithdrawThenAnnounceSendsAgain pins the withdraw rail's obligation.
//
// VALIDATES: a withdrawal takes the route out of the Adj-RIB-Out, so the next
// announce of it is sent.
// PREVENTS: the table outliving the route it models, which would leave a peer
// with no route and this speaker convinced it had one -- a blackhole the
// operator cannot clear by re-announcing.
func TestWithdrawThenAnnounceSendsAgain(t *testing.T) {
	peer, conn := newGroupUpdatesPeer(t, "10.0.0.2", "absent")
	adapter := groupUpdatesReactor([]*Peer{peer}, false)

	require.NoError(t, adapter.AnnounceNLRIBatch(selector.All(), adjOutBatch("2.2.0.1/32", "1.1.1.1"), plugin.OperatorSender()))
	require.NoError(t, adapter.WithdrawNLRIBatch(selector.All(), adjOutBatch("2.2.0.1/32", "1.1.1.1"), plugin.OperatorSender()))
	require.NoError(t, adapter.AnnounceNLRIBatch(selector.All(), adjOutBatch("2.2.0.1/32", "1.1.1.1"), plugin.OperatorSender()))

	assert.Len(t, framesOf(t, conn, false), 3, "announce, withdraw, announce: three messages")
	assert.Equal(t, uint64(0), peer.adjOut.suppressedCount())
}

// TestReplayAnnounceIsNotSuppressed guards the one way this table can blackhole
// a prefix: answering a request to re-send with silence.
//
// `clear bgp rib out` and the RFC 2918 Section 3 route refresh behind it reach
// the RIB plugin's resend rail, which travels this same announce path with the
// session UP. Every route it resends is in the peer's Adj-RIB-Out by
// construction, so without the marker the resend would send nothing at all.
//
// VALIDATES: NLRIBatch.Replay sends a route the peer already holds.
// PREVENTS: a route refresh, and every operator resend, answering with silence.
func TestReplayAnnounceIsNotSuppressed(t *testing.T) {
	peer, conn := newGroupUpdatesPeer(t, "10.0.0.2", "absent")
	adapter := groupUpdatesReactor([]*Peer{peer}, false)

	require.NoError(t, adapter.AnnounceNLRIBatch(selector.All(), adjOutBatch("2.2.0.1/32", "1.1.1.1"), plugin.OperatorSender()))

	replay := adjOutBatch("2.2.0.1/32", "1.1.1.1")
	replay.Replay = true
	require.NoError(t, adapter.AnnounceNLRIBatch(selector.All(), replay, plugin.OperatorSender()))

	assert.Len(t, framesOf(t, conn, false), 2, "a replay re-sends what the peer already holds")
	assert.Equal(t, uint64(0), peer.adjOut.suppressedCount())
}

// TestSessionTeardownEmptiesAdjRIBOut pins the lifecycle.
//
// RFC 4271 Section 6.3 has a receiver delete every route learned over a session
// that closed, so a peer reached over a new connection holds nothing. A table
// kept across the teardown would suppress the re-advertisement the new session
// is owed.
//
// VALIDATES: clearEncodingContexts empties the table, so the same announce
// after a teardown reaches the wire again.
// PREVENTS: a reconnecting peer never being told the routes it lost.
func TestSessionTeardownEmptiesAdjRIBOut(t *testing.T) {
	peer, conn := newGroupUpdatesPeer(t, "10.0.0.2", "absent")
	adapter := groupUpdatesReactor([]*Peer{peer}, false)

	require.NoError(t, adapter.AnnounceNLRIBatch(selector.All(), adjOutBatch("2.2.0.1/32", "1.1.1.1"), plugin.OperatorSender()))

	peer.clearEncodingContexts()

	require.NoError(t, adapter.AnnounceNLRIBatch(selector.All(), adjOutBatch("2.2.0.1/32", "1.1.1.1"), plugin.OperatorSender()))

	assert.Len(t, framesOf(t, conn, false), 2, "a new session is owed the route again")
}

// TestAnnouncePartlyHeldSendsOnlyTheRest drives the case where one UPDATE
// carries several prefixes and the peer holds only some of them.
//
// The peer groups updates, so the batch is one build. Suppressing it whole would
// withhold a prefix the peer has never seen; sending it whole would re-advertise
// one it already holds. The rail builds again over the difference.
//
// VALIDATES: a two-prefix batch whose first prefix the peer already holds
// leaves as ONE UPDATE carrying only the second.
// PREVENTS: a per-BATCH suppression standing in for a per-ROUTE one.
func TestAnnouncePartlyHeldSendsOnlyTheRest(t *testing.T) {
	peer, conn := newGroupUpdatesPeer(t, "10.0.0.2", "absent")
	adapter := groupUpdatesReactor([]*Peer{peer}, false)

	require.NoError(t, adapter.AnnounceNLRIBatch(selector.All(), adjOutBatch("10.10.0.0/24", "1.1.1.1"), plugin.OperatorSender()))
	require.Len(t, framesOf(t, conn, false), 1)

	both := groupUpdatesBatch()
	require.Equal(t, "10.10.0.0/24", groupUpdatesPrefixes[0].String(), "the shared batch leads with the prefix already sent")
	require.NoError(t, adapter.AnnounceNLRIBatch(selector.All(), both, plugin.OperatorSender()))

	frames := framesOf(t, conn, false)
	require.Len(t, frames, 2, "the peer is owed one prefix, so it receives one more UPDATE")
	assert.Equal(t, []netip.Prefix{groupUpdatesPrefixes[1]}, frames[1],
		"only the prefix the peer did not hold")
	assert.Equal(t, uint64(1), peer.adjOut.suppressedCount())
}

// TestAdjRIBOutSignatureIsStableAcrossBatchShape is what makes the test above
// possible for an MP family, and it is the reason the signature cuts the
// MP_REACH_NLRI length octets as well as its payload.
//
// RFC 4760 Section 3 puts the NLRI inside MP_REACH_NLRI, so a batch of two
// prefixes and a batch of one produce different attribute BYTES for the same
// route. A signature that kept them would report every prefix as changed the
// moment it traveled beside a different number of neighbors.
//
// VALIDATES: one route's signature is the same whether it was built alone or
// beside another prefix, for an IPv6 unicast batch.
// PREVENTS: the suppression never firing for any family except IPv4 unicast.
func TestAdjRIBOutSignatureIsStableAcrossBatchShape(t *testing.T) {
	one := adjOutIPv6Signature(t, []string{"2001:db8:1::/48"})
	two := adjOutIPv6Signature(t, []string{"2001:db8:1::/48", "2001:db8:2::/48"})

	assert.Equal(t, one, two, "the batch's prefix count is not a property of any route in it")
}

// adjOutIPv6Signature builds one IPv6 unicast announce and returns the stored
// signature of the routes it carries.
func adjOutIPv6Signature(t *testing.T, prefixes []string) []byte {
	t.Helper()

	nlris := make([]nlri.NLRI, 0, len(prefixes))
	for _, prefix := range prefixes {
		nlris = append(nlris, nlri.NewINET(family.IPv6Unicast, netip.MustParsePrefix(prefix), 0))
	}
	batch := bgptypes.NLRIBatch{
		Family:  family.IPv6Unicast,
		NLRIs:   nlris,
		NextHop: bgptypes.NewNextHopExplicit(netip.MustParseAddr("2001:db8::1")),
	}

	peer, _ := newGroupUpdatesPeer(t, "10.0.0.9", "absent")
	adapter := groupUpdatesReactor([]*Peer{peer}, false)
	facts := announceFactsFor(peer, batch.Family, netip.MustParseAddr("2001:db8::1"), false, &NegotiatedCapabilities{})

	attrHandle := getBuildBuf()
	nlriHandle := getBuildBuf()
	defer putBuildBuf(attrHandle)
	defer putBuildBuf(nlriHandle)

	update, err := adapter.buildBatchAnnounceUpdate(attrHandle.Buf, nlriHandle.Buf, batch, facts)
	require.NoError(t, err)
	require.NotNil(t, update)

	built := newAnnounceUnit(update, nlriHandle.Buf, batch, facts)
	require.True(t, built.readable, "an MP_REACH_NLRI the builder wrote must be readable")
	return built.storedSignature()
}

// TestNLRIWireAtRefusesAnNLRIPastTheBlock is the reader's bound.
//
// nlriWireAt walks a block writeBatchNLRI produced by adding each NLRI's own
// length, and it is the only reader of that layout. A disagreement between the
// length and the write would slice past the block, so the bound is checked
// rather than trusted.
//
// VALIDATES: an NLRI whose encoding does not fit the bytes left answers false
// and returns no slice.
// PREVENTS: a panic on the announce fan-out, reachable from a length function
// that under-reports what the writer wrote.
func TestNLRIWireAtRefusesAnNLRIPastTheBlock(t *testing.T) {
	route := nlri.NewINET(family.IPv4Unicast, netip.MustParsePrefix("10.0.0.0/24"), 0)
	size := nlri.LenWithContext(route, false)
	require.Positive(t, size, "an IPv4 /24 has a wire length")

	block := make([]byte, size)

	wire, end, ok := nlriWireAt(block, route, false, 0)
	require.True(t, ok, "the whole NLRI fits at offset zero")
	assert.Len(t, wire, size)
	assert.Equal(t, size, end)

	_, _, ok = nlriWireAt(block, route, false, 1)
	assert.False(t, ok, "one octet short is one octet too few")

	_, _, ok = nlriWireAt(block[:size-1], route, false, 0)
	assert.False(t, ok, "a truncated block cannot answer for a whole NLRI")
}

// TestAnnounceSignatureRefusesAnUnreadableBlock is the signature reader's bound,
// and its answer is what makes an unreadable build behave as the rail did before
// the Adj-RIB-Out existed.
//
// VALIDATES: an MP family whose attribute block carries no MP_REACH_NLRI, and
// one whose MP_REACH_NLRI is shorter than the NLRI it was given, both answer
// false.
// PREVENTS: a signature read out of the wrong bytes, which would suppress a
// route whose wire had actually changed.
func TestAnnounceSignatureRefusesAnUnreadableBlock(t *testing.T) {
	// One ORIGIN attribute and nothing else: flags 0x40, code 1, length 1.
	origin := []byte{0x40, 0x01, 0x01, 0x00}

	_, ok := announceSignature(origin, family.IPv6Unicast, 4)
	assert.False(t, ok, "an MP family with no MP_REACH_NLRI cannot be read per route")

	// MP_REACH_NLRI (code 14) with a three-octet value, asked for a four-octet NLRI.
	shortMPReach := []byte{0x80, 0x0E, 0x03, 0x00, 0x02, 0x01}

	_, ok = announceSignature(shortMPReach, family.IPv6Unicast, 4)
	assert.False(t, ok, "an MP_REACH_NLRI shorter than its NLRI cannot be read per route")

	_, ok = announceSignature(origin, family.IPv4Unicast, 0)
	assert.True(t, ok, "IPv4 unicast carries its NLRI outside the attributes")
}

// TestWithdrawWithheldUntilSessionAdvertises is api-fast batch 1's first line.
//
// RFC 4271 Section 4.3 identifies a withdrawn route by its destination, "which
// unambiguously identifies the route in the context of the BGP speaker - BGP
// speaker connection to which it has been previously advertised". A connection
// that has advertised nothing has no route for a withdrawal to name, so nothing
// is written.
//
// VALIDATES: the first withdrawal of a session puts no UPDATE on the wire, the
// count rises, and the answer NAMES the peer it was withheld from.
// PREVENTS: the withdrawal of 1.1.0.0/24 that api-fast does not record, and the
// silent no-op that a zero UPDATE count with a bare success would be
// (ai/rules/principles.md).
func TestWithdrawWithheldUntilSessionAdvertises(t *testing.T) {
	peer, conn := newGroupUpdatesPeer(t, "10.0.0.2", "absent")
	adapter := groupUpdatesReactor([]*Peer{peer}, false)

	err := adapter.WithdrawNLRIBatch(selector.All(), adjOutBatch("1.1.0.0/24", "1.1.1.1"), plugin.OperatorSender())

	require.ErrorIs(t, err, route.ErrWithdrawWithheld, "the answer carries the reason")
	assert.Contains(t, err.Error(), "10.0.0.2", "and it names the peer it was withheld from")
	assert.Empty(t, framesOf(t, conn, true), "no UPDATE reaches a peer this session has advertised nothing to")
	assert.Equal(t, uint64(1), peer.adjOut.withheldCount())
}

// TestWithdrawSentAfterAnyNLRIAdvertised is the other polarity, and it is what
// keeps the guard from swallowing a withdrawal an operator meant.
//
// The state arms on any UPDATE that makes a destination reachable, not on an
// API announce alone: a peer holding config-declared or forwarded routes has
// been advertised something and its operator may withdraw it.
//
// VALIDATES: after ONE announce, a withdrawal of a DIFFERENT route -- one the
// peer has never held -- is written.
// PREVENTS: a key-based rule standing in for the connection-based one, which
// would drop api-fast batch 2's withdrawal of 2.2.0.0/25.
func TestWithdrawSentAfterAnyNLRIAdvertised(t *testing.T) {
	peer, conn := newGroupUpdatesPeer(t, "10.0.0.2", "absent")
	adapter := groupUpdatesReactor([]*Peer{peer}, false)

	require.NoError(t, adapter.AnnounceNLRIBatch(selector.All(), adjOutBatch("2.2.0.0/24", "1.1.1.1"), plugin.OperatorSender()))
	require.NoError(t, adapter.WithdrawNLRIBatch(selector.All(), adjOutBatch("2.2.0.0/25", "1.1.1.1"), plugin.OperatorSender()))

	assert.Len(t, framesOf(t, conn, false), 2, "the announce, then the withdrawal of a route the peer never held")
	assert.Equal(t, uint64(0), peer.adjOut.withheldCount())
}

// TestEndOfRIBDoesNotArmTheWithdrawGuard pins the one message that must not
// count as an advertisement.
//
// RFC 4724 Section 2 makes the marker an UPDATE with no reachable NLRI and no
// withdrawn routes. Every compat case sends one before its script runs, so a
// marker that armed the guard would leave api-fast batch 1 withdrawing
// 1.1.0.0/24 exactly as it did before this work.
//
// VALIDATES: an End-of-RIB leaves the connection unadvertised.
// PREVENTS: the guard being armed by every session that reaches Established.
func TestEndOfRIBDoesNotArmTheWithdrawGuard(t *testing.T) {
	peer, _ := newGroupUpdatesPeer(t, "10.0.0.2", "absent")

	require.NoError(t, peer.SendUpdate(message.BuildEOR(family.IPv4Unicast)))

	assert.False(t, peer.hasAdvertised(), "an End-of-RIB advertises no route")
}

// TestAdvertisedStateClearedOnTeardown pins the lifecycle of the guard.
//
// RFC 4271 Section 4.3 scopes "previously advertised" to one BGP speaker to BGP
// speaker CONNECTION, so a peer reached over a new connection has advertised
// nothing again. The state lives on the Session for that reason, and a torn
// down peer holds none.
//
// VALIDATES: a peer whose session has gone answers hasAdvertised false, so its
// next session withholds its first withdrawal as this one did.
// PREVENTS: a reconnecting peer inheriting the previous connection's state,
// which would send a withdrawal naming a route the new connection never carried.
func TestAdvertisedStateClearedOnTeardown(t *testing.T) {
	peer, _ := newGroupUpdatesPeer(t, "10.0.0.2", "absent")
	adapter := groupUpdatesReactor([]*Peer{peer}, false)

	require.NoError(t, adapter.AnnounceNLRIBatch(selector.All(), adjOutBatch("2.2.0.1/32", "1.1.1.1"), plugin.OperatorSender()))
	require.True(t, peer.hasAdvertised(), "the announce armed this connection")

	peer.clearEncodingContexts()
	peer.mu.Lock()
	peer.session = nil
	peer.mu.Unlock()

	assert.False(t, peer.hasAdvertised(), "the connection that advertised is gone")
}
