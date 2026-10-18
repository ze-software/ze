// Design: docs/guide/graceful-restart.md -- LLGR reconnect purge rules
// RFC: rfc/short/rfc9494.md -- Section 4.2, removal on re-establishment
// Related: gr_state.go -- onSessionReestablished, reconnectKeepsFamily

package gr

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
)

// llgrReconnectCap builds an LLGR capability listing each family with the given
// F bit and a nonzero Long-Lived Stale Time.
func llgrReconnectCap(families ...llgrCapFamily) *llgrPeerCap {
	return &llgrPeerCap{Families: families}
}

// downIntoLLGR drops the session of a peer whose GR capability has Restart Time
// zero, so every family listed in llgrCap enters its LLGR period at once.
func downIntoLLGR(t *testing.T, mgr *grStateManager, llgrCap *llgrPeerCap, families ...grCapFamily) {
	t.Helper()
	mgr.onLLGREnter = func(string, family.Family, uint32) {}
	mgr.onLLGRFamilyExpired = func(string, family.Family) {}
	require.True(t, mgr.onSessionDown(testPeer, testCap(0, families...), llgrCap, false), "GR activates")
	require.True(t, mgr.peerActive(testPeer), "the peer is in its LLGR period")
}

// TestOnSessionReestablished_LLGRFBitClearPurges verifies the first Section 4.2
// bullet: a family in its LLGR period whose F bit the new LLGR capability clears
// is purged, even though the new GR capability sets its F bit.
//
// VALIDATES: the new LLGR capability decides for an LLGR-period family.
// PREVENTS: the GR F bit keeping a family the new LLGR capability gave up on,
// which is what the former union of the two F-bit sets did.
//
// RFC requirement: RFC9494-4.2-8 positive -- reconnect in the LLGR period, new GR
// lists ipv4/unicast with F set, new LLGR lists it with F clear: onSessionReestablished
// returns ipv4/unicast for immediate removal and releases the peer state.
// MUTATION: make reconnectKeepsFamily return true once the GR F bit is set, and this goes red.
func TestOnSessionReestablished_LLGRFBitClearPurges(t *testing.T) {
	t.Parallel()

	mgr := newGRStateManager(nil)
	downIntoLLGR(t, mgr, llgrCapIPv4, famIPv4)

	cleared := llgrReconnectCap(llgrCapFamily{Family: family.IPv4Unicast, ForwardState: false, LLST: 3600})
	purged, wasLLGR := mgr.onSessionReestablished(testPeer, testCap(120, famIPv4), cleared)

	assert.True(t, wasLLGR, "the peer was in its LLGR period")
	assert.Equal(t, []family.Family{family.IPv4Unicast}, purged, "F bit clear in the new LLGR capability purges the family")
	assert.False(t, mgr.peerActive(testPeer), "no stale family remains")
}

// TestOnSessionReestablished_FamilyAbsentFromLLGRPurges verifies the second
// Section 4.2 bullet: a family in its LLGR period that the new LLGR capability
// omits is purged, while a sibling it lists with F set is kept.
//
// VALIDATES: omission from the new LLGR capability purges, family by family.
// PREVENTS: a purge that takes the whole peer, or none of it.
//
// RFC requirement: RFC9494-4.2-8 positive -- reconnect in the LLGR period, new GR lists
// ipv4/unicast and ipv6/unicast with F set, new LLGR lists only ipv6/unicast with F set:
// onSessionReestablished returns exactly ipv4/unicast and keeps ipv6/unicast stale.
func TestOnSessionReestablished_FamilyAbsentFromLLGRPurges(t *testing.T) {
	t.Parallel()

	mgr := newGRStateManager(nil)
	both := llgrReconnectCap(
		llgrCapFamily{Family: family.IPv4Unicast, ForwardState: true, LLST: 3600},
		llgrCapFamily{Family: family.IPv6Unicast, ForwardState: true, LLST: 3600},
	)
	downIntoLLGR(t, mgr, both, famIPv4, famIPv6)

	onlyIPv6 := llgrReconnectCap(llgrCapFamily{Family: family.IPv6Unicast, ForwardState: true, LLST: 3600})
	purged, _ := mgr.onSessionReestablished(testPeer, testCap(120, famIPv4, famIPv6), onlyIPv6)

	assert.Equal(t, []family.Family{family.IPv4Unicast}, purged, "the family the new LLGR capability omits is purged")
	assert.True(t, mgr.familyRetained(testPeer, family.IPv6Unicast), "the family it lists with F set stays stale until EOR")
	assert.False(t, mgr.familyRetained(testPeer, family.IPv4Unicast), "the purged family is no longer retained")
}

// TestOnSessionReestablished_NoLLGRCapPurgesLLGRPeriodFamily verifies that a
// re-established session carrying GR but no LLGR capability omits every family
// from the LLGR capability, so an LLGR-period family is purged.
//
// VALIDATES: a nil new LLGR capability is read as one that includes no family.
// PREVENTS: a nil capability skipping the LLGR bullets altogether.
//
// RFC requirement: RFC9494-4.2-8 positive -- reconnect in the LLGR period with a GR capability
// listing ipv4/unicast with F set and no LLGR capability: onSessionReestablished returns
// ipv4/unicast, because "a specific address family is not included in the newly received LLGR
// Capability".
func TestOnSessionReestablished_NoLLGRCapPurgesLLGRPeriodFamily(t *testing.T) {
	t.Parallel()

	mgr := newGRStateManager(nil)
	downIntoLLGR(t, mgr, llgrCapIPv4, famIPv4)

	purged, _ := mgr.onSessionReestablished(testPeer, testCap(120, famIPv4), nil)

	assert.Equal(t, []family.Family{family.IPv4Unicast}, purged)
	assert.False(t, mgr.peerActive(testPeer))
}

// TestOnSessionReestablished_LLGRPeriodGRFBitClearPurges verifies that the RFC
// 4724 rule still applies in the LLGR period: new LLGR sets the F bit, new GR
// clears it, and the family is purged.
//
// VALIDATES: the AND of the two rules (owner decision Q-1, 2026-10-14).
// PREVENTS: the LLGR F bit rescuing a family the GR capability gave up on.
func TestOnSessionReestablished_LLGRPeriodGRFBitClearPurges(t *testing.T) {
	t.Parallel()

	mgr := newGRStateManager(nil)
	downIntoLLGR(t, mgr, llgrCapIPv4, famIPv4)

	purged, _ := mgr.onSessionReestablished(testPeer, testCap(120, famIPv4NoF), llgrCapIPv4)

	assert.Equal(t, []family.Family{family.IPv4Unicast}, purged, "GR F bit clear purges whatever LLGR says")
}

// TestOnSessionReestablished_BothFBitsSetKeepsLLGRPeriodFamily verifies the
// negative polarity: a family in its LLGR period that both new capabilities list
// with the F bit set is kept until its EOR.
//
// VALIDATES: the removal is conditional on a bullet holding.
// PREVENTS: a fix that purges every LLGR-period family on reconnect.
//
// RFC requirement: RFC9494-4.2-8 negative -- reconnect in the LLGR period, new GR and new
// LLGR both list ipv4/unicast with F set: onSessionReestablished returns nothing and the
// family stays retained.
func TestOnSessionReestablished_BothFBitsSetKeepsLLGRPeriodFamily(t *testing.T) {
	t.Parallel()

	mgr := newGRStateManager(nil)
	downIntoLLGR(t, mgr, llgrCapIPv4, famIPv4)

	purged, _ := mgr.onSessionReestablished(testPeer, testCap(120, famIPv4), llgrCapIPv4)

	assert.Empty(t, purged, "no bullet holds, so nothing is removed")
	assert.True(t, mgr.familyRetained(testPeer, family.IPv4Unicast), "stale routes are kept until EOR")
}

// TestOnSessionReestablished_GRPeriodFamilyIgnoresLLGRBullets verifies that the
// LLGR bullets bind only "once the LLGR Period begins": a family still in its
// conventional GR period is kept when the new LLGR capability omits it, even
// though a sibling family of the same peer is in its LLGR period.
//
// VALIDATES: the LLGR period is per family, not per peer.
// PREVENTS: a peer-wide in-LLGR switch applying the LLGR bullets to a GR-period family.
//
// RFC requirement: RFC9494-4.2-8 negative -- ipv4/unicast is in its GR period (listed in
// the old GR capability, Restart Time 120) and ipv6/unicast in its LLGR period (omitted from
// the old GR capability); the new LLGR capability lists only ipv6/unicast: onSessionReestablished
// returns nothing, and ipv4/unicast stays retained.
func TestOnSessionReestablished_GRPeriodFamilyIgnoresLLGRBullets(t *testing.T) {
	t.Parallel()

	mgr := newGRStateManager(nil)
	mgr.onLLGREnter = func(string, family.Family, uint32) {}
	mgr.onLLGRFamilyExpired = func(string, family.Family) {}
	both := llgrReconnectCap(
		llgrCapFamily{Family: family.IPv4Unicast, ForwardState: true, LLST: 3600},
		llgrCapFamily{Family: family.IPv6Unicast, ForwardState: true, LLST: 3600},
	)
	require.True(t, mgr.onSessionDown(testPeer, testCap(120, famIPv4), both, false))
	require.True(t, mgr.familyRetained(testPeer, family.IPv6Unicast), "ipv6/unicast entered LLGR at DOWN")

	onlyIPv6 := llgrReconnectCap(llgrCapFamily{Family: family.IPv6Unicast, ForwardState: true, LLST: 3600})
	purged, wasLLGR := mgr.onSessionReestablished(testPeer, testCap(120, famIPv4, famIPv6), onlyIPv6)

	assert.True(t, wasLLGR, "the peer had a family in its LLGR period")
	assert.Empty(t, purged, "ipv4/unicast is judged by the RFC 4724 rule alone")
	assert.True(t, mgr.familyRetained(testPeer, family.IPv4Unicast))
	assert.True(t, mgr.familyRetained(testPeer, family.IPv6Unicast))
}
