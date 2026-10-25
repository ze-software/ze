// Design: docs/contributing/committing.md -- the create keywords help renders
// Related: actions.go -- the commit action table

package commit

import (
	"slices"
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
		keywordLedgerCarried, keywordNoTest, keywordStructuralRedOK, keywordReviewOverride, keywordBrokenHeadFix,
		keywordRFCChangeOK, keywordDryRun}
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
	for _, retired := range []string{keywordPush, "unverified", "missing-full-verify-ok"} {
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
		{"audit", "base"},
		{"debt-status", "commit"},
		{"review-check", "file", "a"},
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
