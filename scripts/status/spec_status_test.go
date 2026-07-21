package main

// AC-3 (spec-fixit-spec-hygiene-tooling): committed backlog (design/ready/
// in-progress) and idea capture (skeleton) must render as DISTINCT buckets, and
// skeletons past a TTL must be flagged. The bucketing + TTL logic lives in the
// specbucket subpackage so it is importable both by the go-run spec_status.go
// script and by this test (spec_status.go itself is //go:build ignore and cannot
// be compiled into this package).

import (
	"testing"
	"time"

	"codeberg.org/thomas-mangin/ze/scripts/status/specbucket"
)

// TestSpecStatusBacklogSplit validates the committed-backlog vs idea-capture
// split at the heart of AC-3.
func TestSpecStatusBacklogSplit(t *testing.T) {
	cases := map[string]string{
		"in-progress": specbucket.Backlog,
		"ready":       specbucket.Backlog,
		"design":      specbucket.Backlog,
		"skeleton":    specbucket.Idea,
		"blocked":     specbucket.Other,
		"deferred":    specbucket.Other,
		"unknown":     specbucket.Other,
	}
	for status, want := range cases {
		if got := specbucket.Category(status); got != want {
			t.Errorf("Category(%q) = %q, want %q", status, got, want)
		}
	}
}

// TestSkeletonTTLBoundary pins the flag boundary: at exactly the TTL a skeleton
// is not yet flagged; one day past it is (see the boundary table in the spec).
func TestSkeletonTTLBoundary(t *testing.T) {
	base := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	updated := base.Format("2006-01-02")
	day := 24 * time.Hour

	atTTL := base.Add(time.Duration(specbucket.SkeletonTTLDays) * day)
	if specbucket.SkeletonStale(updated, atTTL) {
		t.Errorf("skeleton at exactly TTL (%d days) must not be flagged", specbucket.SkeletonTTLDays)
	}

	pastTTL := base.Add(time.Duration(specbucket.SkeletonTTLDays+1) * day)
	if !specbucket.SkeletonStale(updated, pastTTL) {
		t.Errorf("skeleton one day past TTL (%d days) must be flagged", specbucket.SkeletonTTLDays+1)
	}

	// A skeleton is only idea capture; a non-skeleton is never TTL-flagged.
	if specbucket.Category("skeleton") != specbucket.Idea {
		t.Fatal("skeleton must classify as idea capture")
	}

	// An unparseable date cannot be judged and must not be flagged (flagging
	// noise trains the reader to ignore the flag).
	if specbucket.SkeletonStale("not-a-date", pastTTL) {
		t.Error("unparseable date must not be flagged")
	}
}
