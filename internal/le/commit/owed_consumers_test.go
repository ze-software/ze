// Design: docs/architecture/testing/verify-freshness-scope.md -- the one owed rule every debt surface reads
//
// VALIDATES: Data Flow step 6 and AC-22 at the surfaces that print the derived
// debt (the create report, the session-start warning, debt-status); an
// attestation's refusal of admission keywords.
// PREVENTS: a debt surface reading "0 owed" or nothing while a push is refused,
// and an admission reason dropped in silence.
package commit

import (
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

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
// report reads "0 commit(s) owe a scoped verify".
func TestDebtConsumersNeverSayZeroWithoutTarget(t *testing.T) {
	cases := []struct {
		name  string
		build func(t *testing.T, root string)
		says  string
	}{
		{name: "tree mismatch with an empty list", says: "the scoped record names", build: func(t *testing.T, root string) {
			t.Helper()
			other := newOwedRepository(t)
			foreign := commitOwed(t, other, "a commit this checkout never saw", map[string]string{"x.txt": "x\n"}, nil)
			record, err := scopedRecordFor(t, root, foreign, revParse(t, other, foreign+"^{tree}"))
			if err != nil {
				t.Fatal(err)
			}
			if err := verify.WriteScopedRecord(root, record); err != nil {
				t.Fatal(err)
			}
		}},
		{name: "no green record with prose-only commits", says: "no scoped record", build: func(t *testing.T, root string) {
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
				for _, want := range []string{"HEAD's tree is not covered by a scoped verify", tc.says, scopedCommand} {
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
	want := "nothing owed: HEAD's tree is covered by the scoped verify of " + shortSHA(green)
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
		"no-test":              func(o *Options) { o.NoTest = "a reason" },
		keywordLedgerCarried: func(o *Options) {
			o.LedgerCarried = []string{"0123456789abcdef0123456789abcdef01234567 review: a reason"}
		},
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

// TestAttestRefusalDerivesFromKeywordTable validates that the attestation's
// refusal is derived from the create keyword table, not from a hand list: every
// keyword the table does not declare WithAttest is refused when an attestation
// carries it, through argv exactly as an operator types it, and a keyword added
// to the table without a decision is refused too.
//
// MUTATION: list the refused keywords by hand again and the added keyword below
// is prepared in silence.
func TestAttestRefusalDerivesFromKeywordTable(t *testing.T) {
	root := newOwedRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "attest-derivation-fixture")
	owed := commitOwed(t, root, "a change that owes a review", map[string]string{"a.txt": "a\n"}, nil,
		"Review-owed: demo: no reviewer")
	list := filepath.Join(t.TempDir(), "paths")
	if err := os.WriteFile(list, []byte("a.txt\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	values := map[string]string{"file": "a.txt", "remove": "a.txt", "file-list": list, "remove-list": list,
		keywordLedgerCarried: "0123456789abcdef0123456789abcdef01234567 review: a reason"}
	refused := 0
	for _, keyword := range slices.Sorted(maps.Keys(createKeywords)) {
		rule := createKeywords[keyword]
		if rule.WithAttest || rule.Replacement != "" {
			continue
		}
		refused++
		args := []string{"subject", "the owner attests", keywordAttest, owed, "the owner said yes", "dry-run", keyword}
		if rule.Value {
			value, named := values[keyword]
			if !named {
				value = "a reason"
			}
			args = append(args, value)
		}
		// A Pair keyword takes two values; hunks is the one that refuses an
		// attestation.
		if rule.Pair {
			args = append(args, "tracked.txt", "a.patch")
		}
		options, err := parseCreate(args)
		if err != nil {
			t.Fatalf("%s: parse %q: %v", keyword, args, err)
		}
		if _, err := Create(root, &options); err == nil || !strings.Contains(err.Error(), "attest refused with") {
			t.Errorf("attest with %s answered %v, want it refused", keyword, err)
		}
	}
	if refused == 0 {
		t.Fatal("the create keyword table declares every keyword WithAttest: nothing was judged")
	}

	extended := maps.Clone(createKeywords)
	extended["a-keyword-nobody-decided"] = keywordRule{Value: true, Given: func(*Options) bool { return true }}
	err := refuseUndeclaredWithAttest(&Options{Subject: "the owner attests", Attest: owed, AttestWords: "yes"}, extended)
	if err == nil || !strings.Contains(err.Error(), "attest refused with a-keyword-nobody-decided") {
		t.Fatalf("an undecided keyword answered %v, want it refused by default", err)
	}
}

// TestRefuseUndeclaredWithAttestRefusesLiveKeywordWithoutGiven pins the
// refusal's fail-closed branch: a live keyword the table declares without a
// Given reader cannot be judged, so the attest refusal names it and refuses
// rather than waving it through, while a removed keyword (Replacement set),
// which parsing refuses before Options exists, is still skipped.
//
// MUTATION: skip a rule on Given == nil again and the first case fails.
func TestRefuseUndeclaredWithAttestRefusesLiveKeywordWithoutGiven(t *testing.T) {
	options := &Options{Subject: "the owner attests", Attest: "0123456789abcdef", AttestWords: "yes"}

	live := map[string]keywordRule{"a-live-keyword-without-reader": {Value: true}}
	err := refuseUndeclaredWithAttest(options, live)
	if err == nil {
		t.Fatal("a live keyword with no Given reader was waved through, want it refused")
	}
	if !strings.Contains(err.Error(), "a-live-keyword-without-reader") {
		t.Errorf("the refusal %q does not name the keyword", err)
	}

	removed := map[string]keywordRule{"a-removed-keyword": {Replacement: "./le verify worktree"}}
	if err := refuseUndeclaredWithAttest(options, removed); err != nil {
		t.Errorf("a removed keyword answered %v, want it skipped", err)
	}
}

// TestEveryCreateKeywordReadsOptions pins the table's other half: a keyword
// Create can receive carries the reader that says whether Options holds it, so
// the derived refusal cannot pass one over for want of a reader. Only a removed
// keyword, which parsing refuses before Options exists, has none.
//
// MUTATION: drop the Given of any live keyword and this fails.
func TestEveryCreateKeywordReadsOptions(t *testing.T) {
	for keyword, rule := range createKeywords {
		if rule.Replacement != "" {
			continue
		}
		if rule.Given == nil {
			t.Errorf("create keyword %q has no Given reader: an attestation could not tell whether it was given", keyword)
		}
	}
}

// TestCreateDebtNeverReadsZeroWithoutGreenRecord validates Data Flow step 6 at
// the surface the operator sees, Create's own result.Debt, rather than at
// debtReport: with no green record and only prose commits ahead of the
// upstream, the prepared commit says HEAD's tree is not covered and names the
// command, never a count of zero and never nothing.
//
// MUTATION: drop result.Debt = debtReport(root) from Create and this fails.
func TestCreateDebtNeverReadsZeroWithoutGreenRecord(t *testing.T) {
	root := newOwedRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "create-debt-fixture")
	owed := commitOwed(t, root, "a page that owes a review", map[string]string{"docs/a.md": "a\n"}, nil,
		"Review-owed: demo: no reviewer")
	options := Options{Subject: "the owner attests", Attest: owed, AttestWords: "the owner said yes", DryRun: true}
	prepared, err := Create(root, &options)
	if err != nil {
		t.Fatalf("the attestation was refused: %v", err)
	}
	for _, want := range []string{"HEAD's tree is not covered by a scoped verify", "no scoped record", scopedCommand} {
		if !strings.Contains(prepared.Debt, want) {
			t.Errorf("Create's debt is %q, want it to say %q", prepared.Debt, want)
		}
	}
	if strings.Contains(prepared.Debt, "0 commit(s)") || strings.HasPrefix(prepared.Debt, "nothing owed") {
		t.Errorf("Create's debt is %q with no green record: it reads as nothing owed", prepared.Debt)
	}
}
