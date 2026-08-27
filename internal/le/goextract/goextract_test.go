package goextract

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// VALIDATES: internal/le/goextract moves the declarations it is asked for, moves
// nothing else, and refuses in a way that leaves both files as they were.
// PREVENTS: the three fail-open behaviors of scripts/dev/go_extract.go, each of
// which can destroy a working file -- a symbol that is not there being ignored,
// a destination that cannot be read being overwritten, and the source being
// truncated before the destination write is known to have succeeded.

// twoFuncs is the smallest source a move can be read from: a package clause, an
// import, and two documented functions.
const twoFuncs = `package sample

import "strings"

// Alpha says alpha.
func Alpha() string {
	return strings.ToUpper("alpha")
}

// Beta says beta.
func Beta() string {
	return "beta"
}
`

// fixture writes body to <dir>/<name> and answers the path.
func fixture(t *testing.T, dir, name, body string) string {
	t.Helper()

	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write the fixture %s: %v", path, err)
	}
	return path
}

// read answers a file's whole body, failing the test when it cannot.
func read(t *testing.T, path string) string {
	t.Helper()

	body, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}

// noFormat is the formatter a test uses when the move itself is what is under
// test: it runs no process, so the case needs no goimports on PATH.
func noFormat(_ context.Context, _ string) error { return nil }

func TestMoveTakesADeclarationWithItsDocComment(t *testing.T) {
	dir := t.TempDir()
	source := fixture(t, dir, "sample.go", twoFuncs)
	dest := filepath.Join(dir, "beta.go")

	report, err := Move(t.Context(), Request{Source: source, Dest: dest, Symbols: []string{"Beta"}}, noFormat)
	if err != nil {
		t.Fatalf("Move: %v", err)
	}

	moved := read(t, dest)
	if !strings.Contains(moved, "// Beta says beta.") {
		t.Errorf("the doc comment did not travel with the function:\n%s", moved)
	}
	if !strings.Contains(moved, "func Beta() string {") {
		t.Errorf("the function is not in the destination:\n%s", moved)
	}
	if !strings.HasPrefix(moved, "package sample\n") {
		t.Errorf("the destination carries no package clause:\n%s", moved)
	}

	left := read(t, source)
	if strings.Contains(left, "func Beta") || strings.Contains(left, "// Beta says beta.") {
		t.Errorf("the source still holds what moved:\n%s", left)
	}
	if !strings.Contains(left, "func Alpha() string {") {
		t.Errorf("the source lost a function nobody named:\n%s", left)
	}

	if len(report.Symbols) != 1 || report.Symbols[0].Symbol != "Beta" {
		t.Errorf("the report names %v, want one row for Beta", report.Symbols)
	}
}

func TestMoveAppendsToAnExistingDestination(t *testing.T) {
	dir := t.TempDir()
	source := fixture(t, dir, "sample.go", twoFuncs)
	dest := fixture(t, dir, "beta.go", "package sample\n\n// Gamma is already here.\nfunc Gamma() {}\n")

	if _, err := Move(t.Context(), Request{Source: source, Dest: dest, Symbols: []string{"Beta"}}, noFormat); err != nil {
		t.Fatalf("Move: %v", err)
	}

	moved := read(t, dest)
	if !strings.Contains(moved, "func Gamma() {}") {
		t.Errorf("the append destroyed what the destination already held:\n%s", moved)
	}
	if strings.Count(moved, "package sample") != 1 {
		t.Errorf("the append added a second package clause:\n%s", moved)
	}
	if !strings.Contains(moved, "func Beta() string {") {
		t.Errorf("the function did not arrive:\n%s", moved)
	}
}

func TestMoveEndsTheDestinationsLastLineBeforeAppending(t *testing.T) {
	dir := t.TempDir()
	source := fixture(t, dir, "sample.go", twoFuncs)
	// A destination whose last line has no newline on it. Appending to it
	// without ending that line first joins two declarations into one, which
	// then does not parse.
	dest := fixture(t, dir, "beta.go", "package sample\n\nfunc Gamma() {}")

	if _, err := Move(t.Context(), Request{Source: source, Dest: dest, Symbols: []string{"Beta"}}, noFormat); err != nil {
		t.Fatalf("Move: %v", err)
	}

	moved := read(t, dest)
	if !strings.Contains(moved, "func Gamma() {}\n\n// Beta says beta.") {
		t.Errorf("the append ran into the last line the destination held:\n%q", moved)
	}
}

func TestMoveTakesAWholeDeclarationGroup(t *testing.T) {
	dir := t.TempDir()
	source := fixture(t, dir, "sample.go", `package sample

const (
	First  = 1
	Second = 2
)

func Keep() {}
`)
	dest := filepath.Join(dir, "consts.go")

	// Second is named; First shares its group, so the group moves whole.
	if _, err := Move(t.Context(), Request{Source: source, Dest: dest, Symbols: []string{"Second"}}, noFormat); err != nil {
		t.Fatalf("Move: %v", err)
	}

	moved := read(t, dest)
	for _, name := range []string{"First", "Second"} {
		if !strings.Contains(moved, name) {
			t.Errorf("the group was split: %s stayed behind\n%s", name, moved)
		}
	}
	if left := read(t, source); strings.Contains(left, "First") {
		t.Errorf("half the group is still in the source:\n%s", left)
	}
}

func TestMoveTakesTheBlankLineBetweenTwoAdjacentDeclarations(t *testing.T) {
	dir := t.TempDir()
	source := fixture(t, dir, "sample.go", twoFuncs)
	dest := filepath.Join(dir, "both.go")

	if _, err := Move(t.Context(), Request{Source: source, Dest: dest, Symbols: []string{"Alpha", "Beta"}}, noFormat); err != nil {
		t.Fatalf("Move: %v", err)
	}

	moved := read(t, dest)
	if !strings.Contains(moved, "}\n\n// Beta says beta.") {
		t.Errorf("the blank line between the two declarations did not travel:\n%q", moved)
	}
	if strings.Contains(moved, "}\n\n\n") {
		t.Errorf("the destination carries a run of blank lines:\n%q", moved)
	}
}

func TestMoveCollapsesTheHoleItLeavesBehind(t *testing.T) {
	dir := t.TempDir()
	// Alpha has two blank lines on each side. Removing it leaves four blank
	// lines in a row, and a run of three or more collapses to two.
	source := fixture(t, dir, "sample.go", "package sample\n\nfunc Keep() {}\n\n\nfunc Alpha() {}\n\n\nfunc Also() {}\n")
	dest := filepath.Join(dir, "alpha.go")

	if _, err := Move(t.Context(), Request{Source: source, Dest: dest, Symbols: []string{"Alpha"}}, noFormat); err != nil {
		t.Fatalf("Move: %v", err)
	}

	left := read(t, source)
	if strings.Contains(left, "\n\n\n\n") {
		t.Errorf("the source keeps the hole the extraction left:\n%q", left)
	}
	if !strings.Contains(left, "func Keep() {}\n\n\nfunc Also() {}") {
		t.Errorf("the collapse took more than the run of blank lines:\n%q", left)
	}
}

func TestASymbolThatIsNotDeclaredRefusesTheWholeMove(t *testing.T) {
	dir := t.TempDir()
	source := fixture(t, dir, "sample.go", twoFuncs)
	dest := filepath.Join(dir, "beta.go")

	_, err := Move(t.Context(), Request{Source: source, Dest: dest, Symbols: []string{"Beta", "Zeta"}}, noFormat)
	if err == nil {
		t.Fatal("a symbol that is not declared was accepted")
	}
	if !strings.Contains(err.Error(), "Zeta") {
		t.Errorf("the refusal does not name the symbol: %v", err)
	}
	if strings.Contains(err.Error(), "Beta") {
		t.Errorf("the refusal names a symbol that IS declared: %v", err)
	}

	// Nothing was written: the whole move is refused, not the missing half.
	if left := read(t, source); left != twoFuncs {
		t.Errorf("the source was edited by a refused move:\n%s", left)
	}
	if _, statErr := os.Stat(dest); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("a refused move created the destination: %v", statErr)
	}
}

func TestPlanRefusesADestinationItCannotRead(t *testing.T) {
	dir := t.TempDir()
	source := fixture(t, dir, "sample.go", twoFuncs)

	// A directory reads with EISDIR, which is a failure and is not "the file is
	// not there". The distinction is the whole of the guard: a destination
	// whose CONTENT could not be read is a destination whose content a fresh
	// package clause would destroy, so the refusal has to come before any
	// write rather than at the write.
	dest := filepath.Join(dir, "beta.go")
	if err := os.Mkdir(dest, 0o755); err != nil {
		t.Fatalf("make the unreadable destination: %v", err)
	}

	if _, err := PlanMove(Request{Source: source, Dest: dest, Symbols: []string{"Beta"}}); err == nil {
		t.Fatal("a destination that cannot be read was planned as a new file")
	}

	_, err := Move(t.Context(), Request{Source: source, Dest: dest, Symbols: []string{"Beta"}}, noFormat)
	if err == nil {
		t.Fatal("a destination that cannot be read was accepted")
	}
	if left := read(t, source); left != twoFuncs {
		t.Errorf("the source was edited by a refused move:\n%s", left)
	}
}

func TestADestinationThatCannotBeWrittenLeavesTheSourceWhole(t *testing.T) {
	dir := t.TempDir()
	source := fixture(t, dir, "sample.go", twoFuncs)

	// A path under a directory that does not exist READS as "not there", which
	// is the answer a new destination gives, and then fails to be WRITTEN. So
	// the plan is made, the destination write fails, and the source must still
	// hold every declaration. This is the ordering the whole tool rests on: the
	// source is the only copy until the destination holds one.
	dest := filepath.Join(dir, "nosuchdir", "beta.go")

	_, err := Move(t.Context(), Request{Source: source, Dest: dest, Symbols: []string{"Beta"}}, noFormat)
	if err == nil {
		t.Fatal("a destination that cannot be written reported success")
	}
	if left := read(t, source); left != twoFuncs {
		t.Fatalf("the declaration was deleted from the source and landed nowhere:\n%s", left)
	}
}

func TestSourceAndDestNamingOneFileIsRefused(t *testing.T) {
	dir := t.TempDir()
	source := fixture(t, dir, "sample.go", twoFuncs)

	_, err := Move(t.Context(), Request{Source: source, Dest: filepath.Join(dir, ".", "sample.go"), Symbols: []string{"Beta"}}, noFormat)
	if err == nil {
		t.Fatal("a move into the file it comes out of was accepted")
	}
	if left := read(t, source); left != twoFuncs {
		t.Errorf("the file was rewritten by a refused move:\n%s", left)
	}
}

func TestAnUnparsableSourceRefusesTheMove(t *testing.T) {
	dir := t.TempDir()
	source := fixture(t, dir, "sample.go", "package sample\n\nfunc Beta( {\n")

	_, err := Move(t.Context(), Request{Source: source, Dest: filepath.Join(dir, "beta.go"), Symbols: []string{"Beta"}}, noFormat)
	if err == nil {
		t.Fatal("a source that does not parse was accepted")
	}
	if !strings.Contains(err.Error(), "parse") {
		t.Errorf("the refusal does not say what failed: %v", err)
	}
}

func TestAMoveWithNoSymbolIsRefused(t *testing.T) {
	dir := t.TempDir()
	source := fixture(t, dir, "sample.go", twoFuncs)

	if _, err := Move(t.Context(), Request{Source: source, Dest: filepath.Join(dir, "beta.go")}, noFormat); err == nil {
		t.Fatal("a move naming no symbol was accepted")
	}
}

func TestAFormatterFailureIsReported(t *testing.T) {
	dir := t.TempDir()
	source := fixture(t, dir, "sample.go", twoFuncs)
	dest := filepath.Join(dir, "beta.go")

	broken := func(_ context.Context, path string) error { return errors.New("no formatter for " + path) }
	_, err := Move(t.Context(), Request{Source: source, Dest: dest, Symbols: []string{"Beta"}}, broken)
	if err == nil {
		t.Fatal("a formatter that failed was reported as success")
	}
	// Both files are written by then: the failure is in the tidy-up, and
	// hiding it would leave a developer with unformatted imports and no notice.
	if !strings.Contains(read(t, dest), "func Beta") {
		t.Error("the move was rolled back, so the declaration is in neither file")
	}
}

func TestGoimportsReportsAFileItCannotFormat(t *testing.T) {
	if _, err := exec.LookPath("goimports"); err != nil {
		t.Skipf("goimports is not installed: %v", err)
	}
	dir := t.TempDir()
	broken := fixture(t, dir, "broken.go", "package sample\n\nfunc Beta( {\n")

	if err := Goimports(t.Context(), broken); err == nil {
		t.Fatal("goimports accepted a file that does not parse")
	}
}

func TestPlanWritesNothing(t *testing.T) {
	dir := t.TempDir()
	source := fixture(t, dir, "sample.go", twoFuncs)
	dest := filepath.Join(dir, "beta.go")

	plan, err := PlanMove(Request{Source: source, Dest: dest, Symbols: []string{"Beta"}})
	if err != nil {
		t.Fatalf("PlanMove: %v", err)
	}
	if !strings.Contains(plan.Dest, "func Beta") {
		t.Errorf("the plan does not carry the destination body:\n%s", plan.Dest)
	}
	if left := read(t, source); left != twoFuncs {
		t.Errorf("planning edited the source:\n%s", left)
	}
	if _, statErr := os.Stat(dest); !errors.Is(statErr, os.ErrNotExist) {
		t.Errorf("planning created the destination: %v", statErr)
	}
}

func TestReportIsStructuredDataWithKebabCaseKeys(t *testing.T) {
	report := Report{
		Source:  "a.go",
		Dest:    "b.go",
		Lines:   7,
		Symbols: []Moved{{Symbol: "Beta", FirstLine: 10, LastLine: 13}},
	}

	raw, err := json.Marshal(report)
	if err != nil {
		t.Fatalf("marshal the report: %v", err)
	}
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("decode the report: %v", err)
	}

	for _, key := range []string{"source", "dest", "lines", "symbols"} {
		if _, ok := decoded[key]; !ok {
			t.Errorf("the answer carries no %q key: %v", key, decoded)
		}
	}
	rows, ok := decoded["symbols"].([]any)
	if !ok || len(rows) != 1 {
		t.Fatalf("symbols is not one row set: %v", decoded["symbols"])
	}
	row, ok := rows[0].(map[string]any)
	if !ok {
		t.Fatalf("a symbol row is not an object: %v", rows[0])
	}
	for _, key := range []string{"symbol", "first-line", "last-line"} {
		if _, ok := row[key]; !ok {
			t.Errorf("a symbol row carries no %q key: %v", key, row)
		}
	}
}

func TestTextRendersTheOneLineTheScriptPrinted(t *testing.T) {
	report := Report{
		Source:  "a.go",
		Dest:    "b.go",
		Lines:   7,
		Symbols: []Moved{{Symbol: "Beta"}, {Symbol: "Gamma"}},
	}

	want := "extracted 2 symbols (7 lines) from a.go → b.go\n"
	if got := report.Text(); got != want {
		t.Errorf("Text() = %q, want %q", got, want)
	}
}
