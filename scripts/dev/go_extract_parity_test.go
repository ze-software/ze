// The migration's proof for the one Go TOOL in this directory: the script and
// the command agree, on what they PRINT, on what they EXIT with, and on the
// BYTES they leave behind.
//
// scripts/dev/go_extract.go is being replaced by internal/le/goextract, and the two
// live side by side until the swap (plan/spec-le-is-a-ze-binary.md, step 14).
// This file is what makes that safe, and it is deliberately HERE rather than
// beside the new package: it is a migration artifact, so it is deleted by the
// same commit that deletes the script it compares against.
//
// VALIDATES: spec-le-is-a-ze-binary AC-11 for go-extract -- over one fixture
// tree, the script and the command answer the same summary line and the same
// exit code, and the two trees they leave behind are byte for byte identical.
// An output comparison alone would pass a pair that agrees about what it says
// and disagrees about what it writes, and what this tool writes IS a
// developer's source file.
// PREVENTS: a silent behavior change in a port whose subject is other people's
// code.
//
// TWO differences are DELIBERATE and are normalized rather than compared:
//
//   - The failure prefix. The script writes its message bare; the command
//     writes `error: `, the spelling every ported le tool uses, and follows a
//     refusal with its usage line. normalizeGoExtract strips both.
//   - The stream a REFUSAL lands on is the same for both (stderr), so the two
//     streams are compared TOGETHER rather than separately.
//
// THREE fail-open behaviors of the script are pinned rather than reproduced, so
// each test goes red the day somebody fixes the script -- and the answer then
// is to delete the script and this file together:
// TestScriptGoExtractStillMovesAPartialSetOfSymbols,
// TestScriptGoExtractStillEmptiesTheSourceWhenTheDestinationCannotBeWritten and
// TestScriptGoExtractStillOverwritesADestinationItCannotRead.

package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/goextract"
	"github.com/ze-software/ze/internal/le/leroot"
)

// The two bounds this file needs. A link and a move over a three-file fixture
// are both sub-second on this hardware, so a run past either is a hung process
// rather than a slow one.
const (
	goExtractBuildTimeout = 180 * time.Second
	goExtractRunTimeout   = 60 * time.Second
)

// goExtractScript is the compiled script, built once for the whole test binary.
// A per-case build would relink it for every fixture.
var (
	goExtractOnce   sync.Once
	goExtractBinary string
	goExtractErr    error
)

// goExtractSource is the fixture both halves are pointed at: a package clause,
// an import only one function uses, a const group, and three declarations with
// doc comments. It exercises everything the tool decides -- a doc comment
// traveling with its declaration, a group moving whole, and the blank line
// between two adjacent declarations.
const goExtractSource = `package sample

import "strings"

const (
	First  = 1
	Second = 2
)

// Alpha says alpha.
func Alpha() string {
	return strings.ToUpper("alpha")
}

// Beta says beta.
func Beta() string {
	return "beta"
}

// Gamma says gamma.
func Gamma() string {
	return "gamma"
}
`

// buildGoExtractScript compiles the script under test into a temporary
// directory, once.
func buildGoExtractScript(t *testing.T) string {
	t.Helper()

	goExtractOnce.Do(func() {
		ctx, cancel := context.WithTimeout(context.Background(), goExtractBuildTimeout)
		defer cancel()

		repo, err := filepath.Abs("../..")
		if err != nil {
			goExtractErr = err
			return
		}
		dir, err := os.MkdirTemp("", "go-extract-parity")
		if err != nil {
			goExtractErr = err
			return
		}
		binary := filepath.Join(dir, "go_extract")
		build := exec.CommandContext(ctx, "go", "build", "-o", binary,
			filepath.Join("scripts", "dev", "go_extract.go"))
		build.Dir = repo
		build.Env = append(os.Environ(), "CGO_ENABLED=0")
		if combined, buildErr := build.CombinedOutput(); buildErr != nil {
			goExtractErr = errors.New(string(combined))
			return
		}
		goExtractBinary = binary
	})

	if goExtractErr != nil {
		t.Fatalf("compile scripts/dev/go_extract.go: %v", goExtractErr)
	}
	return goExtractBinary
}

// requireGoimports skips a case when the formatter both halves run is not
// installed. Both run the SAME binary, so its absence makes the comparison
// impossible rather than making either half wrong.
func requireGoimports(t *testing.T) {
	t.Helper()

	if _, err := exec.LookPath("goimports"); err != nil {
		t.Skipf("goimports is not installed: %v", err)
	}
}

// goExtractAnswer is what one half of a pair said: its two streams together,
// and its exit code.
type goExtractAnswer struct {
	output string
	code   int
}

// goExtractTree writes the fixture into a fresh directory and answers its path.
// extra names files that must exist before the move, which is how the
// append-to-an-existing-destination case is set up.
func goExtractTree(t *testing.T, extra map[string]string) string {
	t.Helper()

	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sample.go"), []byte(goExtractSource), 0o600); err != nil {
		t.Fatalf("write the fixture: %v", err)
	}
	for name, body := range extra {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o600); err != nil {
			t.Fatalf("write the fixture %s: %v", name, err)
		}
	}
	return dir
}

// readGoExtractTree answers every regular file under dir, keyed by its path
// relative to dir. It is what makes a WRITING tool comparable: two halves that
// print the same line and write different files have not been ported.
func readGoExtractTree(t *testing.T, dir string) map[string]string {
	t.Helper()

	tree := map[string]string{}
	err := filepath.WalkDir(dir, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			return nil
		}
		body, readErr := os.ReadFile(path) //nolint:gosec // a file this test wrote
		if readErr != nil {
			return readErr
		}
		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			return relErr
		}
		tree[rel] = string(body)
		return nil
	})
	if err != nil {
		t.Fatalf("read the tree at %s: %v", dir, err)
	}
	return tree
}

// runGoExtractScript runs the compiled script from inside dir, the way a
// developer runs it: bare positionals, source first.
func runGoExtractScript(t *testing.T, dir string, args ...string) goExtractAnswer {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), goExtractRunTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, buildGoExtractScript(t), args...) //nolint:gosec // the binary this test compiled
	cmd.Dir = dir

	var combined bytes.Buffer
	cmd.Stdout = &combined
	cmd.Stderr = &combined

	code := 0
	var exitErr *exec.ExitError
	switch err := cmd.Run(); {
	case err == nil:
	case errors.As(err, &exitErr):
		code = exitErr.ExitCode()
	default:
		t.Fatalf("run the script over %s: %v (%s)", dir, err, combined.String())
	}

	return goExtractAnswer{output: combined.String(), code: code}
}

// runGoExtractCommand runs the command from inside dir, through leroot the way
// the binary does, so what is compared is the whole of `le go-extract` and not
// one function inside it.
func runGoExtractCommand(t *testing.T, dir string, args ...string) goExtractAnswer {
	t.Helper()

	restore := chdirForTest(t, dir)
	defer restore()

	var out strings.Builder
	wait := captureGoExtractStderr(t)
	code := leroot.Run("go-extract", goextract.Answer, args, &out, &out)

	return goExtractAnswer{output: out.String() + wait(), code: code}
}

// chdirForTest moves the process into dir and answers the call that puts it
// back. Both halves must resolve a relative path against the same directory,
// and the script resolves it against its own working directory.
func chdirForTest(t *testing.T, dir string) func() {
	t.Helper()

	was, err := os.Getwd()
	if err != nil {
		t.Fatalf("read the working directory: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("enter %s: %v", dir, err)
	}
	return func() {
		if err := os.Chdir(was); err != nil {
			t.Fatalf("return to %s: %v", was, err)
		}
	}
}

// captureGoExtractStderr redirects the process's stderr into a pipe and answers
// the call that restores it and hands back what was written.
//
// The command reports a refusal on os.Stderr, which is where the script writes
// its own, so capturing it is what lets the two be compared at all.
func captureGoExtractStderr(t *testing.T) func() string {
	t.Helper()

	read, write, err := os.Pipe()
	if err != nil {
		t.Fatalf("open a pipe for stderr: %v", err)
	}
	was := os.Stderr
	os.Stderr = write

	done := make(chan string, 1)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, read) //nolint:errcheck // the pipe closes when the writer does
		done <- buf.String()
	}()

	return func() string {
		os.Stderr = was
		write.Close() //nolint:errcheck // the reader ends on close
		captured := <-done
		read.Close() //nolint:errcheck // the copy has ended
		return captured
	}
}

// normalizeGoExtract makes two runs over two temporary trees comparable: the
// tree's own path is removed, and whichever refusal prefix a line carries is
// taken off along with the command's usage line.
func normalizeGoExtract(text, dir string) string {
	text = strings.ReplaceAll(text, dir+string(os.PathSeparator), "")
	text = strings.ReplaceAll(text, dir, "")
	text = strings.ReplaceAll(text, "error: ", "")

	var kept []string
	for line := range strings.SplitSeq(text, "\n") {
		if strings.HasPrefix(line, "usage:") {
			continue
		}
		kept = append(kept, line)
	}
	return strings.Join(kept, "\n")
}

// compareGoExtractHalves runs the script and the command over two copies of one
// fixture and asserts they agreed about everything an operator can see.
func compareGoExtractHalves(t *testing.T, extra map[string]string, scriptArgs, commandArgs []string) {
	t.Helper()

	scriptDir := goExtractTree(t, extra)
	commandDir := goExtractTree(t, extra)

	fromScript := runGoExtractScript(t, scriptDir, scriptArgs...)
	fromCommand := runGoExtractCommand(t, commandDir, commandArgs...)

	if fromScript.code != fromCommand.code {
		t.Errorf("the script exited %d and the command %d\nscript: %s\ncommand: %s",
			fromScript.code, fromCommand.code, fromScript.output, fromCommand.output)
	}
	if want, got := normalizeGoExtract(fromScript.output, scriptDir), normalizeGoExtract(fromCommand.output, commandDir); want != got {
		t.Errorf("the two halves said different things\nscript:  %q\ncommand: %q", want, got)
	}

	scriptTree := readGoExtractTree(t, scriptDir)
	commandTree := readGoExtractTree(t, commandDir)
	if len(scriptTree) != len(commandTree) {
		t.Fatalf("the script left %d file(s) and the command %d", len(scriptTree), len(commandTree))
	}
	for name, body := range scriptTree {
		other, ok := commandTree[name]
		if !ok {
			t.Errorf("the command did not leave %s behind", name)
			continue
		}
		if body != other {
			t.Errorf("the two halves wrote different %s\nscript:\n%s\ncommand:\n%s", name, body, other)
		}
	}
}

func TestGoExtractHalvesAgreeOverOneFunction(t *testing.T) {
	requireGoimports(t)

	compareGoExtractHalves(t, nil,
		[]string{"sample.go", "beta.go", "Beta"},
		[]string{"source", "sample.go", "dest", "beta.go", "symbol", "Beta"})
}

func TestGoExtractHalvesAgreeOverAnExistingDestination(t *testing.T) {
	requireGoimports(t)

	existing := map[string]string{"beta.go": "package sample\n\n// Delta was already here.\nfunc Delta() {}\n"}
	compareGoExtractHalves(t, existing,
		[]string{"sample.go", "beta.go", "Beta"},
		[]string{"source", "sample.go", "dest", "beta.go", "symbol", "Beta"})
}

func TestGoExtractHalvesAgreeOverADeclarationGroup(t *testing.T) {
	requireGoimports(t)

	compareGoExtractHalves(t, nil,
		[]string{"sample.go", "consts.go", "Second"},
		[]string{"source", "sample.go", "dest", "consts.go", "symbol", "Second"})
}

func TestGoExtractHalvesAgreeOverTwoAdjacentDeclarations(t *testing.T) {
	requireGoimports(t)

	// Beta and Gamma are adjacent, so the blank line between them travels with
	// them and the hole they leave collapses. Both decisions are in the bytes.
	compareGoExtractHalves(t, nil,
		[]string{"sample.go", "rest.go", "Beta", "Gamma"},
		[]string{"source", "sample.go", "dest", "rest.go", "symbol", "Beta", "symbol", "Gamma"})
}

func TestGoExtractHalvesAgreeOverEverySymbolAtOnce(t *testing.T) {
	requireGoimports(t)

	// The whole file leaves, which is the case that empties the source down to
	// its package clause and its now-unused import.
	compareGoExtractHalves(t, nil,
		[]string{"sample.go", "all.go", "First", "Alpha", "Beta", "Gamma"},
		[]string{"source", "sample.go", "dest", "all.go",
			"symbol", "First", "symbol", "Alpha", "symbol", "Beta", "symbol", "Gamma"})
}

func TestGoExtractHalvesAgreeThatNoSymbolMatched(t *testing.T) {
	// Neither half writes anything, and both exit 1. The WORDING differs -- the
	// script says no symbol matched and the command names the one that is
	// missing -- so what is compared here is the refusal and the untouched
	// tree, which is what an operator loses or keeps.
	scriptDir := goExtractTree(t, nil)
	commandDir := goExtractTree(t, nil)

	fromScript := runGoExtractScript(t, scriptDir, "sample.go", "zeta.go", "Zeta")
	fromCommand := runGoExtractCommand(t, commandDir,
		"source", "sample.go", "dest", "zeta.go", "symbol", "Zeta")

	if fromScript.code != 1 || fromCommand.code != 1 {
		t.Errorf("the script exited %d and the command %d, want 1 each", fromScript.code, fromCommand.code)
	}
	if got := readGoExtractTree(t, scriptDir); len(got) != 1 || got["sample.go"] != goExtractSource {
		t.Errorf("the script edited the tree for a symbol it did not find: %v", got)
	}
	if got := readGoExtractTree(t, commandDir); len(got) != 1 || got["sample.go"] != goExtractSource {
		t.Errorf("the command edited the tree for a symbol it did not find: %v", got)
	}
}

func TestGoExtractHalvesAgreeThatTheSourceDoesNotParse(t *testing.T) {
	scriptDir := goExtractTree(t, map[string]string{"sample.go": "package sample\n\nfunc Beta( {\n"})
	commandDir := goExtractTree(t, map[string]string{"sample.go": "package sample\n\nfunc Beta( {\n"})

	fromScript := runGoExtractScript(t, scriptDir, "sample.go", "beta.go", "Beta")
	fromCommand := runGoExtractCommand(t, commandDir,
		"source", "sample.go", "dest", "beta.go", "symbol", "Beta")

	if fromScript.code != 1 || fromCommand.code != 1 {
		t.Errorf("the script exited %d and the command %d, want 1 each", fromScript.code, fromCommand.code)
	}
	for name, answer := range map[string]goExtractAnswer{"script": fromScript, "command": fromCommand} {
		if !strings.Contains(answer.output, "parse sample.go") {
			t.Errorf("the %s does not say the source could not be parsed: %q", name, answer.output)
		}
	}
}

func TestScriptGoExtractStillMovesAPartialSetOfSymbols(t *testing.T) {
	requireGoimports(t)

	// The script matches what it can and refuses only when NOTHING matched, so
	// one mistyped name of two moves the other and reports success. The port
	// refuses the whole move and names the symbol that is not there
	// (internal/le/goextract, match).
	//
	// This case asserts the SCRIPT still does it, so it reddens the day
	// somebody fixes the script -- and the answer then is to delete the script
	// and this file together.
	dir := goExtractTree(t, nil)
	answer := runGoExtractScript(t, dir, "sample.go", "beta.go", "Beta", "Zeta")

	if answer.code != 0 {
		t.Fatalf("the script exited %d for a half-matched request: %s", answer.code, answer.output)
	}
	if !strings.Contains(answer.output, "extracted 1 symbols") {
		t.Errorf("the script no longer reports a partial move as success: %q", answer.output)
	}
	if left := readGoExtractTree(t, dir)["sample.go"]; strings.Contains(left, "func Beta") {
		t.Error("the script no longer moves the half it matched")
	}
}

func TestScriptGoExtractStillEmptiesTheSourceWhenTheDestinationCannotBeWritten(t *testing.T) {
	// The script writes the SOURCE first and the destination second, so a
	// destination it cannot write leaves the declaration in neither file. The
	// port writes the destination first (internal/le/goextract, Move).
	//
	// This case asserts the SCRIPT still does it, so it reddens the day
	// somebody fixes the script.
	dir := goExtractTree(t, nil)
	answer := runGoExtractScript(t, dir, "sample.go", filepath.Join("nosuchdir", "beta.go"), "Beta")

	if answer.code != 1 {
		t.Fatalf("the script exited %d for a destination it cannot write: %s", answer.code, answer.output)
	}
	left := readGoExtractTree(t, dir)["sample.go"]
	if strings.Contains(left, "func Beta") {
		t.Error("the script no longer deletes the declaration before it knows the destination took it")
	}
	if _, err := os.Stat(filepath.Join(dir, "nosuchdir", "beta.go")); err == nil {
		t.Error("the destination was written after all, so this case proves nothing")
	}
}

func TestScriptGoExtractStillOverwritesADestinationItCannotRead(t *testing.T) {
	requireGoimports(t)
	if os.Geteuid() == 0 {
		t.Skip("root reads a file whose mode denies reading, so the case cannot be set up")
	}

	// The script reads EVERY destination read failure as "the file is not
	// there" and writes a fresh file over it, so a destination it cannot read
	// is a destination it destroys. A write-only file is the case: reading is
	// refused and writing is not. The port refuses before either file is
	// written (internal/le/goextract, destBody).
	//
	// This case asserts the SCRIPT still does it, so it reddens the day
	// somebody fixes the script.
	const keep = "package sample\n\n// Delta is somebody else's work.\nfunc Delta() {}\n"
	dir := goExtractTree(t, map[string]string{"beta.go": keep})
	if err := os.Chmod(filepath.Join(dir, "beta.go"), 0o222); err != nil {
		t.Fatalf("make the destination write-only: %v", err)
	}

	// The exit code is not the point and is not asserted: goimports cannot read
	// the file either, so the script fails at its last step. The destruction
	// has happened by then, which is what this case exists to record.
	runGoExtractScript(t, dir, "sample.go", "beta.go", "Beta")

	if err := os.Chmod(filepath.Join(dir, "beta.go"), 0o600); err != nil {
		t.Fatalf("make the destination readable again: %v", err)
	}
	if got := readGoExtractTree(t, dir)["beta.go"]; strings.Contains(got, "func Delta") {
		t.Error("the script no longer destroys a destination it could not read")
	}

	// The port refuses the same move, and both files are as they were.
	commandDir := goExtractTree(t, map[string]string{"beta.go": keep})
	if err := os.Chmod(filepath.Join(commandDir, "beta.go"), 0o222); err != nil {
		t.Fatalf("make the destination write-only: %v", err)
	}
	if _, err := goextract.PlanMove(goextract.Request{
		Source:  filepath.Join(commandDir, "sample.go"),
		Dest:    filepath.Join(commandDir, "beta.go"),
		Symbols: []string{"Beta"},
	}); err == nil {
		t.Error("the command planned a move over a destination it could not read")
	}
}
