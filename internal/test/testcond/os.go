// Design: (none -- test utility, no architecture doc)

// Package testcond provides conditional test-skipping helpers.
package testcond

import (
	"runtime"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// RequireOS skips the test unless the current OS matches one of the given names.
// Names use runtime.GOOS values: "linux", "darwin", "freebsd", etc.
func RequireOS(t *testing.T, oses ...string) {
	t.Helper()
	if slices.Contains(oses, runtime.GOOS) {
		return
	}
	t.Skipf("test requires %s (running on %s)", textbuf.Join(oses, " or "), runtime.GOOS)
}
