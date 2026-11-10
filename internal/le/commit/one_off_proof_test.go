// Design: docs/contributing/committing.md -- person acts derived from git trailers, approvals spent after publication.
// Related: owed_trailers.go -- readPersonActs and answering.answered, the discharge this file reaches.
// Related: script.go -- renderApprovalPrune, which spends an approval only after the publish.

package commit

import (
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
	lepath "github.com/ze-software/ze/internal/le/le/path"
	"github.com/ze-software/ze/internal/le/rfc"
)

// obligationSession is the harness session the fixture runs under. The commit
// session it resolves to is approvalSession, so the approval file, the message
// and the script all live where the registered create action looks for them.
const obligationSession = "obligations-follow-publication"

// obligationRefuseSwap is a `git` wrapper that refuses the compare-and-swap the
// generated script publishes with, and runs every other git command for real.
const obligationRefuseSwap = "#!/bin/bash\nif [ \"${1:-}\" = update-ref ]; then echo 'fixture refused publication' >&2; exit 1; fi\nexec \"$ZE_TEST_REAL_GIT\" \"$@\"\n"

// retiredShardPath is where the retired verification-debt ledger kept its
// per-commit shards. No reader of it exists any more; the fixture plants one.
const retiredShardPath = "plan/verification-debt/5184512c.md"

// retiredShardBody is a shard as the retired ledger wrote it when it went
// wrong: rows naming a commit that never existed, with a cover of two for one
// create (journal record-written-before-the-operation-succeeds, 2026-08-28 and
// 2026-09-08).
const retiredShardBody = "# Verification debt\n\n" +
	"| commit | cover | reason |\n" +
	"|--------|-------|--------|\n" +
	"| 0123456789abcdef0123456789abcdef01234567 | 2 | create refused for a 73-character subject |\n" +
	"| 0123456789abcdef0123456789abcdef01234567 | 2 | create refused for tag fix(bgp) |\n"

// TestObligationsFollowSuccessfulPublication proves that a commit's person
// acts exist only once git holds the commit, and that an approval is spent
// only by a publication that landed.
//
// VALIDATES: AC-6 / P-6 of spec-tooling-one-off-defects. Each subtest owns its
// fixture, a real repository with a bare upstream, and drives Create, the
// generated script and deriveDebt, the derivation `./le commit debt-status` and
// `./le commit push` read:
//   - invalid subject and invalid tag (the retired ledger's cover and tag
//     cases): Create refuses, writes nothing under tmp/, spends no approval and
//     owes nothing; the same change with a valid message then lands carrying
//     its approval, and the approval is spent.
//   - failed publication: the script fails at the compare-and-swap, HEAD does
//     not move, the approval stays pending and nothing is owed.
//   - retry: the same script after a failed publication lands the commit once,
//     so the obligation it carries is owed exactly once, and the judge's
//     targeted approval published afterwards discharges it.
//   - foreign predecessor record: a retired shard another session committed,
//     naming a commit that never existed, neither adds to nor answers the
//     derived debt; the real obligation is owed, and discharged by publication.
//
// PREVENTS: a record written before the operation it describes succeeds, an
// approval spent by a commit that never landed, and a retired ledger read back
// as current debt.
//
// MUTATION: (prepare.go, `message, err := Message(options.Subject, options.Body, nil)`, `_, _, _ = nextTag(root, options.Session, "")` inserted on the line above it): the invalid subject and invalid tag subtests go red, because a message file is written under tmp/ before validation.
// MUTATION: (script.go, `lines = append(lines, renderPublish(block, scriptPath))`, `lines = append(lines, renderApprovalPrune(block.Approvals[0]), renderPublish(block, scriptPath))`): the failed publication subtest goes red, because the approval is pruned before the refused compare-and-swap.
// MUTATION: (owed.go, `return derived, nil` at the end of deriveDebt, `if out, _ := exec.Command("git", "-C", root, "ls-files", "plan/verification-debt").Output(); len(out) != 0 { derived.Refusals = append(derived.Refusals, "retired shard: "+strings.TrimSpace(string(out))) }` inserted above it): the foreign predecessor record subtest goes red, because the derivation consults the retired shard.
func TestObligationsFollowSuccessfulPublication(t *testing.T) {
	for _, tc := range []struct {
		name    string
		options Options
		refusal string
	}{
		{
			name:    "invalid subject",
			options: Options{Subject: strings.Repeat("s", messageWidth+1)},
			refusal: "over the",
		},
		{
			name:    "invalid tag",
			options: Options{Subject: "change the tagged unit", Tag: "fix(bgp)"},
			refusal: "tag must start",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := obligationFixture(t)
			approveObligationUnit(t, root)
			pending := readApprovalFile(t, root)
			before := revParse(t, root, "HEAD")
			records := tmpListing(t, root)

			options := tc.options
			options.Session = approvalSession
			options.Files = []string{approvalTagged}
			options.NoTest = "the fixture carries tests only"
			_, err := Create(root, &options)
			if err == nil {
				t.Fatalf("%s was prepared", tc.name)
			}
			if !strings.Contains(err.Error(), tc.refusal) {
				t.Fatalf("refusal %q does not name %q", err, tc.refusal)
			}
			if got := tmpListing(t, root); !slices.Equal(got, records) {
				t.Fatalf("a refused create recorded under tmp/:\nbefore %v\nafter  %v", records, got)
			}
			if got := readApprovalFile(t, root); got != pending {
				t.Fatalf("a refused create spent the approval:\nbefore %q\nafter  %q", pending, got)
			}
			if got := revParse(t, root, "HEAD"); got != before {
				t.Fatalf("a refused create moved HEAD: %s -> %s", before, got)
			}
			assertNothingOwed(t, root, "after a refused create")

			// The legitimate route is still open: the same change with a valid
			// message lands, carries the approval and spends it.
			prepared := createObligationChange(t, root, "")
			runCommitScript(t, root, prepared.Script)
			assertCarriesApproval(t, root)
			if got := readApprovalFile(t, root); strings.Contains(got, approvalReason) {
				t.Fatalf("the landed commit left its approval pending: %q", got)
			}
			assertNothingOwed(t, root, "after the approved commit landed")
		})
	}

	t.Run("failed publication", func(t *testing.T) {
		root := obligationFixture(t)
		approveObligationUnit(t, root)
		pending := readApprovalFile(t, root)
		before := revParse(t, root, "HEAD")
		prepared := createObligationChange(t, root, "")

		output, err := runScriptWithGit(t, root, prepared.Script, obligationRefuseSwap)
		if err == nil {
			t.Fatalf("the script ignored a refused compare-and-swap: %s", output)
		}
		if !strings.Contains(output, "fixture refused publication") {
			t.Fatalf("the failure did not reach the publication: %s", output)
		}
		if got := revParse(t, root, "HEAD"); got != before {
			t.Fatalf("a failed publication moved HEAD: %s -> %s", before, got)
		}
		if got := readApprovalFile(t, root); got != pending {
			t.Fatalf("a failed publication spent the approval:\nbefore %q\nafter  %q", pending, got)
		}
		assertNothingOwed(t, root, "after a failed publication")
	})

	t.Run("retry", func(t *testing.T) {
		root := obligationFixture(t)
		before := revParse(t, root, "HEAD")
		prepared := createObligationChange(t, root, "awaiting the judge")
		if output, err := runScriptWithGit(t, root, prepared.Script, obligationRefuseSwap); err == nil {
			t.Fatalf("the script ignored a refused compare-and-swap: %s", output)
		}
		if got := revParse(t, root, "HEAD"); got != before {
			t.Fatalf("a failed publication moved HEAD: %s -> %s", before, got)
		}
		assertNothingOwed(t, root, "after a failed publication")

		runCommitScript(t, root, prepared.Script)
		landed := revParse(t, root, "HEAD")
		if parent := revParse(t, root, "HEAD^"); parent != before {
			t.Fatalf("the retry landed %s on %s, want exactly one commit on %s", landed, parent, before)
		}
		assertOneItem(t, deriveOwed(t, root, ""), landed, owedItemRFCApproval, approvalUnit)

		dischargeByPublication(t, root, landed)
		assertNothingOwed(t, root, "after the judge's approval was published")
	})

	t.Run("foreign predecessor record", func(t *testing.T) {
		root := obligationFixture(t)
		// Another session committed the retired shard before this one started,
		// and it is on the upstream, as the historical shards are.
		writeCommitFixture(t, root, retiredShardPath, retiredShardBody)
		runCommitGit(t, root, "add", "--", retiredShardPath)
		runCommitGit(t, root, "commit", "-q", "-m", "a retired ledger shard")
		runCommitGit(t, root, "push", "-q", "origin", "main")
		assertNothingOwed(t, root, "with only the retired shard")

		prepared := createObligationChange(t, root, "awaiting the judge")
		runCommitScript(t, root, prepared.Script)
		landed := revParse(t, root, "HEAD")
		derived := deriveOwed(t, root, "")
		if len(derived.Refusals) != 0 {
			t.Fatalf("the retired shard reached the derivation: %v", derived.Refusals)
		}
		assertOneItem(t, derived, landed, owedItemRFCApproval, approvalUnit)

		dischargeByPublication(t, root, landed)
		assertNothingOwed(t, root, "after the judge's approval was published")
		content, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(retiredShardPath))) //nolint:gosec // fixture path under t.TempDir
		if err != nil {
			t.Fatal(err)
		}
		if string(content) != retiredShardBody {
			t.Fatalf("the discharge rewrote the retired shard: %q", content)
		}
	})
}

// obligationFixture is a repository whose upstream holds the tagged unit at
// check(1), with the working tree changing it to check(2), and a native review
// checker so Create judges the change as it does in the Ze checkout.
func obligationFixture(t *testing.T) string {
	t.Helper()
	root := newOwedRepository(t)
	t.Setenv("CLAUDE_CODE_SESSION_ID", obligationSession)
	t.Setenv("ZE_REPO_ROOT", root)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
	if _, err := lepath.CommitSession(root, approvalSession); err != nil {
		t.Fatal(err)
	}
	commitOwed(t, root, "the tagged unit lands", map[string]string{approvalTagged: taggedUnitText("check(1)")}, nil)
	runCommitGit(t, root, "push", "-q", "origin", "main")
	writeCommitFixture(t, root, approvalTagged, taggedUnitText("check(2)"))
	installNativeReviewChecker(t, root)
	if err := os.MkdirAll(filepath.Join(root, "tmp"), 0o750); err != nil {
		t.Fatal(err)
	}
	return root
}

// approveObligationUnit records the judge's ordinary approval of the change,
// the row the next commit carries and then spends.
func approveObligationUnit(t *testing.T, root string) {
	t.Helper()
	if _, err := rfc.Approve(root, approvalSession, approvalUnit, approvalReason); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(readApprovalFile(t, root), approvalReason) {
		t.Fatal("the approval was not recorded")
	}
}

// createObligationChange prepares the tagged change. With rfcChangeOK set the
// commit owes the judge's approval as an RFC-approval-owed trailer; without it
// the change rides on a recorded approval.
func createObligationChange(t *testing.T, root, rfcChangeOK string) Prepared {
	t.Helper()
	prepared, err := Create(root, &Options{
		Session:     approvalSession,
		Subject:     "change the tagged unit",
		Files:       []string{approvalTagged},
		NoTest:      "the fixture carries tests only",
		RFCChangeOK: rfcChangeOK,
	})
	if err != nil {
		t.Fatal(err)
	}
	return prepared
}

// dischargeByPublication records the judge's verdict bound to landed and
// publishes it through the registered create action, then runs the script.
func dischargeByPublication(t *testing.T, root, landed string) {
	t.Helper()
	if payload, code := rfc.Answer([]string{"approve", "commit", landed, "unit", approvalUnit, "reason", "judge: reviewed this exact change"}); code != 0 {
		t.Fatalf("targeted approval = (%#v, %d)", payload, code)
	}
	runCommitScript(t, root, createApprovalPublication(t, landed).Script)
}

// assertCarriesApproval checks HEAD's message carries the ordinary approval
// as its trailer line.
func assertCarriesApproval(t *testing.T, root string) {
	t.Helper()
	message := runCommitGitOutput(t, root, "log", "-1", "--format=%B")
	if !carriesTrailerLine(message, rfc.ApprovalTrailerLine(approvalUnit, approvalReason)) {
		t.Fatalf("the landed commit does not carry its approval: %s", message)
	}
}

// assertNothingOwed derives the push debt and refuses any open item or
// refusal: a commit that never landed owes nothing.
func assertNothingOwed(t *testing.T, root, when string) {
	t.Helper()
	derived := deriveOwed(t, root, "")
	if len(derived.Items) != 0 {
		t.Fatalf("%s: owed %+v, want nothing", when, derived.Items)
	}
	if len(derived.Refusals) != 0 {
		t.Fatalf("%s: refusals %v, want none", when, derived.Refusals)
	}
}

// tmpListing answers every file path under root/tmp, sorted, so a test can see
// whether an operation recorded anything there.
func tmpListing(t *testing.T, root string) []string {
	t.Helper()
	var paths []string
	err := filepath.WalkDir(filepath.Join(root, "tmp"), func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(paths)
	return paths
}

// quotedPathSession is the harness session the quoted-path fixture runs under.
const quotedPathSession = "quoted-path-proof"

// quotedPathSpec is the Spec-work stem the quoted-path commit is attributed
// to, so the history reader (specWorkPopulation) is judged over it.
const quotedPathSpec = "quoted-path-proof"

// TestQuotedPathsReachGeneratedScript proves that a path Git would quote is
// selected and published under its exact raw bytes, and that every reader of
// the generated script decodes the same path.
//
// VALIDATES: P-17 "Git paths" of spec-tooling-one-off-defects. Each subtest is
// a real repository where the named path sits beside NEIGHBOURS: the pieces a
// whitespace split would produce, and the C-quoted spelling Git prints for it
// (`"r\303\251sum\303\251.txt"`), each a real tracked file with its own
// working-tree edit. Create selects the path whole or by hunks; then:
//   - the block marker reader (declaredPaths, which replace consults) and the
//     snapshot reader (scriptPaths, which append, the closure review and the
//     newsfragment gate consult) both answer exactly the raw path;
//   - the generated script publishes a commit whose changed paths are exactly
//     the raw path (read NUL-framed), with the selected bytes;
//   - every neighbour keeps its committed blob and its working-tree edit, and
//     the shared index stays empty;
//   - the Spec-work history reader answers exactly the raw path.
//
// A path holding NUL, tab, CR or LF is refused by name through file, hunks and
// remove alike, before anything is written under tmp/ and with HEAD unmoved.
//
// PREVENTS: a raw entry split on whitespace, Git's quoted display kept as the
// path, a refusal that one route omits, and two readers of one script that
// decode its paths differently.
//
// MUTATION: (script.go declaredPaths, `for _, path := range parseShellWords(raw) {`, `for _, path := range strings.Fields(raw) {`): the block marker reader answers the shell-quoted words split on spaces, and every case goes red at declaredPaths.
// MUTATION: (snapshot.go snapshotIndexEntries, `listed, err := gitIndexOutput(root, index, "ls-files", "-s", "-z")`, `listed, err := gitIndexOutput(root, index, "ls-files", "-s")`): the snapshot reads Git's newline-framed, C-quoted listing, so Create refuses every whole-file case with "snapshot holds a path this commit does not name".
// MUTATION: (input.go normalizePath, `if strings.ContainsAny(raw, "\x00\t\r\n") || strings.Count(raw, "\n") != 0 {`, `if strings.ContainsAny(raw, "\x00") {`): the tab, CR and LF refusals go red on every route.
func TestQuotedPathsReachGeneratedScript(t *testing.T) {
	cases := []struct {
		path       string
		neighbours []string
	}{
		{path: "space name.txt", neighbours: []string{"space", "name.txt"}},
		{path: " leading and trailing .txt ", neighbours: []string{"leading", "and", "trailing", ".txt"}},
		{path: "it's.txt", neighbours: []string{"it", "s.txt", `it\'s.txt`}},
		{path: `double"quote.txt`, neighbours: []string{`"double\"quote.txt"`, "double", "quote.txt"}},
		{path: `back\slash.txt`, neighbours: []string{`"back\\slash.txt"`, "back", "slash.txt"}},
		{path: "résumé.txt", neighbours: []string{`"r\303\251sum\303\251.txt"`}},
	}
	for _, tc := range cases {
		for _, selection := range []string{"whole file", "hunks"} {
			t.Run(tc.path+"/"+selection, func(t *testing.T) {
				root := quotedPathFixture(t, append([]string{tc.path}, tc.neighbours...))
				writeCommitFixture(t, root, tc.path, "selected\n")
				for _, neighbour := range tc.neighbours {
					writeCommitFixture(t, root, neighbour, "neighbour edit\n")
				}
				options := Options{Subject: "publish one exact raw path", SpecWork: quotedPathSpec}
				if selection == "whole file" {
					options.Files = []string{tc.path}
				} else {
					writeCommitFixture(t, root, "chosen.patch", runCommitGitOutput(t, root, "diff", "--", tc.path))
					options.Hunks = []HunkSelection{{Path: tc.path, Patch: "chosen.patch"}}
				}
				prepared, err := Create(root, &options)
				if err != nil {
					t.Fatalf("prepare %q: %v", tc.path, err)
				}
				script, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(prepared.Script))) //nolint:gosec // fixture path under t.TempDir
				if err != nil {
					t.Fatal(err)
				}
				if declared := declaredPaths(string(script)); len(declared) != 1 || !declared[tc.path] {
					t.Errorf("block marker paths = %q, want exactly %q", slices.Sorted(maps.Keys(declared)), tc.path)
				}
				snapshot, err := scriptPaths(prepared.Script, string(script))
				if err != nil {
					t.Fatal(err)
				}
				if !slices.Equal(snapshot, []string{tc.path}) {
					t.Errorf("snapshot paths = %q, want exactly %q", snapshot, tc.path)
				}

				runCommitScript(t, root, prepared.Script)
				if changed := runCommitGitOutput(t, root, "diff-tree", "-r", "-z", "--no-renames", "--name-only", "HEAD^", "HEAD"); changed != tc.path+"\x00" {
					t.Errorf("published paths = %q, want exactly %q", changed, tc.path+"\x00")
				}
				if got := runCommitGitOutput(t, root, "show", "HEAD:"+tc.path); got != "selected\n" {
					t.Errorf("published blob of %q = %q, want the selected bytes", tc.path, got)
				}
				for _, neighbour := range tc.neighbours {
					assertNeighbourUntouched(t, root, neighbour)
				}
				if staged := runCommitGitOutput(t, root, "diff", "--cached", "--name-only", "-z"); staged != "" {
					t.Errorf("shared index = %q, want it empty", staged)
				}
				population, err := specWorkPopulation(root, quotedPathSpec, "HEAD")
				if err != nil {
					t.Fatal(err)
				}
				if !slices.Equal(population.Paths, []string{tc.path}) {
					t.Errorf("Spec-work history paths = %q, want exactly %q", population.Paths, tc.path)
				}
			})
		}
	}

	for _, raw := range []string{"nul\x00name.txt", "tab\tname.txt", "cr\rname.txt", "lf\nname.txt"} {
		for _, route := range []string{"file", "hunks", "remove"} {
			t.Run(strconv.Quote(raw)+"/"+route, func(t *testing.T) {
				var tracked []string
				// A NUL cannot name a file; every other name exists and is
				// tracked, so the refusal is by name and not by absence.
				if !strings.Contains(raw, "\x00") {
					tracked = []string{raw}
				}
				root := quotedPathFixture(t, tracked)
				if len(tracked) != 0 {
					writeCommitFixture(t, root, raw, "selected\n")
				}
				writeCommitFixture(t, root, "chosen.patch", "")
				before := revParse(t, root, "HEAD")
				records := tmpListing(t, root)

				options := Options{Subject: "refuse an unframeable path"}
				switch route {
				case "file":
					options.Files = []string{raw}
				case "hunks":
					options.Hunks = []HunkSelection{{Path: raw, Patch: "chosen.patch"}}
				case "remove":
					options.Remove = []string{raw}
				}
				_, err := Create(root, &options)
				if err == nil {
					t.Fatalf("a %s path %q was prepared", route, raw)
				}
				if !strings.Contains(err.Error(), "control character") {
					t.Fatalf("refusal %q does not name the control character", err)
				}
				if got := tmpListing(t, root); !slices.Equal(got, records) {
					t.Fatalf("a refused create recorded under tmp/:\nbefore %v\nafter  %v", records, got)
				}
				if got := revParse(t, root, "HEAD"); got != before {
					t.Fatalf("a refused create moved HEAD: %s -> %s", before, got)
				}
			})
		}
	}
}

// quotedPathFixture is a repository whose HEAD tracks each of paths with the
// content "before\n", under the quoted-path session, with tmp/ present.
func quotedPathFixture(t *testing.T, paths []string) string {
	t.Helper()
	root := newCommitRepository(t)
	configureCommitAuthor(t, root)
	t.Setenv("CLAUDE_CODE_SESSION_ID", quotedPathSession)
	if len(paths) != 0 {
		for _, path := range paths {
			writeCommitFixture(t, root, path, "before\n")
		}
		runCommitGit(t, root, append([]string{"add", "--"}, paths...)...)
		runCommitGit(t, root, "commit", "-q", "-m", "quoted path baseline")
	}
	if err := os.MkdirAll(filepath.Join(root, "tmp"), 0o750); err != nil {
		t.Fatal(err)
	}
	return root
}

// assertNeighbourUntouched checks that neighbour keeps its committed blob and
// its working-tree edit: the commit neither carried nor reverted it.
func assertNeighbourUntouched(t *testing.T, root, neighbour string) {
	t.Helper()
	if got := runCommitGitOutput(t, root, "show", "HEAD:"+neighbour); got != "before\n" {
		t.Errorf("neighbour %q was published as %q", neighbour, got)
	}
	got, err := os.ReadFile(filepath.Join(root, neighbour)) //nolint:gosec // fixture path under t.TempDir
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "neighbour edit\n" {
		t.Errorf("neighbour %q working copy = %q, want its own edit kept", neighbour, got)
	}
}
