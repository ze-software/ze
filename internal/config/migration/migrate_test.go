package migration

import (
	"errors"
	"testing"

	"codeberg.org/thomas-mangin/ze/internal/config"
)

// TestMigrateNilTree verifies Migrate returns ErrNilTree for nil input.
//
// VALIDATES: Migrate handles nil input gracefully.
// PREVENTS: Panic on nil tree.
func TestMigrateNilTree(t *testing.T) {
	result, err := Migrate(nil)
	if !errors.Is(err, ErrNilTree) {
		t.Errorf("Migrate(nil) error = %v, want ErrNilTree", err)
	}
	if result != nil {
		t.Errorf("Migrate(nil) result = %v, want nil", result)
	}
}

// TestMigrateEmptyTree verifies empty tree returns empty Applied/Skipped.
//
// VALIDATES: Empty tree returns MigrateResult with empty lists.
// PREVENTS: Incorrect counting on empty configs.
func TestMigrateEmptyTree(t *testing.T) {
	tree := config.NewTree()

	result, err := Migrate(tree)
	if err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}
	if result == nil {
		t.Fatal("Migrate() returned nil result")
	}
	if result.Tree == nil {
		t.Error("Migrate().Tree = nil, want non-nil")
	}
	// Empty tree has no patterns to detect, so all transformations skip
	if len(result.Applied) != 0 {
		t.Errorf("Applied = %v, want empty", result.Applied)
	}
	// All transformations should be skipped
	if len(result.Skipped) != len(transformations) {
		t.Errorf("Skipped count = %d, want %d", len(result.Skipped), len(transformations))
	}
}

// TestMigrateNeighborToPeer verifies neighbor→peer transformation.
//
// VALIDATES: neighbor blocks renamed to peer, tracked in Applied.
// PREVENTS: Transformation not being tracked.
func TestMigrateNeighborToPeer(t *testing.T) {
	tree := config.NewTree()
	neighbor := config.NewTree()
	neighbor.Set("router-id", "1.1.1.1")
	tree.AddListEntry("neighbor", "10.0.0.1", neighbor)

	result, err := Migrate(tree)
	if err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	// Check transformation was applied
	if !sliceContains(result.Applied, "neighbor->peer") {
		t.Errorf("Applied = %v, want to contain 'neighbor->peer'", result.Applied)
	}

	// Check peer exists in result tree
	peers := result.Tree.GetList("peer")
	if _, ok := peers["10.0.0.1"]; !ok {
		t.Error("peer 10.0.0.1 not found in result tree")
	}

	// Check neighbor removed
	neighbors := result.Tree.GetList("neighbor")
	if len(neighbors) != 0 {
		t.Errorf("neighbors still exist: %v", neighbors)
	}
}

// TestMigrateSkipsAlreadyMigrated verifies skipped list populated.
//
// VALIDATES: Already-migrated configs have transformations in Skipped.
// PREVENTS: False positives in Applied list.
func TestMigrateSkipsAlreadyMigrated(t *testing.T) {
	// Create a current config (peer, not neighbor)
	tree := config.NewTree()
	peer := config.NewTree()
	peer.Set("router-id", "1.1.1.1")
	tree.AddListEntry("peer", "10.0.0.1", peer)

	result, err := Migrate(tree)
	if err != nil {
		t.Fatalf("Migrate() error = %v", err)
	}

	// neighbor->peer should be skipped (no neighbors)
	if !sliceContains(result.Skipped, "neighbor->peer") {
		t.Errorf("Skipped = %v, want to contain 'neighbor->peer'", result.Skipped)
	}
}

// TestMigrateOriginalUnchangedOnError verifies atomicity.
//
// VALIDATES: Original tree unchanged when migration fails.
// PREVENTS: Partial migration leaving corrupted state.
func TestMigrateOriginalUnchangedOnError(t *testing.T) {
	// Create tree with process block that will cause duplicate error
	tree := config.NewTree()
	peer := config.NewTree()

	// Create old-style api with duplicate processes
	api := config.NewTree()
	api.Set("processes", "[ foo foo ]") // Duplicate!
	peer.AddListEntry("process", config.KeyDefault, api)

	tree.AddListEntry("peer", "10.0.0.1", peer)

	// Store original state
	origPeerCount := len(tree.GetList("peer"))

	result, err := Migrate(tree)

	// Should return error
	if err == nil {
		t.Fatal("Migrate() should return error for duplicate processes")
	}
	if result != nil {
		t.Errorf("Migrate() result = %v, want nil on error", result)
	}

	// Original should be unchanged
	if len(tree.GetList("peer")) != origPeerCount {
		t.Error("Original tree was modified on error")
	}
}

// TestDryRunNilTree verifies DryRun returns ErrNilTree for nil input.
//
// VALIDATES: DryRun handles nil input gracefully.
// PREVENTS: Panic on nil tree.
func TestDryRunNilTree(t *testing.T) {
	result, err := DryRun(nil)
	if !errors.Is(err, ErrNilTree) {
		t.Errorf("DryRun(nil) error = %v, want ErrNilTree", err)
	}
	if result != nil {
		t.Errorf("DryRun(nil) result = %v, want nil", result)
	}
}

// TestDryRunAlreadyDone verifies AlreadyDone populated for migrated configs.
//
// VALIDATES: DryRun shows AlreadyDone for migrated configs.
// PREVENTS: False detection of needed migrations.
func TestDryRunAlreadyDone(t *testing.T) {
	// Create a current config
	tree := config.NewTree()
	peer := config.NewTree()
	tree.AddListEntry("peer", "10.0.0.1", peer)

	result, err := DryRun(tree)
	if err != nil {
		t.Fatalf("DryRun() error = %v", err)
	}

	if !result.WouldSucceed {
		t.Error("WouldSucceed = false, want true")
	}
	// neighbor->peer should be in AlreadyDone
	if !sliceContains(result.AlreadyDone, "neighbor->peer") {
		t.Errorf("AlreadyDone = %v, want to contain 'neighbor->peer'", result.AlreadyDone)
	}
}

// TestDryRunWouldApply verifies WouldApply populated for unmigrated configs.
//
// VALIDATES: DryRun shows WouldApply for unmigrated configs.
// PREVENTS: Missing detection of needed migrations.
func TestDryRunWouldApply(t *testing.T) {
	// Create an old config (neighbor, not peer)
	tree := config.NewTree()
	neighbor := config.NewTree()
	tree.AddListEntry("neighbor", "10.0.0.1", neighbor)

	result, err := DryRun(tree)
	if err != nil {
		t.Fatalf("DryRun() error = %v", err)
	}

	if !result.WouldSucceed {
		t.Error("WouldSucceed = false, want true")
	}
	// neighbor->peer should be in WouldApply
	if !sliceContains(result.WouldApply, "neighbor->peer") {
		t.Errorf("WouldApply = %v, want to contain 'neighbor->peer'", result.WouldApply)
	}
}

// TestDryRunCapturesFailure verifies failure info without returning error.
//
// VALIDATES: DryRun captures failure info in result, not as error.
// PREVENTS: DryRun returning error instead of analysis.
func TestDryRunCapturesFailure(t *testing.T) {
	// Create tree with process block that will cause duplicate error
	tree := config.NewTree()
	peer := config.NewTree()

	api := config.NewTree()
	api.Set("processes", "[ foo foo ]") // Duplicate!
	peer.AddListEntry("process", config.KeyDefault, api)

	tree.AddListEntry("peer", "10.0.0.1", peer)

	result, err := DryRun(tree)

	// DryRun should NOT return error - it returns analysis
	if err != nil {
		t.Fatalf("DryRun() error = %v, want nil (analysis in result)", err)
	}
	if result == nil {
		t.Fatal("DryRun() result = nil")
	}

	if result.WouldSucceed {
		t.Error("WouldSucceed = true, want false")
	}
	if result.FailedAt == "" {
		t.Error("FailedAt = '', want transformation name")
	}
	if result.Error == nil {
		t.Error("Error = nil, want error describing failure")
	}
}

// TestNeedsMigrationNil verifies NeedsMigration returns false for nil.
//
// VALIDATES: NeedsMigration handles nil gracefully.
// PREVENTS: Panic on nil tree.
func TestNeedsMigrationNil(t *testing.T) {
	if NeedsMigration(nil) {
		t.Error("NeedsMigration(nil) = true, want false")
	}
}

// TestNeedsMigrationTrue verifies detection for unmigrated config.
//
// VALIDATES: NeedsMigration returns true for old configs.
// PREVENTS: Failing to detect migration need.
func TestNeedsMigrationTrue(t *testing.T) {
	tree := config.NewTree()
	neighbor := config.NewTree()
	tree.AddListEntry("neighbor", "10.0.0.1", neighbor)

	if !NeedsMigration(tree) {
		t.Error("NeedsMigration() = false, want true for old config")
	}
}

// TestNeedsMigrationFalse verifies detection for already-migrated config.
//
// VALIDATES: NeedsMigration returns false for current configs.
// PREVENTS: False positive migration detection.
func TestNeedsMigrationFalse(t *testing.T) {
	tree := config.NewTree()
	peer := config.NewTree()
	tree.AddListEntry("peer", "10.0.0.1", peer)

	if NeedsMigration(tree) {
		t.Error("NeedsMigration() = true, want false for current config")
	}
}

// sliceContains checks if slice contains value.
func sliceContains(slice []string, value string) bool { //nolint:unparam // value varies across tests
	for _, v := range slice {
		if v == value {
			return true
		}
	}
	return false
}
