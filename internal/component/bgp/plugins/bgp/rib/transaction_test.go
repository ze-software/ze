package rib

import (
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/bgp/nlri"
)

// TestMultipleIndependentTransactions verifies concurrent transactions don't interfere.
//
// VALIDATES: Two OutgoingRIBs can have independent transactions.
//
// PREVENTS: Shared state corruption between peer transactions.
func TestMultipleIndependentTransactions(t *testing.T) {
	rib1 := NewOutgoingRIB()
	rib2 := NewOutgoingRIB()

	// Start transaction on rib1 only
	if err := rib1.BeginTransaction("peer1-batch"); err != nil {
		t.Fatalf("rib1 BeginTransaction failed: %v", err)
	}

	// rib2 should not be in transaction
	if rib2.InTransaction() {
		t.Error("rib2 should not be in transaction")
	}

	// Start transaction on rib2
	if err := rib2.BeginTransaction("peer2-batch"); err != nil {
		t.Fatalf("rib2 BeginTransaction failed: %v", err)
	}

	// Both should be in transaction with different IDs
	if !rib1.InTransaction() {
		t.Error("rib1 should be in transaction")
	}
	if !rib2.InTransaction() {
		t.Error("rib2 should be in transaction")
	}
	if rib1.TransactionID() != "peer1-batch" {
		t.Errorf("rib1 TransactionID = %q, want %q", rib1.TransactionID(), "peer1-batch")
	}
	if rib2.TransactionID() != "peer2-batch" {
		t.Errorf("rib2 TransactionID = %q, want %q", rib2.TransactionID(), "peer2-batch")
	}

	// Queue routes to each
	route1 := testRoute("10.0.0.0/24")
	route2 := testRoute("10.1.0.0/24")
	route3 := testRoute("10.2.0.0/24")

	rib1.QueueAnnounce(route1)
	rib1.QueueAnnounce(route2)
	rib2.QueueAnnounce(route3)

	// Commit rib1 only
	stats1, err := rib1.CommitTransaction()
	if err != nil {
		t.Fatalf("rib1 CommitTransaction failed: %v", err)
	}

	if stats1.RoutesAnnounced != 2 {
		t.Errorf("rib1 RoutesAnnounced = %d, want 2", stats1.RoutesAnnounced)
	}

	// rib1 should no longer be in transaction
	if rib1.InTransaction() {
		t.Error("rib1 should not be in transaction after commit")
	}

	// rib2 should still be in transaction with its route
	if !rib2.InTransaction() {
		t.Error("rib2 should still be in transaction")
	}

	// Commit rib2
	stats2, err := rib2.CommitTransaction()
	if err != nil {
		t.Fatalf("rib2 CommitTransaction failed: %v", err)
	}

	if stats2.RoutesAnnounced != 1 {
		t.Errorf("rib2 RoutesAnnounced = %d, want 1", stats2.RoutesAnnounced)
	}
}

// TestTransactionIsolation_RouteQueuing verifies routes go to correct RIB.
//
// VALIDATES: Routes queued during transaction stay in that RIB only.
//
// PREVENTS: Routes leaking between peer RIBs.
func TestTransactionIsolation_RouteQueuing(t *testing.T) {
	rib1 := NewOutgoingRIB()
	rib2 := NewOutgoingRIB()

	// Start transactions on both
	_ = rib1.BeginTransaction("batch1")
	_ = rib2.BeginTransaction("batch2")

	// Queue different routes
	route1 := testRoute("10.0.0.0/24")
	route2 := testRoute("20.0.0.0/24")

	rib1.QueueAnnounce(route1)
	rib2.QueueAnnounce(route2)

	// Check pending routes are isolated
	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}

	pending1 := rib1.GetTransactionPending(family)
	pending2 := rib2.GetTransactionPending(family)

	if len(pending1) != 1 {
		t.Errorf("rib1 pending = %d, want 1", len(pending1))
	}
	if len(pending2) != 1 {
		t.Errorf("rib2 pending = %d, want 1", len(pending2))
	}

	// Verify the actual routes
	if len(pending1) > 0 {
		inet, ok := pending1[0].NLRI().(*nlri.INET)
		if !ok {
			t.Fatal("rib1 route is not INET")
		}
		if inet.Prefix().String() != "10.0.0.0/24" {
			t.Errorf("rib1 has wrong route: %s", inet.Prefix())
		}
	}
	if len(pending2) > 0 {
		inet, ok := pending2[0].NLRI().(*nlri.INET)
		if !ok {
			t.Fatal("rib2 route is not INET")
		}
		if inet.Prefix().String() != "20.0.0.0/24" {
			t.Errorf("rib2 has wrong route: %s", inet.Prefix())
		}
	}

	// Cleanup
	_, _ = rib1.RollbackTransaction()
	_, _ = rib2.RollbackTransaction()
}

// TestTransactionIsolation_CommitDoesNotAffectOther verifies commit isolation.
//
// VALIDATES: Committing one RIB doesn't affect another's transaction.
//
// PREVENTS: Commit side effects on other peers.
func TestTransactionIsolation_CommitDoesNotAffectOther(t *testing.T) {
	rib1 := NewOutgoingRIB()
	rib2 := NewOutgoingRIB()

	_ = rib1.BeginTransaction("batch1")
	_ = rib2.BeginTransaction("batch2")

	rib1.QueueAnnounce(testRoute("10.0.0.0/24"))
	rib2.QueueAnnounce(testRoute("20.0.0.0/24"))
	rib2.QueueAnnounce(testRoute("20.1.0.0/24"))

	// Commit rib1
	_, _ = rib1.CommitTransaction()

	// rib2's transaction should be unaffected
	if !rib2.InTransaction() {
		t.Error("rib2 transaction was affected by rib1 commit")
	}
	if rib2.TransactionID() != "batch2" {
		t.Error("rib2 transaction ID was changed")
	}

	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
	pending2 := rib2.GetTransactionPending(family)
	if len(pending2) != 2 {
		t.Errorf("rib2 lost routes after rib1 commit: got %d, want 2", len(pending2))
	}

	// Cleanup
	_, _ = rib2.RollbackTransaction()
}

// TestTransactionIsolation_RollbackDoesNotAffectOther verifies rollback isolation.
//
// VALIDATES: Rolling back one RIB doesn't affect another's transaction.
//
// PREVENTS: Rollback side effects on other peers.
func TestTransactionIsolation_RollbackDoesNotAffectOther(t *testing.T) {
	rib1 := NewOutgoingRIB()
	rib2 := NewOutgoingRIB()

	_ = rib1.BeginTransaction("batch1")
	_ = rib2.BeginTransaction("batch2")

	rib1.QueueAnnounce(testRoute("10.0.0.0/24"))
	rib2.QueueAnnounce(testRoute("20.0.0.0/24"))

	// Rollback rib1
	_, _ = rib1.RollbackTransaction()

	// rib2's transaction should be unaffected
	if !rib2.InTransaction() {
		t.Error("rib2 transaction was affected by rib1 rollback")
	}

	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
	pending2 := rib2.GetTransactionPending(family)
	if len(pending2) != 1 {
		t.Errorf("rib2 lost routes after rib1 rollback: got %d, want 1", len(pending2))
	}

	// Commit rib2 should still work
	stats, err := rib2.CommitTransaction()
	if err != nil {
		t.Fatalf("rib2 commit failed after rib1 rollback: %v", err)
	}
	if stats.RoutesAnnounced != 1 {
		t.Errorf("rib2 RoutesAnnounced = %d, want 1", stats.RoutesAnnounced)
	}
}

// TestTransactionIsolation_DifferentLabels verifies label independence.
//
// VALIDATES: Different RIBs can use same label without conflict.
//
// PREVENTS: Label collision between peers.
func TestTransactionIsolation_DifferentLabels(t *testing.T) {
	rib1 := NewOutgoingRIB()
	rib2 := NewOutgoingRIB()

	// Both use same label - should not conflict
	_ = rib1.BeginTransaction("batch")
	_ = rib2.BeginTransaction("batch")

	rib1.QueueAnnounce(testRoute("10.0.0.0/24"))
	rib2.QueueAnnounce(testRoute("20.0.0.0/24"))

	// Commit with label on rib1
	_, err := rib1.CommitTransactionWithLabel("batch")
	if err != nil {
		t.Fatalf("rib1 commit with label failed: %v", err)
	}

	// Commit with label on rib2 should also work
	_, err = rib2.CommitTransactionWithLabel("batch")
	if err != nil {
		t.Fatalf("rib2 commit with label failed: %v", err)
	}
}

// TestTransactionIsolation_WithdrawalsIsolated verifies withdrawal isolation.
//
// VALIDATES: Withdrawals in one RIB don't affect another.
//
// PREVENTS: Withdrawal leakage between peers.
func TestTransactionIsolation_WithdrawalsIsolated(t *testing.T) {
	rib1 := NewOutgoingRIB()
	rib2 := NewOutgoingRIB()

	_ = rib1.BeginTransaction("batch1")
	_ = rib2.BeginTransaction("batch2")

	// Queue announcement in rib1, withdrawal in rib2
	rib1.QueueAnnounce(testRoute("10.0.0.0/24"))

	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
	withdrawNLRI := nlri.NewINET(family, netip.MustParsePrefix("20.0.0.0/24"), 0)
	rib2.QueueWithdraw(withdrawNLRI)

	// Commit both
	stats1, _ := rib1.CommitTransaction()
	stats2, _ := rib2.CommitTransaction()

	if stats1.RoutesAnnounced != 1 {
		t.Errorf("rib1 RoutesAnnounced = %d, want 1", stats1.RoutesAnnounced)
	}
	if stats1.RoutesWithdrawn != 0 {
		t.Errorf("rib1 RoutesWithdrawn = %d, want 0", stats1.RoutesWithdrawn)
	}

	if stats2.RoutesAnnounced != 0 {
		t.Errorf("rib2 RoutesAnnounced = %d, want 0", stats2.RoutesAnnounced)
	}
	if stats2.RoutesWithdrawn != 1 {
		t.Errorf("rib2 RoutesWithdrawn = %d, want 1", stats2.RoutesWithdrawn)
	}
}
