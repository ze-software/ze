// Design: docs/contributing/committing.md -- person acts derived from git trailers, approvals spent after publication.
// Related: owed_trailers.go -- readPersonActs and answering.answered, the discharge this file reaches.
// Related: script.go -- renderApprovalPrune, which spends an approval only after the publish.

package commit

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
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
