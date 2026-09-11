package transaction

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/component/bgp/rib"
	"github.com/ze-software/ze/internal/core/bgp/nlri"
	"github.com/ze-software/ze/internal/core/family"
)

// testRoute creates a test route for a given prefix string.
func testRoute(prefix string) *rib.Route {
	p := netip.MustParsePrefix(prefix)
	fam := family.IPv4Unicast
	n := nlri.NewINET(fam, p, 0)
	nh := netip.MustParseAddr("1.2.3.4")
	return rib.NewRouteWithASPath(n, nh, nil, nil)
}

// testNLRI creates a test NLRI for a given prefix string.
func testNLRI(prefix string) nlri.NLRI {
	p := netip.MustParsePrefix(prefix)
	fam := family.IPv4Unicast
	return nlri.NewINET(fam, p, 0)
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

	// Announce should be canceled, net result is just withdrawal
	if tx.Count() != 0 {
		t.Errorf("expected 0 routes after withdraw canceled announce, got %d", tx.Count())
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

// TestTransaction_AddPathPreservation keeps distinct paths and replaces only the same path.
func TestTransaction_AddPathPreservation(t *testing.T) {
	tests := []struct {
		name   string
		family family.Family
		prefix string
	}{
		{name: "IPv4", family: family.IPv4Unicast, prefix: "10.0.0.0/24"},
		{name: "IPv6", family: family.IPv6Unicast, prefix: "2001:db8::/64"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prefix := netip.MustParsePrefix(tt.prefix)
			nextHop := netip.MustParseAddr("192.0.2.1")
			replacementHop := netip.MustParseAddr("192.0.2.2")
			tx := NewTransaction("paths", "*")
			for _, pathID := range []uint32{0xffffffff, 256, 0, 7} {
				n := nlri.NewINET(tt.family, prefix, pathID)
				tx.QueueAnnounce(rib.NewRouteWithASPath(n, nextHop, nil, nil))
			}
			replacement := nlri.NewINET(tt.family, prefix, 0)
			tx.QueueAnnounce(rib.NewRouteWithASPath(replacement, replacementHop, nil, nil))

			routes := tx.Routes()
			wantIDs := []uint32{0, 7, 256, 0xffffffff}
			if len(routes) != len(wantIDs) {
				t.Fatalf("got %d paths, want %d", len(routes), len(wantIDs))
			}
			for i, route := range routes {
				if got := route.NLRI().PathID(); got != wantIDs[i] {
					t.Errorf("path %d: got ID %d, want %d", i, got, wantIDs[i])
				}
				wantHop := nextHop
				if wantIDs[i] == 0 {
					wantHop = replacementHop
				}
				if got := route.NextHop(); got != wantHop {
					t.Errorf("path %d: got next hop %v, want %v", wantIDs[i], got, wantHop)
				}
			}
		})
	}
}

// TestTransaction_AddPathCancellation isolates withdrawals and reannouncements by path ID.
func TestTransaction_AddPathCancellation(t *testing.T) {
	tx := NewTransaction("paths", "*")
	prefix := netip.MustParsePrefix("10.0.0.0/24")
	nextHop := netip.MustParseAddr("192.0.2.1")
	for _, pathID := range []uint32{0, 7, 8} {
		n := nlri.NewINET(family.IPv4Unicast, prefix, pathID)
		tx.QueueAnnounce(rib.NewRouteWithASPath(n, nextHop, nil, nil))
	}
	tx.QueueWithdraw(nlri.NewINET(family.IPv4Unicast, prefix, 7))
	tx.QueueWithdraw(nlri.NewINET(family.IPv4Unicast, prefix, 9))
	routes := tx.Routes()
	if len(routes) != 2 {
		t.Fatalf("got %d announcements after withdrawal, want 2", len(routes))
	}
	if routes[0].NLRI().PathID() != 0 || routes[1].NLRI().PathID() != 8 {
		t.Fatal("withdrawal canceled a different path")
	}
	withdrawals := tx.Withdrawals()
	if len(withdrawals) != 2 {
		t.Fatalf("got %d withdrawals, want 2", len(withdrawals))
	}
	if withdrawals[0].PathID() != 7 || withdrawals[1].PathID() != 9 {
		t.Fatal("withdrawals lost their path identities")
	}

	reannounced := nlri.NewINET(family.IPv4Unicast, prefix, 7)
	tx.QueueAnnounce(rib.NewRouteWithASPath(reannounced, nextHop, nil, nil))
	tx.QueueWithdraw(nlri.NewINET(family.IPv4Unicast, prefix, 9))
	tx.QueueWithdraw(nlri.NewINET(family.IPv4Unicast, prefix, 0))
	routes = tx.Routes()
	if len(routes) != 2 {
		t.Fatalf("got %d announcements after reannouncement, want 2", len(routes))
	}
	if routes[0].NLRI().PathID() != 7 || routes[1].NLRI().PathID() != 8 {
		t.Fatal("reannouncement or path-zero withdrawal changed another path")
	}
	withdrawals = tx.Withdrawals()
	if len(withdrawals) != 2 {
		t.Fatalf("got %d withdrawals after reannouncement, want 2", len(withdrawals))
	}
	if withdrawals[0].PathID() != 0 || withdrawals[1].PathID() != 9 {
		t.Fatal("reannouncement canceled a different withdrawal")
	}
}

// TestTransaction_RouteOrder makes path selection independent of map and insertion order.
func TestTransaction_RouteOrder(t *testing.T) {
	for _, pathIDs := range [][]uint32{{2, 0, 1}, {1, 2, 0}, {0, 1, 2}} {
		tx := NewTransaction("paths", "*")
		prefix := netip.MustParsePrefix("10.0.0.0/24")
		nextHop := netip.MustParseAddr("192.0.2.1")
		for _, pathID := range pathIDs {
			n := nlri.NewINET(family.IPv4Unicast, prefix, pathID)
			tx.QueueAnnounce(rib.NewRouteWithASPath(n, nextHop, nil, nil))
		}
		for range 10 {
			routes := tx.Routes()
			if len(routes) != 3 {
				t.Fatalf("got %d paths, want 3", len(routes))
			}
			for i, route := range routes {
				if got := route.NLRI().PathID(); got != uint32(i) {
					t.Fatalf("input %v: path %d has ID %d", pathIDs, i, got)
				}
			}
		}
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
