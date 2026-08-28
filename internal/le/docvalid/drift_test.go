// The drift gate's tests, every one of them calling the gate as a FUNCTION
// (spec-le-is-a-ze-binary, AC-5). The same cases used to run the script as a
// subprocess with `go run`, which relinked the whole product per case and
// could not reach any of the gate's internals.
//
// Each case names the claim it makes and the fixture tree it makes it over.
// A fixture tree trips other checks as well -- a DESIGN.md with no Shipped
// Plugins table reports every registered plugin -- so an assertion here keys on
// the MESSAGE it is about, never on the exit code alone.

package docvalid

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// writeDoc writes one fixture file, creating the directories above it.
func writeDoc(t *testing.T, root, rel, content string) {
	t.Helper()
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("create %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// messages answers every finding message of a report, so a case can ask
// whether a claim was reported without depending on the ones it is not about.
func messages(report DriftReport) []string {
	out := make([]string, 0, len(report.Issues))
	for _, iss := range report.Issues {
		out = append(out, iss.Message)
	}
	return out
}

func reports(report DriftReport, fragment string) bool {
	for _, message := range messages(report) {
		if strings.Contains(message, fragment) {
			return true
		}
	}
	return false
}

// VALIDATES: "N+ interop scenarios" is a FLOOR and a bare "N" is exact.
// PREVENTS: the churn of editing docs/DESIGN.md and reddening the gate every
// time a scenario is added, and the over-claim the floor still has to catch.
// This is the case TestDocDriftInteropFloorClaim drove through `go run`.
func TestDriftReadsAnInteropFloorClaim(t *testing.T) {
	const marker = "interop scenarios, actual is"

	for _, tc := range []struct {
		name   string
		claim  string
		reject bool
		want   string
	}{
		{name: "floor below actual is accepted", claim: "2+ interop scenarios run here."},
		{name: "floor equal to actual is accepted", claim: "3+ interop scenarios run here."},
		{name: "exact bare number is accepted", claim: "3 interop scenarios run here."},
		{name: "floor above actual is rejected", claim: "9+ interop scenarios run here.", reject: true,
			want: "claims at least 9 interop scenarios, actual is 3"},
		{name: "bare number still checked exactly", claim: "2 interop scenarios run here.", reject: true,
			want: "claims 2 interop scenarios, actual is 3"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			for _, s := range []string{"a", "b", "c"} {
				writeDoc(t, root, filepath.Join("test", "interop", "scenarios", s, "check.go"), "package scenario\n")
			}
			writeDoc(t, root, "docs/DESIGN.md", "# Design\n\n"+tc.claim+"\n")

			report := Drift(root)
			if got := reports(report, marker); got != tc.reject {
				t.Fatalf("claim %q: interop complaint present=%v, want %v: %v", tc.claim, got, tc.reject, messages(report))
			}
			if tc.want != "" && !reports(report, tc.want) {
				t.Fatalf("the gate did not report %q: %v", tc.want, messages(report))
			}
		})
	}
}

// VALIDATES: a dotfile directory beside the scenarios is not a scenario.
// PREVENTS: a linter's cache directory inflating the count a document is judged
// against, which is what made an accurate page read as drift.
func TestDriftDoesNotCountACacheDirectoryAsAScenario(t *testing.T) {
	root := t.TempDir()
	for _, s := range []string{"a", "b", ".cache"} {
		writeDoc(t, root, filepath.Join("test", "interop", "scenarios", s, "check.go"), "package scenario\n")
	}
	writeDoc(t, root, "docs/DESIGN.md", "# Design\n\n2 interop scenarios run here.\n")

	if report := Drift(root); reports(report, "interop scenarios, actual is") {
		t.Fatalf("a cache directory was counted as a scenario: %v", messages(report))
	}
}

// VALIDATES: the stale text-parser claims are rejected and the current wording
// is accepted.
// PREVENTS: reintroducing the strings.Fields documentation, and a narrow
// stale-claim check that rejects the source-linked wording that replaced it.
// These are TestDocDriftRejectsStaleTextParserFieldsClaim and
// TestDocDriftAllowsScannerTextParserDoc, as function calls.
func TestDriftReadsTheTextParserClaims(t *testing.T) {
	for _, tc := range []struct {
		name  string
		body  string
		wants string
	}{
		{
			name:  "the strings.Fields claim is rejected",
			body:  "# Text Parser Architecture\n\nAll functions allocate via `strings.Fields()`.\n",
			wants: "stale text parser claim references strings.Fields",
		},
		{
			name: "the scanner wording is accepted",
			body: "# Text Parser Architecture\n\nThe parser uses `textparse.NewScanner` for token-by-token scanning.\n",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			writeDoc(t, root, "docs/architecture/api/text-parser.md", tc.body)

			report := Drift(root)
			if tc.wants == "" {
				if len(report.Issues) != 0 {
					t.Fatalf("a clean fixture drew findings: %v", messages(report))
				}
				return
			}
			if !reports(report, tc.wants) {
				t.Fatalf("the gate did not report %q: %v", tc.wants, messages(report))
			}
		})
	}
}

// VALIDATES: a bare README count is exact in BOTH directions and an `N+` claim
// is a floor.
// PREVENTS: a bare headline count drifting unseen, and a regression that starts
// flagging the soft `N+` phrasing the project uses to avoid count re-drift.
// This is TestCheckReadmeMDFlagsBareAndUndercount, as a function call.
func TestDriftReadsReadmeCounts(t *testing.T) {
	root := t.TempDir()
	// Three `func Fuzz` and five `func Test`, text-scanned rather than
	// compiled, so the counts the README is judged against are fixed.
	writeDoc(t, root, "internal/z/z_test.go",
		"package z\n\n"+
			"func FuzzA()\nfunc FuzzB()\nfunc FuzzC()\n"+
			"func TestA()\nfunc TestB()\nfunc TestC()\nfunc TestD()\nfunc TestE()\n")
	writeDoc(t, root, "README.md",
		"# Ze\n\n"+
			"bare over-claim: 5 fuzz targets in the tree\n"+
			"bare exact match: 3 fuzz targets in the tree\n"+
			"at-least over-claim: 9+ fuzz targets in the tree\n"+
			"at-least tolerated: 1+ fuzz targets in the tree\n"+
			"bare undercount: 2 unit tests in the tree\n")

	report := Drift(root)
	for _, want := range []string{
		"claims 5 fuzz targets (bare exact count), actual is 3",
		"claims 9+ fuzz targets, actual is 3",
		"claims 2 unit tests (bare exact count), actual is 5",
	} {
		if !reports(report, want) {
			t.Errorf("the gate did not flag %q: %v", want, messages(report))
		}
	}
	for _, unwanted := range []string{
		"claims 3 fuzz targets (bare exact count)",
		"claims 1+ fuzz targets",
	} {
		if reports(report, unwanted) {
			t.Errorf("the gate flagged the tolerated claim %q: %v", unwanted, messages(report))
		}
	}
}

// VALIDATES: a source file whose scan stops early is reported rather than
// counted short.
// PREVENTS: a low test or fuzz count agreeing with a document that understates
// the tree, because the scan stopped on a line above bufio.MaxScanTokenSize and
// nobody was told. This is TestDocDriftReportsUnreadableSource.
func TestDriftReportsAFileItCouldNotFinishReading(t *testing.T) {
	root := t.TempDir()
	// One line above bufio.MaxScanTokenSize (64 KiB) stops the scan, so the
	// `func Test` below it is never counted.
	writeDoc(t, root, "internal/z/z_test.go",
		"package z\n\n// "+strings.Repeat("x", 70*1024)+"\nfunc TestA()\n")

	report := Drift(root)
	if !reports(report, "read stopped early") {
		t.Fatalf("the gate did not report the unreadable file: %v", messages(report))
	}
	if !strings.Contains(report.Issues[0].File, "z_test.go") {
		t.Fatalf("the gate did not name the unreadable file: %v", report.Issues[0])
	}
}

// VALIDATES: a file the scan cannot OPEN is reported, not counted as zero.
// PREVENTS: the fail-open half the script never covered. os.Open failing
// answered a zero count with no finding, and a zero count agrees with any
// document that understates the tree. A dangling symbolic link reaches it
// without a permission change, so root cannot defeat the fixture.
func TestDriftReportsAFileItCouldNotOpen(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "internal/z/real_test.go", "package z\n\nfunc TestA()\n")
	link := filepath.Join(root, "internal", "z", "gone_test.go")
	if err := os.Symlink(filepath.Join(root, "internal", "z", "no-such-file"), link); err != nil {
		t.Fatalf("create the dangling link: %v", err)
	}
	writeDoc(t, root, "README.md", "# Ze\n\n1 unit tests in the tree\n")

	report := Drift(root)
	if !reports(report, "could not be read") {
		t.Fatalf("the gate counted a file it could not open: %v", messages(report))
	}
	if !strings.Contains(report.Issues[len(report.Issues)-1].File, "gone_test.go") {
		t.Fatalf("the gate did not name the file it could not open: %v", report.Issues)
	}
}

// VALIDATES: a DOCUMENT that exists and cannot be opened is reported, where an
// ABSENT one stays silent.
// PREVENTS: the same fail-open on the reading side: a page nobody can read is
// not a page that agrees with the tree.
func TestDriftTellsAnAbsentDocumentFromAnUnreadableOne(t *testing.T) {
	absent := t.TempDir()
	if report := Drift(absent); len(report.Issues) != 0 {
		t.Fatalf("an empty tree drew findings: %v", messages(report))
	}

	unreadable := t.TempDir()
	link := filepath.Join(unreadable, "README.md")
	if err := os.Symlink(filepath.Join(unreadable, "no-such-readme"), link); err != nil {
		t.Fatalf("create the dangling link: %v", err)
	}
	report := Drift(unreadable)
	if !reports(report, "could not be read") {
		t.Fatalf("an unreadable document was read as agreement: %v", messages(report))
	}
}

// VALIDATES: an unreadable directory is reported AND the walk continues.
// PREVENTS: one closed directory costing the rest of the tree its counts, and
// costing the gate its voice about the part it could not see.
func TestDriftKeepsWalkingPastAnUnreadableDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a 0000 directory, so this fixture cannot be built as root")
	}
	root := t.TempDir()
	writeDoc(t, root, "internal/open/open_test.go", "package open\n\nfunc TestA()\nfunc TestB()\n")
	writeDoc(t, root, "internal/closed/hidden_test.go", "package closed\n\nfunc TestC()\n")
	closed := filepath.Join(root, "internal", "closed")
	if err := os.Chmod(closed, 0o000); err != nil {
		t.Fatalf("close the fixture directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(closed, 0o750) })
	writeDoc(t, root, "README.md", "# Ze\n\n2 unit tests in the tree\n")

	report := Drift(root)
	if !reports(report, "could not be read") {
		t.Fatalf("the gate said nothing about the directory it could not read: %v", messages(report))
	}
	// The open half was still counted: the README claims the two tests that
	// remain readable, and no count complaint is reported for it.
	if reports(report, "unit tests (bare exact count)") {
		t.Fatalf("the walk stopped at the closed directory: %v", messages(report))
	}
}

// VALIDATES: a tree with no test/, internal/ or scenarios directory draws no
// finding of its own.
// PREVENTS: the fix above turning every fixture root into an error, which would
// make the gate unusable over anything but the checkout.
func TestDriftIsSilentAboutADirectoryThatDoesNotExist(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "docs/architecture/api/text-parser.md", "# Text Parser\n\nIt scans.\n")

	if report := Drift(root); len(report.Issues) != 0 {
		t.Fatalf("a tree with no source directories drew findings: %v", messages(report))
	}
}

// VALIDATES: the operator-table check judges a tree that carries docs/features,
// and only such a tree.
// PREVENTS: a fixture root reporting a table it does not owe, and the narrowing
// that fixes it silently disabling the check. This is
// TestDocDriftOperatorTableIgnoresFixtureRoots, as a function call.
func TestDriftScopesTheOperatorTableToTreesThatOweOne(t *testing.T) {
	bare := t.TempDir()
	writeDoc(t, bare, "docs/architecture/api/text-parser.md",
		"# Text Parser Architecture\n\nThe parser uses `textparse.NewScanner`.\n")
	if report := Drift(bare); len(report.Issues) != 0 {
		t.Fatalf("a fixture root with no docs/features was judged: %v", messages(report))
	}

	populated := t.TempDir()
	writeDoc(t, populated, "docs/features/formatting.md", "# Output Formatting\n")
	if report := Drift(populated); !reports(report, "the generated pipe operator reference is missing") {
		t.Fatalf("a tree carrying docs/features and no operator table was accepted: %v", messages(report))
	}
}

// VALIDATES: a published operator table that disagrees with the catalog is
// reported.
// PREVENTS: the published list drifting from the product, which is the whole
// reason this check exists.
func TestDriftComparesThePublishedOperatorTable(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, pipeOperatorReferencePath, "# Pipe operators\n\nsomething else entirely\n")
	if report := Drift(root); !reports(report, "the published pipe operator table and the operator catalog disagree") {
		t.Fatalf("a stale operator table was accepted: %v", messages(report))
	}

	written, err := writeGenerated(root)
	if err != nil {
		t.Fatalf("write the generated table: %v", err)
	}
	if written.Path != pipeOperatorReferencePath {
		t.Fatalf("the writer named %q", written.Path)
	}
	if report := Drift(root); len(report.Issues) != 0 {
		t.Fatalf("the table this tool wrote does not satisfy the tool that checks it: %v", messages(report))
	}
}

// VALIDATES: suite derivation reads the linked functional owner, with no `le`
// binary in the fixture tree.
// PREVENTS: doc verification restoring an old-le subprocess or reading its
// absence as an empty functional population.
func TestDriftDerivesSuitesWithoutAnLESubprocess(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "docs/functional-tests.md", "The functional test target runs 2 suites: bgp, web.\n")

	report := Drift(root)
	files := map[string]bool{}
	for _, issue := range report.Issues {
		if strings.Contains(issue.Message, "could not derive") {
			t.Fatalf("linked functional catalog was reported missing: %v", messages(report))
		}
		files[issue.File] = true
	}
	if !files[functionalTestsDoc] {
		t.Errorf("%s was not compared with the native suite catalog: %v", functionalTestsDoc, messages(report))
	}
}

// VALIDATES: a feature inventory row carrying an unknown status is reported,
// and a missing table is reported too.
// PREVENTS: a status nobody defined passing as a documented one.
func TestDriftReadsTheFeatureInventory(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "docs/features.md",
		"| Feature | Status | Description |\n|---|---|---|\n| a | supported | fine |\n| b | almost | not a status |\n")
	report := Drift(root)
	if !reports(report, `unknown feature status "almost"`) {
		t.Fatalf("an unknown status was accepted: %v", messages(report))
	}
	if reports(report, `unknown feature status "supported"`) {
		t.Fatalf("a documented status was rejected: %v", messages(report))
	}

	empty := t.TempDir()
	writeDoc(t, empty, "docs/features.md", "# Features\n\nnothing here\n")
	if report := Drift(empty); !reports(report, "feature inventory table not found") {
		t.Fatalf("a features page with no table was accepted: %v", messages(report))
	}
}

// VALIDATES: the comparison page's family claims are checked in both
// directions against the registry this build carries.
// PREVENTS: the page claiming a family the binary does not have, or denying one
// it does.
func TestDriftReadsTheComparisonTable(t *testing.T) {
	root := t.TempDir()
	// ipv4/unicast is always in the registry; ipv4/mpls-vpn is not, unless the
	// build carries the plugin that registers it.
	writeDoc(t, root, "docs/comparison.md",
		"| AFI/SAFI | Ze | Other |\n|---|---|---|\n| IPv4 Unicast | No | Yes |\n")

	if report := Drift(root); !reports(report, `claims Ze lacks "IPv4 Unicast"`) {
		t.Fatalf("a denied family that IS registered was accepted: %v", messages(report))
	}
}

// VALIDATES: two runs over one tree answer the same report.
// PREVENTS: the package-level finding list the script kept, which made a second
// run in one process report the first run's findings as well.
func TestDriftDoesNotShareStateBetweenRuns(t *testing.T) {
	root := t.TempDir()
	writeDoc(t, root, "internal/z/z_test.go",
		"package z\n\n// "+strings.Repeat("x", 70*1024)+"\nfunc TestA()\n")

	first := Drift(root)
	second := Drift(root)
	if len(first.Issues) != len(second.Issues) {
		t.Fatalf("run one reported %d findings and run two reported %d", len(first.Issues), len(second.Issues))
	}
}

// VALIDATES: vanished tells a file that is GONE from one that is present and
// unreadable.
// PREVENTS: the shared-checkout race being read as a defect, and a real
// unreadable file being written off as a race. Another session's tmp/ file
// disappearing under a walk is what made this distinction load bearing.
func TestVanishedTellsAGoneFileFromAnUnreadableOne(t *testing.T) {
	root := t.TempDir()
	gone := filepath.Join(root, "gone.go")
	if err := os.WriteFile(gone, []byte("package z\n"), 0o600); err != nil {
		t.Fatalf("write the fixture: %v", err)
	}
	if err := os.Remove(gone); err != nil {
		t.Fatalf("remove the fixture: %v", err)
	}
	if !vanished(gone) {
		t.Errorf("a file that was deleted does not read as vanished")
	}

	link := filepath.Join(root, "dangling.go")
	if err := os.Symlink(filepath.Join(root, "no-such-target"), link); err != nil {
		t.Fatalf("create the dangling link: %v", err)
	}
	if vanished(link) {
		t.Errorf("a dangling symbolic link reads as vanished, so the file it names is never reported")
	}
}
