package archerrorinventory

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestErrorInventoryPreRank proves the pre-rank is derived from declared
// facts and never leaves 1..3: an operator surface with error severity or a
// refusal is 1, with warning severity 2, and ./le tooling 3 whatever its
// severity. A surface or severity nobody declared is refused, never ranked,
// and the boundaries 0 and 4 are no rank: Valid says so and JSON refuses to
// render them. Method: preRankOf over chosen pairs, then over every declared
// surface and severity, then the PreRank values at and past each boundary.
func TestErrorInventoryPreRank(t *testing.T) {
	for _, tc := range []struct {
		surface  Surface
		severity Severity
		want     PreRank
	}{
		{SurfaceWeb, SeverityRefusal, PreRankOperatorError},
		{SurfaceLocalCLI, SeverityError, PreRankOperatorError},
		{SurfaceLog, SeverityError, PreRankOperatorError},
		{SurfaceLog, SeverityWarning, PreRankOperatorWarning},
		{SurfaceDoctor, SeverityWarning, PreRankOperatorWarning},
		{SurfaceTooling, SeverityRefusal, PreRankTooling},
		{SurfaceTooling, SeverityWarning, PreRankTooling},
	} {
		got, err := preRankOf(tc.surface, tc.severity)
		if err != nil {
			t.Errorf("preRankOf(%s, %s): %v", tc.surface, tc.severity, err)
			continue
		}
		if got != tc.want {
			t.Errorf("preRankOf(%s, %s) = %d, want %d", tc.surface, tc.severity, got, tc.want)
		}
	}

	for _, tc := range []struct {
		surface  Surface
		severity Severity
	}{
		{SurfaceUnspecified, SeverityRefusal},
		{SurfaceWeb, SeverityUnspecified},
		{SurfaceTooling, SeverityUnspecified},
		{Surface(200), SeverityError},
		{SurfaceWeb, Severity(200)},
	} {
		if got, err := preRankOf(tc.surface, tc.severity); err == nil {
			t.Errorf("preRankOf(%d, %d) = %d, want a refusal", tc.surface, tc.severity, got)
		}
	}

	for _, surface := range surfaces {
		for _, severity := range []Severity{SeverityRefusal, SeverityError, SeverityWarning} {
			got, err := preRankOf(surface, severity)
			if err != nil {
				t.Errorf("preRankOf(%s, %s): %v", surface, severity, err)
				continue
			}
			if !got.Valid() {
				t.Errorf("preRankOf(%s, %s) = %d, outside 1..3", surface, severity, got)
			}
		}
	}

	for _, tc := range []struct {
		rank  PreRank
		valid bool
		json  string
	}{
		{0, false, ""},
		{1, true, "1"},
		{3, true, "3"},
		{4, false, ""},
	} {
		if tc.rank.Valid() != tc.valid {
			t.Errorf("PreRank(%d).Valid() = %v, want %v", tc.rank, !tc.valid, tc.valid)
		}
		encoded, err := json.Marshal(tc.rank)
		if !tc.valid {
			if err == nil {
				t.Errorf("PreRank(%d) rendered as %s, want a refusal", tc.rank, encoded)
			}
			continue
		}
		if err != nil {
			t.Errorf("PreRank(%d) did not render: %v", tc.rank, err)
			continue
		}
		if string(encoded) != tc.json {
			t.Errorf("PreRank(%d) rendered %s, want %s", tc.rank, encoded, tc.json)
		}
	}
}

// scanFixture scans the testdata tree dir with no floor, and answers its
// rows and the text of the file rel inside it.
func scanFixture(t *testing.T, dir, rel string) ([]Row, string) {
	t.Helper()
	inventory, err := Scan(dir, Floors{})
	if err != nil {
		t.Fatalf("scan %s: %v", dir, err)
	}
	text, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatalf("fixture %s: %v", rel, err)
	}
	return inventory.Rows, string(text)
}

// TestErrorInventoryMergesSourcesOnce is AC-2 and R-5: the fixture tree's
// registered handler returns a gRPC status writer call, so operator-error and
// this package's writer scan both name that file and line. Method: prove the
// writer scan alone does list it (the overlap is real), then that the merged
// inventory holds one row there, from operator-error, with its surface and
// status; then that mergeRows keeps the earlier source whatever order the
// rows arrive in, and one row per file and line within one source.
func TestErrorInventoryMergesSourcesOnce(t *testing.T) {
	const handlers = "internal/plugins/fixture/handlers.go"
	rows, text := scanFixture(t, fixtureTree, handlers)
	line := lineOf(t, text, "// site:overlap")

	written, err := writerRows(fixtureTree, nil)
	if err != nil {
		t.Fatalf("writer scan: %v", err)
	}
	writerRow := rowAt(t, written, handlers, line)
	if writerRow.Surface != SurfaceGRPC {
		t.Fatalf("the writer scan names %s:%d as %s, want grpc: the overlap fixture no longer overlaps", handlers, line, writerRow.Surface)
	}

	row := rowAt(t, rows, handlers, line)
	if row.Source != SourceOperatorError {
		t.Errorf("merged row source %s, want operator-error", row.Source)
	}
	if row.Surface != SurfaceDaemonCommand {
		t.Errorf("merged row surface %s, want daemon-command", row.Surface)
	}
	if row.Status != StatusPlainNamesCommand {
		t.Errorf("merged row status %s, want plain-names-command (a pass-through whose literal names ze show)", row.Status)
	}

	site := func(source Source, file string, line int, status Status) Row {
		return Row{Source: source, File: file, Line: line, Status: status}
	}
	merged := mergeRows([]Row{
		site(SourceWriter, "a.go", 1, StatusPlain),
		site(SourceDoctorCoverage, "a.go", 1, StatusPlainNamesCommand),
		site(SourceOperatorError, "a.go", 1, StatusTyped),
		site(SourceWriter, "a.go", 2, StatusPlain),
		site(SourceDoctorCoverage, "a.go", 2, StatusTyped),
		site(SourceWriter, "b.go", 1, StatusPlain),
		site(SourceWriter, "b.go", 1, StatusTyped),
	})
	want := map[siteKey]Row{
		{file: "a.go", line: 1}: site(SourceOperatorError, "a.go", 1, StatusTyped),
		{file: "a.go", line: 2}: site(SourceDoctorCoverage, "a.go", 2, StatusTyped),
		{file: "b.go", line: 1}: site(SourceWriter, "b.go", 1, StatusPlain),
	}
	if len(merged) != len(want) {
		t.Fatalf("mergeRows answered %d rows, want %d: %+v", len(merged), len(want), merged)
	}
	for _, got := range merged {
		expected, found := want[siteKey{file: got.File, line: got.Line}]
		if !found {
			t.Errorf("mergeRows answered %s:%d, which no input row named", got.File, got.Line)
			continue
		}
		if got.Source != expected.Source || got.Status != expected.Status {
			t.Errorf("%s:%d kept %s %s, want %s %s", got.File, got.Line, got.Source, got.Status, expected.Source, expected.Status)
		}
	}
}

// journalTree holds the pre-fix and fixed forms of three shapes of the
// error-names-no-fix-command journal class.
const journalTree = "testdata/journal"

// TestErrorInventoryJournalShapes is AC-4: the local CLI client's
// "hint: is the daemon running?" stderr write, the ./le test qemu cache
// refusal that names no command, and a registered handler's StatusError
// answer each read plain before their fix, and typed after it (the operator
// error type with a fix); a fixed form naming a ./le command in plain text
// reads plain-names-command. Method: scan the journal fixture and read the
// row at each marked line, with its surface and source.
func TestErrorInventoryJournalShapes(t *testing.T) {
	for _, tc := range []struct {
		file    string
		marker  string
		surface Surface
		source  Source
		status  Status
	}{
		{"internal/component/cli/client/main.go", "// shape:cli-before", SurfaceLocalCLI, SourceWriter, StatusPlain},
		{"internal/component/cli/client/main.go", "// shape:cli-after", SurfaceLocalCLI, SourceWriter, StatusTyped},
		{"internal/le/test/qemu/run.go", "// shape:le-before", SurfaceTooling, SourceWriter, StatusPlain},
		{"internal/le/test/qemu/run.go", "// shape:le-after", SurfaceTooling, SourceWriter, StatusTyped},
		{"internal/le/test/qemu/run.go", "// shape:le-named", SurfaceTooling, SourceWriter, StatusPlainNamesCommand},
		{"internal/plugins/peers/handlers.go", "// shape:handler-before", SurfaceDaemonCommand, SourceOperatorError, StatusPlain},
		{"internal/plugins/peers/handlers.go", "// shape:handler-after", SurfaceDaemonCommand, SourceOperatorError, StatusTyped},
	} {
		t.Run(strings.TrimPrefix(tc.marker, "// shape:"), func(t *testing.T) {
			rows, text := scanFixture(t, journalTree, tc.file)
			row := rowAt(t, rows, tc.file, lineOf(t, text, tc.marker))
			if row.Surface != tc.surface {
				t.Errorf("surface %s, want %s", row.Surface, tc.surface)
			}
			if row.Source != tc.source {
				t.Errorf("source %s, want %s", row.Source, tc.source)
			}
			if row.Status != tc.status {
				t.Errorf("status %s, want %s (message %q)", row.Status, tc.status, row.Message)
			}
		})
	}
}

// findingsTree holds doctor-coverage's three Fix shapes.
const findingsTree = "testdata/findings"

// TestErrorInventoryReportsDoctorFindingsOnce is the error-inventory half of
// AC-10: the report action over a tree holding a Diagnostic literal with Fix
// set, one with Fix "" and one with no Fix lists each once, on the doctor
// surface and from doctor-coverage, typed for the first and plain for the
// others. Method: run the registered report, then read the scan's rows at
// each marked line.
func TestErrorInventoryReportsDoctorFindingsOnce(t *testing.T) {
	useTree(t, findingsTree)
	text, code := runRegistered(t, "report")
	if code != 0 {
		t.Fatalf("report exited %d:\n%s", code, text)
	}
	if !strings.Contains(text, "\nsurface=doctor sites=3 typed=1 plain-names-command=0 plain=2\n") {
		t.Errorf("report does not count the three findings:\n%s", text)
	}

	const file = "internal/plugins/finding/finding.go"
	rows, source := scanFixture(t, findingsTree, file)
	if len(rows) != 3 {
		t.Errorf("the scan found %d rows, want the 3 findings: %+v", len(rows), rows)
	}
	for _, tc := range []struct {
		marker string
		status Status
	}{
		{"// finding:fixed", StatusTyped},
		{"// finding:empty", StatusPlain},
		{"// finding:missing", StatusPlain},
	} {
		row := rowAt(t, rows, file, lineOf(t, source, tc.marker))
		if row.Surface != SurfaceDoctor {
			t.Errorf("%s surface %s, want doctor", tc.marker, row.Surface)
		}
		if row.Source != SourceDoctorCoverage {
			t.Errorf("%s source %s, want doctor-coverage", tc.marker, row.Source)
		}
		if row.Status != tc.status {
			t.Errorf("%s status %s, want %s", tc.marker, row.Status, tc.status)
		}
	}
}

// TestErrorInventoryRefusesThinSource proves a scan whose source finds fewer
// sites than its floor is refused, never answered as a cleaner tree: a floor
// above what the fixture tree holds is ErrThinScan naming that source, and
// floors the fixture meets admit it. Method: Scan the fixture tree with one
// floor raised at a time.
func TestErrorInventoryRefusesThinSource(t *testing.T) {
	for _, tc := range []struct {
		source Source
		floors Floors
	}{
		{SourceOperatorError, Floors{OperatorErrorSites: 3}},
		{SourceDoctorCoverage, Floors{DoctorCoverageSites: 3}},
		{SourceWriter, Floors{WriterRows: 100}},
	} {
		_, err := Scan(fixtureTree, tc.floors)
		if err == nil {
			t.Errorf("%s floor: admitted, want ErrThinScan", tc.source)
			continue
		}
		if !strings.HasPrefix(err.Error(), ErrThinScan.Error()) {
			t.Errorf("%s floor: %v, want ErrThinScan", tc.source, err)
		}
		if !strings.Contains(err.Error(), tc.source.String()+" answered") {
			t.Errorf("%s floor: %v does not name the source", tc.source, err)
		}
	}
	if _, err := Scan(fixtureTree, Floors{OperatorErrorSites: 2, DoctorCoverageSites: 2, WriterRows: 8}); err != nil {
		t.Errorf("floors the fixture meets refused: %v", err)
	}
	if _, err := Scan(filepath.Join(fixtureTree, "ai", "doors", "api-surfaces.md"), Floors{}); !errors.Is(err, errNotATree) {
		t.Errorf("a file as the root: %v, want errNotATree", err)
	}
}

// TestErrorInventoryReportQuotesMessage proves a message holding a newline
// stays on its row's one line in the text report.
func TestErrorInventoryReportQuotesMessage(t *testing.T) {
	report := treeReport(Inventory{Rows: []Row{{
		Surface: SurfaceWeb, File: "a.go", Line: 1, Status: StatusPlain,
		PreRank: PreRankOperatorError, Source: SourceWriter, Writer: "http.Error",
		Message: "first\nsecond",
	}}})
	text := report.Text()
	if !strings.Contains(text, `http.Error "first\nsecond"`) {
		t.Errorf("the message is not quoted on its row:\n%s", text)
	}
	if strings.Contains(text, "\nsecond") {
		t.Errorf("the message's newline breaks its row:\n%s", text)
	}
}
