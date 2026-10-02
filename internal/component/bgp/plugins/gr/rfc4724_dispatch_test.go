// Design: docs/guide/graceful-restart.md -- Receiving Speaker procedures
// RFC: rfc/short/rfc4724.md -- Section 4.2, deleting stale routes on End-of-RIB, Restart Time
// expiry and re-establishment without preserved forwarding state
// Overview: gr.go -- handleEOREvent, handleStateEvent and onTimerExpired turn each trigger into
// a RIB command
// Related: rfc4724_retention_test.go -- newRecordedGRPlugin, the production callbacks with the
// dispatch captured

package gr

import (
	"slices"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
)

// rfc4724RetainingPlugin builds a plugin with the production callbacks whose peer advertised
// cap and then lost its TCP session, so its routes are retained and marked stale. The
// dispatches of that session-down sequence are not returned: each test reads only the
// commands its own trigger produces.
func rfc4724RetainingPlugin(t *testing.T, cap *grPeerCap) (*grPlugin, func() []string) {
	t.Helper()
	gp, rec := newRecordedGRPlugin()
	gp.peerCaps[testPeer] = cap
	gp.handleStateEvent(testPeer, map[string]any{"state": "down", "reason": "tcp-failure"})
	before := len(rec.all())
	require.Equal(t, 3, before, "the session-down sequence purges, retains and marks stale")
	return gp, func() []string { return rec.all()[before:] }
}

// rfc4724EOR is the End-of-RIB event payload handleEOREvent reads.
func rfc4724EOR(fam family.Family) map[string]any {
	return map[string]any{"eor": map[string]any{"family": fam.String()}}
}

// TestRFC4724EndOfRIBRemovesThatFamilysStaleRoutes delivers an End-of-RIB for IPv4 unicast
// while both IPv4 and IPv6 unicast routes are retained as stale.
//
// VALIDATES: RFC 4724 Section 4.2 -- "Once the End-of-RIB marker for an address family is
// received from the peer, it MUST immediately remove any routes from the peer that are still
// marked as stale for that address family." handleEOREvent dispatches
// "request bgp rib purge-stale <peer> ipv4/unicast" at once; TestRIBPurgeStaleFamilyCommand
// (plugins/rib) proves that command removes the family's stale routes.
// PREVENTS: onEORReceived answering true while no RIB command is sent.
//
// RFC requirement: RFC4724-4.2-11 positive -- an End-of-RIB for IPv4 unicast from a peer whose routes are retained as stale dispatches exactly one command, purge-stale for that peer and IPv4 unicast, before the handler returns.
func TestRFC4724EndOfRIBRemovesThatFamilysStaleRoutes(t *testing.T) {
	gp, sent := rfc4724RetainingPlugin(t, testCap(120, famIPv4, famIPv6))

	gp.handleEOREvent(testPeer, rfc4724EOR(family.IPv4Unicast))

	assert.Equal(t, []string{
		"request bgp rib purge-stale " + testPeer + " " + family.IPv4Unicast.String(),
	}, sent(), "End-of-RIB removes the stale routes of its own family")
}

// TestRFC4724EndOfRIBLeavesOtherFamiliesStale delivers the IPv4 unicast End-of-RIB and then
// one for a family the peer never retained.
//
// VALIDATES: RFC 4724 Section 4.2 -- the removal is scoped "for that address family": the
// IPv6 unicast stale routes are not purged by the IPv4 End-of-RIB, and an End-of-RIB for a
// family with no stale routes sends nothing.
// PREVENTS: a purge of the whole peer on the first End-of-RIB, which deletes the routes of
// families whose End-of-RIB has not arrived.
//
// RFC requirement: RFC4724-4.2-11 negative -- the IPv4 unicast End-of-RIB never dispatches a purge for IPv6 unicast or for the whole peer, and an End-of-RIB for IPv4 multicast, a family the peer did not retain, dispatches nothing.
func TestRFC4724EndOfRIBLeavesOtherFamiliesStale(t *testing.T) {
	gp, sent := rfc4724RetainingPlugin(t, testCap(120, famIPv4, famIPv6))

	gp.handleEOREvent(testPeer, rfc4724EOR(family.IPv4Unicast))
	gp.handleEOREvent(testPeer, rfc4724EOR(family.IPv4Multicast))

	got := sent()
	assert.NotContains(t, got, "request bgp rib purge-stale "+testPeer+" "+family.IPv6Unicast.String(),
		"IPv6 unicast has not sent its End-of-RIB, so its stale routes stay")
	assert.NotContains(t, got, "request bgp rib purge-stale "+testPeer,
		"an End-of-RIB never purges the whole peer")
	assert.Len(t, got, 1, "only the IPv4 unicast End-of-RIB dispatches a command")
}

// TestRFC4724RestartTimeExpiryDeletesTheStaleRoutes lets a one-second Restart Time elapse
// while the session stays down.
//
// VALIDATES: RFC 4724 Section 4.2 -- "If the session does not get re-established within the
// 'Restart Time' that the peer advertised previously, the Receiving Speaker MUST delete all
// the stale routes from the peer that it is retaining." The production timer callback
// (onTimerExpired) dispatches "request bgp rib release-routes <peer>", the command that
// deletes every retained route of the peer (rib_commands.go).
// PREVENTS: the timer firing into a callback that deletes nothing.
//
// RFC requirement: RFC4724-4.2-7 positive -- with a Restart Time of 1 second and no re-establishment, the plugin dispatches release-routes for the peer once the time elapses.
func TestRFC4724RestartTimeExpiryDeletesTheStaleRoutes(t *testing.T) {
	_, sent := rfc4724RetainingPlugin(t, testCap(1, famIPv4, famIPv6))

	require.Eventually(t, func() bool {
		return slices.Contains(sent(), "request bgp rib release-routes "+testPeer)
	}, 5*time.Second, 20*time.Millisecond, "the Restart Time elapsed and the stale routes were kept")
}

// TestRFC4724ReestablishedWithinRestartTimeKeepsTheRoutes re-establishes the session before
// the one-second Restart Time elapses, then waits past it.
//
// VALIDATES: RFC 4724 Section 4.2 -- the deletion is conditional on the session NOT being
// re-established within the Restart Time: re-established in time, with the Forwarding State
// bit set for every family, no route is released or purged.
// PREVENTS: a Restart Time timer that keeps running after re-establishment and deletes the
// routes the restarting peer is about to refresh.
//
// RFC requirement: RFC4724-4.2-7 negative -- re-established within a 1-second Restart Time with F set for both families, nothing is dispatched during the following 2 seconds: no release-routes, no purge-stale.
func TestRFC4724ReestablishedWithinRestartTimeKeepsTheRoutes(t *testing.T) {
	gp, sent := rfc4724RetainingPlugin(t, testCap(1, famIPv4, famIPv6))

	gp.handleStateEvent(testPeer, map[string]any{"state": "up"})
	time.Sleep(2 * time.Second)

	assert.Empty(t, sent(), "a session re-established in time keeps its routes")
}

// TestRFC4724ReestablishedWithoutForwardingStatePurgesTheFamily re-establishes the session
// with each of the three capability shapes Section 4.2 names.
//
// VALIDATES: RFC 4724 Section 4.2 -- "if the 'Forwarding State' bit for a specific address
// family is not set in the newly received Graceful Restart Capability, or if a specific
// address family is not included in the newly received Graceful Restart Capability, or if
// the Graceful Restart Capability is not received in the re-established session at all, then
// the Receiving Speaker MUST immediately remove all the stale routes from the peer that it is
// retaining for that address family." handleStateEvent dispatches purge-stale for exactly the
// families concerned when the "up" event arrives; TestRIBPurgeStaleFamilyCommand (plugins/rib)
// proves the command removes them.
// PREVENTS: onSessionReestablished answering the right families while nothing is purged.
//
// RFC requirement: RFC4724-4.2-8 positive -- on re-establishment the plugin dispatches purge-stale for IPv4 unicast when its F bit is clear, for IPv6 unicast when the new capability omits it, and for both families when no Graceful Restart Capability is received.
func TestRFC4724ReestablishedWithoutForwardingStatePurgesTheFamily(t *testing.T) {
	purgeIPv4 := "request bgp rib purge-stale " + testPeer + " " + family.IPv4Unicast.String()
	purgeIPv6 := "request bgp rib purge-stale " + testPeer + " " + family.IPv6Unicast.String()
	cases := []struct {
		name   string
		newCap *grPeerCap
		want   []string
	}{
		{"forwarding state bit clear", testCap(120, famIPv4NoF, famIPv6), []string{purgeIPv4}},
		{"family not included", testCap(120, famIPv4), []string{purgeIPv6}},
		{"capability not received", nil, []string{purgeIPv4, purgeIPv6}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			gp, sent := rfc4724RetainingPlugin(t, testCap(120, famIPv4, famIPv6))
			if tc.newCap == nil {
				delete(gp.peerCaps, testPeer)
			} else {
				gp.peerCaps[testPeer] = tc.newCap
			}

			gp.handleStateEvent(testPeer, map[string]any{"state": "up"})

			assert.ElementsMatch(t, tc.want, sent(), "exactly the families without preserved forwarding state are purged")
		})
	}
}

// TestRFC4724ReestablishedWithForwardingStateKeepsTheRoutes re-establishes the session with
// a capability that lists both families with the Forwarding State bit set.
//
// VALIDATES: RFC 4724 Section 4.2 -- removal on re-establishment is conditional on the F bit
// being clear or the family being absent: with F set for every retained family, the stale
// routes stay until End-of-RIB.
// PREVENTS: a purge of every family on re-establishment, which deletes the routes the peer
// preserved forwarding state for.
//
// RFC requirement: RFC4724-4.2-8 negative -- re-established with F set for IPv4 and IPv6 unicast, the plugin dispatches no purge-stale at all.
func TestRFC4724ReestablishedWithForwardingStateKeepsTheRoutes(t *testing.T) {
	gp, sent := rfc4724RetainingPlugin(t, testCap(120, famIPv4, famIPv6))

	gp.handleStateEvent(testPeer, map[string]any{"state": "up"})

	assert.Empty(t, sent(), "F set for every family: nothing is purged on re-establishment")
}
