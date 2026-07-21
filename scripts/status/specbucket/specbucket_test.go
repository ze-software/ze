package specbucket

import (
	"testing"
	"time"
)

// TestCategory pins the committed-backlog vs idea-capture split (AC-3).
func TestCategory(t *testing.T) {
	cases := map[string]string{
		"in-progress": Backlog,
		"ready":       Backlog,
		"design":      Backlog,
		"skeleton":    Idea,
		"blocked":     Other,
		"deferred":    Other,
		"unknown":     Other,
		"":            Other,
	}
	for status, want := range cases {
		if got := Category(status); got != want {
			t.Errorf("Category(%q) = %q, want %q", status, got, want)
		}
	}
}

// TestSkeletonStaleBoundary pins the TTL flag boundary: not flagged at exactly
// the TTL, flagged one day past it; unparseable dates are never flagged.
func TestSkeletonStaleBoundary(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	updated := base.Format("2006-01-02")
	day := 24 * time.Hour

	atTTL := base.Add(time.Duration(SkeletonTTLDays) * day)
	if SkeletonStale(updated, atTTL) {
		t.Errorf("at exactly TTL (%d days) must not be flagged", SkeletonTTLDays)
	}
	pastTTL := base.Add(time.Duration(SkeletonTTLDays+1) * day)
	if !SkeletonStale(updated, pastTTL) {
		t.Errorf("one day past TTL (%d days) must be flagged", SkeletonTTLDays+1)
	}
	if SkeletonStale("not-a-date", pastTTL) {
		t.Error("unparseable date must not be flagged")
	}
}
