// The migration's proof for this tool: the script and the command agree.
//
// scripts/lint/consistency.go is being replaced by internal/le/consistency, and the
// two live side by side until the swap (plan/spec-le-is-a-ze-binary.md, step
// 14). This file is what makes that safe, and it is deliberately HERE rather
// than beside the new package: it is a migration artifact, so it is deleted by
// the same commit that deletes the script it compares against.
//
// VALIDATES: spec-le-is-a-ze-binary AC-11 -- over one tree, the script and the
// command report the same findings, group them the same way, and answer the
// same exit code.
// PREVENTS: a silent behavior change in a port that nobody reads the output of.
// The whole gate is 1250 lines of report on this checkout; a check that stopped
// firing, a message that lost a word, or a severity that moved would pass every
// other test in this repository.
//
// Two differences are DELIBERATE and are asserted rather than compared:
//
//   - Colors. The script writes raw ANSI (\033[31m); a compiled Ze package may
//     not, so the command writes the semantic palette
//     (docs/architecture/cli/color-system.md). Both are stripped before
//     comparison, and TestTextColorsBySeverity pins the command's own.
//   - A tree that cannot be read. The script's walk returns nil on the error,
//     so it reports NOTHING and exits 0 over a tree it never read. The command
//     reports it. TestPortReportsWhatTheScriptFailsOpenOn pins that difference
//     in the direction of the fix.

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/le/consistency"
)

// ansiSGR matches a color escape, so a comparison can be about the findings
// rather than about the palette.
var ansiSGR = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string { return ansiSGR.ReplaceAllString(s, "") }

// The two bounds this test needs. A link and a walk of a fixture tree are
// both sub-second on this hardware, so a run past either is a hung process
// rather than a slow one.
const (
	buildTimeout = 120 * time.Second
	runTimeout   = 120 * time.Second
)

// scriptPath is the compiled script under test, built once for the whole test
// binary. A per-case build would relink it four times, and a t.TempDir() in a
// sync.Once would be removed when the first case that touched it ended.
var scriptPath string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "consistency-parity")
	if err != nil {
		panic("BUG: consistency parity test: cannot create a temporary directory")
	}
	code := 1
	if buildErr := buildScript(dir); buildErr != nil {
		os.Stderr.WriteString(buildErr.Error() + "\n") //nolint:errcheck // test setup
	} else {
		code = m.Run()
	}
	os.RemoveAll(dir) //nolint:errcheck // temporary directory
	os.Exit(code)     //nolint:gocritic // the two defers this function would own live in buildScript
}

// buildScript compiles the script under test into dir. It is a function of its
// own so its context can be canceled by a defer: TestMain ends in os.Exit,
// which runs none.
func buildScript(dir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), buildTimeout)
	defer cancel()

	repo, err := filepath.Abs("../..")
	if err != nil {
		return fmt.Errorf("resolve the repository root: %w", err)
	}
	scriptPath = filepath.Join(dir, "consistency")
	build := exec.CommandContext(ctx, "go", "build", "-o", scriptPath, "scripts/lint/consistency.go")
	build.Dir = repo
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if combined, err := build.CombinedOutput(); err != nil {
		return fmt.Errorf("compile scripts/lint/consistency.go: %w\n%s", err, combined)
	}
	return nil
}

// runScript runs the script from cwd over the path it is given, which is how
// the gate invokes it: from inside the tree, with "." as the path, so the file
// names it prints are relative to the tree exactly as the command's are.
func runScript(t *testing.T, cwd, arg string) (output string, code int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()
	cmd := exec.CommandContext(ctx, scriptPath, arg)
	cmd.Dir = cwd
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
		t.Fatalf("run the script in %s over %s: %v (%s)", cwd, arg, err, stderr.String())
	}
	if stderr.Len() > 0 {
		t.Fatalf("the script wrote to stderr, so it did not run: %s", stderr.String())
	}
	return stdout.String(), code
}

// runCommand runs the port over root and renders it the way the bare command
// does.
func runCommand(root string) (output string, code int) {
	report := consistency.Check(root)
	if report.Errors > 0 {
		return report.Text(), 1
	}
	return report.Text(), 0
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

// The two patterns the gate looks for in test files as well as source files,
// assembled so this file is not itself a finding of the gate it tests.
var (
	statusBody = "package fixture\n\nvar r = Response{Status: " + strconv.Quote("done") + "}\n"
	splitBody  = "package fixture\n\nvar parts = strings.Split(s, " + strconv.Quote(",") + ")\n"
)

// TestScriptAndCommandAgree is AC-11. Each tree is checked by both, and the
// reports must be the same report.
func TestScriptAndCommandAgree(t *testing.T) {
	cases := []struct {
		name string
		// ordered says the finding ORDER is comparable too. It is not when a
		// tree draws cross-reference findings: the script iterates a map to
		// produce them, so their order changes between runs of the script
		// itself. The command sorts. Where the order is not comparable the
		// lines are still compared as a set, which pins every finding, every
		// group count and the summary.
		ordered bool
		build   func(t *testing.T, dir string)
	}{
		{
			name:    "clean tree",
			ordered: true,
			build: func(t *testing.T, dir string) {
				write(t, dir, "short.go", "package fixture\n\nvar c = 1\n")
			},
		},
		{
			name:    "every check but cross-references",
			ordered: true,
			build: func(t *testing.T, dir string) {
				write(t, dir, "tags.go", "package fixture\n\ntype T struct {\n\tA string `json:\"a_b\"`\n\tB string `json:\"cD\"`\n}\n")
				write(t, dir, "status.go", statusBody)
				write(t, dir, "split.go", splitBody)
				write(t, dir, "internal/thing/thing.go", "package thing\n\nvar c = 1\n")
				write(t, dir, "internal/thing/big.go", "// Design: none\n"+strings.Repeat("var _ = 1\n", 1001))
				write(t, dir, "internal/thing/thing_test.go", "package thing\n")
				write(t, dir, "internal/thing/doc.go", "package thing\n")
			},
		},
		{
			name: "cross references",
			build: func(t *testing.T, dir string) {
				write(t, dir, "internal/a/one.go", "// Design: d\n// Detail: two.go\npackage a\n")
				write(t, dir, "internal/a/two.go", "// Design: d\npackage a\n")
				write(t, dir, "internal/a/three.go", "// Design: d\n// Related: gone.go\npackage a\n")
				write(t, dir, "internal/b/four.go", "// Design: d\n// Overview: ../a/one.go\npackage b\n")
				write(t, dir, "internal/b/five.go", "// Design: d\n// Related: four.go\npackage b\n")
			},
		},
		{
			name: "plugin structure",
			build: func(t *testing.T, dir string) {
				write(t, dir, "internal/component/bgp/plugins/cmd/thing/thing.go", "// Design: d\npackage thing\n")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			tc.build(t, dir)

			scriptOut, scriptCode := runScript(t, dir, ".")
			commandOut, commandCode := runCommand(dir)

			if scriptCode != commandCode {
				t.Errorf("the script exited %d and the command exited %d", scriptCode, commandCode)
			}

			script, port := stripANSI(scriptOut), stripANSI(commandOut)
			if tc.ordered && script != port {
				t.Errorf("the reports differ:\nscript:\n%s\ncommand:\n%s", script, port)
			}
			if diff := lineSetDiff(script, port); diff != "" {
				t.Errorf("the reports hold different lines:\n%s", diff)
			}
		})
	}
}

// lineSetDiff answers the lines one report holds and the other does not, in
// both directions, or "" when they hold the same lines.
func lineSetDiff(script, port string) string {
	left, right := strings.Split(script, "\n"), strings.Split(port, "\n")
	sort.Strings(left)
	sort.Strings(right)
	if strings.Join(left, "\n") == strings.Join(right, "\n") {
		return ""
	}
	var out strings.Builder
	counts := map[string]int{}
	for _, line := range left {
		counts[line]++
	}
	for _, line := range right {
		counts[line]--
	}
	for _, line := range sortedKeys(counts) {
		switch {
		case counts[line] > 0:
			out.WriteString("  only the script: " + line + "\n")
		case counts[line] < 0:
			out.WriteString("  only the command: " + line + "\n")
		}
	}
	return out.String()
}

func sortedKeys(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for key := range m {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

// TestPortReportsWhatTheScriptFailsOpenOn is the one behavior the port does NOT
// preserve, asserted in the direction of the fix. The script's walk returns nil
// on a read error, so a tree it cannot enter produces no finding and the gate
// passes. That is the failure this gate exists to prevent, applied to itself.
func TestPortReportsWhatTheScriptFailsOpenOn(t *testing.T) {
	dir := t.TempDir()
	const name = "no-such-tree"

	scriptOut, scriptCode := runScript(t, dir, name)
	if scriptCode != 0 || !strings.Contains(stripANSI(scriptOut), "All consistency checks passed") {
		t.Fatalf("the script no longer fails open here, so this test has nothing left to say: exit %d\n%s", scriptCode, scriptOut)
	}

	commandOut, commandCode := runCommand(filepath.Join(dir, name))
	if commandCode == 0 {
		t.Errorf("the command passed a tree it could not read:\n%s", commandOut)
	}
	if !strings.Contains(stripANSI(commandOut), "unreadable") {
		t.Errorf("the command did not say the tree was unreadable:\n%s", commandOut)
	}
}

// TestCommandDeclaresItsAnswerShape is what the script had no way to do: the
// answer carries rows, so the row operators act on the findings instead of
// being refused (ai/rules/cli.md). Reading it back from the engine's registry
// proves the declaration reached the engine both binaries share.
func TestCommandDeclaresItsAnswerShape(t *testing.T) {
	shape, declared := command.ShapeForCommand("consistency")
	if !declared {
		t.Fatal("the consistency command declares no answer shape")
	}
	if shape != command.ShapeMap {
		t.Errorf("the consistency command declares shape %v, want ShapeMap", shape)
	}
}
