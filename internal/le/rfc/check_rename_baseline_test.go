package rfc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixtureGizmoCIPath is where the rename tests below move gadget.ci. It stays in
// the same directory, so the same carrier holds both paths.
const fixtureGizmoCIPath = "test/plugin/gizmo.ci"

// renameFixtureTip commits the corpus with gadget.ci, then commits a tip that
// removes gadget.ci and writes body at gizmo.ci.
func renameFixtureTip(t *testing.T, body string) string {
	t.Helper()

	base := fixtureCorpus()
	base[fixtureGadgetCIPath] = fixtureGadgetCI
	return commitFixtureTip(t, base,
		map[string]string{fixtureGadgetCIPath: fixtureRemoved, fixtureGizmoCIPath: body}, nil)
}

// VALIDATES: AC-1 and the wiring row -- a tip commit that renames a tagged carrier
// file and leaves its bytes alone owes no discrimination record for the covers it
// moved, because the HEAD^ baseline follows a rename whose blob id is unchanged.
// METHOD: two commits; the first holds gadget.ci with an unproven gated tag, the
// second moves it byte for byte to gizmo.ci. The pair test below changes one
// byte and must owe again.
// PREVENTS: every gated tag in a renamed file reading as newly added, which
// blocks a byte-pure rename behind about 787 records nobody owes.
func TestCheckOwesNothingForBytePureRename(t *testing.T) {
	root := renameFixtureTip(t, fixtureGadgetCI)

	report, code := Check(root, nil)
	if code != 0 || len(report.Violations) != 0 {
		t.Fatalf("a byte-pure rename answered %d with %d violation(s):\n%s",
			code, len(report.Violations), report.Text())
	}
	if report.DiscriminationOwed != 0 {
		t.Errorf("owed is %d after a byte-pure rename, want 0", report.DiscriminationOwed)
	}
}

// VALIDATES: AC-2 -- the backlog baseline (origin/main) follows the same
// byte-pure rename, so the unpushed rename adds nothing to the published backlog.
// METHOD: origin/main holds gadget.ci, the next commit renames it byte for byte,
// the tip adds nothing tagged. Without rename following, gizmo.ci's cover reads
// as added since origin/main and the backlog measures 1.
func TestCheckBacklogFollowsBytePureRename(t *testing.T) {
	base := fixtureCorpus()
	base[fixtureGadgetCIPath] = fixtureGadgetCI
	root := checkFixtureTree(t, base)
	gitFixture(t, root, []string{"init", "-q"})
	commitFixture(t, root, "pushed")
	layFixture(t, root, map[string]string{fixtureGadgetCIPath: fixtureRemoved,
		fixtureGizmoCIPath: fixtureGadgetCI})
	commitFixture(t, root, "unpushed, byte-pure rename")
	layFixture(t, root, fixtureCorpusNudge())
	commitFixture(t, root, "unpushed, nothing tagged")
	gitFixture(t, root, []string{"update-ref", "refs/remotes/origin/main", "HEAD~2"})

	report, code := Check(root, nil)
	if code != 0 || len(report.Violations) != 0 {
		t.Fatalf("the backlog tree answered %d with %d violation(s):\n%s",
			code, len(report.Violations), report.Text())
	}
	if report.DiscriminationBacklog == nil {
		t.Fatalf("origin/main resolves, so the backlog must be measured:\n%s", report.Text())
	}
	if *report.DiscriminationBacklog != 0 {
		t.Errorf("the backlog is %d after a byte-pure rename, want 0:\n%s",
			*report.DiscriminationBacklog, report.Text())
	}
}

// VALIDATES: AC-3, above git's similarity threshold -- a rename that changes one
// byte is reviewed as an edit, so every unproven gated cover in it is owed, as
// it is today.
// METHOD: the one-byte edit sits outside the tag lines, so the only thing that
// makes the cover new is that the baseline refuses to follow the rename.
func TestCheckOwesAgainWhenRenameChangesOneByte(t *testing.T) {
	edited := strings.Replace(fixtureGadgetCI, "contains=gadget", "contains=gadgeT", 1)
	if edited == fixtureGadgetCI {
		t.Fatal("the fixture edit changed nothing")
	}
	assertRenameOwesOne(t, renameFixtureTip(t, edited))
}

// VALIDATES: AC-3, below git's similarity threshold -- a rename whose body is
// rewritten is not followed either.
func TestCheckOwesAgainWhenRenameRewritesTheFile(t *testing.T) {
	var rewritten strings.Builder
	rewritten.WriteString(fixtureGadgetCI)
	for range 60 {
		rewritten.WriteString("expect=stdout:contains=a-line-the-base-never-held\n")
	}
	assertRenameOwesOne(t, renameFixtureTip(t, rewritten.String()))
}

func assertRenameOwesOne(t *testing.T, root string) {
	t.Helper()

	report, code := Check(root, nil)
	if code != 2 || report.DiscriminationOwed != 1 {
		t.Fatalf("a rename with an edit answered %d owing %d, want exit 2 owing 1:\n%s",
			code, report.DiscriminationOwed, report.Text())
	}
	if !strings.Contains(report.Text(), fixtureGizmoCIPath) {
		t.Errorf("the obligation does not name the renamed file:\n%s", report.Text())
	}
}

// VALIDATES: a rename map git cannot read leaves the baseline unknown, never
// empty: "no renames" would be a silent wrong answer (ai/rules/principles.md).
// METHOD: the reader over a directory that is no repository, over a revision
// that does not resolve, and the parser over truncated raw output each answer
// false; and coversAt, which needs the map, answers false over the same tree.
func TestCheckRenameMapUnreadableAccusesNobody(t *testing.T) {
	plain := t.TempDir()
	if err := os.WriteFile(filepath.Join(plain, "note.txt"), []byte("not a repository\n"), 0o600); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	if renames, ok := exactRenamesSince(plain, priorRevision, nil); ok || renames != nil {
		t.Errorf("a tree with no git answered %v, %v, want nil, false", renames, ok)
	}

	root := renameFixtureTip(t, fixtureGadgetCI)
	if renames, ok := exactRenamesSince(root, "no-such-revision", nil); ok || renames != nil {
		t.Errorf("an unresolvable revision answered %v, %v, want nil, false", renames, ok)
	}
	if covers, ok := coversAt(plain, priorRevision, nil, nil); ok || covers != nil {
		t.Errorf("coversAt over a tree with no git answered %v, %v, want nil, false", covers, ok)
	}

	sha := strings.Repeat("a", 40)
	truncated := ":100644 100644 " + sha + " " + sha + " R100\x00" + fixtureGadgetCIPath + "\x00"
	if renames, ok := parseExactRenames([]byte(truncated), nil); ok || renames != nil {
		t.Errorf("truncated raw output answered %v, %v, want nil, false", renames, ok)
	}

	carriers, err := headCarriers(root)
	if err != nil {
		t.Fatalf("carriers: %v", err)
	}
	renames, ok := exactRenamesSince(root, priorRevision, carriers)
	if !ok || renames[fixtureGadgetCIPath] != fixtureGizmoCIPath || len(renames) != 1 {
		t.Errorf("the byte-pure rename read as %v, %v, want exactly %s -> %s",
			renames, ok, fixtureGadgetCIPath, fixtureGizmoCIPath)
	}
}
