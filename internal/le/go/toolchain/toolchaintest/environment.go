// Design: docs/contributing/running-commands.md -- the Go environment le runs commands under
// Related: ../gotoolchain.go -- Toolchain.Environment and TrimpathGoFlags, the contract checked here
//
// Package toolchaintest holds the checks a test of an le action makes against
// the environment gotoolchain.Toolchain.Environment built for it.
package toolchaintest

import (
	"os"
	"slices"
	"strings"
	"testing"

	gotoolchain "github.com/ze-software/ze/internal/le/go/toolchain"
)

// CheckEnvironmentTail pins the tail Toolchain.Environment builds over
// overrides for a toolchain rooted at root: the overrides in order, then the
// one GOFLAGS entry gotoolchain.TrimpathGoFlags answers for that environment,
// carrying -trimpath. Environment drops every inherited GOFLAGS entry before it
// appends its own, an empty one included, so exactly one GOFLAGS entry is
// present.
//
// root MUST exist. Where Go cannot answer its effective GOFLAGS, Environment
// adds no entry, and a test that reached that branch by accident would pass
// with the GOFLAGS append removed, so the check refuses it: only
// TestEnvironmentWithoutAGoAnswerRunsAndWarnsOnce forces that branch, on
// purpose.
func CheckEnvironmentTail(t testing.TB, label string, environment, overrides []string, root string) {
	t.Helper()
	flags, err := gotoolchain.TrimpathGoFlags(append(os.Environ(), overrides...), root)
	if err != nil {
		t.Errorf("%s: Go cannot answer its effective GOFLAGS in %s (%v); give the fixture a root that exists", label, root, err)
		return
	}
	if !slices.Contains(strings.Fields(strings.TrimPrefix(flags, "GOFLAGS=")), "-trimpath") {
		t.Errorf("%s: TrimpathGoFlags answered %q, which carries no -trimpath", label, flags)
	}
	want := append(slices.Clone(overrides), flags)
	if len(environment) < len(want) {
		t.Errorf("%s has %d entries, want at least %d", label, len(environment), len(want))
		return
	}
	if got := environment[len(environment)-len(want):]; !slices.Equal(got, want) {
		t.Errorf("%s tail differs:\n got: %q\nwant: %q", label, got, want)
	}
	goFlags := 0
	for _, entry := range environment {
		if strings.HasPrefix(entry, "GOFLAGS=") {
			goFlags++
		}
	}
	if goFlags != 1 {
		t.Errorf("%s carries %d GOFLAGS entries, want 1", label, goFlags)
	}
}
