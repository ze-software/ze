package callsite

import "testing"

// VALIDATES: Package(1) names the package of the function that calls it, and
// Package(2) the package one frame further up.
// PREVENTS: a registry stamping every entry with its own package, which would
// derive one prefix for every registrant.
func TestPackageNamesTheCaller(t *testing.T) {
	const want = "github.com/ze-software/ze/internal/core/callsite"
	if got := Package(1); got != want {
		t.Fatalf("Package(1) = %q, want %q", got, want)
	}
	if got := callerOf(); got != want {
		t.Fatalf("Package(2) from a helper = %q, want %q", got, want)
	}
}

func callerOf() string { return Package(2) }

// VALIDATES: a stack shorter than the skip answers "".
// PREVENTS: an out-of-range skip naming some unrelated frame.
func TestPackageAnswersEmptyPastTheStack(t *testing.T) {
	if got := Package(10000); got != "" {
		t.Fatalf("Package(10000) = %q, want empty", got)
	}
}
