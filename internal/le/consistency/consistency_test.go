// The consistency gate's checks, driven as FUNCTIONS over fixture trees.
//
// The gate's previous test (internal/le/consistency/consistency_test.go) forked the tool
// as a subprocess over a fixture tree and matched substrings in its combined
// output. What that proved, and where each proof now lives:
//
//	old assertion                                  | now proven by
//	-----------------------------------------------|---------------------------
//	output contains "unreadable"                    | TestCheckReportsUnreadableFile, on Finding.Check
//	output names the file                           | the same test, on Finding.File
//	output lacks "All consistency checks passed"    | the same test, on Report.Errors, plus
//	                                                | TestTextSaysPassOnlyWhenEmpty in report_test.go
//	a readable file draws no "unreadable"           | TestCheckPassesReadableFile
//	the tool RUNS end to end as a program           | test/ui/le-consistency-answers.ci, over the built binary
//
// Nothing was dropped: the text-matching half moved to report_test.go, where
// the rendering is the subject, and the process half moved to the .ci, where a
// built binary is the subject. What is added here is a case per check, which
// the subprocess test never had, and the walk-error case the old tool could not
// pass at all.
//
// VALIDATES: each of the seven checks finds what it exists to find, names it at
// a path relative to the tree, and answers the same findings in the same order
// on every run; and a tree the tool could not read in full is REPORTED rather
// than counted clean.
// PREVENTS: the fail-open shape -- an unreadable file, an unreadable directory
// or a tree that is not there produces no finding, and an empty report is
// indistinguishable from a clean tree.

package consistency

import (
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/env"
)

// writeFixture writes body to <dir>/<name>, creating parent directories, and
// answers the directory it wrote into.
func writeFixture(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("create fixture directory for %s: %v", name, err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write fixture %s: %v", name, err)
	}
}

// Two checks scan test files as well as source files, so a fixture written as
// one literal would make THIS file a finding of the gate it tests. Both are
// assembled instead, which keeps the pattern out of the source text.
var (
	statusFixture = "package fixture\n\nvar r = Response{Status: " + strconv.Quote("done") + "}\n"
	splitFixture  = "package fixture\n\nvar parts = strings.Split(s, " + strconv.Quote(",") + ")\n"
)

// findingsFor answers every finding one check produced.
func findingsFor(report Report, check string) []Finding {
	var kept []Finding
	for _, finding := range report.Findings {
		if finding.Check == check {
			kept = append(kept, finding)
		}
	}
	return kept
}

// TestCheckReportsUnreadableFile drives a .go file holding one line above
// bufio.MaxScanTokenSize. Scan stops on it, so every check over the file sees
// nothing. Saying nothing is the fail-open shape: it is indistinguishable from
// a clean file, so the tool must say the file was not checked.
//
// Both file kinds are driven because two different readers can notice. A source
// file is read twice, by the line scanner and by the size counter, so either
// one reporting is enough. A TEST file is read only by the line scanner: the
// size check exempts it. Without that second case, the scanner could stop
// reporting and every assertion here would still pass on the counter's finding.
func TestCheckReportsUnreadableFile(t *testing.T) {
	for _, name := range []string{"toolong.go", "toolong_test.go"} {
		t.Run(name, func(t *testing.T) {
			dir := t.TempDir()
			writeFixture(t, dir, name, "package fixture\n\n// c is "+strings.Repeat("x", 70*1024)+"\nvar c = 1\n")

			report := Check(dir)

			unreadable := findingsFor(report, "unreadable")
			if len(unreadable) == 0 {
				t.Fatalf("a file the tool could not read in full drew no finding: %+v", report.Findings)
			}
			if unreadable[0].File != name {
				t.Errorf("the unreadable finding names %q, want %s", unreadable[0].File, name)
			}
			if unreadable[0].Severity != SeverityError {
				t.Errorf("an unchecked file is a %s finding, want %s", unreadable[0].Severity, SeverityError)
			}
			if report.Errors == 0 {
				t.Error("the report counted no error, so the gate would exit 0 over a file it could not read")
			}
		})
	}
}

// TestCheckPassesReadableFile pins the other side: a file the tool reads in
// full and that breaks no rule leaves the report empty.
func TestCheckPassesReadableFile(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "short.go", "package fixture\n\nvar c = 1\n")

	report := Check(dir)

	if len(report.Findings) != 0 {
		t.Errorf("a clean fixture drew findings: %+v", report.Findings)
	}
	if report.Errors != 0 || report.Warnings != 0 {
		t.Errorf("a clean fixture counted %d errors and %d warnings", report.Errors, report.Warnings)
	}
}

// TestCheckReportsUnwalkableTree is the walk half of the same fail-open shape.
// A tree the walk cannot enter yields no files, every check over it finds
// nothing, and an empty report reads as a clean tree. The walk error is the
// only thing that distinguishes them, so it is reported rather than dropped.
func TestCheckReportsUnwalkableTree(t *testing.T) {
	report := Check(filepath.Join(t.TempDir(), "no-such-tree"))

	if len(findingsFor(report, "unreadable")) == 0 {
		t.Fatalf("a tree that could not be walked drew no finding: %+v", report.Findings)
	}
	if report.Errors == 0 {
		t.Error("the report counted no error, so the gate would exit 0 over a tree it never read")
	}
}

// TestCheckKeepsWalkingPastAnUnreadableDirectory pins that the walk error is
// reported AND the walk continues: one closed directory must not cost the rest
// of the tree its checks.
func TestCheckKeepsWalkingPastAnUnreadableDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root reads a 0000 directory, so this fixture cannot be built as root")
	}
	dir := t.TempDir()
	writeFixture(t, dir, "closed/hidden.go", "package fixture\n")
	writeFixture(t, dir, "open/tagged.go", "package fixture\n\ntype T struct {\n\tA string `json:\"a_b\"`\n}\n")
	if err := os.Chmod(filepath.Join(dir, "closed"), 0o000); err != nil {
		t.Fatalf("close the fixture directory: %v", err)
	}
	t.Cleanup(func() { _ = os.Chmod(filepath.Join(dir, "closed"), 0o750) })

	report := Check(dir)

	if len(findingsFor(report, "unreadable")) == 0 {
		t.Errorf("the unreadable directory drew no finding: %+v", report.Findings)
	}
	if len(findingsFor(report, "json-kebab-case")) == 0 {
		t.Errorf("the walk stopped at the unreadable directory: the file beside it was never checked: %+v", report.Findings)
	}
}

// TestCheckFindsEachKind is one fixture per check, which is the regression net
// the subprocess test never had. Each case names the check and the message it
// must produce, so a check that silently stops firing fails here.
func TestCheckFindsEachKind(t *testing.T) {
	cases := []struct {
		name     string
		file     string
		body     string
		check    string
		severity string
		message  string
		line     int
	}{
		{
			name:     "snake case json tag",
			file:     "tags.go",
			body:     "package fixture\n\ntype T struct {\n\tA string `json:\"a_b\"`\n}\n",
			check:    "json-kebab-case",
			severity: SeverityError,
			message:  `snake_case JSON tag "a_b" — use kebab-case`,
			line:     4,
		},
		{
			name:     "camel case json tag",
			file:     "tags.go",
			body:     "package fixture\n\ntype T struct {\n\tA string `json:\"aB\"`\n}\n",
			check:    "json-kebab-case",
			severity: SeverityError,
			message:  `camelCase JSON tag "aB" — use kebab-case`,
			line:     4,
		},
		{
			name:     "hardcoded status literal",
			file:     "status.go",
			body:     statusFixture,
			check:    "status-constants",
			severity: SeverityWarn,
			message:  "hardcoded Status: " + strconv.Quote("done") + " — use plugin.Status* constant",
			line:     3,
		},
		{
			name:     "missing design comment",
			file:     "internal/thing/thing.go",
			body:     "package thing\n\nvar c = 1\n",
			check:    "design-refs",
			severity: SeverityWarn,
			message:  "missing // Design: comment",
		},
		{
			name:     "comma split materialized",
			file:     "split.go",
			body:     splitFixture,
			check:    "split-count",
			severity: SeverityWarn,
			message:  "comma strings.Split materializes via a pre-count -- use stringsx.SplitCount when keeping the slice",
			line:     3,
		},
		{
			name:     "stale cross reference",
			file:     "internal/thing/thing.go",
			body:     "// Design: docs/architecture/core-design.md — fixture\n// Detail: gone.go\npackage thing\n",
			check:    "cross-refs",
			severity: SeverityError,
			message:  "stale ref to gone.go (file does not exist)",
			line:     2,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFixture(t, dir, tc.file, tc.body)

			report := Check(dir)

			found := findingsFor(report, tc.check)
			if len(found) != 1 {
				t.Fatalf("%s produced %d findings, want 1: %+v", tc.check, len(found), report.Findings)
			}
			if found[0].Message != tc.message {
				t.Errorf("message is %q, want %q", found[0].Message, tc.message)
			}
			if found[0].Severity != tc.severity {
				t.Errorf("severity is %q, want %q", found[0].Severity, tc.severity)
			}
			if found[0].File != filepath.ToSlash(tc.file) {
				t.Errorf("file is %q, want %q", found[0].File, tc.file)
			}
			if found[0].Line != tc.line {
				t.Errorf("line is %d, want %d", found[0].Line, tc.line)
			}
		})
	}
}

// TestCheckReportsFileSize pins the one threshold the size check has. 1000 is
// the boundary: 1000 lines pass and 1001 fail (ai/rules/go-standards.md).
func TestCheckReportsFileSize(t *testing.T) {
	for _, tc := range []struct {
		name  string
		lines int
		want  int
	}{
		{name: "at the limit", lines: 1000, want: 0},
		{name: "one over the limit", lines: 1001, want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFixture(t, dir, "big.go", strings.Repeat("var _ = 1\n", tc.lines))

			found := findingsFor(Check(dir), "file-size")

			if len(found) != tc.want {
				t.Fatalf("%d lines produced %d file-size findings, want %d", tc.lines, len(found), tc.want)
			}
			if tc.want == 1 && found[0].Message != "1001 lines (max 1000)" {
				t.Errorf("message is %q, want %q", found[0].Message, "1001 lines (max 1000)")
			}
		})
	}
}

// TestCheckSkipsTestFilesAndExemptNames pins the exemptions, which are what
// keeps the gate's warnings actionable. A rule nobody can satisfy is noise.
func TestCheckSkipsTestFilesAndExemptNames(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "internal/thing/thing_test.go", "package thing\n")
	writeFixture(t, dir, "internal/thing/register.go", "package thing\n")
	writeFixture(t, dir, "internal/thing/doc.go", "package thing\n")

	report := Check(dir)

	if len(report.Findings) != 0 {
		t.Errorf("a test file and two exempt names drew findings: %+v", report.Findings)
	}
}

// TestCheckReportsPathsRelativeToTheTree is what makes `le consistency` answer
// the same file names from any working directory: a finding's File is relative
// to the tree that was checked, never to where the process happened to start.
func TestCheckReportsPathsRelativeToTheTree(t *testing.T) {
	dir := t.TempDir()
	writeFixture(t, dir, "internal/thing/thing.go", "package thing\n")

	report := Check(dir)

	if len(report.Findings) == 0 {
		t.Fatal("the fixture drew no finding, so there is no path to check")
	}
	for _, finding := range report.Findings {
		if filepath.IsAbs(finding.File) {
			t.Errorf("finding names an absolute path %q", finding.File)
		}
		if strings.HasPrefix(finding.File, "..") {
			t.Errorf("finding names a path outside the tree %q", finding.File)
		}
	}
	if report.Findings[0].File != "internal/thing/thing.go" {
		t.Errorf("finding names %q, want internal/thing/thing.go", report.Findings[0].File)
	}
}

// TestCheckIsDeterministic pins the property the gate's output could not claim
// before: two runs over one tree answer the same findings in the same order.
// checkCrossRefs iterates a map, so the ORDER is the part at risk.
func TestCheckIsDeterministic(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"a", "b", "c", "d", "e", "f", "g", "h"} {
		writeFixture(t, dir, filepath.Join("internal", name, "thing.go"),
			"// Design: docs/architecture/core-design.md — fixture\n// Detail: other.go\npackage thing\n")
		writeFixture(t, dir, filepath.Join("internal", name, "other.go"),
			"// Design: docs/architecture/core-design.md — fixture\npackage thing\n")
	}

	first := Check(dir)
	second := Check(dir)

	if len(first.Findings) != len(second.Findings) {
		t.Fatalf("two runs found %d and %d findings", len(first.Findings), len(second.Findings))
	}
	for i := range first.Findings {
		if first.Findings[i] != second.Findings[i] {
			t.Fatalf("finding %d differs between runs:\n\t%+v\n\t%+v", i, first.Findings[i], second.Findings[i])
		}
	}
}

// TestAnswerRefusesArguments is the command's grammar: `le consistency` takes
// none, so a word after it is the caller's error rather than a path nobody
// asked to be checked.
func TestAnswerRefusesArguments(t *testing.T) {
	payload, code := Answer([]string{"internal"})

	if code == 0 {
		t.Error("an argument was accepted")
	}
	if payload != nil {
		t.Errorf("a refused call answered a payload: %+v", payload)
	}
}

// TestAnswerVerdictFollowsTheReport pins the exit code to the findings: an
// error is a failed gate, a warning is not, and neither is a flattened 1.
func TestAnswerVerdictFollowsTheReport(t *testing.T) {
	for _, tc := range []struct {
		name string
		file string
		body string
		want int
	}{
		{name: "clean tree", file: "short.go", body: "package fixture\n\nvar c = 1\n", want: 0},
		{name: "warning only", file: "status.go", body: statusFixture, want: 0},
		{name: "error", file: "tags.go", body: "package fixture\n\ntype T struct {\n\tA string `json:\"a_b\"`\n}\n", want: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			writeFixture(t, dir, "go.mod", "module fixture\n")
			writeFixture(t, dir, "feature-gates.txt", "")
			writeFixture(t, dir, tc.file, tc.body)
			// env caches os.Environ() on first read, so the cache has to be
			// dropped for the new value to be seen, and dropped again after so
			// the next test reads the real environment (internal/core/env,
			// ensureCache).
			t.Setenv("ZE_REPO_ROOT", dir)
			env.ResetCache()
			t.Cleanup(env.ResetCache)

			payload, code := Answer(nil)

			if code != tc.want {
				t.Errorf("`le consistency` exited %d, want %d: %+v", code, tc.want, payload)
			}
			report, ok := payload.(Report)
			if !ok {
				t.Fatalf("the command answered %T, not a Report", payload)
			}
			if (report.Errors > 0) != (tc.want == 1) {
				t.Errorf("the report counted %d errors and the command exited %d", report.Errors, code)
			}
		})
	}
}
