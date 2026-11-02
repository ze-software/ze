// Design: docs/contributing/committing.md -- the one owed rule every push debt surface reads
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

	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// TestDebtConsumersNameAnOpenPersonAct validates Data Flow step 6 and AC-22 at
// the surfaces that print the answer: with an open person act, the create
// report and the session-start warning each count it and name the commit that
// owes it, and debt-status answers owed. None of them names a verify.
//
// MUTATION: let DebtWarning or debtReport judge only the refusals and this
// fails: the warning is empty and the report reads nothing owed.
func TestDebtConsumersNameAnOpenPersonAct(t *testing.T) {
	root := newOwedRepository(t)
	closure := owedClosure(t, root)

	warning, err := DebtWarning(root)
	if err != nil {
		t.Fatal(err)
	}
	report := debtReport(root)
	for surface, text := range map[string]string{"session-start warning": warning, "create report": report} {
		for _, want := range []string{"1 person act(s) owed"} {
			if !strings.Contains(text, want) {
				t.Errorf("the %s is %q, want it to say %q", surface, text, want)
			}
		}
		if strings.Contains(text, "le verify") {
			t.Errorf("the %s is %q, which sends an agent to a verify", surface, text)
		}
	}
	if !strings.HasPrefix(warning, "push debt: ") || !strings.Contains(warning, "./le commit push refuses until they clear") ||
		!strings.Contains(warning, shortSHA(closure)) {
		t.Errorf("the warning is %q, want the AC-22 line shape naming %s", warning, shortSHA(closure))
	}
	if _, code, err := answerDebtStatus(root, ""); err != nil || code != 1 {
		t.Errorf("debt-status answered (%d, %v), want 1: owed", code, err)
	}
}

// TestDebtReportSaysNothingOwedWithNoVerifyRecord pins the other side of the
// one owed rule: a HEAD with no open person act and no refusal reads "nothing
// owed" on the create report and nothing on the session-start warning, with
// no verify record in the checkout.
//
// MUTATION: make owed answer true for a HEAD with nothing open and this fails.
func TestDebtReportSaysNothingOwedWithNoVerifyRecord(t *testing.T) {
	root := newOwedRepository(t)
	commitOwed(t, root, "a change", map[string]string{"a.go": "package a\n"}, nil)
	if report := debtReport(root); report != "nothing owed" {
		t.Fatalf("the create report over a HEAD owing nothing is %q, want %q", report, "nothing owed")
	}
	if warning, err := DebtWarning(root); err != nil || warning != "" {
		t.Fatalf("a HEAD owing nothing warned %q (%v)", warning, err)
	}
}

// TestAttestRefusesAdmissionKeywords validates that an attestation refuses each
// admission keyword, naming why, rather than dropping it in silence.
//
// MUTATION: remove refuseAttestAdmission and this fails: create prepares the
// attestation and the keyword's reason is recorded nowhere.
func TestAttestRefusesAdmissionKeywords(t *testing.T) {
	for keyword, set := range map[string]func(*Options){
		keywordReviewOverride: func(o *Options) { o.ReviewOverride = "a reason" },
		keywordRFCChangeOK:    func(o *Options) { o.RFCChangeOK = "a reason" },
		"no-test":             func(o *Options) { o.NoTest = "a reason" },
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
	for _, parameter := range createParameters {
		keyword := parameter.Keyword
		if parameter.Retired != "" || createPolicy[keyword].WithAttest {
			continue
		}
		refused++
		args := []string{"subject", "the owner attests", keywordAttest, owed, "the owner said yes", "dry-run", keyword}
		// A Pair keyword takes two values; hunks is the one that refuses an
		// attestation.
		switch {
		case parameter.Pair != "":
			args = append(args, "tracked.txt", "a.patch")
		case parameter.Kind != leaction.KindSwitch:
			value, named := values[keyword]
			if !named {
				value = "a reason"
			}
			args = append(args, value)
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

	extended := append(slices.Clone(createParameters), leaction.Parameter{Keyword: "a-keyword-nobody-decided",
		Kind: leaction.KindString, Value: "reason", Requirement: leaction.Optional, Help: "h"})
	policy := maps.Clone(createPolicy)
	policy["a-keyword-nobody-decided"] = keywordPolicy{Given: func(*Options) bool { return true }}
	err := refuseUndeclaredWithAttest(&Options{Subject: "the owner attests", Attest: owed, AttestWords: "yes"}, extended, policy)
	if err == nil || !strings.Contains(err.Error(), "attest refused with a-keyword-nobody-decided") {
		t.Fatalf("an undecided keyword answered %v, want it refused by default", err)
	}
}

// TestRefuseUndeclaredWithAttestRefusesLiveKeywordWithoutGiven pins the
// refusal's fail-closed branch: a live keyword the table declares without a
// Given reader cannot be judged, so the attest refusal names it and refuses
// rather than waving it through, while a retired keyword (Retired set), which
// parsing refuses before Options exists, is still skipped.
//
// MUTATION: skip a rule on Given == nil again and the first case fails.
func TestRefuseUndeclaredWithAttestRefusesLiveKeywordWithoutGiven(t *testing.T) {
	options := &Options{Subject: "the owner attests", Attest: "0123456789abcdef", AttestWords: "yes"}

	live := []leaction.Parameter{{Keyword: "a-live-keyword-without-reader",
		Kind: leaction.KindString, Value: "reason", Requirement: leaction.Optional, Help: "h"}}
	err := refuseUndeclaredWithAttest(options, live, map[string]keywordPolicy{})
	if err == nil {
		t.Fatal("a live keyword with no Given reader was waved through, want it refused")
	}
	if !strings.Contains(err.Error(), "a-live-keyword-without-reader") {
		t.Errorf("the refusal %q does not name the keyword", err)
	}

	removed := []leaction.Parameter{{Keyword: "a-removed-keyword", Retired: "./le verify worktree"}}
	if err := refuseUndeclaredWithAttest(options, removed, map[string]keywordPolicy{}); err != nil {
		t.Errorf("a removed keyword answered %v, want it skipped", err)
	}
}

// TestEveryCreateKeywordReadsOptions pins the table's other half: a keyword
// Create can receive carries the reader that says whether Options holds it, so
// the derived refusal cannot pass one over for want of a reader. Only a retired
// keyword, which parsing refuses before Options exists, has none, and the
// policy names no keyword the table does not declare live.
//
// MUTATION: drop the Given of any live keyword and this fails.
func TestEveryCreateKeywordReadsOptions(t *testing.T) {
	live := make(map[string]bool, len(createParameters))
	for _, parameter := range createParameters {
		if parameter.Retired != "" {
			continue
		}
		live[parameter.Keyword] = true
		if createPolicy[parameter.Keyword].Given == nil {
			t.Errorf("create keyword %q has no Given reader: an attestation could not tell whether it was given", parameter.Keyword)
		}
	}
	for keyword := range createPolicy {
		if !live[keyword] {
			t.Errorf("createPolicy names %q, which create does not take as a live keyword", keyword)
		}
	}
}

// TestCreateDebtNamesAnOpenPersonAct validates Data Flow step 6 at the
// surface the operator sees, Create's own result.Debt, rather than at
// debtReport: with an open Review-owed ahead of the upstream, the prepared
// commit counts the person act owed, never nothing and never a verify.
//
// MUTATION: drop result.Debt = debtReport(root) from Create and this fails.
func TestCreateDebtNamesAnOpenPersonAct(t *testing.T) {
	root := newOwedRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", "create-debt-fixture")
	owed := owedClosure(t, root)
	options := Options{Subject: "the owner attests", Attest: owed, AttestWords: "the owner said yes", DryRun: true}
	prepared, err := Create(root, &options)
	if err != nil {
		t.Fatalf("the attestation was refused: %v", err)
	}
	if !strings.Contains(prepared.Debt, "1 person act(s) owed") {
		t.Errorf("Create's debt is %q, want it to count the person act owed", prepared.Debt)
	}
	if strings.HasPrefix(prepared.Debt, "nothing owed") || strings.Contains(prepared.Debt, "le verify") {
		t.Errorf("Create's debt is %q over an open Review-owed", prepared.Debt)
	}
}
