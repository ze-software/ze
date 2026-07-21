// Design: (none -- build-tool leaf for scripts/status/spec_status.go)
//
// Package specbucket classifies spec statuses into the buckets the spec
// inventory renders, and decides when a skeleton idea-capture stub is stale.
//
// It exists as its own package so the logic is testable: the inventory front
// end (scripts/status/spec_status.go) is a single-file `go run` script tagged
// //go:build ignore, so it cannot be compiled into the scripts/status package
// alongside the tests. A shared leaf package is importable from both the script
// (`go run` follows imports) and the package test.
//
// Buckets separate committed backlog (work someone chose to start: design,
// ready, in-progress) from idea capture (skeleton stubs that are a title plus a
// template and may never be developed). Counting the two together inflates the
// apparent open-work backlog, which is failure mode #3 in the 2026-07-16 audit
// (spec-fixit-spec-hygiene-tooling).
package specbucket

import "time"

// Bucket names.
const (
	Backlog = "backlog" // committed work: design / ready / in-progress
	Idea    = "idea"    // idea capture: skeleton stubs
	Other   = "other"   // blocked / deferred / unknown
)

// SkeletonTTLWeeks is how long a skeleton may sit untouched before it is flagged
// for triage. Six weeks: long enough to survive a normal planning lull, short
// enough that a two-month-old stub (the audit's example) trips it.
const SkeletonTTLWeeks = 6

// SkeletonTTLDays is the TTL expressed in days for age arithmetic.
const SkeletonTTLDays = SkeletonTTLWeeks * 7

// Category maps a spec status to its inventory bucket.
func Category(status string) string {
	switch status {
	case "in-progress", "ready", "design":
		return Backlog
	case "skeleton":
		return Idea
	default:
		return Other
	}
}

// SkeletonStale reports whether a skeleton last updated on `updated`
// (YYYY-MM-DD) is past the TTL relative to `now`. At exactly the TTL it is not
// yet stale (the flag boundary); beyond it, it is. An unparseable date is
// treated as not stale: it cannot be judged, and a false flag trains the reader
// to ignore the flag. TTL never deletes -- it promotes-or-flags (R-2); deletion
// stays a human action.
func SkeletonStale(updated string, now time.Time) bool {
	t, err := time.Parse("2006-01-02", updated)
	if err != nil {
		return false
	}
	ageDays := now.Sub(t).Hours() / 24
	return ageDays > float64(SkeletonTTLDays)
}
