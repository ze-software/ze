package rfc

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// The rename fixture moves the selftest's tagged unit test, named for no RFC, to
// the name the naming rule asks for.
const (
	renameTarget  = "internal/sample/rfc9999_widget_test.go"
	renameDocRel  = "docs/widget-notes.md"
	renameDocBody = "The proof is `" + selftestTestPath + "::TestWidget`.\n" +
		"A plain " + selftestTestPath + " mention stays.\n" +
		"[the test](" + selftestTestPath + ")\n"
	renameAuditRel = "rfc/audit/rfc9999.json"
	// renameTestSource is the selftest unit test as go vet accepts it, because
	// the check type-checks every package that holds a tag. The tag is spelled
	// as a concatenation so the commit gate's unanchored pattern does not read
	// this file-scope literal as a tag of rename_test.go itself.
	renameTestSource = "package sample\n\nimport \"testing\"\n\n" +
		"// RFC requirement: " + selftestRIDSend + " positive -- SendWidget answers the count it\n" +
		"// was given, so a speaker that sends one widget sends exactly one.\n" +
		"func TestWidget(t *testing.T) {\n\tif SendWidget(1) != 1 {\n\t\tt.Fatal(\"the widget was not sent\")\n\t}\n}\n"
)

// renameFixture is a committed git tree holding the selftest's tagged unit test,
// a verified discrimination record and a stamped audit verdict for it, and a doc
// citing it twice and mentioning it once.
func renameFixture(t *testing.T) string {
	t.Helper()

	files := selftestDiscriminationSources()
	files[selftestTestPath] = renameTestSource
	record := sealFixture(t, files, DiscriminationRecord{
		RID: selftestRIDSend, Polarity: PolarityPositive, Unit: selftestDiscriminationUnit,
		Route: RouteMutant, Producer: selftestProducerUnit, Break: selftestBreak,
	})
	files[selftestDiscriminationRel] = discriminationArtifact(t, record)
	files[renameDocRel] = renameDocBody
	// The tag scanner type-checks every tagged Go package, which needs a module.
	files["go.mod"] = "module example.com/widget\n\ngo 1.27.0\n"
	root := checkFixtureTree(t, files)
	gitFixture(t, root, []string{"init", "-q"})
	from := stampPending(t, root, selftestStem, map[string]any{selftestRIDSend: map[string]any{
		"verdict": VerdictWeak, "note": "the widget test asserts one count only",
	}})
	if _, err := auditStamp(root, selftestStem, from, stampModeNew, stampNow); err != nil {
		t.Fatalf("stamp the fixture verdict: %v", err)
	}
	if err := os.Remove(from); err != nil {
		t.Fatalf("remove the pending file: %v", err)
	}
	layFixture(t, root, nil)
	commitFixture(t, root, "fixture")
	return root
}

func readRel(t *testing.T, root, rel string) string {
	t.Helper()

	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("read %s: %v", rel, err)
	}
	return string(raw)
}

func existsRel(root, rel string) bool {
	_, err := os.Lstat(filepath.Join(root, filepath.FromSlash(rel)))
	return err == nil
}

// treeDigest fingerprints every file under root outside .git, so a refusal can
// be shown to have written nothing at all.
func treeDigest(t *testing.T, root string) map[string][32]byte {
	t.Helper()

	out := map[string][32]byte{}
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		raw, err := os.ReadFile(path) // #nosec G304 -- this test's own fixture tree
		if err != nil {
			return err
		}
		out[path] = sha256.Sum256(raw)
		return nil
	})
	if err != nil {
		t.Fatalf("digest the fixture: %v", err)
	}
	return out
}

// assertRenameRefused runs pairs and asserts a refusal naming every want, with
// the tree byte for byte as it was.
func assertRenameRefused(t *testing.T, root string, pairs []renamePair, wants ...string) {
	t.Helper()

	before := treeDigest(t, root)
	_, err := renameFiles(root, pairs)
	if err == nil {
		t.Fatalf("the rename of %v was not refused", pairs)
	}
	for _, want := range wants {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("the refusal omits %q: %v", want, err)
		}
	}
	after := treeDigest(t, root)
	if len(before) != len(after) {
		t.Fatalf("a refused rename changed the file count from %d to %d", len(before), len(after))
	}
	for path, sum := range before {
		if after[path] != sum {
			t.Errorf("a refused rename wrote %s", path)
		}
	}
}

func renameOne() []renamePair {
	return []renamePair{{Source: selftestTestPath, Target: renameTarget}}
}

// VALIDATES: AC-4 -- the target holds the source's exact bytes, the source is
// gone, and the record's unit names the target.
func TestRenameMovesFileAndRewritesDiscriminationUnits(t *testing.T) {
	root := renameFixture(t)
	original := readRel(t, root, selftestTestPath)

	if _, err := renameFiles(root, renameOne()); err != nil {
		t.Fatalf("rename: %v", err)
	}
	if existsRel(root, selftestTestPath) {
		t.Error("the source is still there")
	}
	if got := readRel(t, root, renameTarget); got != original {
		t.Error("the target's bytes differ from the source's")
	}
	records := readRel(t, root, selftestDiscriminationRel)
	if strings.Contains(records, `"`+selftestTestPath) {
		t.Errorf("a record still names the old path:\n%s", records)
	}
	if !strings.Contains(records, `"`+renameTarget+"::TestWidget\"") {
		t.Errorf("no record names the new unit:\n%s", records)
	}
}

// VALIDATES: AC-4 and A-1 -- the rewritten record still verifies: unit-sha and
// claim-sha do not depend on the path of a _test.go file.
func TestRenameKeepsDiscriminationRecordsVerified(t *testing.T) {
	root := renameFixture(t)
	if _, err := renameFiles(root, renameOne()); err != nil {
		t.Fatalf("rename: %v", err)
	}
	records, err := loadDiscrimination(root)
	if err != nil {
		t.Fatalf("load the records: %v", err)
	}
	covers, err := tagCoversIn(root)
	if err != nil {
		t.Fatalf("resolve the units: %v", err)
	}
	verdicts, err := verifyDiscrimination(root, records, covers)
	if err != nil {
		t.Fatalf("verify: %v", err)
	}
	if len(verdicts) == 0 {
		t.Fatal("no record was verified, so the test proves nothing")
	}
	for _, verdict := range verdicts {
		if !verdict.Verified() {
			t.Errorf("%s %s reads %s after the rename: %s", verdict.Record.Unit, verdict.Record.RID,
				verdict.State, verdict.Detail)
		}
	}
}

// VALIDATES: AC-5 and A-1 -- every audit key naming the source names the
// target, and the verdict still reads fresh.
func TestRenameKeepsAuditVerdictsFresh(t *testing.T) {
	root := renameFixture(t)
	if state := stampFreshness(t, root)[selftestRIDSend].State; state != FreshState {
		t.Fatalf("the fixture verdict reads %s before the rename, want %s", state, FreshState)
	}
	if _, err := renameFiles(root, renameOne()); err != nil {
		t.Fatalf("rename: %v", err)
	}
	audit := readRel(t, root, renameAuditRel)
	if strings.Contains(audit, `"`+selftestTestPath) {
		t.Errorf("the audit still names the old path:\n%s", audit)
	}
	if !strings.Contains(audit, `"`+renameTarget+"::") {
		t.Errorf("the audit does not name the new path:\n%s", audit)
	}
	if state := stampFreshness(t, root)[selftestRIDSend]; state.State != FreshState {
		t.Errorf("the verdict reads %s after the rename (moved %v), want %s", state.State, state.Moved, FreshState)
	}
}

// VALIDATES: AC-6 -- both citations are rewritten and the plain mention is left
// and listed.
func TestRenameRewritesCitationsAndListsPlainMentions(t *testing.T) {
	root := renameFixture(t)
	report, err := renameFiles(root, renameOne())
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	want := "The proof is `" + renameTarget + "::TestWidget`.\n" +
		"A plain " + selftestTestPath + " mention stays.\n" +
		"[the test](" + renameTarget + ")\n"
	if got := readRel(t, root, renameDocRel); got != want {
		t.Errorf("the doc reads:\n%s\nwant:\n%s", got, want)
	}
	for _, where := range []string{renameDocRel + ":1", renameDocRel + ":3"} {
		if !slices.Contains(report.Citations, where) {
			t.Errorf("the report does not list the citation at %s: %v", where, report.Citations)
		}
	}
	if !slices.Contains(report.Mentions, renameDocRel+":2") {
		t.Errorf("the report does not list the plain mention: %v", report.Mentions)
	}
}

// VALIDATES: AC-7 -- an existing target is refused and nothing is written.
func TestRenameRefusesExistingTarget(t *testing.T) {
	root := renameFixture(t)
	layFixture(t, root, map[string]string{renameTarget: "package sample\n"})
	commitFixture(t, root, "the target appears")
	assertRenameRefused(t, root, renameOne(), renameTarget, "already exists")
}

// VALIDATES: AC-7 -- a target in another directory is refused.
func TestRenameRefusesDirectoryChange(t *testing.T) {
	root := renameFixture(t)
	assertRenameRefused(t, root, []renamePair{{Source: selftestTestPath,
		Target: "internal/other/rfc9999_widget_test.go"}}, "another directory")
}

// VALIDATES: AC-7 and R-8 -- a target whose GOOS/GOARCH suffix differs is refused,
// including a suffix go/build knows and no toolchain port carries (`_sparc`,
// `_zos`), which `go tool dist list` alone does not name.
func TestRenameRefusesBuildSuffixChange(t *testing.T) {
	root := renameFixture(t)
	for _, target := range []string{"internal/sample/rfc9999_widget_linux_test.go",
		"internal/sample/rfc9999_widget_sparc_test.go", "internal/sample/rfc9999_widget_zos_test.go"} {
		assertRenameRefused(t, root, []renamePair{{Source: selftestTestPath, Target: target}}, "GOOS/GOARCH")
	}
}

// VALIDATES: R-8 -- the suffix judgement is go/build's own: a name go/build
// constrains to a platform no port builds still moves the platform set when the
// suffix is dropped, and a suffix kept across the rename moves nothing.
func TestBuildSuffixMovesFollowsGoBuild(t *testing.T) {
	platforms, err := goPlatforms()
	if err != nil {
		t.Fatalf("platforms: %v", err)
	}
	cases := []struct {
		source, target string
		moves          bool
	}{
		{"x_sparc_test.go", "rfc1_x_test.go", true},
		{"x_zos_test.go", "rfc1_x_test.go", true},
		{"x_hurd_amd64p32_test.go", "rfc1_x_test.go", true},
		{"x_android_test.go", "rfc1_x_linux_test.go", true},
		{"x_linux_test.go", "rfc1_x_linux_test.go", false},
		{"x_linux_arm64_test.go", "rfc1_x_linux_arm64_test.go", false},
		{"widget_test.go", "rfc1_widget_test.go", false},
	}
	for _, c := range cases {
		moves, err := buildSuffixMoves(c.source, c.target, platforms)
		if err != nil {
			t.Fatalf("%s -> %s: %v", c.source, c.target, err)
		}
		if moves != c.moves {
			t.Errorf("%s -> %s moves the platform set: %v, want %v", c.source, c.target, moves, c.moves)
		}
	}
}

// VALIDATES: AC-7 -- a source or a target that is not a _test.go file is refused.
func TestRenameRefusesNonTestFile(t *testing.T) {
	root := renameFixture(t)
	assertRenameRefused(t, root, []renamePair{{Source: selftestProducerPath,
		Target: "internal/sample/rfc9999_widget.go"}}, selftestProducerPath, "not a _test.go file")
	assertRenameRefused(t, root, []renamePair{{Source: selftestTestPath,
		Target: "internal/sample/rfc9999_widget.go"}}, "not a _test.go file")
	assertRenameRefused(t, root, []renamePair{{Source: selftestTestPath,
		Target: "../rfc9999_widget_test.go"}}, "not a clean path")
}

// VALIDATES: AC-7 and R-1 -- a source edited since HEAD, or one HEAD never held,
// is refused.
func TestRenameRefusesDirtyOrUntrackedSource(t *testing.T) {
	root := renameFixture(t)
	writeFixtureFiles(t, root, map[string]string{selftestTestPath: renameTestSource + "\n"})
	assertRenameRefused(t, root, renameOne(), selftestTestPath, "differs from HEAD")

	fresh := renameFixture(t)
	untracked := "internal/sample/loose_test.go"
	writeFixtureFiles(t, fresh, map[string]string{untracked: "package sample\n"})
	assertRenameRefused(t, fresh, []renamePair{{Source: untracked,
		Target: "internal/sample/still_loose_test.go"}}, untracked, "not tracked")
}

// VALIDATES: AC-8 -- a target the naming rule refuses for the source's tags is
// refused, naming the stem it expected.
func TestRenameRefusesTargetFailingTheNamingRule(t *testing.T) {
	root := renameFixture(t)
	assertRenameRefused(t, root, []renamePair{{Source: selftestTestPath,
		Target: "internal/sample/gadget_test.go"}}, "naming rule", "cites rfc9999", "internal/sample/rfc9999_gadget_test.go")
	assertRenameRefused(t, root, []renamePair{{Source: selftestTestPath,
		Target: "internal/sample/rfc8888_widget_test.go"}}, "naming rule", "rfc9999")
}

// VALIDATES: AC-9 -- one refused pair in a plan refuses the whole plan, naming
// it, and writes nothing; the action answers 2.
func TestRenamePlanRefusesWholeBatchOnOneBadPair(t *testing.T) {
	root := renameFixture(t)
	bad := renamePair{Source: selftestProducerPath, Target: "internal/sample/rfc9999_widget.go"}
	assertRenameRefused(t, root, append(renameOne(), bad), selftestProducerPath)

	plan := filepath.Join(t.TempDir(), "plan.txt")
	body := selftestTestPath + " " + renameTarget + "\n# a comment\n\n" + bad.Source + " " + bad.Target + "\n"
	if err := os.WriteFile(plan, []byte(body), 0o600); err != nil {
		t.Fatalf("plan: %v", err)
	}
	setRenameRoot(t, root)
	if _, code := Answer([]string{"rename", keyPlan, plan}); code != 2 {
		t.Errorf("a plan with a refused pair answered %d, want 2", code)
	}
	if !existsRel(root, selftestTestPath) || existsRel(root, renameTarget) {
		t.Error("the good pair of a refused plan was moved")
	}
}

// VALIDATES: AC-9 -- two pairs naming one target are refused.
func TestRenamePlanRefusesTwoPairsOneTarget(t *testing.T) {
	root := renameFixture(t)
	assertRenameRefused(t, root, []renamePair{{Source: selftestTestPath, Target: renameTarget},
		{Source: "internal/sample/other_test.go", Target: renameTarget}}, "target of two pairs")
}

// VALIDATES: AC-9 -- plan with from/to, from without to, and no form at all are
// refused with exit 2.
func TestRenameRefusesPlanWithFromTo(t *testing.T) {
	root := renameFixture(t)
	setRenameRoot(t, root)
	for what, args := range map[string][]string{
		"plan with from and to": {"rename", keyPlan, "x.txt", keyFrom, selftestTestPath, keyTo, renameTarget},
		"from without to":       {"rename", keyFrom, selftestTestPath},
		"no form":               {"rename"},
		"propose with plan":     {"rename", keyPropose, "out.txt", keyPlan, "x.txt"},
		"under without propose": {"rename", keyFrom, selftestTestPath, keyTo, renameTarget, keyUnder, "internal"},
	} {
		if _, code := Answer(args); code != 2 {
			t.Errorf("%s answered %d, want 2", what, code)
		}
	}
	if !existsRel(root, selftestTestPath) || existsRel(root, renameTarget) {
		t.Error("a refused invocation moved the file")
	}
}

// VALIDATES: AC-9b -- propose writes one pair per part (b) finding, leaves out a
// colliding target and names it, and never names a correctly named file.
func TestRenameProposeWritesOnePairPerFindingAndNamesCollisions(t *testing.T) {
	tagged := func(pkg, function string) string {
		return "package " + pkg + "\n\n// RFC requirement: " + selftestRIDSend + " positive -- it sends one.\n" +
			"func " + function + "() {}\n"
	}
	root := checkFixtureTree(t, map[string]string{
		selftestTestPath:                        selftestTestSource,
		selftestProducerPath:                    selftestProducerSource,
		"internal/other/gadget_test.go":         tagged("other", "TestGadget"),
		"internal/other/rfc9999_gadget_test.go": "package other\n\n" + namingMarkerText + " the name is taken\n",
		"internal/good/rfc9999_good_test.go":    tagged("good", "TestGood"),
	})
	report, err := proposeRenames(root, "", "plan.txt")
	if err != nil {
		t.Fatalf("propose: %v", err)
	}
	want := selftestTestPath + " " + renameTarget + "\n"
	if got := readRel(t, root, "plan.txt"); got != want {
		t.Errorf("the plan reads %q, want %q", got, want)
	}
	if report.Pairs != 1 {
		t.Errorf("the report counts %d pair(s), want 1", report.Pairs)
	}
	if len(report.Collisions) != 1 || !strings.Contains(report.Collisions[0], "internal/other/gadget_test.go") ||
		!strings.Contains(report.Collisions[0], "internal/other/rfc9999_gadget_test.go") {
		t.Errorf("the collision is not named with its taken target: %v", report.Collisions)
	}
	if strings.Contains(report.Text(), "good") {
		t.Errorf("the correctly named file appears in the report:\n%s", report.Text())
	}
	if _, err := proposeRenames(root, "", "plan.txt"); err == nil {
		t.Error("propose overwrote an existing plan file")
	}
	narrowed, err := proposeRenames(root, "internal/other", "narrow.txt")
	if err != nil || narrowed.Pairs != 0 || len(narrowed.Collisions) != 1 {
		t.Errorf("under internal/other answered %+v, %v; want no pair and the one collision", narrowed, err)
	}
}

// VALIDATES: R-2 -- an evidence file another session changed between the
// rename's read and its write refuses the batch, and nothing is written.
func TestRenameRefusesJSONChangedDuringRename(t *testing.T) {
	root := renameFixture(t)
	plan, err := planRename(root, renameOne())
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	concurrent := readRel(t, root, selftestDiscriminationRel) + "\n"
	writeFixtureFiles(t, root, map[string]string{selftestDiscriminationRel: concurrent})
	if _, err := applyRename(root, plan); err == nil || !strings.Contains(err.Error(), selftestDiscriminationRel) {
		t.Fatalf("a concurrent edit answered %v, want a refusal naming the file", err)
	}
	if got := readRel(t, root, selftestDiscriminationRel); got != concurrent {
		t.Error("the concurrent edit was clobbered")
	}
	if !existsRel(root, selftestTestPath) || existsRel(root, renameTarget) {
		t.Error("the file moved although the batch was refused")
	}
}

// VALIDATES: AC-10 -- the report lists the move, each evidence file with its
// key count, each citation, each plain mention, and the next step.
func TestRenameReportNamesIndexUpdate(t *testing.T) {
	root := renameFixture(t)
	report, err := renameFiles(root, renameOne())
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	text := report.Text()
	for _, want := range []string{"moved " + selftestTestPath + " -> " + renameTarget,
		"rewrote " + selftestDiscriminationRel + ": 1 key(s)", "rewrote " + renameAuditRel,
		"citation rewritten: " + renameDocRel + ":1", "plain mention left unchanged, read it: " + renameDocRel + ":2",
		"next: ./le rfc index-update"} {
		if !strings.Contains(text, want) {
			t.Errorf("the report omits %q:\n%s", want, text)
		}
	}
}

// VALIDATES: AC-1, AC-4, AC-5 and R-9 together -- a rename through the action,
// committed, leaves every finding of `./le rfc check` as it was, with the moved
// path read for the old one, and owes nothing new.
func TestRenameEndToEndCheckFindingsUnchanged(t *testing.T) {
	root := renameFixture(t)
	commitFixtureEmptyTip(t, root)
	before, beforeCode := Check(root, nil)

	setRenameRoot(t, root)
	if _, code := Answer([]string{"rename", keyFrom, selftestTestPath, keyTo, renameTarget}); code != 0 {
		t.Fatalf("the rename answered %d", code)
	}
	layFixture(t, root, nil)
	commitFixture(t, root, "rename")
	after, afterCode := Check(root, nil)

	if afterCode != beforeCode {
		t.Errorf("the exit code moved from %d to %d:\n%s", beforeCode, afterCode, after.Text())
	}
	moved := make([]string, 0, len(before.Violations))
	for _, violation := range before.Violations {
		moved = append(moved, strings.ReplaceAll(violation, selftestTestPath, renameTarget))
	}
	if !slices.Equal(moved, after.Violations) {
		t.Errorf("the findings moved.\nbefore:\n%s\nafter:\n%s", strings.Join(moved, "\n"),
			strings.Join(after.Violations, "\n"))
	}
	if after.DiscriminationOwed != 0 || after.DiscriminationProven != before.DiscriminationProven {
		t.Errorf("owed %d and proven %d after the rename, want 0 and %d",
			after.DiscriminationOwed, after.DiscriminationProven, before.DiscriminationProven)
	}
	if before.DiscriminationProven == 0 {
		t.Errorf("the fixture proves nothing, so the comparison is vacuous (cannot run: %q):\n%s", before.CannotRun, before.Text())
	}
}

// setRenameRoot points Answer at the fixture. env.Get answers from a cache built
// once from os.Environ(), so Setenv alone would leave Answer on the developer's
// checkout.
func setRenameRoot(t *testing.T, root string) {
	t.Helper()

	t.Setenv("ZE_REPO_ROOT", root)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
}

// VALIDATES: N-4 of review round 1 -- a write that fails part-way answers the
// report of what was written, through the action, rather than dropping it: the
// move happened, so the operator must be told.
// METHOD: the discrimination directory is made read-only, so the plan reads it,
// the source moves, the audit file is rewritten, and the discrimination rewrite
// fails.
func TestRenameReportsPartialWritesWhenItStops(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root, so a read-only directory proves nothing")
	}
	root := renameFixture(t)
	evidence := filepath.Join(root, filepath.FromSlash(discriminationRel))
	if err := os.Chmod(evidence, 0o500); err != nil {
		t.Fatalf("fixture: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(evidence, 0o700) }) //nolint:errcheck // TempDir cleanup needs the bit back

	setRenameRoot(t, root)
	payload, code := Answer([]string{"rename", keyFrom, selftestTestPath, keyTo, renameTarget})
	if code != 2 {
		t.Fatalf("a rename that stopped part-way answered %d, want 2", code)
	}
	report, ok := payload.(RenameReport)
	if !ok {
		t.Fatalf("the action answered %T, want the partial RenameReport", payload)
	}
	// The audit file sorts before the discrimination file, so it was rewritten
	// before the write that failed, and the report must say so.
	want := []RenameRewrite{{File: renameAuditRel, Keys: 2}}
	if report.Stopped == "" || !slices.Equal(report.Moves, renameOne()) || !slices.Equal(report.Evidence, want) {
		t.Errorf("the partial report reads moves %v, evidence %v, stopped %q; want the move, %v, a reason",
			report.Moves, report.Evidence, report.Stopped, want)
	}
	for _, want := range []string{"moved " + selftestTestPath, "stopped part-way"} {
		if !strings.Contains(report.Text(), want) {
			t.Errorf("the partial report omits %q:\n%s", want, report.Text())
		}
	}
}

// VALIDATES: N-5 of review round 1 -- the evidence rewrite moves the path fields
// and keys only: a record's `unit` and `producer`, an audit requirement's
// `tests`, `units` and `code` keys. A `break`, a note or a fingerprint value
// spelling the old path keeps its bytes, and so does the formatting.
func TestRenameEvidenceRewriteTouchesPathFieldsOnly(t *testing.T) {
	const source, target = "internal/sample/widget_test.go", "internal/sample/rfc9999_widget_test.go"
	pairs := []renamePair{{Source: source, Target: target}}
	records := "{\n  \"rfc\": \"rfc9999\",\n  \"records\": [\n    {\n" +
		"      \"unit\": \"%s::TestWidget\",\n      \"producer\":   \"%s\",\n" +
		"      \"break\": \"" + source + "\",\n      \"route\": \"" + source + "::TestWidget\",\n" +
		"      \"claim-sha\": \"" + source + "x::T\"\n    }\n  ]\n}\n"
	audit := "{\n  \"reaudit_note\": \"" + source + "\",\n  \"requirements\": {\n" +
		"    \"RFC9999-2-1\": {\n      \"note\": \"" + source + "::TestWidget\",\n" +
		"      \"tests\": {\"%s::TestWidget\": \"" + source + "\"},\n" +
		"      \"units\": {\"%s::TestWidget\": \"bb\"},\n      \"code\": {\"%s\": 1}\n    }\n  }\n}\n"
	for _, c := range []struct {
		name   string
		layout string
		fields func(evidenceString) bool
		keys   int
	}{
		{"discrimination", records, discriminationPathField, 2},
		{"audit", audit, auditPathKey, 3},
	} {
		spell := func(path string) string {
			return strings.ReplaceAll(c.layout, "%s", path)
		}
		got, keys, err := rewriteEvidencePaths([]byte(spell(source)), pairs, c.fields)
		if err != nil {
			t.Fatalf("%s: %v", c.name, err)
		}
		if keys != c.keys || string(got) != spell(target) {
			t.Errorf("%s moved %d path(s), want %d, and reads:\n%s\nwant:\n%s", c.name, keys, c.keys,
				got, spell(target))
		}
	}
}

// VALIDATES: I-1 of review round 2 -- an evidence path field whose bytes spell
// the source with an escape (`\/`) is refused rather than rewritten in a second
// spelling, and nothing is written.
// METHOD: the fixture's discrimination record is committed with its unit value
// escaped; JSON decodes it to the source path, so the field walk finds it.
func TestRenameRefusesEscapedEvidencePath(t *testing.T) {
	root := renameFixture(t)
	plain := readRel(t, root, selftestDiscriminationRel)
	quoted := `"` + selftestTestPath + "::"
	if !strings.Contains(plain, quoted) {
		t.Fatalf("fixture: the record holds no unit %s", quoted)
	}
	escaped := strings.Replace(plain, quoted, `"`+strings.ReplaceAll(selftestTestPath, "/", `\/`)+"::", 1)
	writeFixtureFiles(t, root, map[string]string{selftestDiscriminationRel: escaped})
	commitFixture(t, root, "escape the unit")
	assertRenameRefused(t, root, renameOne(), selftestDiscriminationRel, "is escaped")
}

// VALIDATES: I-1 of review round 2 -- an evidence file that is not one JSON
// value is refused, naming the file, and nothing is written.
func TestRenameRefusesTruncatedEvidenceJSON(t *testing.T) {
	root := renameFixture(t)
	plain := readRel(t, root, selftestDiscriminationRel)
	// The cut drops the closing brace only: the decoder then answers io.EOF
	// between two tokens, inside the open object, with every unit still read.
	writeFixtureFiles(t, root, map[string]string{selftestDiscriminationRel: plain[:strings.LastIndex(plain, "}")]})
	commitFixture(t, root, "truncate the record")
	assertRenameRefused(t, root, renameOne(), selftestDiscriminationRel, "not JSON the rename can rewrite")
}

// VALIDATES: I-1 of review round 2 -- the evidence walk answers an error for
// anything but one JSON value: nothing, two values, a cut inside a string, a cut
// between tokens inside an open object or array, and one whole value followed by
// a cut one.
func TestEvidenceStringsRefusesAllButOneValue(t *testing.T) {
	for _, c := range []struct {
		text string
		ok   bool
	}{
		{`{"unit": "a_test.go"}`, true},
		{"", false},
		{`{} {}`, false},
		{`{"unit": "a_te`, false},
		{`{"unit": "a_test.go"`, false},
		{`["a_test.go", `, false},
		{`{} {"unit": "a_test.go"`, false},
	} {
		_, err := evidenceStrings([]byte(c.text))
		if (err == nil) != c.ok {
			t.Errorf("evidenceStrings(%q) answered %v, want ok=%v", c.text, err, c.ok)
		}
	}
}

// VALIDATES: N-6 of review round 1 -- a citation the grammar expands from braces
// holds no literal path to replace, so the rename lists it as stale for a reader
// rather than leaving it citing a file that is gone.
func TestRenameListsBraceCitationsLeftOnTheOldPath(t *testing.T) {
	root := renameFixture(t)
	const braceRel = "docs/brace-notes.md"
	layFixture(t, root, map[string]string{braceRel: "The pair is `internal/sample/widget{_test,}.go`.\n"})
	commitFixture(t, root, "a brace citation")
	report, err := renameFiles(root, renameOne())
	if err != nil {
		t.Fatalf("rename: %v", err)
	}
	if !slices.Contains(report.Stale, braceRel+":1") {
		t.Errorf("the brace citation is not listed as stale: %v\n%s", report.Stale, report.Text())
	}
}

// VALIDATES: N-8 of review round 1 (AC-17 scope) -- the naming rule refuses a
// target only where the check would judge it, a unit carrier: the same name is
// refused under internal/sample/ and accepted under internal/le/, which no
// carrier holds.
func TestRenameJudgesTheNameOfUnitCarriersOnly(t *testing.T) {
	root := renameFixture(t)
	const looseHeld, looseTool = "internal/sample/loose_test.go", "internal/le/sample/loose_test.go"
	layFixture(t, root, map[string]string{looseHeld: "package sample\n", looseTool: "package sample\n"})
	commitFixture(t, root, "two untagged tests")
	table, err := carriers(root)
	if err != nil {
		t.Fatalf("carriers: %v", err)
	}
	if _, held := CarrierFor(looseTool, table); held {
		t.Fatalf("%s is held by a carrier, so it cannot show the scope", looseTool)
	}
	assertRenameRefused(t, root, []renamePair{{Source: looseHeld,
		Target: "internal/sample/rfc9999_loose_test.go"}}, "naming rule")
	if _, err := renameFiles(root, []renamePair{{Source: looseTool,
		Target: "internal/le/sample/rfc9999_loose_test.go"}}); err != nil {
		t.Errorf("a file no carrier holds was refused a name: %v", err)
	}
}
