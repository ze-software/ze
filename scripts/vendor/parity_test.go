// The migration's proof for this pair of tools: the scripts and the commands
// agree.
//
// scripts/vendor/check_web.go and scripts/vendor/sync_web.go are being replaced
// by internal/le/vendorweb, and the two live side by side until the swap
// (plan/spec-le-is-a-ze-binary.md, step 14). This file is what makes that safe,
// and it is deliberately HERE rather than beside the new package: it is a
// migration artifact, so it is deleted by the same commit that deletes the
// scripts it compares against.
//
// VALIDATES: spec-le-is-a-ze-binary AC-11 -- over one tree, each script and its
// command answer the same exit code, the same stdout and the same stderr, and
// for the WRITING half the same resulting tree.
// PREVENTS: a silent behavior change in a port that nobody reads the output of.
// A sync that stopped writing one asset, or a check that stopped reporting one
// kind of problem, would pass every other test in this repository.
//
// Two differences are DELIBERATE and are asserted rather than compared:
//
//   - The failure prefix. The scripts write `check_web: ` and `sync_web: ` in
//     front of the one line they put on stderr, naming a file that the swap
//     deletes. The commands write `error: `, the spelling every ported le tool
//     uses. normalize strips whichever one it finds, so the MESSAGE is compared
//     and the program's own name is not.
//   - The sync's fail-open. A run that could read no vendored source at all
//     prints "all consumer copies are up to date" and exits 0, which is the
//     answer a genuinely synced tree gets. The port refuses it, the way the
//     check half of this pair has refused its own empty population since
//     2026-08-15. TestScriptSyncStillFailsOpenOnAnEmptyVendorTree pins the
//     script's behavior, so it goes red the day somebody fixes the script --
//     and the answer then is to delete the script and this file together.

package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/le/vendorweb"
)

// The two bounds this file needs. A link and a walk of a fixture tree are both
// sub-second on this hardware, so a run past either is a hung process rather
// than a slow one.
const (
	parityBuildTimeout = 120 * time.Second
	parityRunTimeout   = 120 * time.Second
)

// parityBinary maps a script file name to the binary built from it, once for
// the whole test binary. A per-case build would relink them for every fixture.
var parityBinary = map[string]string{}

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "vendor-parity")
	if err != nil {
		panic("BUG: vendor parity test: cannot create a temporary directory")
	}
	code := 1
	if buildErr := buildVendorScripts(dir); buildErr != nil {
		os.Stderr.WriteString(buildErr.Error() + "\n") //nolint:errcheck // test setup
	} else {
		code = m.Run()
	}
	os.RemoveAll(dir) //nolint:errcheck // temporary directory
	os.Exit(code)     //nolint:gocritic // the defers this function would own live in buildVendorScripts
}

// buildVendorScripts compiles both scripts under test into dir. It is a
// function of its own so its context can be canceled by a defer: TestMain ends
// in os.Exit, which runs none.
func buildVendorScripts(dir string) error {
	ctx, cancel := context.WithTimeout(context.Background(), parityBuildTimeout)
	defer cancel()

	repo, err := filepath.Abs("../..")
	if err != nil {
		return fmt.Errorf("resolve the repository root: %w", err)
	}

	for _, source := range []string{"check_web.go", "sync_web.go"} {
		binary := filepath.Join(dir, strings.TrimSuffix(source, ".go"))
		build := exec.CommandContext(ctx, "go", "build", "-o", binary, filepath.Join("scripts", "vendor", source))
		build.Dir = repo
		build.Env = append(os.Environ(), "CGO_ENABLED=0")
		if combined, buildErr := build.CombinedOutput(); buildErr != nil {
			return fmt.Errorf("compile scripts/vendor/%s: %w\n%s", source, buildErr, combined)
		}
		parityBinary[source] = binary
	}

	return nil
}

// runParityScript runs one compiled script over root, the way the gate invokes
// it: from inside the tree, with --root naming the same tree.
func runParityScript(t *testing.T, source, root string, args ...string) (stdout, stderr string, code int) {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), parityRunTimeout)
	defer cancel()

	cmd := exec.CommandContext(ctx, parityBinary[source], append([]string{"--root", root}, args...)...)
	cmd.Dir = root

	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut

	err := cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		code = 0
	case errors.As(err, &exitErr):
		code = exitErr.ExitCode()
	default:
		t.Fatalf("run %s over %s: %v (%s)", source, root, err, errOut.String())
	}

	return out.String(), errOut.String(), code
}

// runCheckCommand renders the port the way the bare `le vendor-web-check`
// command does: the report on stdout, the one failure line on stderr, and the
// tool's verdict as the exit code.
func runCheckCommand(root string, updates bool) (stdout, stderr string, code int) {
	report, err := vendorweb.Check(root, updates)
	if err != nil {
		return report.Text(), "error: " + err.Error() + "\n", 1
	}
	return report.Text(), "", 0
}

// runSyncCommand is runCheckCommand for the writing half. The warnings are the
// stderr reading of the same rows the report carries.
func runSyncCommand(root string) (stdout, stderr string, code int) {
	report, err := vendorweb.Sync(root)

	var errOut strings.Builder
	for _, warning := range report.Warnings() {
		errOut.WriteString("warning: " + warning + "\n")
	}

	if err != nil {
		errOut.WriteString("error: " + err.Error() + "\n")
		return report.Text(), errOut.String(), 1
	}

	return report.Text(), errOut.String(), 0
}

// normalize makes two runs over two temporary trees comparable: the tree's own
// path is replaced, and whichever program-name prefix the failure line carries
// is taken off. What is left is the message and the paths inside the tree.
func normalize(text, root string) string {
	text = strings.ReplaceAll(text, root, "<root>")
	for _, prefix := range []string{"check_web: ", "sync_web: ", "error: "} {
		text = strings.ReplaceAll(text, prefix, "")
	}
	return text
}

// parityTreeDigest answers a stable description of every file under root, so
// two trees can be compared without naming a file.
func parityTreeDigest(t *testing.T, root string) string {
	t.Helper()

	var out strings.Builder

	err := filepath.WalkDir(root, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		body, readErr := os.ReadFile(path) //nolint:gosec // a fixture tree this test built
		if readErr != nil {
			return readErr
		}
		out.WriteString(rel)
		out.WriteString("\x00")
		out.Write(body)
		out.WriteString("\x00")
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}

	return out.String()
}

// compareRuns asserts the script and the command answered the same thing.
func compareRuns(t *testing.T, scriptOut, scriptErr string, scriptCode int, scriptRoot string,
	commandOut, commandErr string, commandCode int, commandRoot string,
) {
	t.Helper()

	if scriptCode != commandCode {
		t.Errorf("the script exited %d and the command exited %d\nscript stderr: %s\ncommand stderr: %s",
			scriptCode, commandCode, scriptErr, commandErr)
	}
	if got, want := normalize(commandOut, commandRoot), normalize(scriptOut, scriptRoot); got != want {
		t.Errorf("stdout differs:\nscript:\n%s\ncommand:\n%s", want, got)
	}
	if got, want := normalize(commandErr, commandRoot), normalize(scriptErr, scriptRoot); got != want {
		t.Errorf("stderr differs:\nscript:\n%s\ncommand:\n%s", want, got)
	}
}

// vendorParityCase is one tree, built twice: the script reads one copy and the
// command reads the other, so a run that WRITES cannot see what the other run
// did. The two trees are identical apart from their temporary path, which
// normalize takes out of the comparison.
type vendorParityCase struct {
	name  string
	build func(t *testing.T, root string)
}

// buildPair returns two identical fixture trees.
func buildPair(t *testing.T, tc vendorParityCase) (scriptRoot, commandRoot string) {
	t.Helper()

	scriptRoot = vendorFixture(t, nil)
	commandRoot = vendorFixture(t, nil)

	if tc.build != nil {
		tc.build(t, scriptRoot)
		tc.build(t, commandRoot)
	}

	return scriptRoot, commandRoot
}

// vendorParityCases are the trees both halves are compared over. Each names a
// verdict one of the two programs can reach.
var vendorParityCases = []vendorParityCase{
	{name: "a tree already in sync"},
	{
		name: "a drifted consumer copy",
		build: func(t *testing.T, root string) {
			vendorFixtureFile(t, root, "internal/component/web/assets/htmx.min.js", "// an edited consumer copy\n")
		},
	},
	{
		name: "a missing consumer copy",
		build: func(t *testing.T, root string) {
			if err := os.Remove(filepath.Join(root, "internal", "component", "lg", "assets", "hx-sse.min.js")); err != nil {
				t.Fatalf("remove the fixture copy: %v", err)
			}
		},
	},
	{
		name: "a vendored package no consumer holds",
		build: func(t *testing.T, root string) {
			vendorFixtureFile(t, root, "third_party/web/orphan/orphan.js", "// nobody embeds this\n")
		},
	},
	{
		name: "one file name vendored twice",
		build: func(t *testing.T, root string) {
			vendorFixtureFile(t, root, "third_party/web/other/htmx.min.js", "// a second source of one name\n")
		},
	},
	{
		name: "a missing consumer directory",
		build: func(t *testing.T, root string) {
			if err := os.RemoveAll(filepath.Join(root, "internal", "chaos", "web", "assets")); err != nil {
				t.Fatalf("remove the fixture consumer: %v", err)
			}
		},
	},
	{
		name: "an unreadable vendor source",
		build: func(t *testing.T, root string) {
			path := filepath.Join(root, "third_party", "web", "htmx", "htmx.min.js")
			if err := os.Remove(path); err != nil {
				t.Fatalf("remove the fixture source: %v", err)
			}
			// A directory where the file was: os.ReadFile refuses it for every
			// user, which a permission bit would not do under root.
			if err := os.Mkdir(path, 0o750); err != nil {
				t.Fatalf("build the unreadable source: %v", err)
			}
		},
	},
}

// TestCheckScriptAndCommandAgree is AC-11 for the read-only half.
func TestCheckScriptAndCommandAgree(t *testing.T) {
	for _, tc := range vendorParityCases {
		t.Run(tc.name, func(t *testing.T) {
			scriptRoot, commandRoot := buildPair(t, tc)

			scriptOut, scriptErr, scriptCode := runParityScript(t, "check_web.go", scriptRoot)
			commandOut, commandErr, commandCode := runCheckCommand(commandRoot, false)

			compareRuns(t, scriptOut, scriptErr, scriptCode, scriptRoot,
				commandOut, commandErr, commandCode, commandRoot)
		})
	}
}

// TestSyncScriptAndCommandAgree is AC-11 for the WRITING half. The two streams
// are compared, and so is the tree each run left behind: a sync that wrote the
// right lines and the wrong bytes would satisfy an output comparison alone.
func TestSyncScriptAndCommandAgree(t *testing.T) {
	for _, tc := range vendorParityCases {
		t.Run(tc.name, func(t *testing.T) {
			scriptRoot, commandRoot := buildPair(t, tc)

			before := parityTreeDigest(t, scriptRoot)

			scriptOut, scriptErr, scriptCode := runParityScript(t, "sync_web.go", scriptRoot)
			commandOut, commandErr, commandCode := runSyncCommand(commandRoot)

			compareRuns(t, scriptOut, scriptErr, scriptCode, scriptRoot,
				commandOut, commandErr, commandCode, commandRoot)

			script := strings.ReplaceAll(parityTreeDigest(t, scriptRoot), scriptRoot, "<root>")
			command := strings.ReplaceAll(parityTreeDigest(t, commandRoot), commandRoot, "<root>")
			if script != command {
				t.Errorf("the two runs left different trees behind\nscript:\n%q\ncommand:\n%q", script, command)
			}

			// The control for the cases that DO write: a comparison of two
			// trees neither run touched would pass whatever the sync did.
			if tc.name == "a drifted consumer copy" && script == strings.ReplaceAll(before, scriptRoot, "<root>") {
				t.Error("neither run wrote anything, so the tree comparison proves nothing")
			}
		})
	}
}

// TestUpdateReportScriptAndCommandAgree is AC-11 for the network command. The
// manifest carries no version, so checkVersion returns before it queries
// registry.npmjs.org and both sides answer without a socket.
func TestUpdateReportScriptAndCommandAgree(t *testing.T) {
	scriptRoot := vendorFixture(t, nil)
	commandRoot := vendorFixture(t, nil)

	scriptOut, scriptErr, scriptCode := runParityScript(t, "check_web.go", scriptRoot, "--updates")
	commandOut, commandErr, commandCode := runCheckCommand(commandRoot, true)

	compareRuns(t, scriptOut, scriptErr, scriptCode, scriptRoot,
		commandOut, commandErr, commandCode, commandRoot)

	if !strings.Contains(commandOut, "npm registry") {
		t.Errorf("the registry report did not run, so this case compares the drift walk twice:\n%s", commandOut)
	}
}

// TestScriptSyncStillFailsOpenOnAnEmptyVendorTree is the one behavior the port
// does NOT preserve, asserted in the direction of the fix. The script writes
// nothing because it can read nothing, and reports the tree as up to date.
func TestScriptSyncStillFailsOpenOnAnEmptyVendorTree(t *testing.T) {
	scriptRoot := vendorFixture(t, nil)
	commandRoot := vendorFixture(t, nil)

	for _, root := range []string{scriptRoot, commandRoot} {
		if err := os.RemoveAll(filepath.Join(root, "third_party", "web")); err != nil {
			t.Fatalf("empty the fixture vendor tree: %v", err)
		}
	}

	scriptOut, _, scriptCode := runParityScript(t, "sync_web.go", scriptRoot)
	if scriptCode != 0 || !strings.Contains(scriptOut, "all consumer copies are up to date") {
		t.Fatalf("the script no longer fails open here, so this test has nothing left to say: exit %d\n%s", scriptCode, scriptOut)
	}

	commandOut, _, commandCode := runSyncCommand(commandRoot)
	if commandCode == 0 {
		t.Errorf("the command reported success having read nothing:\n%s", commandOut)
	}
	if strings.Contains(commandOut, "up to date") {
		t.Errorf("the command called the tree up to date:\n%s", commandOut)
	}
}
