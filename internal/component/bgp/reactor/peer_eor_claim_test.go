// Tests for the per-session End-of-RIB claim that keeps the two EoR producers
// (initial sync, and a route server's post-replay AnnounceEOR) from both putting
// the same family's marker on the wire.

package reactor

import (
	"sync"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/ze-software/ze/internal/core/family"
)

// TestInitialSyncEORClaimedOncePerFamilyPerSession pins RFC 4724 Section 2: one
// End-of-RIB per family per session, however many producers race for it.
//
// VALIDATES: the first claimant for a family wins; every later claimant is told
// to stand down, and a different family is unaffected.
// PREVENTS: a peer receiving the same family's End-of-RIB twice. The previous
// de-duplicator was a TIME-WINDOW test (AnnounceEOR gating on shouldQueue, i.e.
// on sendingInitialRoutes still being set); when a route-server replay finished
// after that flag cleared, the guard failed open and the marker went out twice.
func TestInitialSyncEORClaimedOncePerFamilyPerSession(t *testing.T) {
	p := &Peer{}

	require.True(t, p.claimInitialSyncEOR(family.IPv4Unicast), "first claimant sends")
	require.False(t, p.claimInitialSyncEOR(family.IPv4Unicast), "second claimant must stand down")
	require.False(t, p.claimInitialSyncEOR(family.IPv4Unicast), "and stays stood down")

	require.True(t, p.claimInitialSyncEOR(family.IPv6Unicast),
		"a different family still owes the peer its own marker")
}

// TestInitialSyncEORClaimReleasedOnFailedSend pins that a claim whose send failed
// does not strand the family.
//
// VALIDATES: releaseInitialSyncEOR lets the other producer deliver the marker.
// PREVENTS: the opposite failure to the duplicate -- claiming, failing to write,
// and leaving the peer with NO End-of-RIB for that family, which is the barrier
// the functional suite waits on.
func TestInitialSyncEORClaimReleasedOnFailedSend(t *testing.T) {
	p := &Peer{}

	require.True(t, p.claimInitialSyncEOR(family.IPv4Unicast))
	p.releaseInitialSyncEOR(family.IPv4Unicast) // send failed, nothing on the wire
	require.True(t, p.claimInitialSyncEOR(family.IPv4Unicast),
		"after a released claim the marker is still owed, so the next producer may send it")
}

// TestInitialSyncEORResetPerSession pins that a reconnect owes a fresh marker.
//
// VALIDATES: resetInitialSyncEOR clears every claim, as the session teardown
// defer does.
// PREVENTS: a reconnected peer never receiving End-of-RIB because the previous
// session's claim survived.
func TestInitialSyncEORResetPerSession(t *testing.T) {
	p := &Peer{}

	require.True(t, p.claimInitialSyncEOR(family.IPv4Unicast))
	require.False(t, p.claimInitialSyncEOR(family.IPv4Unicast))

	p.resetInitialSyncEOR()

	require.True(t, p.claimInitialSyncEOR(family.IPv4Unicast),
		"a new session owes the peer End-of-RIB again")
}

// TestInitialSyncEORClaimIsAtomicUnderRace pins that the claim is check-and-record
// in one step.
//
// VALIDATES: with many goroutines claiming the same family concurrently, exactly
// one is granted.
// PREVENTS: a check-then-mark split, where both producers pass the check before
// either records, which is precisely the shape of the bug being fixed -- the two
// producers run on different goroutines (sendInitialRoutes and the route server's
// replay goroutine).
func TestInitialSyncEORClaimIsAtomicUnderRace(t *testing.T) {
	p := &Peer{}

	const claimants = 64
	var wg sync.WaitGroup
	granted := make(chan struct{}, claimants)

	wg.Add(claimants)
	for range claimants {
		go func() {
			defer wg.Done()
			if p.claimInitialSyncEOR(family.IPv4Unicast) {
				granted <- struct{}{}
			}
		}()
	}
	wg.Wait()
	close(granted)

	require.Len(t, granted, 1, "exactly one goroutine may put the marker on the wire")
}
