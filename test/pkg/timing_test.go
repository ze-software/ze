package functional

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestTimingCacheBasic verifies basic timing cache operations.
//
// VALIDATES: Cache can store and retrieve timing data.
// PREVENTS: Lost timing data between runs.
func TestTimingCacheBasic(t *testing.T) {
	tc := NewTimingCache()

	// Initially no timing for unknown test
	tm := tc.Get("unknown")
	if tm != nil {
		t.Errorf("Get(unknown) = %v, want nil", tm)
	}

	// Update timing
	tc.Update("test1", 5*time.Second)
	tm = tc.Get("test1")
	if tm == nil {
		t.Fatal("Get(test1) = nil after Update")
	}
	if tm.Actual != 5*time.Second {
		t.Errorf("Get(test1).Actual = %v, want %v", tm.Actual, 5*time.Second)
	}
}

// TestTimingCacheHistory verifies rolling history tracking.
//
// VALIDATES: History keeps last N runs and calculates expected time.
// PREVENTS: Stale expected times from old runs.
func TestTimingCacheHistory(t *testing.T) {
	tc := NewTimingCache()

	// Add several timing entries
	for i := 1; i <= 7; i++ {
		tc.Update("test1", time.Duration(i)*time.Second)
	}

	tm := tc.Get("test1")
	if tm == nil {
		t.Fatal("Get(test1) = nil")
	}

	// History should be limited to 5 entries
	if len(tm.History) > 5 {
		t.Errorf("History length = %d, want <= 5", len(tm.History))
	}

	// Expected should be average of history
	if tm.Expected == 0 {
		t.Error("Expected = 0, want calculated average")
	}
}

// TestTimingCacheSaveLoad verifies persistence.
//
// VALIDATES: Cache can save to file and load back.
// PREVENTS: Timing data lost between program runs.
func TestTimingCacheSaveLoad(t *testing.T) {
	tmpDir := t.TempDir()
	cachePath := filepath.Join(tmpDir, "test_times.json")

	// Create and populate cache
	tc := NewTimingCacheWithPath(cachePath)
	tc.Update("test1", 3*time.Second)
	tc.Update("test2", 7*time.Second)

	// Save to file
	if err := tc.Save(); err != nil {
		t.Fatalf("Save() error = %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Fatal("Cache file not created")
	}

	// Load into new cache
	tc2 := NewTimingCacheWithPath(cachePath)
	if err := tc2.Load(); err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	tm1 := tc2.Get("test1")
	if tm1 == nil {
		t.Fatal("After Load, Get(test1) = nil")
	}
	if tm1.Actual != 3*time.Second {
		t.Errorf("After Load, Get(test1).Actual = %v, want %v", tm1.Actual, 3*time.Second)
	}
}

// TestTimingCacheETA verifies ETA calculation.
//
// VALIDATES: ETA returns sum of expected times for selected tests.
// PREVENTS: Wrong ETA shown to user.
func TestTimingCacheETA(t *testing.T) {
	tc := NewTimingCache()

	tc.Update("test1", 2*time.Second)
	tc.Update("test2", 3*time.Second)
	tc.Update("test3", 5*time.Second)

	// ETA for subset of tests
	eta := tc.ETA([]string{"test1", "test3"})
	want := 7 * time.Second

	// Allow some tolerance for average calculations
	if eta < want-time.Second || eta > want+time.Second {
		t.Errorf("ETA = %v, want approximately %v", eta, want)
	}
}

// TestTimingEntry verifies Timing struct methods.
//
// VALIDATES: Timing struct stores and calculates correctly.
// PREVENTS: Wrong time calculations.
func TestTimingEntry(t *testing.T) {
	tm := &Timing{
		Actual: 5 * time.Second,
		History: []time.Duration{
			3 * time.Second,
			4 * time.Second,
			5 * time.Second,
		},
	}

	tm.UpdateExpected()

	// Expected should be average of history
	wantExpected := 4 * time.Second
	if tm.Expected != wantExpected {
		t.Errorf("Expected = %v, want %v", tm.Expected, wantExpected)
	}
}
