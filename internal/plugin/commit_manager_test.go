package plugin

import (
	"net/netip"
	"testing"

	"codeberg.org/thomas-mangin/zebgp/internal/bgp/nlri"
	"codeberg.org/thomas-mangin/zebgp/internal/rib"
)

// testRoute creates a test route for a given prefix string.
func testRoute(prefix string) *rib.Route {
	p := netip.MustParsePrefix(prefix)
	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
	n := nlri.NewINET(family, p, 0)
	nh := netip.MustParseAddr("1.2.3.4")
	return rib.NewRoute(n, nh, nil)
}

// testNLRI creates a test NLRI for a given prefix string.
func testNLRI(prefix string) nlri.NLRI {
	p := netip.MustParsePrefix(prefix)
	family := nlri.Family{AFI: nlri.AFIIPv4, SAFI: nlri.SAFIUnicast}
	return nlri.NewINET(family, p, 0)
}

// TestTransaction_QueueAnnounce verifies route queuing in transactions.
//
// VALIDATES: Routes are queued and retrievable.
// PREVENTS: Routes being lost or duplicated.
func TestTransaction_QueueAnnounce(t *testing.T) {
	tx := NewTransaction("batch1", "*")

	route := testRoute("10.0.0.0/24")
	tx.QueueAnnounce(route)

	if tx.Count() != 1 {
		t.Errorf("expected 1 route, got %d", tx.Count())
	}

	routes := tx.Routes()
	if len(routes) != 1 {
		t.Errorf("expected 1 route from Routes(), got %d", len(routes))
	}
}

// TestTransaction_QueueWithdraw verifies withdrawal queuing.
//
// VALIDATES: Withdrawals are queued correctly.
// PREVENTS: Withdrawals being lost.
func TestTransaction_QueueWithdraw(t *testing.T) {
	tx := NewTransaction("batch1", "*")

	n := testNLRI("10.0.0.0/24")
	tx.QueueWithdraw(n)

	if tx.WithdrawalCount() != 1 {
		t.Errorf("expected 1 withdrawal, got %d", tx.WithdrawalCount())
	}
}

// TestTransaction_AnnounceThenWithdraw verifies announce+withdraw cancellation.
//
// VALIDATES: Withdraw cancels preceding announce for same prefix.
// PREVENTS: Sending announce then immediate withdraw (wasted traffic).
func TestTransaction_AnnounceThenWithdraw(t *testing.T) {
	tx := NewTransaction("batch1", "*")

	route := testRoute("10.0.0.0/24")
	tx.QueueAnnounce(route)
	if tx.Count() != 1 {
		t.Fatalf("expected 1 route after announce")
	}

	n := testNLRI("10.0.0.0/24")
	tx.QueueWithdraw(n)

	// Announce should be cancelled, net result is just withdrawal
	if tx.Count() != 0 {
		t.Errorf("expected 0 routes after withdraw cancelled announce, got %d", tx.Count())
	}
	if tx.WithdrawalCount() != 1 {
		t.Errorf("expected 1 withdrawal, got %d", tx.WithdrawalCount())
	}
}

// TestTransaction_ReplaceAnnounce verifies duplicate announce replacement.
//
// VALIDATES: Second announce for same prefix replaces first.
// PREVENTS: Duplicate routes in transaction.
func TestTransaction_ReplaceAnnounce(t *testing.T) {
	tx := NewTransaction("batch1", "*")

	route1 := testRoute("10.0.0.0/24")
	route2 := testRoute("10.0.0.0/24") // Same prefix, different route object

	tx.QueueAnnounce(route1)
	tx.QueueAnnounce(route2)

	if tx.Count() != 1 {
		t.Errorf("expected 1 route after replacement, got %d", tx.Count())
	}
}

// TestCommitManager_StartAndGet verifies basic commit lifecycle.
//
// VALIDATES: Can start and retrieve commits by name.
// PREVENTS: Lost commits, wrong commit returned.
func TestCommitManager_StartAndGet(t *testing.T) {
	cm := NewCommitManager()

	if err := cm.Start("batch1", "*"); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	tx, err := cm.Get("batch1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	if tx.Name() != "batch1" {
		t.Errorf("expected name 'batch1', got %q", tx.Name())
	}

	if tx.PeerSelector() != "*" {
		t.Errorf("expected peer selector '*', got %q", tx.PeerSelector())
	}
}

// TestCommitManager_DuplicateStart verifies duplicate name rejection.
//
// VALIDATES: Cannot start commit with same name twice.
// PREVENTS: Overwriting active commits.
func TestCommitManager_DuplicateStart(t *testing.T) {
	cm := NewCommitManager()

	if err := cm.Start("batch1", "*"); err != nil {
		t.Fatalf("first Start failed: %v", err)
	}

	err := cm.Start("batch1", "*")
	if err == nil {
		t.Error("expected error for duplicate start, got nil")
	}
}

// TestCommitManager_ConcurrentCommits verifies multiple concurrent commits.
//
// VALIDATES: Multiple commits can be active simultaneously.
// PREVENTS: Commits interfering with each other.
func TestCommitManager_ConcurrentCommits(t *testing.T) {
	cm := NewCommitManager()

	if err := cm.Start("batch1", "*"); err != nil {
		t.Fatalf("Start batch1 failed: %v", err)
	}
	if err := cm.Start("batch2", "192.168.1.1"); err != nil {
		t.Fatalf("Start batch2 failed: %v", err)
	}

	// Verify both accessible
	tx1, err := cm.Get("batch1")
	if err != nil {
		t.Fatalf("Get batch1 failed: %v", err)
	}
	tx2, err := cm.Get("batch2")
	if err != nil {
		t.Fatalf("Get batch2 failed: %v", err)
	}

	// Verify they're different
	if tx1.Name() == tx2.Name() {
		t.Error("batch1 and batch2 should be different")
	}

	// Queue routes to each
	tx1.QueueAnnounce(testRoute("10.0.0.0/24"))
	tx2.QueueAnnounce(testRoute("10.1.0.0/24"))

	// Verify routes stayed separate
	if tx1.Count() != 1 || tx2.Count() != 1 {
		t.Errorf("expected 1 route each, got tx1=%d tx2=%d", tx1.Count(), tx2.Count())
	}
}

// TestCommitManager_End verifies commit removal.
//
// VALIDATES: End removes and returns commit.
// PREVENTS: Zombie commits, lost route data.
func TestCommitManager_End(t *testing.T) {
	cm := NewCommitManager()

	if err := cm.Start("batch1", "*"); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	tx, err := cm.Get("batch1")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	// Queue a route
	tx.QueueAnnounce(testRoute("10.0.0.0/24"))

	// End should return the transaction with routes
	endedTx, err := cm.End("batch1")
	if err != nil {
		t.Fatalf("End failed: %v", err)
	}

	if endedTx.Count() != 1 {
		t.Errorf("ended transaction should have 1 route, got %d", endedTx.Count())
	}

	// Commit should no longer exist
	if _, err := cm.Get("batch1"); err == nil {
		t.Error("expected error getting ended commit, got nil")
	}
}

// TestCommitManager_Rollback verifies discard functionality.
//
// VALIDATES: Rollback removes commit and returns discard count.
// PREVENTS: Routes being sent after rollback.
func TestCommitManager_Rollback(t *testing.T) {
	cm := NewCommitManager()

	if err := cm.Start("batch1", "*"); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	tx, _ := cm.Get("batch1")
	tx.QueueAnnounce(testRoute("10.0.0.0/24"))

	discarded, err := cm.Rollback("batch1")
	if err != nil {
		t.Fatalf("Rollback failed: %v", err)
	}

	if discarded != 1 {
		t.Errorf("expected 1 discarded, got %d", discarded)
	}

	// Commit should no longer exist
	if _, err := cm.Get("batch1"); err == nil {
		t.Error("expected error getting rolled back commit, got nil")
	}
}

// TestCommitManager_List verifies listing active commits.
//
// VALIDATES: List returns all active commit names.
// PREVENTS: Missing commits in list.
func TestCommitManager_List(t *testing.T) {
	cm := NewCommitManager()

	// Empty list
	if len(cm.List()) != 0 {
		t.Error("expected empty list initially")
	}

	_ = cm.Start("batch1", "*")
	_ = cm.Start("batch2", "*")

	list := cm.List()
	if len(list) != 2 {
		t.Errorf("expected 2 commits in list, got %d", len(list))
	}

	// Should contain both names (order not guaranteed)
	found := make(map[string]bool)
	for _, name := range list {
		found[name] = true
	}
	if !found["batch1"] || !found["batch2"] {
		t.Errorf("list should contain batch1 and batch2, got %v", list)
	}
}

// TestCommitManager_GetNotFound verifies error for missing commit.
//
// VALIDATES: Get returns error for non-existent commit.
// PREVENTS: Nil pointer dereference.
func TestCommitManager_GetNotFound(t *testing.T) {
	cm := NewCommitManager()

	_, err := cm.Get("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent commit")
	}
}

// TestCommitManager_EmptyName verifies rejection of empty names.
//
// VALIDATES: Empty commit name is rejected.
// PREVENTS: Unnamed commits that can't be referenced.
func TestCommitManager_EmptyName(t *testing.T) {
	cm := NewCommitManager()

	err := cm.Start("", "*")
	if err == nil {
		t.Error("expected error for empty commit name")
	}
}

// TestTransaction_Families verifies family tracking.
//
// VALIDATES: Families returns unique families with routes.
// PREVENTS: Missing family in EOR.
func TestTransaction_Families(t *testing.T) {
	tx := NewTransaction("batch1", "*")

	// Add IPv4 route
	tx.QueueAnnounce(testRoute("10.0.0.0/24"))

	families := tx.Families()
	if len(families) != 1 {
		t.Errorf("expected 1 family, got %d", len(families))
	}
}
