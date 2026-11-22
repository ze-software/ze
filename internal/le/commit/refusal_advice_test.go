package commit

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/commit/changecheck"
)

// VALIDATES: plan/journal/guard-message-teaches-the-violation.md rows dated
// 2026-11-12 (bashWorktreeCopy advising "Use git merge or cherry-pick"),
// 2026-08-10 (hook remediation advising "tmp/out-$$.log" / "tmp/<subfolder>/"),
// and 2026-08-19 (hook remediation advising "scripts/dev/ze-run.sh" and
// "// rfc-test-change-approved:") are refused by CheckRefusalAdvice on commit,
// while compliant remediation guidance passes.
// PREVENTS: a guard or gate refusal message steering agents onto banned git
// verbs, loose tmp/ scratch paths, or retired scripts/markers.
func TestCheckRefusalAdviceReproducesHistoricalViolations(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		path     string
		source   string
		wantHit  string
		wantPass bool
	}{
		{
			name:    "2026-11-12 worktree copy refusal advises git merge and cherry-pick",
			path:    "internal/le/hookruntime/bash.go",
			source:  "package hookruntime\nconst msg = \"Blocked: copy out of worktree. Use git merge or cherry-pick.\"\n",
			wantHit: "Use git merge",
		},
		{
			name:    "2026-08-10 scratch hook refusal advises tmp/out-$$.log",
			path:    "internal/le/hookruntime/systemtmp.go",
			source:  "package hookruntime\nconst msg = \"Use tmp/out-$$.log instead of /tmp.\"\n",
			wantHit: "tmp/out-",
		},
		{
			name:    "2026-08-19 raw-test refusal advises retired scripts/dev/ze-run.sh",
			path:    "internal/le/hookruntime/bash.go",
			source:  "package hookruntime\nconst msg = \"Run scripts/dev/ze-run.sh instead.\"\n",
			wantHit: "scripts/dev/",
		},
		{
			name:    "2026-08-19 RFC refusal advises retired rfc-test-change-approved marker",
			path:    "internal/le/rfc/gate.go",
			source:  "package rfc\nconst msg = \"Add // rfc-test-change-approved: reason\"\n",
			wantHit: "rfc-test-change-approved:",
		},
		{
			name:     "compliant refusal advising ./le commit create passes",
			path:     "internal/le/hookruntime/bash.go",
			source:   "package hookruntime\nconst msg = \"Commit on the worktree branch with ./le commit create.\"\n",
			wantPass: true,
		},
		{
			name:     "test file outside scope is ignored",
			path:     "internal/le/hookruntime/bash_test.go",
			source:   "package hookruntime\nconst fixture = \"Use git merge or cherry-pick\"\n",
			wantPass: true,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			change := changecheck.Change{
				Paths: []string{tc.path},
				After: func(rel string) (string, bool, error) {
					if rel == tc.path {
						return tc.source, true, nil
					}
					return "", false, nil
				},
			}
			got, err := CheckRefusalAdvice(change)
			if err != nil {
				t.Fatalf("CheckRefusalAdvice: %v", err)
			}
			if tc.wantPass {
				if got != "" {
					t.Fatalf("unexpected refusal:\n%s", got)
				}
				return
			}
			if !strings.Contains(got, tc.wantHit) {
				t.Fatalf("CheckRefusalAdvice = %q, want refusal mentioning %q", got, tc.wantHit)
			}
		})
	}
}

// VALIDATES: CheckRefusalAdvice is registered in changecheck and green over the
// entire live repository under internal/le/{hookruntime,commit,rfc}.
// PREVENTS: an unregistered check or an unfixed banned-advice string remaining
// in the guard/gate packages.
func TestCheckRefusalAdviceRegisteredAndGreenOverCheckout(t *testing.T) {
	t.Parallel()

	registered := changecheck.Registered()
	if !slices.ContainsFunc(registered, func(c changecheck.Check) bool { return c.Name == refusalAdviceCheckName }) {
		t.Fatalf("%q is not registered in changecheck: %+v", refusalAdviceCheckName, registered)
	}

	root := filepath.Clean("../../..")
	hits, err := CheckRefusalAdviceTree(root)
	if err != nil {
		t.Fatalf("CheckRefusalAdviceTree: %v", err)
	}
	if len(hits) != 0 {
		t.Fatalf("live checkout holds banned guard refusal advice:\n%s", formatRefusalAdviceHits(hits))
	}
}
