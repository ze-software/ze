// Design: docs/contributing/navigating-the-code.md -- the symbol map of one Go file
//
// VALIDATES: `le go symbols list path <file>` prints, byte for byte, what
// `gopls symbols <file>` prints for a file that holds every declaration
// shape (testdata/shapes.symbols is gopls v0.23.0's output for
// testdata/shapes.go), and refuses a missing path, a directory, a file that
// is not Go and a file that does not parse, each with the command that fixes it.
// PREVENTS: the documented `Name Kind <line>:<col>-<line>:<col>` map drifting
// from the positions `gopls definition` and `gopls references` take.
package gosymbols

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// shapesFile is the fixture, and shapesSymbols what gopls printed for it.
const (
	shapesFile    = "testdata/shapes.go"
	shapesSymbols = "testdata/shapes.symbols"
)

func readFixture(t *testing.T, path string) []byte {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestListMatchesGoplsSymbols(t *testing.T) {
	t.Parallel()
	symbols, err := List(shapesFile, readFixture(t, shapesFile))
	if err != nil {
		t.Fatalf("List(%s): %v", shapesFile, err)
	}
	got := Report{File: shapesFile, Symbols: symbols}.Text()
	want := string(readFixture(t, shapesSymbols))
	if got == want {
		return
	}
	gotLines, wantLines := strings.Split(got, "\n"), strings.Split(want, "\n")
	for index := range max(len(gotLines), len(wantLines)) {
		var gotLine, wantLine string
		if index < len(gotLines) {
			gotLine = gotLines[index]
		}
		if index < len(wantLines) {
			wantLine = wantLines[index]
		}
		if gotLine != wantLine {
			t.Fatalf("line %d differs from gopls:\n got: %q\nwant: %q", index+1, gotLine, wantLine)
		}
	}
}

func TestAnswerListsThroughTheAction(t *testing.T) {
	t.Parallel()
	answer, code := Answer([]string{"list", "path", shapesFile})
	if code != 0 {
		t.Fatalf("Answer(list path %s) = code %d, want 0", shapesFile, code)
	}
	report, ok := answer.(Report)
	if !ok {
		t.Fatalf("Answer(list path %s) answered %T, want Report", shapesFile, answer)
	}
	if got, want := report.Text(), string(readFixture(t, shapesSymbols)); got != want {
		t.Errorf("the action's map differs from gopls's:\n%s", got)
	}
	encoded, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("json.Marshal(report): %v", err)
	}
	for _, key := range []string{`"kind":"Struct"`, `"start-line":`, `"start-column":`, `"members":[`} {
		if !strings.Contains(string(encoded), key) {
			t.Errorf("the JSON answer lacks %s: %s", key, encoded)
		}
	}
	if _, code := Answer([]string{"list"}); code != exitUsage {
		t.Errorf("Answer(list) with no path = code %d, want %d", code, exitUsage)
	}
}

func TestLoadRefusesWhatIsNoGoFile(t *testing.T) {
	t.Parallel()
	directory := t.TempDir()
	notGo := filepath.Join(directory, "notes.md")
	broken := filepath.Join(directory, "broken.go")
	for path, content := range map[string]string{notGo: "# notes\n", broken: "package broken\n\nfunc {\n"} {
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	for _, row := range []struct {
		path    string
		code    int
		problem string
		fix     string
	}{
		{filepath.Join(directory, "absent.go"), exitCannotRun, "no file at ", "ls " + filepath.Join(directory, "*.go")},
		{directory, exitUsage, "is a directory, not a Go file", "ls " + filepath.Join(directory, "*.go")},
		{notGo, exitUsage, "is not a Go file", "ls " + filepath.Join(directory, "*.go")},
		{broken, exitProblem, "does not parse as Go", "./le go symbols list path " + broken},
	} {
		_, code, refused := load(row.path)
		if refused == nil {
			t.Errorf("load(%s) answered a map, want a refusal", row.path)
			continue
		}
		if code != row.code {
			t.Errorf("load(%s) refused with code %d, want %d", row.path, code, row.code)
		}
		if !strings.Contains(refused.Problem(), row.problem) {
			t.Errorf("load(%s) problem %q, want it to say %q", row.path, refused.Problem(), row.problem)
		}
		if refused.Fix() != row.fix {
			t.Errorf("load(%s) fix %q, want %q", row.path, refused.Fix(), row.fix)
		}
	}
	// The parse refusal prints the parser's error, so the reader sees the
	// position of the syntax error.
	if _, _, refused := load(broken); refused == nil || !strings.Contains(refused.Error(), broken+":3:") {
		t.Errorf("the parse refusal does not print the parser's position %s:3: %v", broken, refused)
	}
}
