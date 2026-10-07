package testqemu

import (
	"slices"
	"strings"
	"testing"
)

// VALIDATES: `all-tests test test/<dir>/<name>.ci` runs that one test through
// the VM suite that walks its directory, selecting it by stem, and runs no other
// suite and no other phase; a per-test-namespace suite still gets its
// namespace preparation first.
// PREVENTS: a one-test run that falls back to the whole population (hours of
// guest time for one answer), or selects a directory no suite walks.
func TestOneTestRunsOnlyTheSuiteThatWalksItsDirectory(t *testing.T) {
	for _, tc := range []struct {
		test        string
		wantWords   []string
		wantCommand int // namespace preparation, then the suite
	}{
		{test: "test/policy/policy-boot-apply.ci", wantWords: []string{"policy", "policy-boot-apply"}, wantCommand: 2},
		{test: "test/plugin/show-mtu-host.ci", wantWords: []string{"bgp", "plugin", "show-mtu-host"}, wantCommand: 1},
	} {
		t.Run(tc.test, func(t *testing.T) {
			run := vmFixture(t)
			run.Test = tc.test
			rec := &recorder{}
			run.Run = rec.run

			report, code := run.Execute()
			if code != 0 {
				t.Fatalf("a one-test run whose child answered 0 exited %d: %v", code, report.Failed)
			}
			if len(rec.calls) != tc.wantCommand {
				t.Fatalf("%d commands ran, want %d: %q", len(rec.calls), tc.wantCommand, rec.calls)
			}
			suite := strings.Join(rec.calls[len(rec.calls)-1], " ")
			if !strings.Contains(suite, " test "+strings.Join(tc.wantWords, " ")+" ") {
				t.Fatalf("the suite command %q does not select %v", suite, tc.wantWords)
			}
			if slices.Contains(rec.calls[len(rec.calls)-1], allTests) {
				t.Fatalf("the suite command %q still selects every test", suite)
			}
			if len(report.Phases) != 1 {
				t.Fatalf("%d phases reported, want 1", len(report.Phases))
			}
			if report.Selection != tc.test {
				t.Fatalf("the report names selection %q, want %q", report.Selection, tc.test)
			}
		})
	}
}

// VALIDATES: a one-test run naming a path no VM suite walks is refused before
// any child starts.
// PREVENTS: a refusal read as an empty pass.
func TestOneTestOutsideEverySuiteIsRefused(t *testing.T) {
	for _, test := range []string{"test/nowhere/x.ci", "test/policy/x.et", "internal/x.ci"} {
		run := vmFixture(t)
		run.Test = test
		rec := &recorder{}
		run.Run = rec.run

		if _, code := run.Execute(); code == 0 {
			t.Fatalf("%s: a run naming no suite's test exited 0", test)
		}
		if len(rec.calls) != 0 {
			t.Fatalf("%s: %d commands ran before the refusal", test, len(rec.calls))
		}
	}
}
