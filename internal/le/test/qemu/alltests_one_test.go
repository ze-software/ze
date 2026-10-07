package testqemu

import (
	"slices"
	"strings"
	"testing"
)

// VALIDATES: `all-tests test test/<dir>/<name>.ci` runs that one test through
// the VM suite that walks its directory, selecting it by stem, and runs no other
// suite and no other phase; a per-test-namespace suite still gets its
// namespace preparation first. The stem is the LAST word: the runner reads its
// flags up to the first test name, so a `-p` after the stem is itself read as a
// test name ("test \"-p\" not found", the first real guest run).
// PREVENTS: a one-test run that falls back to the whole population (hours of
// guest time for one answer), selects a directory no suite walks, or reports
// its path as an `option=` tag population.
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
			argv := rec.calls[len(rec.calls)-1]
			suite := strings.Join(argv, " ")
			verbs := tc.wantWords[:len(tc.wantWords)-1]
			stem := tc.wantWords[len(tc.wantWords)-1]
			if !strings.Contains(suite, " test "+strings.Join(verbs, " ")+" ") {
				t.Fatalf("the suite command %q does not name the suite %v", suite, verbs)
			}
			if argv[len(argv)-1] != stem {
				t.Fatalf("the suite command %q does not end with the test %q", suite, stem)
			}
			if slices.Contains(rec.calls[len(rec.calls)-1], allTests) {
				t.Fatalf("the suite command %q still selects every test", suite)
			}
			if len(report.Phases) != 1 {
				t.Fatalf("%d phases reported, want 1", len(report.Phases))
			}
			if report.Test != tc.test {
				t.Fatalf("the report names test %q, want %q", report.Test, tc.test)
			}
			if report.Selection != "" {
				t.Fatalf("a one-test run records the tag population %q", report.Selection)
			}
			text := report.Text()
			if !strings.Contains(text, "only "+tc.test+" ran") {
				t.Fatalf("the report text does not name the one test %q:\n%s", tc.test, text)
			}
			if strings.Contains(text, "option=") {
				t.Fatalf("the report text reads the test path as an option tag:\n%s", text)
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
