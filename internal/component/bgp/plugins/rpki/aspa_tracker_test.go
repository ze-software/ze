package rpki

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// TestASPATrackerAdd verifies route tracking with reverse index.
//
// VALIDATES: AC-5 — routes tracked for re-validation with correct path data.
// PREVENTS: Routes lost or reverse index not populated.
func TestASPATrackerAdd(t *testing.T) {
	tr := NewASPATracker()
	tr.Track(trackedRoute{
		key:       routeKey{peerAddr: "10.0.0.1", family: "ipv4/unicast", prefix: "192.168.0.0/24", pathID: 0},
		peerName:  "peer1",
		peerASN:   64500,
		msgID:     1,
		path:      []uint32{64500, 64501, 64502},
		aspaState: ASPAValid,
	})

	assert.Equal(t, 1, tr.count())
}

// TestASPATrackerRemove verifies route removal from tracker and reverse index.
//
// VALIDATES: Withdrawal removes route from primary map and reverse index.
// PREVENTS: Stale entries causing spurious re-validation.
func TestASPATrackerRemove(t *testing.T) {
	tr := NewASPATracker()
	key := routeKey{peerAddr: "10.0.0.1", family: "ipv4/unicast", prefix: "192.168.0.0/24", pathID: 0}
	tr.Track(trackedRoute{
		key:       key,
		path:      []uint32{64500, 64501},
		aspaState: ASPAValid,
	})

	tr.Remove(key)
	assert.Equal(t, 0, tr.count())

	// Reverse index should also be empty.
	tr.mu.Lock()
	assert.Empty(t, tr.reverseIndex)
	tr.mu.Unlock()
}

// TestASPATrackerRevalidate verifies re-validation on cache change.
//
// VALIDATES: AC-5 — cache change triggers re-verification of affected routes.
// PREVENTS: Stale ASPA states persisting after cache update.
func TestASPATrackerRevalidate(t *testing.T) {
	// RFC requirement: DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-7-1 positive -- when ASPA data changes,
	// affected tracked routes are re-verified and the route whose state flipped (Valid -> Invalid)
	// is returned for re-dispatch.
	cache := NewASPACache()
	// Initially: 64501 authorizes 64500 as provider.
	cache.Set(64501, []uint32{64500})
	cache.Set(64502, []uint32{64501})

	tr := NewASPATracker()
	key := routeKey{peerAddr: "10.0.0.1", family: "ipv4/unicast", prefix: "10.0.0.0/24", pathID: 0}
	tr.Track(trackedRoute{
		key:       key,
		peerName:  "peer1",
		peerASN:   64500,
		msgID:     1,
		path:      []uint32{64500, 64501, 64502},
		aspaState: ASPAValid,
	})

	// Change ASPA: 64502 no longer authorizes 64501.
	cache.Set(64502, []uint32{99999})

	changed := tr.Revalidate(cache, []uint32{64502})
	assert.Len(t, changed, 1)
	assert.Equal(t, ASPAInvalid, changed[0].aspaState)
}

// TestASPATrackerReverseIndex verifies reverse index correctly identifies affected routes.
//
// VALIDATES: Routes indexed by all ASNs in their path.
// PREVENTS: Missing routes during re-validation.
func TestASPATrackerReverseIndex(t *testing.T) {
	tr := NewASPATracker()

	// Route 1: path [100, 200, 300]
	tr.Track(trackedRoute{
		key:       routeKey{peerAddr: "10.0.0.1", family: "ipv4/unicast", prefix: "1.0.0.0/8", pathID: 0},
		path:      []uint32{100, 200, 300},
		aspaState: ASPAUnknown,
	})

	// Route 2: path [100, 400, 500]
	tr.Track(trackedRoute{
		key:       routeKey{peerAddr: "10.0.0.1", family: "ipv4/unicast", prefix: "2.0.0.0/8", pathID: 0},
		path:      []uint32{100, 400, 500},
		aspaState: ASPAUnknown,
	})

	// Change for customer-AS 200 should only affect route 1.
	cache := NewASPACache()
	cache.Set(200, []uint32{100})
	cache.Set(300, []uint32{200})

	changed := tr.Revalidate(cache, []uint32{200})
	assert.Len(t, changed, 1)
	assert.Equal(t, "1.0.0.0/8", changed[0].key.prefix)
}

// TestASPATrackerRevalidateNoChange verifies no false positives.
//
// VALIDATES: Route with unchanged state not returned.
// PREVENTS: Spurious event emission.
func TestASPATrackerRevalidateNoChange(t *testing.T) {
	// RFC requirement: DRAFT-IETF-SIDROPS-ASPA-VERIFICATION-7-1 negative -- re-verification after an
	// ASPA change that does not alter a route's outcome returns nothing, so no spurious re-dispatch
	// is triggered for that route.
	cache := NewASPACache()
	cache.Set(64501, []uint32{64500})

	tr := NewASPATracker()
	tr.Track(trackedRoute{
		key:       routeKey{peerAddr: "10.0.0.1", family: "ipv4/unicast", prefix: "10.0.0.0/24", pathID: 0},
		path:      []uint32{64500, 64501},
		aspaState: ASPAValid,
	})

	// Cache didn't actually change the outcome for this route.
	changed := tr.Revalidate(cache, []uint32{64501})
	assert.Empty(t, changed)
}

// TestASPATrackerRemoveNonexistent verifies no panic on absent key.
//
// VALIDATES: Remove of non-tracked route is safe no-op.
// PREVENTS: Panic on withdrawal for unknown route.
func TestASPATrackerRemoveNonexistent(t *testing.T) {
	tr := NewASPATracker()
	tr.Remove(routeKey{peerAddr: "x", family: "y", prefix: "z", pathID: 0})
	assert.Equal(t, 0, tr.count())
}

// TestASPATrackerUpdate verifies re-tracking a route updates its data.
//
// VALIDATES: Existing route replaced when same key re-tracked.
// PREVENTS: Stale path data after route update.
func TestASPATrackerUpdate(t *testing.T) {
	tr := NewASPATracker()
	key := routeKey{peerAddr: "10.0.0.1", family: "ipv4/unicast", prefix: "10.0.0.0/24", pathID: 0}

	tr.Track(trackedRoute{
		key:       key,
		path:      []uint32{100, 200},
		aspaState: ASPAValid,
	})

	tr.Track(trackedRoute{
		key:       key,
		path:      []uint32{100, 300, 400},
		aspaState: ASPAUnknown,
	})

	assert.Equal(t, 1, tr.count())

	// Old ASN 200 should not be in reverse index.
	tr.mu.Lock()
	_, has200 := tr.reverseIndex[200]
	_, has300 := tr.reverseIndex[300]
	tr.mu.Unlock()
	assert.False(t, has200)
	assert.True(t, has300)
}
