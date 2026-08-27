// Related: jobkey.go -- the fingerprint these tests drive from its entry point
//
// VALIDATES: spec-le-is-a-ze-binary AC-11 for the work key of
// scripts/dev/ze-run.sh. A label names the TARGET, and a parameterized target
// represents many jobs under one name. Therefore, the key includes the command
// AND the make command-line variables that the caller typed.
// PREVENTS: the 2026-08-19 defect, where `make ze-unit-pkg-test PKG=./a` and
// the same target on `./b` shared one verdict and a package that was never
// compiled read as passing (plan/journal/stale-artifact-reused.md).

package lejob

import (
	"crypto/sha256"
	"encoding/hex"
	"slices"
	"testing"
)

// TestMakeParamsKeepsOnlyVariableDefinitions verifies the SHAPE rule. A
// definition starts with a letter or an underscore and ends at an `=`. Thus,
// the flags that make adds for itself are excluded. Neither `-j4` nor a
// jobserver address changes a verdict, and that address differs per invocation.
func TestMakeParamsKeepsOnlyVariableDefinitions(t *testing.T) {
	cases := []struct {
		name  string
		flags string
		want  []string
	}{
		{"no flags at all", "", nil},
		{"a bare definition, which is what make writes with no other flags", "PKG=./a", []string{"PKG=./a"}},
		{"the separator make writes when the command line also carried flags", "-j4 -- PKG=./a", []string{"PKG=./a"}},
		{"a jobserver address is not a parameter", "-j --jobserver-auth=fifo:/tmp/x -- PKG=./a", []string{"PKG=./a"}},
		{"two definitions sort into one order whichever way they were typed", "RUN=X PKG=./a", []string{"PKG=./a", "RUN=X"}},
		{"the same two the other way round", "PKG=./a RUN=X", []string{"PKG=./a", "RUN=X"}},
		{"a dotted name is a definition", "ze.tags=ze_web", []string{"ze.tags=ze_web"}},
		{"a tab separates definitions too", "PKG=./a\tRUN=X", []string{"PKG=./a", "RUN=X"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Setenv("MAKEFLAGS", tc.flags)
			if got := MakeParams(); !slices.Equal(got, tc.want) {
				t.Errorf("MakeParams() of %q = %v, want %v", tc.flags, got, tc.want)
			}
		})
	}
}

// TestMakeParamsDropsTheAdmissionKnobs verifies the four names that control how
// a job is ADMITTED. They do not change what it judges. A session that reduces
// the slot count or increases the stall window must still share a running job
// instead of running a duplicate.
func TestMakeParamsDropsTheAdmissionKnobs(t *testing.T) {
	t.Setenv("MAKEFLAGS", "ZE_RUN_SLOTS=1 ZE_JOB_STALL_SECONDS=600 ZE_VERIFY_MAX_LOCK_AGE=600 MAY_ATTACH=0 PKG=./a")
	want := []string{"PKG=./a"}
	if got := MakeParams(); !slices.Equal(got, want) {
		t.Errorf("MakeParams() = %v, want %v: the admission knobs must not split a key", got, want)
	}
}

// TestJobKeyIsTheCommandAndTheParameters verifies the exact byte stream used for
// the key. The shell half and this half must use this same stream before an
// attachment can occur between implementations.
func TestJobKeyIsTheCommandAndTheParameters(t *testing.T) {
	sum := sha256.Sum256([]byte("CMD=make ze-lint\nPKG=./a\n"))
	want := hex.EncodeToString(sum[:])

	got := JobKey([]string{"make", "ze-lint"}, []string{"PKG=./a"})
	if got != want {
		t.Errorf("JobKey = %s, want %s (CMD= line, then one parameter per line)", got, want)
	}
}

// TestJobKeySeparatesDifferentWork is the whole point: two sessions testing
// different packages must not share one verdict, and two sessions asking the
// same question must.
func TestJobKeySeparatesDifferentWork(t *testing.T) {
	argv := []string{"make", "ze-unit-pkg-test"}
	onA := JobKey(argv, []string{"PKG=./a"})
	onB := JobKey(argv, []string{"PKG=./b"})
	if onA == onB {
		t.Error("two packages share one key: the 2026-08-19 defect is back")
	}
	if again := JobKey(argv, []string{"PKG=./a"}); again != onA {
		t.Error("the same work keyed twice answered two keys, so nothing would ever attach")
	}

	other := JobKey([]string{"make", "ze-lint"}, []string{"PKG=./a"})
	if other == onA {
		t.Error("two commands share one key, which is the hand-queued route's own defect")
	}
}
