package irr

import (
	"testing"
	"time"
)

// TestDueFromCheckedAt proves a stored dataset is due checked-at plus the
// interval, not refreshed-at plus the interval (AC-3, R-2). Method: a dataset
// answered at T and stopped at T+10 min is planned at T+40 min; it must be
// due at T+60 min. A refreshed-at anchor would make a one-family set due at
// once, and a "from start" anchor would make it due at T+100 min.
func TestDueFromCheckedAt(t *testing.T) {
	checkedAt := time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)
	now := checkedAt.Add(40 * time.Minute)
	cfg := defaultOwnerConfig()

	due, ok := dueOf(setState{stored: true, storedCheckedAt: checkedAt}, cfg, now)
	if !ok {
		t.Fatal("a stored dataset with refresh-interval 3600 has no due time")
	}
	if want := checkedAt.Add(time.Hour); !due.Equal(want) {
		t.Fatalf("due %s, want %s (checked-at plus the interval)", due, want)
	}

	cfg.refreshInterval = 900 * time.Second
	due, _ = dueOf(setState{stored: true, storedCheckedAt: checkedAt}, cfg, now)
	if want := checkedAt.Add(15 * time.Minute); !due.Equal(want) {
		t.Fatalf("interval 900: due %s, want %s", due, want)
	}
}

// TestColdReferenceDueNow proves a referenced set with nothing stored is due
// at once, also with refresh-interval 0 (AC-6, AC-10 survivor).
func TestColdReferenceDueNow(t *testing.T) {
	now := time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)
	for _, interval := range []time.Duration{0, time.Hour} {
		cfg := defaultOwnerConfig()
		cfg.refreshInterval = interval
		due, ok := dueOf(setState{}, cfg, now)
		if !ok || !due.Equal(now) {
			t.Fatalf("interval %s: cold set due %s ok=%v, want now", interval, due, ok)
		}
	}
}

// TestManualIntervalNeverDue proves refresh-interval 0 schedules no automatic
// refresh of a stored set (AC-10), while a failed fetch is still retried.
func TestManualIntervalNeverDue(t *testing.T) {
	now := time.Date(2026, 11, 15, 10, 0, 0, 0, time.UTC)
	cfg := defaultOwnerConfig()
	cfg.refreshInterval = 0
	if due, ok := dueOf(setState{stored: true, storedCheckedAt: now.Add(-48 * time.Hour)}, cfg, now); ok {
		t.Fatalf("manual: stored set due %s, want no automatic refresh", due)
	}
	if _, ok := dueOf(setState{answeredAt: now}, cfg, now); ok {
		t.Fatal("manual: a set answered empty is due again, want manual only")
	}
	retry := now.Add(retryInterval)
	if due, ok := dueOf(setState{stored: true, storedCheckedAt: now, retryAt: retry}, cfg, now); !ok || !due.Equal(retry) {
		t.Fatalf("manual: a failing set due %s ok=%v, want the retry at %s", due, ok, retry)
	}
}

// TestRefreshIntervalBoundary proves the parser keeps the YANG range
// 0 | 60..86400: 0, 60 and 86400 are accepted, 59 and 86401 refused.
func TestRefreshIntervalBoundary(t *testing.T) {
	for _, accepted := range []string{"0", "60", "86400"} {
		if _, err := readSeconds(accepted); err != nil {
			t.Errorf("refresh-interval %s refused: %v", accepted, err)
		}
	}
	for _, refused := range []string{"1", "59", "86401", "-1", "x"} {
		if _, err := readSeconds(refused); err == nil {
			t.Errorf("refresh-interval %s accepted", refused)
		}
	}
}
