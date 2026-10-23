// Design: docs/architecture/testing/verify-freshness-scope.md -- the one owed rule every debt surface reads
//
// VALIDATES: Data Flow step 6 and AC-22 at the surfaces that print the derived
// debt (the create report, the session-start warning, debt-status); an
// attestation's refusal of admission keywords.
// PREVENTS: a debt surface reading "0 owed" or nothing while a push is refused,
// and an admission reason dropped in silence.
package commit

import (
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/verify"
)

// TestDebtConsumersNeverSayZeroWithoutTarget validates Data Flow step 6 and
// AC-22 at the surfaces that print the answer: with no push target and nothing
// to list, the create report and the session-start warning each say HEAD's tree
// is not covered by a green verify, name why and ./le verify worktree, and
// never print a count of zero; debt-status answers owed. Two shapes reach it:
// a green record over a tree this branch never held while HEAD is the upstream,
// and no green record at all with only prose commits ahead of the upstream.
//
// MUTATION: let DebtWarning or debtReport judge only the listed commits and the
// open items, as they did, and both cases fail: the warning is empty and the
// report reads "0 commit(s) owe a full verify".
func TestDebtConsumersNeverSayZeroWithoutTarget(t *testing.T) {
	cases := []struct {
		name  string
		build func(t *testing.T, root string)
		says  string
	}{
		{name: "tree mismatch with an empty list", says: "the green record names", build: func(t *testing.T, root string) {
			t.Helper()
			other := newOwedRepository(t)
			foreign := commitOwed(t, other, "a commit this checkout never saw", map[string]string{"x.txt": "x\n"}, nil)
			record, err := verify.NewGreenRecord(foreign, revParse(t, other, foreign+"^{tree}"), 1, time.Now())
			if err != nil {
				t.Fatal(err)
			}
			if err := verify.WriteGreenRecord(root, record); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "no green record with prose-only commits", says: "no green record", build: func(t *testing.T, root string) {
			t.Helper()
			commitOwed(t, root, "a page", map[string]string{"docs/a.md": "a\n"}, nil)
			commitOwed(t, root, "a spec", map[string]string{"plan/b.md": "b\n"}, nil)
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := newOwedRepository(t)
			tc.build(t, root)
			derived := deriveOwed(t, root, "")
			if len(derived.LeftBehind) != 0 || derived.Target != pushTargetNone {
				t.Fatalf("fixture: target %d left behind %+v, want no target and nothing listed",
					derived.Target, derived.LeftBehind)
			}

			warning, err := DebtWarning(root)
			if err != nil {
				t.Fatal(err)
			}
			report := debtReport(root)
			for surface, text := range map[string]string{"session-start warning": warning, "create report": report} {
				for _, want := range []string{"HEAD's tree is not covered by a green verify", tc.says, verifyWorktreeCommand} {
					if !strings.Contains(text, want) {
						t.Errorf("the %s is %q, want it to say %q", surface, text, want)
					}
				}
				if strings.Contains(text, "0 commit(s)") {
					t.Errorf("the %s is %q, a count of zero with no verified tree", surface, text)
				}
			}
			if !strings.HasPrefix(warning, "verification debt: ") || !strings.Contains(warning, "push refused until cleared") {
				t.Errorf("the warning is %q, want the AC-22 line shape", warning)
			}
			if _, code, err := answerDebtStatus(root, ""); err != nil || code != 1 {
				t.Errorf("debt-status answered (%d, %v), want 1: owed", code, err)
			}
		})
	}
}

// TestDebtReportSaysNothingOwedOnlyWhenCovered pins the other side of the one
// owed rule: a verified HEAD reads "nothing owed" on the create report and
// nothing on the session-start warning, and never a count.
//
// MUTATION: make owed answer true for a covered HEAD and this fails.
func TestDebtReportSaysNothingOwedOnlyWhenCovered(t *testing.T) {
	root := newOwedRepository(t)
	green := commitOwed(t, root, "the verified change", map[string]string{"a.txt": "a\n"}, nil)
	writeOwedGreen(t, root, green)
	want := "nothing owed: HEAD's tree is covered by the green verify of " + shortSHA(green)
	if report := debtReport(root); report != want {
		t.Fatalf("the create report over a verified HEAD is %q, want %q", report, want)
	}
	if warning, err := DebtWarning(root); err != nil || warning != "" {
		t.Fatalf("a verified HEAD warned %q (%v)", warning, err)
	}
}

// TestAttestRefusesAdmissionKeywords validates that an attestation refuses each
// admission keyword, naming why, rather than dropping it in silence.
//
// MUTATION: remove refuseAttestAdmission and this fails: create prepares the
// attestation and the keyword's reason is recorded nowhere.
func TestAttestRefusesAdmissionKeywords(t *testing.T) {
	for keyword, set := range map[string]func(*Options){
		keywordReviewOverride:  func(o *Options) { o.ReviewOverride = "a reason" },
		keywordRFCChangeOK:     func(o *Options) { o.RFCChangeOK = "a reason" },
		keywordStructuralRedOK: func(o *Options) { o.StructuralRedOK = "a reason" },
		keywordBrokenHeadFix:   func(o *Options) { o.BrokenHeadFix = "a reason" },
	} {
		t.Run(keyword, func(t *testing.T) {
			root := newOwedRepository(t)
			t.Setenv("CLAUDE_CODE_SESSION_ID", "attest-admission-fixture")
			owed := commitOwed(t, root, "a change that owes a review", map[string]string{"a.txt": "a\n"}, nil,
				"Review-owed: demo: no reviewer")
			options := Options{Subject: "the owner attests", Attest: owed, AttestWords: "the owner said yes"}
			set(&options)
			_, err := Create(root, &options)
			if err == nil {
				t.Fatalf("attest with %s was prepared", keyword)
			}
			if !strings.Contains(err.Error(), "attest refused with "+keyword) || !strings.Contains(err.Error(), "runs no gate") {
				t.Fatalf("attest with %s refused as %v, want the refusal naming it and why", keyword, err)
			}
		})
	}
}
