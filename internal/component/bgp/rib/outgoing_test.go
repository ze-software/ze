package rib

import (
	"errors"
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
)

// Helper to create test routes.
func testRoute(prefix string) *Route {
	p := netip.MustParsePrefix(prefix)
	fam := family.IPv4Unicast
	n := nlri.NewINET(fam, p, 0)
	return NewRoute(n, netip.MustParseAddr("1.2.3.4"), nil)
}

// TestOutgoingRIB_Transaction_BeginCommit verifies basic transaction lifecycle.
//
// VALIDATES: BeginTransaction starts transaction, CommitTransaction ends it and returns routes.
//
// PREVENTS: Routes being sent immediately during transaction mode.
func TestOutgoingRIB_Transaction_BeginCommit(t *testing.T) {
	rib := newOutgoingRIB()

	// Initially not in transaction
	if rib.InTransaction() {
		t.Error("expected not in transaction initially")
	}

	// Begin transaction
	if err := rib.BeginTransaction("test1"); err != nil {
		t.Fatalf("BeginTransaction failed: %v", err)
	}

	if !rib.InTransaction() {
		t.Error("expected in transaction after BeginTransaction")
	}

	if rib.TransactionID() != "test1" {
		t.Errorf("TransactionID = %q, want %q", rib.TransactionID(), "test1")
	}

	// Queue routes during transaction
	route1 := testRoute("10.0.0.0/24")
	route2 := testRoute("10.1.0.0/24")
	rib.QueueAnnounce(route1)
	rib.QueueAnnounce(route2)

	// Commit transaction
	stats, err := rib.CommitTransaction()
	if err != nil {
		t.Fatalf("CommitTransaction failed: %v", err)
	}

	// Verify stats
	if stats.RoutesAnnounced != 2 {
		t.Errorf("RoutesAnnounced = %d, want 2", stats.RoutesAnnounced)
	}

	// No longer in transaction
	if rib.InTransaction() {
		t.Error("expected not in transaction after CommitTransaction")
	}
}

// TestOutgoingRIB_Transaction_Rollback verifies rollback discards routes.
//
// VALIDATES: RollbackTransaction discards queued routes without returning them.
//
// PREVENTS: Routes being sent after rollback.
func TestOutgoingRIB_Transaction_Rollback(t *testing.T) {
	rib := newOutgoingRIB()

	if err := rib.BeginTransaction("rollback-test"); err != nil {
		t.Fatalf("BeginTransaction failed: %v", err)
	}

	// Queue routes
	rib.QueueAnnounce(testRoute("10.0.0.0/24"))
	rib.QueueAnnounce(testRoute("10.1.0.0/24"))

	// Rollback
	stats, err := rib.RollbackTransaction()
	if err != nil {
		t.Fatalf("RollbackTransaction failed: %v", err)
	}

	// Verify discarded count
	if stats.RoutesDiscarded != 2 {
		t.Errorf("RoutesDiscarded = %d, want 2", stats.RoutesDiscarded)
	}

	// No longer in transaction
	if rib.InTransaction() {
		t.Error("expected not in transaction after RollbackTransaction")
	}

	// Pending should be empty (routes were discarded, not transferred)
	fam := family.IPv4Unicast
	if routes := rib.getPending(fam); len(routes) != 0 {
		t.Errorf("GetPending returned %d routes, want 0 after rollback", len(routes))
	}
}

// TestOutgoingRIB_Transaction_NestedError verifies nested transactions are rejected.
//
// VALIDATES: BeginTransaction returns error if already in transaction.
//
// PREVENTS: Undefined behavior from nested transactions.
func TestOutgoingRIB_Transaction_NestedError(t *testing.T) {
	rib := newOutgoingRIB()

	if err := rib.BeginTransaction("first"); err != nil {
		t.Fatalf("BeginTransaction failed: %v", err)
	}

	// Try to start nested transaction
	err := rib.BeginTransaction("second")
	if err == nil {
		t.Error("expected error for nested transaction")
	}
	if !errors.Is(err, ErrAlreadyInTransaction) {
		t.Errorf("error = %v, want ErrAlreadyInTransaction", err)
	}
}

// TestOutgoingRIB_Transaction_CommitWithoutBegin verifies commit without begin fails.
//
// VALIDATES: CommitTransaction returns error when not in transaction.
//
// PREVENTS: Accidental commits outside transaction context.
func TestOutgoingRIB_Transaction_CommitWithoutBegin(t *testing.T) {
	rib := newOutgoingRIB()

	_, err := rib.CommitTransaction()
	if err == nil {
		t.Error("expected error for commit without begin")
	}
	if !errors.Is(err, ErrNoTransaction) {
		t.Errorf("error = %v, want ErrNoTransaction", err)
	}
}

// TestOutgoingRIB_Transaction_RollbackWithoutBegin verifies rollback without begin fails.
//
// VALIDATES: RollbackTransaction returns error when not in transaction.
//
// PREVENTS: Accidental rollbacks outside transaction context.
func TestOutgoingRIB_Transaction_RollbackWithoutBegin(t *testing.T) {
	rib := newOutgoingRIB()

	_, err := rib.RollbackTransaction()
	if err == nil {
		t.Error("expected error for rollback without begin")
	}
	if !errors.Is(err, ErrNoTransaction) {
		t.Errorf("error = %v, want ErrNoTransaction", err)
	}
}

// TestOutgoingRIB_Transaction_LabelMismatch verifies label matching on commit.
//
// VALIDATES: CommitTransactionWithLabel returns error if labels don't match.
//
// PREVENTS: Committing wrong transaction accidentally.
func TestOutgoingRIB_Transaction_LabelMismatch(t *testing.T) {
	rib := newOutgoingRIB()

	if err := rib.BeginTransaction("batch1"); err != nil {
		t.Fatalf("BeginTransaction failed: %v", err)
	}

	// Try to commit with wrong label
	_, err := rib.commitTransactionWithLabel("batch2")
	if err == nil {
		t.Error("expected error for label mismatch")
	}
	if !errors.Is(err, ErrLabelMismatch) {
		t.Errorf("error = %v, want ErrLabelMismatch", err)
	}

	// Should still be in transaction
	if !rib.InTransaction() {
		t.Error("expected still in transaction after label mismatch")
	}

	// Correct label should work
	_, err = rib.commitTransactionWithLabel("batch1")
	if err != nil {
		t.Errorf("CommitTransactionWithLabel with correct label failed: %v", err)
	}
}

// TestOutgoingRIB_Transaction_WithdrawalsIncluded verifies withdrawals are tracked.
//
// VALIDATES: Transaction stats include withdrawal count.
//
// PREVENTS: Missing withdrawal accounting in commit stats.
func TestOutgoingRIB_Transaction_WithdrawalsIncluded(t *testing.T) {
	rib := newOutgoingRIB()

	if err := rib.BeginTransaction("withdraw-test"); err != nil {
		t.Fatalf("BeginTransaction failed: %v", err)
	}

	// Queue announcements and withdrawals
	rib.QueueAnnounce(testRoute("10.0.0.0/24"))

	fam := family.IPv4Unicast
	withdrawNLRI := nlri.NewINET(fam, netip.MustParsePrefix("10.1.0.0/24"), 0)
	rib.QueueWithdraw(withdrawNLRI)

	stats, err := rib.CommitTransaction()
	if err != nil {
		t.Fatalf("CommitTransaction failed: %v", err)
	}

	if stats.RoutesAnnounced != 1 {
		t.Errorf("RoutesAnnounced = %d, want 1", stats.RoutesAnnounced)
	}
	if stats.RoutesWithdrawn != 1 {
		t.Errorf("RoutesWithdrawn = %d, want 1", stats.RoutesWithdrawn)
	}
}

// TestOutgoingRIB_Transaction_EmptyCommit verifies empty transaction commits cleanly.
//
// VALIDATES: Committing empty transaction succeeds with zero counts.
//
// PREVENTS: Errors on legitimate empty transactions.
func TestOutgoingRIB_Transaction_EmptyCommit(t *testing.T) {
	rib := newOutgoingRIB()

	if err := rib.BeginTransaction("empty"); err != nil {
		t.Fatalf("BeginTransaction failed: %v", err)
	}

	// Commit without adding any routes
	stats, err := rib.CommitTransaction()
	if err != nil {
		t.Fatalf("CommitTransaction failed: %v", err)
	}

	if stats.RoutesAnnounced != 0 {
		t.Errorf("RoutesAnnounced = %d, want 0", stats.RoutesAnnounced)
	}
	if stats.RoutesWithdrawn != 0 {
		t.Errorf("RoutesWithdrawn = %d, want 0", stats.RoutesWithdrawn)
	}
}

// TestOutgoingRIB_Transaction_GetPendingRoutes verifies pending routes are accessible.
//
// VALIDATES: GetTransactionPending returns routes queued during transaction.
//
// PREVENTS: Inability to inspect transaction contents before commit.
func TestOutgoingRIB_Transaction_GetPendingRoutes(t *testing.T) {
	rib := newOutgoingRIB()

	if err := rib.BeginTransaction("inspect"); err != nil {
		t.Fatalf("BeginTransaction failed: %v", err)
	}

	rib.QueueAnnounce(testRoute("10.0.0.0/24"))
	rib.QueueAnnounce(testRoute("10.1.0.0/24"))

	fam := family.IPv4Unicast
	routes := rib.getTransactionPending(fam)

	if len(routes) != 2 {
		t.Errorf("GetTransactionPending returned %d routes, want 2", len(routes))
	}

	// Rollback to clean up
	if _, err := rib.RollbackTransaction(); err != nil {
		t.Fatalf("RollbackTransaction failed: %v", err)
	}
}

// TestOutgoingRIB_QueueWithdraw_RemovesBothExactAndSuffixed verifies that
// QueueWithdraw cancels pending announcements whether the route key is an
// exact NLRI index match or has an AS-PATH hash suffix appended.
//
// PREVENTS: Fast-path exact delete skipping suffixed entries (regression).
func TestOutgoingRIB_QueueWithdraw_RemovesBothExactAndSuffixed(t *testing.T) {
	r := newOutgoingRIB()

	fam := family.IPv4Unicast
	prefix := "10.0.0.0/24"

	// Queue a route without AS-PATH (exact key = NLRI index).
	plain := testRoute(prefix)
	r.QueueAnnounce(plain)

	// Queue a route with AS-PATH (key = NLRI index + 8-byte hash suffix).
	asPath := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001}},
		},
	}
	withAS := testRouteWithASPath(prefix, "1.2.3.4", nil, asPath)
	r.QueueAnnounce(withAS)

	pending := r.getPending(fam)
	if len(pending) != 2 {
		t.Fatalf("pending before withdraw = %d, want 2", len(pending))
	}

	// Withdraw the NLRI: both pending entries must be removed.
	n := nlri.NewINET(fam, netip.MustParsePrefix(prefix), 0)
	r.QueueWithdraw(n)

	pending = r.getPending(fam)
	if len(pending) != 0 {
		t.Errorf("pending after withdraw = %d, want 0", len(pending))
	}
}

// TestOutgoingRIB_FlushWithdrawals_RemovesSuffixedSentEntries verifies that
// FlushWithdrawals removes sent-cache entries with AS-PATH hash suffixes.
//
// PREVENTS: Loop reversal optimization losing suffix-matched deletes.
func TestOutgoingRIB_FlushWithdrawals_RemovesSuffixedSentEntries(t *testing.T) {
	r := newOutgoingRIB()

	fam := family.IPv4Unicast
	prefix := "10.0.0.0/24"

	// Queue and flush a route with AS-PATH to populate the sent cache.
	asPath := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001}},
		},
	}
	withAS := testRouteWithASPath(prefix, "1.2.3.4", nil, asPath)
	r.QueueAnnounce(withAS)
	flushed := r.flushPending(fam)
	if len(flushed) != 1 {
		t.Fatalf("FlushPending = %d, want 1", len(flushed))
	}

	sentBefore := r.getSentRoutes()
	if len(sentBefore) != 1 {
		t.Fatalf("sent before withdrawal = %d, want 1", len(sentBefore))
	}

	// Queue a withdrawal for the same NLRI, then flush.
	n := nlri.NewINET(fam, netip.MustParsePrefix(prefix), 0)
	r.QueueWithdraw(n)
	r.flushWithdrawals(fam)

	sentAfter := r.getSentRoutes()
	if len(sentAfter) != 0 {
		t.Errorf("sent after FlushWithdrawals = %d, want 0", len(sentAfter))
	}
}

// TestOutgoingRIB_RemoveFromSent_RemovesBothExactAndSuffixed verifies that
// RemoveFromSent clears both exact-match and suffix-match entries.
//
// PREVENTS: Fast-path early return skipping suffixed entries (regression).
func TestOutgoingRIB_RemoveFromSent_RemovesBothExactAndSuffixed(t *testing.T) {
	r := newOutgoingRIB()

	fam := family.IPv4Unicast
	prefix := "10.0.0.0/24"

	// Populate sent cache with a plain route and a route with AS-PATH.
	plain := testRoute(prefix)
	r.QueueAnnounce(plain)
	r.flushPending(fam)

	asPath := &attribute.ASPath{
		Segments: []attribute.ASPathSegment{
			{Type: attribute.ASSequence, ASNs: []uint32{65001}},
		},
	}
	withAS := testRouteWithASPath(prefix, "1.2.3.4", nil, asPath)
	r.QueueAnnounce(withAS)
	r.flushPending(fam)

	sentBefore := r.getSentRoutes()
	if len(sentBefore) != 2 {
		t.Fatalf("sent before RemoveFromSent = %d, want 2", len(sentBefore))
	}

	// RemoveFromSent must remove both entries.
	n := nlri.NewINET(fam, netip.MustParsePrefix(prefix), 0)
	r.removeFromSent(n)

	sentAfter := r.getSentRoutes()
	if len(sentAfter) != 0 {
		t.Errorf("sent after RemoveFromSent = %d, want 0", len(sentAfter))
	}
}
