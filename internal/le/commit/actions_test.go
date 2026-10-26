// Design: docs/contributing/committing.md -- the create keywords help renders
// Related: actions.go -- the commit action table

package commit

import (
	"errors"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	leaction "github.com/ze-software/ze/internal/le/le/action"
)

// TestCommitCreateHelpRendersEveryLiveKeyword validates AC-6: `le commit create
// --help` renders every live keyword from the table, subject required and the
// path and body keywords repeatable, and none of the three retired ones, and
// it answers 0 because the reader asked a question. Method: read the listing's
// create row, then render the page the help word prints.
//
// MUTATION: drop Retired from push and it is listed; drop Repeat from body and
// its row says it is given once.
func TestCommitCreateHelpRendersEveryLiveKeyword(t *testing.T) {
	list := Actions()
	index := slices.IndexFunc(list.Actions, func(row leaction.Row) bool { return row.Verb == "create" })
	if index < 0 {
		t.Fatal("the commit listing holds no create action")
	}
	live := map[string]leaction.Parameter{}
	for _, parameter := range list.Actions[index].Parameters {
		live[parameter.Keyword] = parameter
	}
	want := []string{keywordSubject, keywordBody, keywordFile, keywordFileList, keywordHunks, keywordRemove,
		keywordRemoveList, keywordReplace, keywordAppend, keywordScript, keywordSession, keywordTag, keywordAttest,
		keywordLedgerCarried, keywordNoTest, keywordReviewOverride, keywordRFCChangeOK, keywordDryRun}
	if len(live) != len(want) {
		t.Errorf("create lists %d keywords, want the %d live ones", len(live), len(want))
	}
	page, held := list.ActionPage("le", "create")
	if !held {
		t.Fatal("the commit listing renders no create page")
	}
	var out strings.Builder
	page.WriteTo(&out, false)
	text := out.String()
	for _, keyword := range want {
		if _, listed := live[keyword]; !listed {
			t.Errorf("create does not list %q", keyword)
		}
		if !strings.Contains(text, keyword) {
			t.Errorf("the create page does not render %q:\n%s", keyword, text)
		}
	}
	if live[keywordSubject].Requirement != leaction.Required {
		t.Error("subject is not marked required")
	}
	for _, keyword := range []string{keywordBody, keywordFile, keywordFileList, keywordRemove, keywordRemoveList} {
		if !live[keyword].Repeat {
			t.Errorf("%s is not marked repeatable", keyword)
		}
	}
	for _, retired := range []string{keywordPush, "unverified", "missing-full-verify-ok", "structural-red-ok", "broken-head-fix"} {
		if _, listed := live[retired]; listed {
			t.Errorf("create lists the retired keyword %q", retired)
		}
		if strings.Contains(text, retired) {
			t.Errorf("the create page renders the retired keyword %q:\n%s", retired, text)
		}
	}
	if _, code := Answer([]string{"create", "--help"}); code != 0 {
		t.Errorf("create --help answered %d, want 0", code)
	}
}

// TestTheRetiredRedAdmissionsNameTheBody proves the two admissions the deleted
// structural red check took are refused by name, with their value, and that the
// refusal names the replacement: the reds go in the body. Method: one create
// line per keyword through Answer, which must answer the usage code and print
// a refusal naming the keyword and `body`.
func TestTheRetiredRedAdmissionsNameTheBody(t *testing.T) {
	for _, keyword := range []string{"structural-red-ok", "broken-head-fix"} {
		t.Run(keyword, func(t *testing.T) {
			_, err := parseCreate([]string{"subject", "s", "file", "a", keyword, "a reason"})
			if err == nil {
				t.Fatalf("create accepted %q, whose gate is deleted", keyword)
			}
			if !strings.Contains(err.Error(), strconv.Quote(keyword)) || !strings.Contains(err.Error(), "use body") {
				t.Fatalf("the refusal %q does not name %q and its replacement body", err, keyword)
			}
		})
	}
}

// TestCommitRefusesWhatItsTableRefuses validates AC-17 for commit: a retired
// verb is refused naming its replacement, and the table's own refusals (an
// unknown keyword, a retired keyword, a value missing, a word after push's
// authorisation) answer 2, the code commit answered for each before its grammar
// moved onto the table. Method: drive Answer, the entry point, and read codes.
func TestCommitRefusesWhatItsTableRefuses(t *testing.T) {
	for _, line := range [][]string{
		{"debt-list"},
		{"no-such-verb"},
		{"create", "subject", "s", "mystery"},
		{"create", "subject", "s", "unverified"},
		{"create", "subject"},
		{"create", "subject", "s", "attest", "abc"},
		{"session", "a", "b"},
		{"check", "spec"},
		{"debt-status", "commit"},
		{"check", "file", "a"},
		{"push"},
		{"push", "the owner said yes", "again"},
	} {
		if _, code := Answer(line); code != 2 {
			t.Errorf("commit %q answered %d, want 2", line, code)
		}
	}
	if _, code := Answer(nil); code != 0 {
		t.Errorf("bare commit answered %d, want its listing and 0", code)
	}
}

// TestReviewCheckNotCleanExitsOne validates AC-31: `le commit check`
// over a spec with no recorded review ran, found the review missing, and
// answers 1, the found-a-problem code, never 3, which means it could not run.
// Method: run the action body over a fresh checkout that holds no review.
func TestReviewCheckNotCleanExitsOne(t *testing.T) {
	root := newCommitRepository(t)
	answer, code := answerReviewCheck(root, leaction.Arguments{keywordSpec: {"spec-nothing-reviewed"}})
	if code != 1 {
		t.Fatalf("check over a missing review answered %d, want 1", code)
	}
	review, isReview := answer.(ReviewResult)
	if !isReview {
		t.Fatalf("check answered %T, want a ReviewResult", answer)
	}
	if review.Clean || len(review.Problems) == 0 {
		t.Errorf("check answered 1 without naming the problem: %+v", review)
	}
}

// TestCommitExecutionFailureIsNotAUsageError validates AC-49: `le commit
// create` answers 2 only when it refuses the keywords the line gave, 3 when
// the operation could not run (here, git has no HEAD to prepare against), and
// 1 when it ran and a gate refused it. Method: run the create body three ways
// over fixture checkouts, and read each code with its error line.
func TestCommitExecutionFailureIsNotAUsageError(t *testing.T) {
	refused := newCommitRepository(t)
	_, code := answerCreate(refused, leaction.Arguments{keywordSubject: {"s"}, keywordFile: {"tracked.txt"},
		keywordAppend: {""}, keywordReplace: {""}})
	if code != 2 {
		t.Errorf("append with replace answered %d, want the keyword refusal 2", code)
	}

	headless := t.TempDir()
	writeCommitFixture(t, headless, "go.mod", "module fixture\n")
	writeCommitFixture(t, headless, "feature-gates.txt", "")
	writeCommitFixture(t, headless, "tracked.txt", "tracked\n")
	runCommitGit(t, headless, "init", "-q")
	_, code = answerCreate(headless, leaction.Arguments{keywordSubject: {"s"}, keywordFile: {"tracked.txt"}})
	if code != 3 {
		t.Errorf("a checkout with no HEAD answered %d, want could-not-run 3", code)
	}

	if got := exitCodeOf(errors.New("a gate refused the commit")); got != 1 {
		t.Errorf("an unmarked failure answered %d, want ran-and-failed 1", got)
	}
	if got := exitCodeOf(fmt.Errorf("wrapped: %w", couldNotRun(errors.New("x")))); got != 3 {
		t.Errorf("a wrapped could-not-run answered %d, want 3", got)
	}
}
