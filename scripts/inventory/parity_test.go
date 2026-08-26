// The migration's proof for these two tools: the scripts and the commands
// agree.
//
// scripts/inventory/inventory.go and commands.go are being replaced by
// letools/inventory and letools/commandlist, and the two sides live together
// until the swap (plan/spec-le-is-a-ze-binary.md, step 14). This file is what
// makes that safe, and it is deliberately HERE rather than beside the new
// packages: it is a migration artifact, so it is deleted by the same commit
// that deletes the scripts it compares against.
//
// VALIDATES: spec-le-is-a-ze-binary AC-11 -- over one tree and one registry,
// each script and its command report the same page, the same JSON and the same
// exit code.
// PREVENTS: a silent behavior change in a port that nobody reads the output of.
// The inventory page is 500 lines of table on this checkout; a count that
// stopped being taken, a section that stopped rendering, or a column that moved
// would pass every other test in this repository.
//
// Two differences are DELIBERATE and are asserted rather than compared:
//
//   - The generation timestamp. Both sides stamp the minute they ran in, so the
//     line is normalized before comparison and pinned by its own case.
//   - A file the walk lists and cannot open. The script returns nil there, so
//     it publishes a count short by whatever it skipped and exits 0. The
//     command reports it. TestPortReportsWhatTheScriptSkips pins that
//     difference in the direction of the fix.
//
// The scripts are built with the tags THIS test binary carries, read from its
// own build info. Both sides then load the same plugin set, which is what makes
// a registry-derived comparison mean anything: a reduced tag set compiles
// modules out, and the two sides would disagree about the product rather than
// about the port.

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/letools/commandlist"
	"github.com/ze-software/ze/letools/inventory"
)

// The two bounds this test needs. A link of the product and a walk of a fixture
// tree are both well inside a minute on this hardware, so a run past either is
// a hung process rather than a slow one.
const (
	buildTimeout = 300 * time.Second
	runTimeout   = 300 * time.Second
)

// scriptPaths are the compiled scripts under test, built once for the whole
// test binary. A per-case build would relink the product for every case.
var scriptPaths = map[string]string{}

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "inventory-parity")
	if err != nil {
		panic("BUG: inventory parity test: cannot create a temporary directory")
	}
	code := 1
	if buildErr := buildScripts(dir); buildErr != nil {
		os.Stderr.WriteString(buildErr.Error() + "\n") //nolint:errcheck // test setup
	} else {
		code = m.Run()
	}
	os.RemoveAll(dir) //nolint:errcheck // temporary directory
	os.Exit(code)     //nolint:gocritic // the two defers this function would own live in buildScripts
}

// buildTags answers the build tags this test binary was compiled with, so the
// scripts can be compiled with the same ones.
func buildTags() []string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return nil
	}
	for _, setting := range info.Settings {
		if setting.Key != "-tags" {
			continue
		}
		return strings.FieldsFunc(setting.Value, func(r rune) bool { return r == ',' })
	}
	return nil
}

// buildScripts compiles both scripts under test into dir. It is a function of
// its own so its context can be canceled by a defer: TestMain ends in os.Exit,
// which runs none.
func buildScripts(dir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()

	repo, err := filepath.Abs("../..")
	if err != nil {
		return fmt.Errorf("resolve the repository root: %w", err)
	}

	tags := buildTags()
	for name, source := range map[string]string{
		"inventory": "scripts/inventory/inventory.go",
		"commands":  "scripts/inventory/commands.go",
	} {
		out := filepath.Join(dir, name)
		args := []string{"build", "-o", out}
		if len(tags) > 0 {
			args = append(args, "-tags", strings.Join(tags, ","))
		}
		args = append(args, source)

		build := exec.CommandContext(ctx, "go", args...) //nolint:gosec // arguments are this file's own literals plus its build tags
		build.Dir = repo
		build.Env = append(os.Environ(), "CGO_ENABLED=0")
		if combined, buildErr := build.CombinedOutput(); buildErr != nil {
			return fmt.Errorf("compile %s: %w\n%s", source, buildErr, combined)
		}
		scriptPaths[name] = out
	}
	return nil
}

// runScript runs one compiled script and answers its stdout and exit code. A
// script that writes to stderr has not run, so that is a fatal error here.
func runScript(t *testing.T, name string, args ...string) (output string, code int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()

	path, built := scriptPaths[name]
	if !built {
		t.Fatalf("no script named %q was built", name)
	}
	cmd := exec.CommandContext(ctx, path, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()

	var exitErr *exec.ExitError
	switch {
	case err == nil:
		code = 0
	case errors.As(err, &exitErr):
		code = exitErr.ExitCode()
	default:
		t.Fatalf("run %s %v: %v (%s)", name, args, err, stderr.String())
	}
	if stderr.Len() > 0 {
		t.Fatalf("%s wrote to stderr, so it did not run: %s", name, stderr.String())
	}
	return stdout.String(), code
}

// generatedLine matches the one line of the inventory page that says when the
// page was made. Both sides stamp the minute they ran in, so a run that
// straddles a minute boundary would differ on it and on nothing else.
var generatedLine = regexp.MustCompile(`(?m)^Generated: .*$`)

// generatedField matches the same fact in the JSON rendering.
var generatedField = regexp.MustCompile(`"generated": *"[^"]*"`)

func normalizeText(s string) string {
	return generatedLine.ReplaceAllString(s, "Generated: <when>")
}

func normalizeJSON(s string) string {
	return generatedField.ReplaceAllString(s, `"generated": "<when>"`)
}

// fixture builds a tree holding one of everything the inventory counts, so the
// comparison covers every count rather than only the ones this checkout draws.
func fixture(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	write(t, dir, "internal/plugins/thing/yang/thing.yang",
		"module thing {\n  rpc thing-list { }\n  rpc thing-clear { }\n}\n")
	write(t, dir, "internal/component/config/yang/modules/core.yang",
		"module core {\n  rpc core-show { }\n}\n")
	write(t, dir, "internal/thing/one.go", "package thing\n\nvar a = 1\n")
	write(t, dir, "internal/thing/two.go", "package thing\n")
	write(t, dir, "pkg/lib/lib.go", "package lib\n")
	write(t, dir, "cmd/ze/main.go", "package main\n\nfunc main() {}\n")
	write(t, dir, "test/ui/one.ci", "cmd=foreground:exec=ze thing list\n")
	write(t, dir, "test/ui/two.ci", "cmd=foreground:exec=ze other\n")
	write(t, dir, "test/bgp/three.ci", "cmd=foreground:exec=ze core show\n")
	return dir
}

func write(t *testing.T, dir, name, body string) {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatalf("create %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

// TestInventoryPagesAgree is AC-11 for `ze-inventory`. The page a developer
// reads must be the page the gate has always printed.
func TestInventoryPagesAgree(t *testing.T) {
	for _, tc := range []struct {
		name string
		root func(t *testing.T) string
	}{
		{"fixture tree", fixture},
		{"empty tree", func(t *testing.T) string { return t.TempDir() }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := tc.root(t)

			scriptOut, scriptCode := runScript(t, "inventory", "--root", root)
			inv, err := inventory.Collect(root)
			if err != nil {
				t.Fatalf("the command refused a tree the script accepted: %v", err)
			}

			if scriptCode != 0 {
				t.Errorf("the script exited %d and the command answered no error", scriptCode)
			}
			if got, want := normalizeText(inv.Text()), normalizeText(scriptOut); got != want {
				t.Errorf("the pages differ:\nscript:\n%s\ncommand:\n%s", want, got)
			}
		})
	}
}

// TestInventoryJSONAgrees is AC-11 for `make ze-inventory-json`. The JSON is a
// separate contract: a consumer reads keys, and a key that changed name would
// break it while the page still looked right.
func TestInventoryJSONAgrees(t *testing.T) {
	root := fixture(t)

	scriptOut, _ := runScript(t, "inventory", "--root", root, "--json")
	inv, err := inventory.Collect(root)
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	raw, err := json.MarshalIndent(inv, "", "  ")
	if err != nil {
		t.Fatalf("marshal the answer: %v", err)
	}

	if got, want := normalizeJSON(string(raw)), normalizeJSON(strings.TrimSpace(scriptOut)); got != want {
		t.Errorf("the JSON differs:\nscript:\n%s\ncommand:\n%s", want, got)
	}
}

// TestCommandListPagesAgree is AC-11 for `ze-command-list`. Both sides read the
// live registry of their own process, so this compares two processes that
// linked the same plugin set.
func TestCommandListPagesAgree(t *testing.T) {
	scriptOut, scriptCode := runScript(t, "commands")
	commands, err := commandlist.Collect()
	if err != nil {
		t.Fatalf("collect: %v", err)
	}

	if scriptCode != 0 {
		t.Errorf("the script exited %d", scriptCode)
	}
	if commands.Text() != scriptOut {
		t.Errorf("the pages differ:\nscript:\n%s\ncommand:\n%s", scriptOut, commands.Text())
	}
	if len(commands) < 10 {
		t.Errorf("the comparison ran over %d commands, which is too few to mean anything", len(commands))
	}
}

// TestCommandListJSONAgrees is AC-11 for `make ze-command-list-json`. The
// answer is the array the script printed, which is why the payload is a slice
// rather than an object wrapping one.
func TestCommandListJSONAgrees(t *testing.T) {
	scriptOut, _ := runScript(t, "commands", "--json")
	commands, err := commandlist.Collect()
	if err != nil {
		t.Fatalf("collect: %v", err)
	}
	raw, err := json.MarshalIndent(commands, "", "  ")
	if err != nil {
		t.Fatalf("marshal the answer: %v", err)
	}
	if string(raw) != strings.TrimSpace(scriptOut) {
		t.Errorf("the JSON differs:\nscript:\n%s\ncommand:\n%s", scriptOut, raw)
	}
}

// TestPortReportsWhatTheScriptSkips is the one behavior the port does NOT
// preserve, asserted in the direction of the fix.
//
// Every number the inventory publishes is a count of what the walk saw. The
// script's walk returns nil for a file it cannot open, so the file is counted
// and its contents are not: a dangling symbolic link under internal/ leaves the
// line count short, the page says nothing, and the exit status is 0.
func TestPortReportsWhatTheScriptSkips(t *testing.T) {
	root := fixture(t)
	if err := os.Symlink(filepath.Join(root, "nowhere"), filepath.Join(root, "internal", "thing", "gone.go")); err != nil {
		t.Fatalf("create the dangling link: %v", err)
	}

	scriptOut, scriptCode := runScript(t, "inventory", "--root", root)
	if scriptCode != 0 {
		t.Fatalf("the script no longer publishes a short count here, so this test has nothing left to say: exit %d\n%s", scriptCode, scriptOut)
	}
	// Five .go files are now listed and four can be read: the file count
	// includes the link, and the line count does not. Eight lines is what the
	// four readable files hold, so the page states five files holding the lines
	// of four.
	for _, want := range []string{"| Go files | 5 |", "| Go lines | 8 |"} {
		if !strings.Contains(scriptOut, want) {
			t.Fatalf("the script no longer reports %s, so the short-count claim needs re-deriving:\n%s", want, scriptOut)
		}
	}

	if _, err := inventory.Collect(root); err == nil {
		t.Error("the command published counts taken from a walk that skipped a file")
	}
}

// TestCommandsDeclareTheirAnswerShapes is what the scripts had no way to do.
// The two answers have different shapes, and declaring each lets the engine
// admit the operators it can support and refuse the rest by name
// (ai/rules/cli.md).
func TestCommandsDeclareTheirAnswerShapes(t *testing.T) {
	for name, want := range map[string]command.AnswerShape{
		"inventory":    command.ShapeDoc,
		"command-list": command.ShapeMap,
	} {
		t.Run(name, func(t *testing.T) {
			shape, declared := command.ShapeForCommand(name)
			if !declared {
				t.Fatalf("the %s command declares no answer shape", name)
			}
			if shape != want {
				t.Errorf("the %s command declares shape %v, want %v", name, shape, want)
			}
		})
	}
}
