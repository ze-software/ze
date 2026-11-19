// Design: docs/contributing/rfc-conformance-gates.md -- commit-owned tracked relocation.
// Related: rename_test.go -- evidence, citation and refusal fixtures.
package rfc

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// A tracked relocation prepares the exact destination while keeping the
// original bytes available for the commit script's guarded removal.
func TestRenameTrackedCarrierRetainsSourceUntilCommit(t *testing.T) {
	root := renameFixture(t)
	original := readRel(t, root, selftestTestPath)
	if _, err := renameFiles(root, renameOne()); err != nil {
		t.Fatalf("prepare the tracked relocation: %v", err)
	}
	if got := readRel(t, root, renameTarget); got != original {
		t.Error("the prepared target differs from the original source")
	}
	if got := readRel(t, root, selftestTestPath); got != original {
		t.Error("the tracked source changed before its commit-owned removal")
	}
}

// The prepared target has its own inode and preserves the source permissions;
// a later destination edit must not modify the still-retained source.
func TestRenameTrackedCarrierTargetIsIndependent(t *testing.T) {
	root := renameFixture(t)
	source := filepath.Join(root, filepath.FromSlash(selftestTestPath))
	target := filepath.Join(root, filepath.FromSlash(renameTarget))
	if err := os.Chmod(source, 0o640); err != nil {
		t.Fatal(err)
	}
	original := readRel(t, root, selftestTestPath)
	if _, err := renameFiles(root, renameOne()); err != nil {
		t.Fatalf("prepare the tracked relocation: %v", err)
	}
	sourceInfo, err := os.Stat(source)
	if err != nil {
		t.Fatalf("the tracked source was not retained: %v", err)
	}
	targetInfo, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if os.SameFile(sourceInfo, targetInfo) {
		t.Fatal("the prepared target aliases the retained source's inode")
	}
	if got := targetInfo.Mode().Perm(); got != sourceInfo.Mode().Perm() {
		t.Fatalf("prepared target permissions = %o, want %o", got, sourceInfo.Mode().Perm())
	}
	if err := os.WriteFile(target, []byte("an independent destination edit\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if got := readRel(t, root, selftestTestPath); got != original {
		t.Error("editing the destination modified the retained source")
	}
}

// The response distinguishes a prepared tracked relocation from a completed
// move, puts its commit-owned file/remove step before the after-commit review,
// and never advises ledger regeneration, which a moved carrier does not owe
// (rfc-conformance-gates.md, "Moving a tagged test file").
func TestRenameTrackedCarrierReportsPreparation(t *testing.T) {
	root := renameFixture(t)
	report, err := renameFiles(root, renameOne())
	if err != nil {
		t.Fatalf("prepare the tracked relocation: %v", err)
	}
	if got := renamePreparedPairs(t, report); !slices.Equal(got, renameOne()) {
		t.Errorf("prepared relocations = %v, want %v", got, renameOne())
	}
	if len(report.Moves) != 0 {
		t.Errorf("reported completed moves %v while their tracked sources remain", report.Moves)
	}
	text := report.Text()
	if want := "prepared " + selftestTestPath + " -> " + renameTarget; !strings.Contains(text, want) {
		t.Errorf("the report omits %q:\n%s", want, text)
	}
	commit := strings.Index(text, "./le commit create")
	if commit < 0 {
		t.Errorf("the report omits the commit-owned relocation command:\n%s", text)
	}
	review := strings.Index(text, "after commit: "+renameNextStep)
	if review < 0 {
		t.Errorf("the report omits the after-commit review %q:\n%s", renameNextStep, text)
	}
	if commit >= 0 {
		if review < commit {
			t.Errorf("the report reviews the tree before committing the relocation:\n%s", text)
		}
	}
	if strings.Contains(text, "./le rfc update") {
		t.Errorf("the report advises ledger regeneration a moved carrier does not owe:\n%s", text)
	}
}

// A target appearing after planning must not cause the tool to delete an
// earlier independent copy or either source. The partial report names only
// the copy this run created, and evidence and citations remain untouched.
func TestRenameTrackedCarrierKeepsCreatedTargetsOnFailure(t *testing.T) {
	const secondSource = "internal/sample/extra_widget_test.go"
	const secondTarget = "internal/sample/rfc9999_extra_widget_test.go"
	const concurrent = "a destination created by another writer\n"
	root := renameFixture(t)
	writeFixtureFiles(t, root, map[string]string{
		secondSource: strings.ReplaceAll(renameTestSource, "TestWidget", "TestExtraWidget"),
	})
	commitFixture(t, root, "second tracked carrier")
	pairs := renameOne()
	pairs = append(pairs, renamePair{Source: secondSource, Target: secondTarget})
	plan, err := planRename(root, pairs)
	if err != nil {
		t.Fatalf("plan two tracked relocations: %v", err)
	}
	before := map[string]string{
		selftestDiscriminationRel: readRel(t, root, selftestDiscriminationRel),
		renameAuditRel:            readRel(t, root, renameAuditRel),
		renameDocRel:              readRel(t, root, renameDocRel),
	}
	firstBody := readRel(t, root, selftestTestPath)
	secondBody := readRel(t, root, secondSource)
	writeFixtureFiles(t, root, map[string]string{secondTarget: concurrent})
	report, err := applyRename(root, plan)
	if err == nil {
		t.Fatal("the target collision was accepted")
	}
	if report.Stopped == "" {
		t.Error("the response does not report the partial preparation")
	}
	if got := renamePreparedPairs(t, report); !slices.Equal(got, renameOne()) {
		t.Errorf("partial prepared relocations = %v, want %v", got, renameOne())
	}
	if got := readRel(t, root, renameTarget); got != firstBody {
		t.Error("the completed independent copy was changed or discarded")
	}
	if got := readRel(t, root, selftestTestPath); got != firstBody {
		t.Error("the first tracked source was changed")
	}
	if got := readRel(t, root, secondSource); got != secondBody {
		t.Error("the second tracked source was changed")
	}
	if got := readRel(t, root, secondTarget); got != concurrent {
		t.Error("the other writer's destination was changed")
	}
	for rel, want := range before {
		if got := readRel(t, root, rel); got != want {
			t.Errorf("%s changed although source preparation stopped", rel)
		}
	}
}

// Staging or committing an unchanged new carrier changes its ownership. The
// plan must refuse before writing rather than delete a now-tracked source.
func TestRenameRefusesGitStateChangedSincePlanning(t *testing.T) {
	cases := []struct {
		name   string
		commit bool
	}{
		{name: "staged"},
		{name: "committed", commit: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := newRenameCarrierFixture(t)
			original := readRel(t, root, selftestTestPath)
			path := filepath.Join(root, filepath.FromSlash(selftestTestPath))
			before, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			plan, err := planRename(root, renameOne())
			if err != nil {
				t.Fatalf("plan the untracked carrier: %v", err)
			}
			gitFixture(t, root, []string{"add", "--", selftestTestPath})
			if tc.commit {
				gitFixture(t, root, []string{"-c", "user.email=fixture@example.invalid",
					"-c", "user.name=rfc-fixture", "commit", "-q", "-m", "publish the carrier"})
			}
			after, err := os.Lstat(path)
			if err != nil {
				t.Fatal(err)
			}
			if !os.SameFile(before, after) {
				t.Fatal("the Git-state control replaced the source inode")
			}
			if after.Mode() != before.Mode() {
				t.Fatal("the Git-state control changed the source mode")
			}
			if got := readRel(t, root, selftestTestPath); got != original {
				t.Fatal("the Git-state control changed the source bytes")
			}
			report, err := applyRename(root, plan)
			if err == nil {
				t.Error("the plan accepted a changed Git binding")
			}
			if !existsRel(root, selftestTestPath) {
				t.Error("the now-tracked source was removed")
			}
			if existsRel(root, renameTarget) {
				t.Error("a destination was written before confirming the current Git binding")
			}
			if len(report.Files) != 0 {
				t.Errorf("the refused plan reports writes: %v", report.Files)
			}
		})
	}
}

// Decode the public response so these tests compile before Prepared is added
// to the producer; a missing population remains an observable failed assertion.
func renamePreparedPairs(t *testing.T, report RenameReport) []renamePair {
	t.Helper()
	body, err := json.Marshal(report)
	if err != nil {
		t.Fatal(err)
	}
	var answer struct {
		Prepared []renamePair `json:"prepared"`
	}
	if err := json.Unmarshal(body, &answer); err != nil {
		t.Fatal(err)
	}
	return answer.Prepared
}

// publishRenameFixture models the native file/remove boundary inside only this
// test's disposable repository. Working sources MUST survive until its commit
// succeeds; unchanged copies are removed only after publication. It deliberately
// leaves ledger generation and every evidence verifier to the calling test.
func publishRenameFixture(t *testing.T, root string, report RenameReport) {
	t.Helper()
	if report.Stopped != "" {
		t.Fatal("an incomplete fixture preparation cannot be published")
	}
	if len(report.Prepared) == 0 {
		t.Fatal("the fixture publication has no tracked relocation")
	}
	if len(report.Files) == 0 {
		t.Fatal("the fixture publication has no file population")
	}
	type removal struct {
		pair renamePair
		body []byte
		info os.FileInfo
	}
	var removals []removal
	for _, pair := range report.Prepared {
		body, ok := gitOutput(root, "show", headRevision+":"+pair.Source)
		if !ok {
			t.Fatalf("read the fixture's tracked source %s", pair.Source)
		}
		if got := readRel(t, root, pair.Source); !bytes.Equal([]byte(got), body) {
			t.Fatalf("the fixture source %s changed before publication", pair.Source)
		}
		info, err := os.Lstat(filepath.Join(root, filepath.FromSlash(pair.Source)))
		if err != nil {
			t.Fatal(err)
		}
		removals = append(removals, removal{pair: pair, body: body, info: info})
	}
	gitFixture(t, root, append([]string{"add", "--"}, report.Files...))
	for _, item := range removals {
		gitFixture(t, root, []string{"update-index", "--force-remove", "--", item.pair.Source})
		if got := readRel(t, root, item.pair.Source); !bytes.Equal([]byte(got), item.body) {
			t.Fatalf("the fixture source %s did not survive staging", item.pair.Source)
		}
	}
	gitFixture(t, root, []string{"-c", "user.email=fixture@example.invalid",
		"-c", "user.name=rfc-fixture", "commit", "-q", "-m", "rename"})
	for _, item := range removals {
		target, ok := gitOutput(root, "show", headRevision+":"+item.pair.Target)
		if !ok {
			t.Fatalf("the fixture target %s was not published", item.pair.Target)
		}
		if !bytes.Equal(target, item.body) {
			t.Fatalf("the published target %s differs from the source", item.pair.Target)
		}
		retained, ok := gitOutput(root, "ls-tree", "--name-only", "-z", headRevision, "--", item.pair.Source)
		if !ok {
			t.Fatalf("inspect the fixture source %s after publication", item.pair.Source)
		}
		if len(retained) != 0 {
			t.Fatalf("the fixture commit retained the old carrier %s", item.pair.Source)
		}
		path := filepath.Join(root, filepath.FromSlash(item.pair.Source))
		info, err := os.Lstat(path)
		if err != nil {
			t.Fatal(err)
		}
		if !os.SameFile(info, item.info) {
			t.Fatalf("the fixture source %s was replaced; its copy remains", item.pair.Source)
		}
		if info.Mode() != item.info.Mode() {
			t.Fatalf("the fixture source %s changed mode; its copy remains", item.pair.Source)
		}
		if got := readRel(t, root, item.pair.Source); !bytes.Equal([]byte(got), item.body) {
			t.Fatalf("the fixture source %s changed; its copy remains", item.pair.Source)
		}
		if err := os.Remove(path); err != nil {
			t.Fatal(err)
		}
	}
}
