// Related: goversion.go -- the gate these tests drive from its entry point
//
// Every test here calls the gate as a function, over a fixture tree under
// testdata/. The fixture trees are what let a case prove the gate goes RED: a
// gate added to a tree that already agrees has never been seen to fail, and
// until it has, its silence over the checkout means nothing.

package goversion

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/le/lepath"
	"github.com/ze-software/ze/internal/le/leroot"
)

// declared is the minor version every fixture tree is judged against. The
// fixtures name their own versions, so this number never follows go.mod.
const declared = "1.27"

// fixture answers a fixture tree's root and every file in it, as the pair Check
// takes. The walk stands in for git's index, which has nothing to say about a
// directory the live gate deliberately does not enter.
func fixture(t *testing.T, name string) (string, []string) {
	t.Helper()

	root := filepath.Join("testdata", name)
	var files []string
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		files = append(files, rel)
		return nil
	})
	if err != nil {
		t.Fatalf("walk the %s fixture: %v", name, err)
	}
	if len(files) == 0 {
		t.Fatalf("the %s fixture holds no file", name)
	}
	sort.Strings(files)
	return root, files
}

// reasons answers the reason of each finding, in order, so a case names what
// the gate said rather than a count.
func reasons(result Result) []string {
	out := make([]string, 0, len(result.Findings))
	for _, finding := range result.Findings {
		out = append(out, finding.Reason)
	}
	return out
}

// VALIDATES: a tree whose carriers name the declared minor passes, and the
// third-party stage beside them is excluded rather than judged.
// PREVENTS: a gate that reaches its verdict by judging nothing. The carrier
// count is what separates agreement from a walk that read no carrier.
func TestATreeThatAgreesPassesAndNamesWhatItDidNotJudge(t *testing.T) {
	root, files := fixture(t, "agree")

	result, err := Check(root, files, declared)
	if err != nil {
		t.Fatalf("check the agreeing fixture: %v", err)
	}
	if !result.Valid {
		t.Fatalf("an agreeing tree was reported invalid: %v", reasons(result))
	}
	if result.Carriers != 2 {
		t.Errorf("the run judged %d carriers, want 2 (one Dockerfile stage, one Go literal)", result.Carriers)
	}
	if len(result.Excluded) != 1 {
		t.Fatalf("the run excluded %d stages, want 1", len(result.Excluded))
	}
	if result.Excluded[0].Reason != ExcludedNoModuleCopy {
		t.Errorf("the excluded stage reads %q, want %q", result.Excluded[0].Reason, ExcludedNoModuleCopy)
	}
	if result.Excluded[0].Names != "golang:1.23-alpine" {
		t.Errorf("the excluded stage names %q, want golang:1.23-alpine", result.Excluded[0].Names)
	}
}

// VALIDATES: a carrier one minor behind the declaration is a finding, in both
// carrier kinds, and the gate answers invalid.
// PREVENTS: the whole failure this gate exists for, unobserved. This is the RED
// the agreeing tree cannot produce.
func TestADriftedCarrierIsAFinding(t *testing.T) {
	root, files := fixture(t, "drift")

	result, err := Check(root, files, declared)
	if err != nil {
		t.Fatalf("check the drifted fixture: %v", err)
	}
	if result.Valid {
		t.Fatal("a tree whose carriers name 1.26 was reported valid")
	}
	if got := reasons(result); len(got) != 2 || got[0] != ReasonMismatch || got[1] != ReasonMismatch {
		t.Fatalf("the run answered %v, want two %s findings", got, ReasonMismatch)
	}
	for _, finding := range result.Findings {
		if finding.Declared != declared {
			t.Errorf("%s names declared %q, want %q", finding.Carrier, finding.Declared, declared)
		}
		if !strings.Contains(finding.Names, "1.26") {
			t.Errorf("%s does not report what the carrier says: %q", finding.Carrier, finding.Names)
		}
	}
	if !strings.Contains(result.Text(), "go-version: FAILED") {
		t.Errorf("the page does not carry the verdict:\n%s", result.Text())
	}
}

// VALIDATES: a carrier this gate cannot read is a finding, both when its tag
// carries no version and when its base is not a golang image.
// PREVENTS: the fail-open reading, where an unreadable carrier contributes no
// finding and the run passes. A carrier nobody can date is the same drift.
func TestAnUnreadableCarrierIsAFindingRatherThanASkip(t *testing.T) {
	root, files := fixture(t, "unreadable")

	result, err := Check(root, files, declared)
	if err != nil {
		t.Fatalf("check the unreadable fixture: %v", err)
	}
	if result.Valid {
		t.Fatal("a tree whose carriers name no readable version was reported valid")
	}
	got := reasons(result)
	sort.Strings(got)
	want := []string{ReasonUnreadableBase, ReasonUnreadableTag}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("the run answered %v, want %v", got, want)
	}
}

// VALIDATES: a tree holding only stages that build other sources judges nothing,
// and that is an error rather than a pass.
// PREVENTS: the empty-population pass. A run that judged no carrier prints what
// a clean tree prints, and telling those two apart is why the count exists.
func TestATreeOfThirdPartyCarriersIsRefusedRatherThanPassed(t *testing.T) {
	root, files := fixture(t, "thirdparty")

	result, err := Check(root, files, declared)
	if err == nil {
		t.Fatalf("a tree with no carrier answered a verdict: %+v", result)
	}
	if !strings.Contains(err.Error(), "no build carrier") {
		t.Errorf("the refusal reads %q, and does not say the walk judged nothing", err)
	}
}

// VALIDATES: the walk enters neither vendor/ nor testdata/, and reads both
// carrier kinds everywhere else.
// PREVENTS: this package's own fixtures being judged as if they were the
// checkout's carriers, which would make a fixture that must drift into a
// finding about the tree.
func TestTheWalkEntersNeitherVendorNorTestdata(t *testing.T) {
	for _, row := range []struct {
		path string
		want bool
	}{
		{"docker/Dockerfile", true},
		{"test/interop/Dockerfile.ze", true},
		{"internal/le/evidence/evidence.go", true},
		{"vendor/example.com/other/Dockerfile", false},
		{"internal/le/goversion/testdata/drift/Dockerfile", false},
		{"internal/le/goversion/testdata/drift/image.go", false},
		{"docker/compose.yaml", false},
		{"README.md", false},
	} {
		if got := walked(row.path); got != row.want {
			t.Errorf("walked(%q) = %v, want %v", row.path, got, row.want)
		}
	}
}

// VALIDATES: an image reference is read for the golang name, the minor version,
// and whether either could be read at all.
// PREVENTS: a variant tag or a digest pin reading as a different minor, and a
// tag with no version reading as one.
func TestAnImageReferenceIsReadForItsMinorVersion(t *testing.T) {
	for _, row := range []struct {
		reference string
		minor     string
		isGolang  bool
		readable  bool
	}{
		{"golang:1.27", "1.27", true, true},
		{"golang:1.27-alpine", "1.27", true, true},
		{"golang:1.27-bookworm", "1.27", true, true},
		{"golang:1.27.1-alpine", "1.27", true, true},
		{"golang:1.27@sha256:abc", "1.27", true, true},
		{"docker.io/library/golang:1.27", "1.27", true, true},
		{"golang:1.26-alpine", "1.26", true, true},
		{"golang:latest", "", true, false},
		{"golang:alpine", "", true, false},
		{"golang:1", "", true, false},
		{"golang:${GO_VERSION}", "", true, false},
		{"golang", "", true, false},
		{"alpine:3.21", "", false, false},
		{"mygolang:1.27", "", false, false},
	} {
		minor, isGolang, readable := minorOf(row.reference)
		if minor != row.minor || isGolang != row.isGolang || readable != row.readable {
			t.Errorf("minorOf(%q) = (%q, %v, %v), want (%q, %v, %v)",
				row.reference, minor, isGolang, readable, row.minor, row.isGolang, row.readable)
		}
	}
}

// VALIDATES: the go directive is read as a minor version, and a go.mod carrying
// none or two is refused.
// PREVENTS: an empty declared version reaching the comparison, where every
// carrier would then disagree with a version nothing declares.
func TestTheGoDirectiveIsTheOnlyDeclaration(t *testing.T) {
	for _, body := range []string{"module x\n\ngo 1.27.0\n", "module x\n\ngo 1.27\n", "go 1.27.0\nrequire (\n\tgo 1.26.0\n)\n"} {
		minor, err := declaredMinor(body)
		if err != nil {
			t.Errorf("declaredMinor(%q): %v", body, err)
			continue
		}
		if minor != declared {
			t.Errorf("declaredMinor(%q) = %q, want %q", body, minor, declared)
		}
	}
	for _, body := range []string{"module x\n", "module x\ngo 1.27.0\ngo 1.26.0\n", "toolchain go1.27.0\n"} {
		if minor, err := declaredMinor(body); err == nil {
			t.Errorf("declaredMinor(%q) answered %q, want a refusal", body, minor)
		}
	}
}

// VALIDATES: the checkout's own go.mod is readable, and Check refuses to judge
// against an empty declaration.
// PREVENTS: a read failure reaching the comparison as an empty string, where a
// zero would then look like a version.
func TestTheCheckoutDeclaresAMinorAndAnEmptyOneIsRefused(t *testing.T) {
	root, err := lepath.Root()
	if err != nil {
		t.Fatalf("find checkout: %v", err)
	}
	minor, err := Declared(root)
	if err != nil {
		t.Fatalf("read the checkout's go.mod: %v", err)
	}
	if strings.Count(minor, ".") != 1 || minor == "" {
		t.Errorf("the checkout declares %q, want a <major>.<minor>", minor)
	}

	fixtureRoot, files := fixture(t, "agree")
	if _, err := Check(fixtureRoot, files, ""); err == nil {
		t.Error("a check with no declared version answered a verdict")
	}
}

// VALIDATES: a tracked path git holds and the working tree has already deleted
// is passed over rather than failing the read.
// PREVENTS: every developer mid-refactor getting a read failure from a gate that
// has nothing to judge in a file with no content left.
func TestADeletedFileIsPassedOverRatherThanRead(t *testing.T) {
	root, files := fixture(t, "agree")

	result, err := Check(root, append(files, "Dockerfile.gone"), declared)
	if err != nil {
		t.Fatalf("check a tree naming a deleted file: %v", err)
	}
	if !result.Valid {
		t.Errorf("a deleted file changed the verdict: %v", reasons(result))
	}
	if _, statErr := os.Stat(filepath.Join(root, "Dockerfile.gone")); !os.IsNotExist(statErr) {
		t.Errorf("the fixture holds Dockerfile.gone, so the case proved nothing")
	}
}

// VALIDATES: each selftest case behaves as it declares, named one by one.
// PREVENTS: the comparison breaking silently. A failure now names which of the
// fifteen properties stopped holding.
func TestEachSelftestCaseHolds(t *testing.T) {
	for _, testCase := range selftestCases {
		t.Run(testCase.name, func(t *testing.T) {
			if failure := testCase.run(); failure != "" {
				t.Error(failure)
			}
		})
	}
}

// VALIDATES: the selftest answers one result per case, and passes here.
// PREVENTS: a selftest that reports OK having run nothing.
func TestSelftestAnswersOneResultPerCase(t *testing.T) {
	report := Selftest()
	if len(report.Results) != len(selftestCases) {
		t.Fatalf("the selftest answered %d results, want one per case (%d)", len(report.Results), len(selftestCases))
	}
	if failures := report.Failures(); len(failures) != 0 {
		t.Errorf("the selftest failed: %v", failures)
	}
	if code := report.Code(2); code != 0 {
		t.Errorf("a passing selftest answers %d, want 0", code)
	}
	for index, result := range report.Results {
		if result.Case != selftestCases[index].name {
			t.Errorf("result %d names %q, want %q", index, result.Case, selftestCases[index].name)
		}
	}
}

// VALIDATES: a broken selftest answers 2, which is not the 1 a drifted tree
// answers.
// PREVENTS: a caller that reads the two codes apart losing the distinction. A
// broken gate and a drifted carrier need different responses.
func TestABrokenSelftestAnswersTwoAndADriftedTreeOne(t *testing.T) {
	broken := leroot.NewSelftestReport("ok", "failed", leroot.Fail("agrees", "the comparison stopped comparing"))
	if code := broken.Code(2); code != 2 {
		t.Errorf("a failing selftest answers %d, want 2", code)
	}

	root, files := fixture(t, "drift")
	result, err := Check(root, files, declared)
	if err != nil {
		t.Fatalf("check the drifted fixture: %v", err)
	}
	if result.Valid {
		t.Fatal("a drifted tree was reported valid")
	}
}
