// Design: docs/contributing/rfc-conformance-gates.md -- Moving a tagged test file.
// Related: rename_test.go -- the tracked-source fixtures and refusal assertions.
package rfc

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// newRenameCarrierFixture adds a tagged source only after committing the fixture,
// so HEAD and the index have never held the source the command must move.
func newRenameCarrierFixture(t *testing.T) string {
	t.Helper()

	files := selftestDiscriminationSources()
	delete(files, selftestTestPath)
	files["go.mod"] = "module example.com/widget\n\ngo 1.27.0\n"
	return commitFixtureTree(t, files, map[string]string{selftestTestPath: renameTestSource})
}

// A new tagged carrier reaches the canonical command and keeps its exact body,
// including the requirement tag, when it takes the required RFC filename.
func TestRenameMovesNewTaggedCarrier(t *testing.T) {
	root := newRenameCarrierFixture(t)
	setRenameRoot(t, root)
	answer, code := Answer([]string{"move", keyFrom, selftestTestPath, keyTo, renameTarget})
	if code != 0 {
		t.Fatalf("move the new tagged carrier: exit %d: %v", code, answer)
	}
	if existsRel(root, selftestTestPath) {
		t.Error("the source remains after the move")
	}
	if got := readRel(t, root, renameTarget); got != renameTestSource {
		t.Errorf("the body or tag changed during the move:\n%s", got)
	}
}

// Proof and audit keys follow a new carrier without replacing the recorded
// fingerprints; the existing verifiers must still accept both after the move.
func TestRenameNewTaggedCarrierKeepsEvidence(t *testing.T) {
	files := selftestDiscriminationSources()
	files[selftestTestPath] = renameTestSource
	record := sealFixture(t, files, &DiscriminationRecord{
		RID: selftestRIDSend, Polarity: PolarityPositive, Unit: selftestDiscriminationUnit,
		Route: RouteMutant, Producer: selftestProducerUnit, Break: selftestBreak,
	})
	evidence := map[string]string{}
	putDiscrimination(t, evidence, record)
	root := newRenameCarrierFixture(t)
	writeFixtureFiles(t, root, evidence)
	from := stampPending(t, root, selftestStem, map[string]any{selftestRIDSend: map[string]any{
		"verdict": VerdictWeak, "note": "the widget test asserts one count only",
	}})
	if _, err := auditStamp(root, selftestStem, from, stampModeNew); err != nil {
		t.Fatalf("stamp the new carrier's audit: %v", err)
	}
	if err := os.Remove(from); err != nil {
		t.Fatalf("remove the pending audit: %v", err)
	}
	proof := readRel(t, root, selftestDiscriminationRel)
	audit := readRel(t, root, renameAuditRel)
	if _, err := renameFiles(root, renameOne()); err != nil {
		t.Fatalf("move the new carrier with evidence: %v", err)
	}
	for rel, original := range map[string]string{selftestDiscriminationRel: proof, renameAuditRel: audit} {
		want := strings.ReplaceAll(original, selftestTestPath+"::", renameTarget+"::")
		if got := readRel(t, root, rel); got != want {
			t.Errorf("%s changed more than the path keys:\n%s", rel, got)
		}
	}
	records, err := loadDiscrimination(root)
	if err != nil {
		t.Fatalf("load the moved records: %v", err)
	}
	covers, err := tagCoversIn(root)
	if err != nil {
		t.Fatalf("resolve the moved carrier: %v", err)
	}
	verdicts, err := verifyDiscrimination(root, records, covers)
	if err != nil {
		t.Fatalf("verify the moved records: %v", err)
	}
	if len(verdicts) != 1 {
		t.Fatalf("verified %d records, want the one new carrier record", len(verdicts))
	}
	if !verdicts[0].Verified() {
		t.Errorf("the moved record reads %s: %s", verdicts[0].State, verdicts[0].Detail)
	}
	if got := stampFreshness(t, root)[selftestRIDSend]; got.State != FreshState {
		t.Errorf("the moved audit reads %s, want %s", got.State, FreshState)
	}
}

// Admitting a new carrier must retain the same batch, path, target and naming
// refusals, with every refusal leaving the entire fixture unchanged.
func TestRenameNewTaggedCarrierRefusesUnsafePairs(t *testing.T) {
	cases := []struct {
		name   string
		pairs  []renamePair
		files  map[string]string
		reason string
	}{
		{"taken target", renameOne(), map[string]string{renameTarget: "package sample\n"}, "already exists"},
		{"other directory", []renamePair{{selftestTestPath, "internal/other/rfc9999_widget_test.go"}}, nil, "another directory"},
		{"build suffix", []renamePair{{selftestTestPath, "internal/sample/rfc9999_widget_linux_test.go"}}, nil, "GOOS/GOARCH"},
		{"wrong RFC", []renamePair{{selftestTestPath, "internal/sample/rfc8888_widget_test.go"}}, nil, "naming rule"},
		{"no RFC name", []renamePair{{selftestTestPath, "internal/sample/other_test.go"}}, nil, "naming rule"},
		{"outside path", []renamePair{{"../widget_test.go", renameTarget}}, nil, "not a clean path"},
		{"missing source", []renamePair{{"internal/sample/missing_test.go", renameTarget}}, nil, "cannot be read"},
		{"shared source", append(renameOne(), renameOne()...), nil, "source by two pairs"},
		{"shared target", append(renameOne(), renamePair{"internal/sample/missing_test.go", renameTarget}), nil, "target of two pairs"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := newRenameCarrierFixture(t)
			writeFixtureFiles(t, root, tc.files)
			assertRenameRefused(t, root, tc.pairs, tc.reason)
		})
	}
}

// Removing a tracked file from the index and recreating it must not turn an
// edited HEAD source into a new carrier eligible to bypass the dirty guard.
func TestRenameRefusesRecreatedTrackedCarrier(t *testing.T) {
	root := renameFixture(t)
	gitFixture(t, root, []string{"rm", "--cached", "--", selftestTestPath})
	writeFixtureFiles(t, root, map[string]string{selftestTestPath: renameTestSource + "\n"})
	assertRenameRefused(t, root, renameOne(), "differs from HEAD")
}

// A new tagged source is admitted only when Git confirms it is untracked and
// not ignored, and its directory entry is a regular file.
func TestRenameNewTaggedCarrierRefusesUnconfirmedSources(t *testing.T) {
	t.Run("staged source", func(t *testing.T) {
		root := newRenameCarrierFixture(t)
		gitFixture(t, root, []string{"add", "--", selftestTestPath})
		assertRenameRefused(t, root, renameOne(), "not a new untracked carrier")
	})
	t.Run("ignored source", func(t *testing.T) {
		root := newRenameCarrierFixture(t)
		writeFixtureFiles(t, root, map[string]string{".gitignore": "/" + selftestTestPath + "\n"})
		assertRenameRefused(t, root, renameOne(), "not a new untracked carrier")
	})
	t.Run("symlink source", func(t *testing.T) {
		root := newRenameCarrierFixture(t)
		const body = "internal/sample/widget-body.txt"
		writeFixtureFiles(t, root, map[string]string{body: renameTestSource, selftestTestPath: fixtureRemoved})
		if err := os.Symlink("widget-body.txt", filepath.Join(root, filepath.FromSlash(selftestTestPath))); err != nil {
			t.Fatalf("create the fixture's symlink source: %v", err)
		}
		assertRenameRefused(t, root, renameOne(), "not a regular file")
	})
	t.Run("no HEAD", func(t *testing.T) {
		files := selftestDiscriminationSources()
		files[selftestTestPath] = renameTestSource
		root := checkFixtureTree(t, files)
		assertRenameRefused(t, root, renameOne(), "cannot inspect HEAD")
	})
}

// A source body changed after planning must refuse before any source, proof or
// citation moves, since the plan judged the original body's tags and names.
func TestRenameRefusesNewCarrierChangedAfterPlanning(t *testing.T) {
	root := newRenameCarrierFixture(t)
	proof := "[{\"unit\":\"" + selftestDiscriminationUnit + "\"}]\n"
	writeFixtureFiles(t, root, map[string]string{selftestDiscriminationRel: proof})
	plan, err := planRename(root, renameOne())
	if err != nil {
		t.Fatalf("plan the new carrier's move: %v", err)
	}
	changed := renameTestSource + "\n// A concurrent edit.\n"
	writeFixtureFiles(t, root, map[string]string{selftestTestPath: changed})
	if _, err := applyRename(root, plan); err == nil {
		t.Fatal("the move accepted a source changed after planning")
	}
	if got := readRel(t, root, selftestTestPath); got != changed {
		t.Error("the concurrent source edit was changed")
	}
	if got := readRel(t, root, selftestDiscriminationRel); got != proof {
		t.Error("the refused move changed its proof metadata")
	}
	if existsRel(root, renameTarget) {
		t.Error("the refused move created its target")
	}
}
