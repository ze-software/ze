// Related: attribution.go -- the comparison these cases drive through Attribute
// Related: observe.go -- the readings and the classes asserted here
//
// VALIDATES: each failing test of a log is re-run as on disk and with the
// named files at HEAD, and lands in exactly one class (AC-15); a file HEAD
// lacks is deleted from the HEAD build (A-4); the failing tests come from the
// log through runlog, and a test the log reports passed is never re-run.
// PREVENTS: a red another session's edit causes being read as the session's,
// and a run that did not compile or a red that did not come back being
// folded into another class.

package testattribution

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	testoverlay "github.com/ze-software/ze/internal/le/test/overlay"
)

// The outputs `go test -v -run '^Name$'` prints, for the fake runner.
const (
	outputPass    = "=== RUN   %s\n--- PASS: %s (0.00s)\nPASS\nok  \texample.com/pkg\t0.10s\n"
	outputFail    = "=== RUN   %s\n    x_test.go:3: boom\n--- FAIL: %s (0.00s)\nFAIL\nFAIL\texample.com/pkg\t0.10s\nFAIL\n"
	outputUnbuilt = "# example.com/pkg [example.com/pkg.test]\n./helper.go:4:2: undefined: Helper\n" +
		"FAIL\texample.com/pkg [build failed]\nFAIL\n"
	outputAbsent = "testing: warning: no tests to run\nPASS\nok  \texample.com/pkg\t0.10s [no tests to run]\n"
)

// run is one scripted run of the fake runner: what one side answers.
type run struct {
	passed bool
	output string
}

func pass() run    { return run{passed: true, output: outputPass} }
func fail() run    { return run{passed: false, output: outputFail} }
func unbuilt() run { return run{passed: false, output: outputUnbuilt} }
func absent() run  { return run{passed: true, output: outputAbsent} }

// scripted is a fake Runner: for each test, what the on-disk run and the HEAD
// overlay run answer. It records every argv it was handed.
type scripted struct {
	disk  map[string]run
	head  map[string]run
	calls [][]string
}

func (s *scripted) run(argv []string) (bool, string, error) {
	s.calls = append(s.calls, argv)
	name := strings.TrimSuffix(strings.TrimPrefix(argvValue(argv, "-run"), "^"), "$")
	side := s.disk
	if slices.Contains(argv, "-overlay") {
		side = s.head
	}
	answer, held := side[name]
	if !held {
		return false, "", nil
	}
	return answer.passed, strings.ReplaceAll(answer.output, "%s", name), nil
}

// argvValue answers the word after flag in argv, or "".
func argvValue(argv []string, flag string) string {
	at := slices.Index(argv, flag)
	if at < 0 || at+1 >= len(argv) {
		return ""
	}
	return argv[at+1]
}

// fakeAttributor answers an Attributor over the scripted runner, a HEAD reader
// that answers Absent for any path holding "new", and a writer that records
// the files it was handed.
func fakeAttributor(runner *scripted, written *[]testoverlay.File) Attributor {
	return Attributor{
		Run: runner.run,
		Head: func(rel string) (testoverlay.File, error) {
			if strings.Contains(rel, "new") {
				return testoverlay.File{Rel: rel, Absent: true}, nil
			}
			return testoverlay.File{Rel: rel, Content: "package pkg\n"}, nil
		},
		Write: func(files []testoverlay.File) (string, error) {
			*written = files
			return "/scratch/attribution/run-1/overlay.json", nil
		},
		Timeout: "10m",
	}
}

// TestAttributionClassifiesEachFailingTest scripts one failing test per pair
// of readings and asserts the class each lands in, that the HEAD build's
// compiler line reaches the undetermined row, and that a caused row makes the
// report exit 1.
func TestAttributionClassifiesEachFailingTest(t *testing.T) {
	cases := []struct {
		test   string
		disk   run
		head   run
		class  Class
		detail string
	}{
		{"TestCaused", fail(), pass(), ClassCaused, "green with the named files at HEAD"},
		{"TestAdded", fail(), absent(), ClassAdded, "the named files add it"},
		{"TestNotCaused", fail(), fail(), ClassNotCaused, "red as on disk and with the named files at HEAD"},
		{"TestHeadUnbuilt", fail(), unbuilt(), ClassUndetermined, "./helper.go:4:2: undefined: Helper"},
		{"TestDiskUnbuilt", unbuilt(), pass(), ClassUndetermined, "as on disk does not compile now"},
		{"TestRedAtHeadOnly", pass(), fail(), ClassRedAtHeadOnly, "fix a red rather than cause one"},
		{"TestFlaky", pass(), pass(), ClassNotReproduced, "did not come back"},
		{"TestElsewhere", absent(), absent(), ClassNotRun, "holds no test of this name"},
	}
	runner := &scripted{disk: map[string]run{}, head: map[string]run{}}
	failed := make([]string, 0, len(cases))
	for _, c := range cases {
		runner.disk[c.test] = c.disk
		runner.head[c.test] = c.head
		failed = append(failed, c.test)
	}
	var written []testoverlay.File

	report, err := fakeAttributor(runner, &written).Attribute(Request{Log: "tmp/unit.log", Failed: failed,
		Package: "internal/le/commit", Tags: "ze_core ze_bgp", Files: []string{"internal/le/commit/create.go"}})
	if err != nil {
		t.Fatalf("Attribute refused: %v", err)
	}
	if len(report.Rows) != len(cases) {
		t.Fatalf("got %d rows for %d failing tests: %+v", len(report.Rows), len(cases), report.Rows)
	}
	for index, c := range cases {
		row := report.Rows[index]
		if row.Test != c.test {
			t.Errorf("row %d names %s, want %s", index, row.Test, c.test)
		}
		if row.Class != c.class {
			t.Errorf("%s: class %q, want %q (on disk %s, at HEAD %s)", c.test, row.Class, c.class, row.OnDisk, row.AtHead)
		}
		if !strings.Contains(row.Detail, c.detail) {
			t.Errorf("%s: detail %q does not say %q", c.test, row.Detail, c.detail)
		}
	}
	if code := report.ExitCode(); code != exitCaused {
		t.Errorf("a report with a caused row exits %d, want %d", code, exitCaused)
	}
	if len(runner.calls) != 2*len(cases) {
		t.Errorf("ran %d times for %d failing tests, want one on-disk and one HEAD run each", len(runner.calls), len(cases))
	}
}

// TestAttributionRunsTheTestWithTheCallersTagsAndTheOverlay asserts the argv
// of both runs: the package, the caller's tags and the anchored test name on
// both, and the overlay only on the HEAD run.
func TestAttributionRunsTheTestWithTheCallersTagsAndTheOverlay(t *testing.T) {
	runner := &scripted{disk: map[string]run{"TestCaused": fail()}, head: map[string]run{"TestCaused": pass()}}
	var written []testoverlay.File

	_, err := fakeAttributor(runner, &written).Attribute(Request{Log: "l", Failed: []string{"TestCaused"},
		Package: "internal/le/commit", Tags: "ze_core ze_bgp", Files: []string{"internal/le/commit/create.go"}})
	if err != nil {
		t.Fatalf("Attribute refused: %v", err)
	}
	if len(runner.calls) != 2 {
		t.Fatalf("ran %d times, want 2", len(runner.calls))
	}
	for index, argv := range runner.calls {
		if got := argvValue(argv, "-tags"); got != "ze_core ze_bgp" {
			t.Errorf("run %d carries tags %q, want the caller's", index, got)
		}
		if got := argvValue(argv, "-run"); got != "^TestCaused$" {
			t.Errorf("run %d selects %q, want the anchored name", index, got)
		}
		if got := argv[len(argv)-1]; got != "./internal/le/commit" {
			t.Errorf("run %d tests %q, want ./internal/le/commit", index, got)
		}
	}
	if slices.Contains(runner.calls[0], "-overlay") {
		t.Errorf("the on-disk run carries an overlay: %v", runner.calls[0])
	}
	if got := argvValue(runner.calls[1], "-overlay"); got != "/scratch/attribution/run-1/overlay.json" {
		t.Errorf("the HEAD run carries overlay %q, want the written one", got)
	}
}

// TestAttributionDeletesAFileHeadLacks states A-4 at the attribution's own
// boundary: a named file HEAD does not hold reaches the overlay as Absent, so
// the HEAD build sees no file there, and a file HEAD holds carries its content.
func TestAttributionDeletesAFileHeadLacks(t *testing.T) {
	runner := &scripted{disk: map[string]run{"TestAdded": fail()}, head: map[string]run{"TestAdded": absent()}}
	var written []testoverlay.File

	report, err := fakeAttributor(runner, &written).Attribute(Request{Log: "l", Failed: []string{"TestAdded"},
		Package: "internal/le/commit", Tags: "ze_core",
		Files: []string{"internal/le/commit/new_test.go", "internal/le/commit/create.go"}})
	if err != nil {
		t.Fatalf("Attribute refused: %v", err)
	}
	want := []testoverlay.File{
		{Rel: "internal/le/commit/new_test.go", Absent: true},
		{Rel: "internal/le/commit/create.go", Content: "package pkg\n"},
	}
	if !slices.Equal(written, want) {
		t.Errorf("the overlay holds %+v, want %+v", written, want)
	}
	if report.Rows[0].Class != ClassAdded {
		t.Errorf("a red test the named files add is %q, want %q", report.Rows[0].Class, ClassAdded)
	}
}

// TestHeadReaderAnswersAbsentForAFileTheRevisionLacks drives the real git
// reader over a tree object written in a scratch repository: a file the tree
// holds answers its content with its last newline, a file it lacks that is on
// disk answers Absent, a directory is refused, and a path on neither side is
// refused.
func TestHeadReaderAnswersAbsentForAFileTheRevisionLacks(t *testing.T) {
	for _, key := range []string{"GIT_DIR", "GIT_INDEX_FILE", "GIT_WORK_TREE"} {
		t.Setenv(key, "")
		os.Unsetenv(key) //nolint:errcheck // t.Setenv restores the value after the test
	}
	tree := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		out, err := exec.CommandContext(t.Context(), "git", append([]string{"-C", tree}, args...)...).CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v: %s", args, err, out)
		}
		return strings.TrimSpace(string(out))
	}
	write := func(rel, content string) {
		t.Helper()
		target := filepath.Join(tree, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(target), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(target, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	git("init", "-q")
	write("pkg/held.go", "package pkg\n")
	git("update-index", "--add", "pkg/held.go")
	revision := git("write-tree")
	write("pkg/held.go", "package pkg\n\nfunc Changed() {}\n")
	write("pkg/added.go", "package pkg\n")

	read := headReader(tree, revision)

	held, err := read("pkg/held.go")
	if err != nil {
		t.Fatalf("a held file was refused: %v", err)
	}
	if held != (testoverlay.File{Rel: "pkg/held.go", Content: "package pkg\n"}) {
		t.Errorf("a held file answered %+v, want its content at the revision", held)
	}
	added, err := read("pkg/added.go")
	if err != nil {
		t.Fatalf("an added file was refused: %v", err)
	}
	if added != (testoverlay.File{Rel: "pkg/added.go", Absent: true}) {
		t.Errorf("a file the revision lacks answered %+v, want Absent", added)
	}
	if _, err := read("pkg"); err == nil {
		t.Errorf("a directory was accepted as a file")
	}
	if _, err := read("pkg/nowhere.go"); err == nil {
		t.Errorf("a path on neither side was accepted")
	}
}

// TestFailedTestsComeFromTheLogAndPassedOnesAreNotRerun reads a log through
// readFailed, the command's own route to runlog, and attributes it: the
// tests the log reports passed are never handed to the runner.
func TestFailedTestsComeFromTheLogAndPassedOnesAreNotRerun(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "unit.log")
	log := "=== RUN   TestGreen\n--- PASS: TestGreen (0.00s)\n=== RUN   TestRed\n" +
		"    red_test.go:9: boom\n--- FAIL: TestRed (0.01s)\nFAIL\nFAIL\texample.com/pkg\t0.20s\n"
	if err := os.WriteFile(logPath, []byte(log), 0o600); err != nil {
		t.Fatal(err)
	}
	failed, err := readFailed(logPath)
	if err != nil {
		t.Fatalf("readFailed: %v", err)
	}
	if !slices.Equal(failed, []string{"TestRed"}) {
		t.Fatalf("the log's failing tests are %v, want [TestRed]", failed)
	}

	runner := &scripted{disk: map[string]run{"TestRed": fail()}, head: map[string]run{"TestRed": fail()}}
	var written []testoverlay.File
	report, err := fakeAttributor(runner, &written).Attribute(Request{Log: logPath, Failed: failed,
		Package: "pkg", Tags: "ze_core", Files: []string{"pkg/red.go"}})
	if err != nil {
		t.Fatalf("Attribute refused: %v", err)
	}
	for _, argv := range runner.calls {
		if argvValue(argv, "-run") == "^TestGreen$" {
			t.Errorf("a test the log reports passed was re-run: %v", argv)
		}
	}
	if code := report.ExitCode(); code != exitNoneCaused {
		t.Errorf("a report whose only row is not-caused exits %d, want 0", code)
	}
}

// TestAttributionRefusesALogWithNoFailure states that a log naming no failing
// test answers an error rather than an empty report that reads as "none".
func TestAttributionRefusesALogWithNoFailure(t *testing.T) {
	runner := &scripted{}
	var written []testoverlay.File
	_, err := fakeAttributor(runner, &written).Attribute(Request{Log: "l", Package: "pkg", Tags: "ze_core",
		Files: []string{"pkg/a.go"}})
	if err == nil {
		t.Fatalf("a log with no failing test was attributed")
	}
	if len(runner.calls) != 0 {
		t.Errorf("a refused attribution still ran %d tests", len(runner.calls))
	}
}

// TestReportExitsTwoWhenARowReachedNoVerdict states that an undetermined row
// with no caused row is not reported as "none caused".
func TestReportExitsTwoWhenARowReachedNoVerdict(t *testing.T) {
	report := Report{Rows: []Row{{Test: "TestA", Class: ClassNotCaused}, {Test: "TestB", Class: ClassUndetermined}}}
	if code := report.ExitCode(); code != exitNoVerdict {
		t.Errorf("an undetermined row exits %d, want %d", code, exitNoVerdict)
	}
	if !strings.Contains(report.Text(), "reached no verdict") {
		t.Errorf("the text does not say a row reached no verdict:\n%s", report.Text())
	}
}

// TestTheVerdictStatesWhatItCompared states R-5: the report says the named
// files were at HEAD and every other file as on disk, so a not-caused row is
// never read as wider than that.
func TestTheVerdictStatesWhatItCompared(t *testing.T) {
	runner := &scripted{disk: map[string]run{"TestRed": fail()}, head: map[string]run{"TestRed": fail()}}
	var written []testoverlay.File
	report, err := fakeAttributor(runner, &written).Attribute(Request{Log: "l", Failed: []string{"TestRed"},
		Package: "pkg", Tags: "ze_core", Files: []string{"pkg/a.go", "pkg/b.go"}})
	if err != nil {
		t.Fatalf("Attribute refused: %v", err)
	}
	for _, want := range []string{"pkg/a.go, pkg/b.go at HEAD", "every other file as on disk",
		"together with another session's edit"} {
		if !strings.Contains(report.Compared, want) {
			t.Errorf("the comparison %q does not say %q", report.Compared, want)
		}
		if !strings.Contains(report.Text(), want) {
			t.Errorf("the text does not carry %q", want)
		}
	}
}

// TestCleanPathRefusesAPathOutsideTheTree covers the path boundary: a leading
// ./ and a trailing slash are dropped, and an absolute path, a climb out of
// the tree and the root itself are refused.
func TestCleanPathRefusesAPathOutsideTheTree(t *testing.T) {
	if got, err := CleanPath("./internal/le/commit/"); err != nil || got != "internal/le/commit" {
		t.Errorf("CleanPath(./internal/le/commit/) = %q, %v", got, err)
	}
	for _, bad := range []string{"/etc/passwd", "../other", "..", ".", "a/../.."} {
		if got, err := CleanPath(bad); err == nil {
			t.Errorf("CleanPath(%q) accepted %q", bad, got)
		}
	}
}
