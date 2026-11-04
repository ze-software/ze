// Design: docs/contributing/committing.md -- an ordinary judge verdict can approve only one publication.
// Related: approval_prepared_replay_test.go -- publication-time replay of targeted approvals.
package commit

import (
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/rfc"
	testweakened "github.com/ze-software/ze/internal/le/test/weakened"
)

// TestOrdinaryApprovalCannotPublishDifferentAppendedContents records one real
// ordinary verdict, captures it in two native appended blocks with different
// snapshots, and executes the unchanged combined script with real Git. Block A
// must publish and prune its row; block B must refuse the spent verdict without
// losing its retry message or any unrelated pending approval.
func TestOrdinaryApprovalCannotPublishDifferentAppendedContents(t *testing.T) {
	cases := []struct {
		name     string
		approve  func([]string) (any, int)
		source   testweakened.ApprovalSource
		baseline string
		first    string
		second   string
		findings func(*Prepared) int
	}{
		{
			name:     "RFC",
			approve:  rfc.Answer,
			source:   testweakened.RFCApprovals(approvalSession),
			baseline: taggedUnitText("check(1)"),
			first:    taggedUnitText("check(2)"),
			second:   taggedUnitText("check(3)"),
			findings: func(prepared *Prepared) int { return len(prepared.RFCChanges) },
		},
		{
			name:     "Weakening",
			approve:  testweakened.Answer,
			source:   testweakened.WeakeningApprovals(approvalSession),
			baseline: ledgerTestBody("TestThing"),
			first: "package pkg\n\nfunc TestThing(t *testing.T) {\n" +
				"\tt.Skip(\"later\")\n\trequire.Equal(t, 1, got)\n}\n",
			second:   "package pkg\n\nfunc TestThing(t *testing.T) {\n\tt.Skip(\"later\")\n}\n",
			findings: func(prepared *Prepared) int { return len(prepared.Weakened) },
		},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			root := newCommitRepository(t)
			configureCommitAuthor(t, root)
			t.Setenv("CLAUDE_CODE_SESSION_ID", "ordinary-append-replay-"+test.name)
			t.Setenv("ZE_REPO_ROOT", root)
			t.Setenv("ZE_AGENT_ID", "")
			env.ResetCache()
			t.Cleanup(env.ResetCache)
			if _, err := lepath.CommitSession(root, approvalSession); err != nil {
				t.Fatal(err)
			}
			commitFixture(t, root, "ordinary approval baseline", map[string]string{
				approvalTagged:       test.baseline,
				"pkg/unused_test.go": ledgerTestBody("TestUnused"),
			}, nil)
			base := headOf(t, root)
			installNativeReviewChecker(t, root)

			// Snapshot the real writer's unrelated row before adding the selected
			// row. Successful A pruning must restore these exact bytes; comparing
			// against the two-row pre-run file would wrongly require A not to prune.
			ordinaryAppendApprove(t, test.approve, "pkg.TestUnused", "judge: reserved for a later change")
			remainingPending := readLedgerFile(t, root, test.source.Path)
			const reason = "judge: reviewed the first ordinary change"
			ordinaryAppendApprove(t, test.approve, approvalUnit, reason)
			pending := readLedgerFile(t, root, test.source.Path)
			rows, problems := testweakened.LoadApprovals(root, test.source)
			if len(problems) != 0 {
				t.Fatalf("fixture approval rows cannot be read: %v", problems)
			}
			if len(rows) != 2 {
				t.Fatalf("fixture has %d approval rows, want two: %s", len(rows), pending)
			}
			for _, row := range rows {
				if row.Target != "" {
					t.Fatalf("fixture recorded a bound rather than ordinary row: %#v", row)
				}
				if row.Carrier != "" {
					t.Fatalf("fixture verdict was already spent before preparation: %#v", row)
				}
			}

			writeCommitFixture(t, root, approvalTagged, test.first)
			first := ordinaryAppendCreate(t, "ordinary approved block A")
			writeCommitFixture(t, root, approvalTagged, test.second)
			second := ordinaryAppendCreate(t, "ordinary replay block B", "append", "script", first.Script)
			if second.Script != first.Script {
				t.Fatalf("append made another script: %s, want %s", second.Script, first.Script)
			}
			if second.Message == first.Message {
				t.Fatal("append reused block A's message instead of reserving block B's")
			}
			trailer := test.source.TrailerLine(approvalUnit, reason)
			for _, prepared := range []*Prepared{first, second} {
				if test.findings(prepared) == 0 {
					t.Fatalf("fixture block %s did not reach the %s content gate", prepared.Message, test.name)
				}
				message := readLedgerFile(t, root, prepared.Message)
				if !carriesTrailerLine(message, trailer) {
					t.Fatalf("fixture message did not capture the ordinary verdict: %s", message)
				}
				if got := strings.Count(message, trailer); got != 1 {
					t.Fatalf("fixture message captured the verdict %d times: %s", got, message)
				}
				if strings.Contains(message, rfc.ApprovalForTrailer) {
					t.Fatalf("fixture message unexpectedly bound an ordinary verdict: %s", message)
				}
			}
			if got := readLedgerFile(t, root, test.source.Path); got != pending {
				t.Fatal("preparation changed pending approvals before either block published")
			}
			if got := headOf(t, root); got != base {
				t.Fatalf("preparation published a block: %s -> %s", base, got)
			}
			secondMessage := readLedgerFile(t, root, second.Message)

			output, runErr := runScript(t, root, first.Script)
			published := strings.Fields(runCommitGitOutput(t, root, "rev-list", "--reverse", base+"..HEAD"))
			if len(published) == 0 {
				t.Fatalf("fixture failed before block A published, not at replay: %v\n%s", runErr, output)
			}
			firstCommit := published[0]
			if got := revParse(t, root, firstCommit+"^"); got != base {
				t.Fatalf("block A parent = %s, want original %s", got, base)
			}
			if got := runCommitGitOutput(t, root, "show", firstCommit+":"+approvalTagged); got != test.first {
				t.Fatalf("block A published different content at %s: %q, want %q", firstCommit, got, test.first)
			}
			if message := runCommitGitOutput(t, root, "show", "-s", "--format=%B", firstCommit); !carriesTrailerLine(message, trailer) {
				t.Fatalf("block A did not publish its captured verdict at %s: %s", firstCommit, message)
			}
			t.Logf("block A published %s with content %q and verdict %q", firstCommit, test.first, trailer)
			for _, commit := range published[1:] {
				content := runCommitGitOutput(t, root, "show", commit+":"+approvalTagged)
				message := runCommitGitOutput(t, root, "show", "-s", "--format=%B", commit)
				t.Errorf("unexpected later publication %s after block A %s: content=%q, captured B content=%q, message=%q", commit, firstCommit, content, test.second, message)
			}
			if runErr == nil {
				t.Errorf("combined native script accepted ordinary approval replay; publications=%q\n%s", published, output)
			}
			if got := headOf(t, root); got != firstCommit {
				t.Errorf("block B moved HEAD after block A: %s -> %s", firstCommit, got)
			}
			if !strings.Contains(output, "was not published") {
				t.Errorf("script did not report block B's publication refusal: %v\n%s", runErr, output)
			}
			if !strings.Contains(output, approvalUnit) {
				t.Errorf("script did not name the spent ordinary approval unit: %v\n%s", runErr, output)
			}
			if got := readLedgerFile(t, root, test.source.Path); got != remainingPending {
				t.Errorf("A did not prune only its spent row, or B changed remaining pending rows:\ngot: %s\nwant: %s", got, remainingPending)
			}
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(first.Message))); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("published block A retained its message, or its state is unreadable: %v", err)
			}
			message, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(second.Message)))
			if err != nil {
				t.Errorf("refused block B lost its retry message: %v", err)
				return
			}
			if string(message) != secondMessage {
				t.Error("refused block B changed its captured retry message")
			}
		})
	}
}

func ordinaryAppendApprove(t *testing.T, approve func([]string) (any, int), unit, reason string) {
	t.Helper()
	payload, code := approve([]string{"approve", "unit", unit, "reason", reason})
	if code != 0 {
		t.Fatalf("registered ordinary approval of %s = (%#v, %d)", unit, payload, code)
	}
	report, ok := payload.(rfc.ApprovalReport)
	if !ok {
		t.Fatalf("registered ordinary approval result = %T, want rfc.ApprovalReport", payload)
	}
	if report.Target != "" {
		t.Fatalf("registered ordinary approval acquired target %s", report.Target)
	}
}

func ordinaryAppendCreate(t *testing.T, subject string, extra ...string) *Prepared {
	t.Helper()
	arguments := []string{"create", "subject", subject, "file", approvalTagged,
		"session", approvalSession, "no-test", "the fixture carries tests only"}
	payload, code := Answer(slices.Concat(arguments, extra))
	if code != 0 {
		t.Fatalf("registered native preparation of %q = (%#v, %d)", subject, payload, code)
	}
	prepared, ok := payload.(*Prepared)
	if !ok {
		t.Fatalf("registered native preparation result = %T, want *Prepared", payload)
	}
	return prepared
}
