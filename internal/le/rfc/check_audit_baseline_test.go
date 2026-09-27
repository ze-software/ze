package rfc

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// checkAuditBaselineRel is where the fixture's one audit file lives.
const checkAuditBaselineRel = auditRel + "/rfc9999.json"

// checkAuditBaselineTree commits a corpus whose audit records a 'weak' finding
// on the send row, then commits tip on top of it and checks the result out
// detached, which is the shape `./le verify worktree` judges.
//
// The finding is written by the production stamp, so it carries every field the
// loader demands. tip receives the stamped document and answers the files the
// tip commit writes; nil answers the untagged nudge, a tip that leaves the
// audit alone.
func checkAuditBaselineTree(t *testing.T, tip func(document map[string]any) map[string]string) string {
	t.Helper()

	root := checkFixtureTree(t, fixtureCorpus())
	gitFixture(t, root, []string{"init", "-q"})
	from := stampPending(t, root, "rfc9999", map[string]any{selftestRIDSend: map[string]any{
		"verdict": VerdictWeak, "note": "the widget test asserts nothing about the send",
	}})
	if _, err := auditStamp(root, "rfc9999", from, stampNow); err != nil {
		t.Fatalf("stamp the base finding: %v", err)
	}
	if err := os.Remove(from); err != nil {
		t.Fatalf("remove the pending file: %v", err)
	}
	layFixture(t, root, nil)
	commitFixture(t, root, "base")

	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(checkAuditBaselineRel)))
	if err != nil {
		t.Fatalf("read the stamped audit: %v", err)
	}
	var document map[string]any
	if err := json.Unmarshal(raw, &document); err != nil {
		t.Fatalf("parse the stamped audit: %v", err)
	}
	files := fixtureCorpusNudge()
	if tip != nil {
		files = tip(document)
	}
	layFixture(t, root, files)
	commitFixture(t, root, "tip")
	gitFixture(t, root, []string{"checkout", "-q", "--detach"})
	return root
}

// checkAuditBaselineSendVerdict answers the send row's verdict in document.
func checkAuditBaselineSendVerdict(t *testing.T, document map[string]any) (map[string]any, map[string]any) {
	t.Helper()

	requirements, held := document["requirements"].(map[string]any)
	if !held {
		t.Fatalf("the stamped audit has no requirements object: %v", document)
	}
	verdict, held := requirements[selftestRIDSend].(map[string]any)
	if !held {
		t.Fatalf("the stamped audit has no verdict for %s: %v", selftestRIDSend, requirements)
	}
	return requirements, verdict
}

// VALIDATES: the audit findings and verdict ratchets see what the COMMIT UNDER TEST did.
// baselineAudits read HEAD until 2026-09-27, and in the detached verify worktree the tree
// equals HEAD, so a verdict the tip commit deleted, or a finding it turned into 'enforced',
// was already in the baseline and both ratchets answered clean at the one gate that runs
// (plan/journal/check-cannot-see-the-change-it-looks-for.md).
// METHOD: the base commit records a 'weak' finding. The tip commit deletes it, or upgrades
// it to 'enforced' with no fingerprint moved and no upgrade_reason, and the fixture is
// checked out detached with no working-tree edit. A tip commit that leaves the audit alone
// is the control: neither ratchet may fire there.
func TestCheckAuditRatchetSeesTipCommit(t *testing.T) {
	for _, one := range []struct {
		name       string
		tip        func(t *testing.T, document map[string]any)
		violations []string
	}{
		{
			name: "verdict deleted",
			tip: func(t *testing.T, document map[string]any) {
				requirements, _ := checkAuditBaselineSendVerdict(t, document)
				delete(requirements, selftestRIDSend)
			},
			violations: []string{
				selftestRIDSend + " carried a verdict at HEAD^ and carries none now",
				"the 'weak' finding on " + selftestRIDSend + " was DELETED",
			},
		},
		{
			name: "finding upgraded unchanged",
			tip: func(t *testing.T, document map[string]any) {
				_, verdict := checkAuditBaselineSendVerdict(t, document)
				verdict["verdict"] = VerdictEnforced
			},
			violations: []string{
				selftestRIDSend + " went from 'weak' to 'enforced' while every tagged unit stayed byte-identical",
			},
		},
	} {
		t.Run(one.name, func(t *testing.T) {
			root := checkAuditBaselineTree(t, func(document map[string]any) map[string]string {
				one.tip(t, document)
				return map[string]string{checkAuditBaselineRel: pyDump(document) + "\n"}
			})
			report, code := Check(root)
			joined := strings.Join(report.Violations, "\n")
			for _, want := range one.violations {
				if code != 2 || !strings.Contains(joined, want) {
					t.Errorf("the tip commit's change was not seen, want %q, exit %d:\n%s", want, code, report.Text())
				}
			}
		})
	}

	t.Run("audit untouched", func(t *testing.T) {
		report, _ := Check(checkAuditBaselineTree(t, nil))
		joined := strings.Join(report.Violations, "\n")
		for _, quiet := range []string{"carries none now", "was DELETED", "while every tagged unit stayed byte-identical"} {
			if strings.Contains(joined, quiet) {
				t.Errorf("a tip commit that left the audit alone fired %q:\n%s", quiet, report.Text())
			}
		}
	})
}

// VALIDATES: the extraction ratchet sees what the COMMIT UNDER TEST did. baselineExtractions
// read HEAD until 2026-09-27, so in the detached verify worktree a sign-off the tip commit
// deleted was already absent from the baseline, and the ratchet answered clean
// (plan/journal/check-cannot-see-the-change-it-looks-for.md).
// METHOD: the base commit carries the fixture's signed extraction artifact, the tip commit
// deletes it, and the fixture is checked out detached with no working-tree edit. A tip
// commit that leaves the artifact alone is the control.
func TestCheckExtractionRatchetSeesTipCommit(t *testing.T) {
	const violation = selftestStem + " had an extraction sign-off at HEAD^ and has none now"
	for _, one := range []struct {
		name    string
		deleted bool
	}{
		{name: "sign-off deleted", deleted: true},
		{name: "sign-off untouched", deleted: false},
	} {
		t.Run(one.name, func(t *testing.T) {
			root := checkFixtureTree(t, fixtureCorpus())
			gitFixture(t, root, []string{"init", "-q"})
			commitFixture(t, root, "base")
			files := fixtureCorpusNudge()
			if one.deleted {
				files = nil
				if err := os.Remove(filepath.Join(root, filepath.FromSlash(checkFixtureExtractionRel))); err != nil {
					t.Fatalf("delete the extraction artifact: %v", err)
				}
			}
			layFixture(t, root, files)
			commitFixture(t, root, "tip")
			gitFixture(t, root, []string{"checkout", "-q", "--detach"})
			report, code := Check(root)
			seen := strings.Contains(strings.Join(report.Violations, "\n"), violation)
			if one.deleted && (code != 2 || !seen) {
				t.Fatalf("the tip commit's deletion was not seen, want %q, exit %d:\n%s", violation, code, report.Text())
			}
			if !one.deleted && seen {
				t.Fatalf("a tip commit that left the artifact alone fired %q:\n%s", violation, report.Text())
			}
		})
	}
}
