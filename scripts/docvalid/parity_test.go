// The migration's proof for these two tools: the scripts and the commands
// agree.
//
// scripts/docvalid/doc_drift.go and commands.go are being replaced by
// letools/docvalid, and the two sides live together until the swap
// (plan/spec-le-is-a-ze-binary.md, step 14). This file is what makes that safe,
// and it is deliberately HERE rather than beside the new package: it is a
// migration artifact, so it is deleted by the same commit that deletes the
// scripts it compares against.
//
// VALIDATES: spec-le-is-a-ze-binary AC-11 -- over one tree and one registry,
// each script and its command report the same findings, write the same bytes
// and answer the same exit code.
// PREVENTS: a silent behavior change in a port that nobody reads the output of.
// The contract table is 400 lines on this checkout and the drift gate reads
// eight documents; a check that stopped firing or a message that lost a word
// would pass every other test in this repository.
//
// Three differences are DELIBERATE and are asserted rather than compared:
//
//   - Colors, and the stream. The script writes raw ANSI to stderr; a compiled
//     Ze package may not write an escape at all, so the command writes the
//     semantic palette (docs/architecture/cli/color-system.md) and the engine
//     renders its ONE payload to stdout. Both sides are compared with the
//     escapes stripped and the two streams joined.
//   - A file the scan lists and cannot open. The script answers a zero count
//     and says nothing, so a document that understates the tree agrees with it.
//     TestPortReportsTheFileTheScriptCountsBlind pins that difference in the
//     direction of the fix.
//   - The row order of the command table. The script sorts on the wire method
//     alone, with an unstable sort, over rows a map walk produced, so two runs
//     of it print two different tables. TestScriptStillSortsOnOneKeyAlone pins
//     the defect that is still in the script, and the rows are compared as a
//     SET.
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
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/letools/docvalid"
)

// The two bounds this test needs. A link of the product and a walk of this
// checkout are both well inside five minutes on this hardware, so a run past
// either is a hung process rather than a slow one.
const (
	buildTimeout = 300 * time.Second
	runTimeout   = 300 * time.Second
)

// scriptPaths are the compiled scripts under test, built once for the whole
// test binary. A per-case build would relink the product for every case.
var scriptPaths = map[string]string{}

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "docvalid-parity")
	if err != nil {
		panic("BUG: docvalid parity test: cannot create a temporary directory")
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
		"doc_drift": "scripts/docvalid/doc_drift.go",
		"commands":  "scripts/docvalid/commands.go",
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

// runScript runs one compiled script and answers its output and exit code.
//
// The two streams are JOINED, because the drift script writes its clean line to
// stdout and its findings to stderr, and the command answers one payload the
// engine renders to stdout. What is compared is the report, not which file
// descriptor carried it.
func runScript(t *testing.T, name string, args ...string) (output string, code int) {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), runTimeout)
	defer cancel()

	path, built := scriptPaths[name]
	if !built {
		t.Fatalf("no script named %q was built", name)
	}
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.Dir = repoRoot(t)
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
	return stdout.String() + stderr.String(), code
}

// ansiSGR matches a color escape, so a comparison is about the findings rather
// than about the palette.
var ansiSGR = regexp.MustCompile(`\x1b\[[0-9;]*m`)

func stripANSI(s string) string { return ansiSGR.ReplaceAllString(s, "") }

// runDriftCommand runs the port over root and renders it the way the bare
// command does.
func runDriftCommand(root string) (output string, code int) {
	report := docvalid.Drift(root)
	if len(report.Issues) > 0 {
		return report.Text(), 1
	}
	return report.Text(), 0
}

// TestDriftScriptAndCommandAgree is AC-11 for the drift gate. Each tree is
// judged by both, and the reports must be the same report.
func TestDriftScriptAndCommandAgree(t *testing.T) {
	cases := []struct {
		name  string
		build func(t *testing.T, dir string)
	}{
		{
			name:  "an empty tree claims nothing",
			build: func(_ *testing.T, _ string) {},
		},
		{
			name: "interop floor and exact claims",
			build: func(t *testing.T, dir string) {
				for _, s := range []string{"a", "b", "c"} {
					write(t, dir, filepath.Join("test", "interop", "scenarios", s, "check.py"), "x\n")
				}
				write(t, dir, "docs/DESIGN.md", "# Design\n\n9+ interop scenarios run here.\n2 interop scenarios there.\n")
			},
		},
		{
			name: "README headline counts, bare and at-least",
			build: func(t *testing.T, dir string) {
				write(t, dir, "internal/z/z_test.go",
					"package z\n\nfunc FuzzA()\nfunc FuzzB()\nfunc FuzzC()\n"+
						"func TestA()\nfunc TestB()\nfunc TestC()\nfunc TestD()\nfunc TestE()\n")
				write(t, dir, "README.md",
					"# Ze\n\n5 fuzz targets in the tree\n3 fuzz targets in the tree\n"+
						"9+ fuzz targets in the tree\n1+ fuzz targets in the tree\n2 unit tests in the tree\n")
			},
		},
		{
			name: "the stale text parser claim",
			build: func(t *testing.T, dir string) {
				write(t, dir, "docs/architecture/api/text-parser.md",
					"# Text Parser Architecture\n\nAll functions allocate via `strings.Fields()`.\n")
			},
		},
		{
			name: "the feature inventory statuses",
			build: func(t *testing.T, dir string) {
				write(t, dir, "docs/features.md",
					"| Feature | Status | Description |\n|---|---|---|\n| a | supported | fine |\n| b | almost | no |\n")
			},
		},
		{
			name: "a features directory with no operator table",
			build: func(t *testing.T, dir string) {
				write(t, dir, "docs/features/formatting.md", "# Output Formatting\n")
			},
		},
		{
			name: "the suite derivation fails with no le to ask",
			build: func(t *testing.T, dir string) {
				write(t, dir, "Makefile", "ze-functional-test - Run ze functional tests (bgp, web)\n")
				write(t, dir, "docs/functional-tests.md", "The functional test target runs 2 suites: bgp, web.\n")
			},
		},
		{
			name: "a source file the scan cannot finish",
			build: func(t *testing.T, dir string) {
				write(t, dir, "internal/z/z_test.go",
					"package z\n\n// "+strings.Repeat("x", 70*1024)+"\nfunc TestA()\n")
				write(t, dir, "README.md", "# Ze\n\n1 unit tests in the tree\n")
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			tc.build(t, dir)

			scriptOut, scriptCode := runScript(t, "doc_drift", "--root", dir)
			commandOut, commandCode := runDriftCommand(dir)

			if scriptCode != commandCode {
				t.Errorf("the script exited %d and the command exited %d", scriptCode, commandCode)
			}
			if got, want := stripANSI(commandOut), stripANSI(scriptOut); got != want {
				t.Errorf("the reports differ:\nscript:\n%s\ncommand:\n%s", want, got)
			}
		})
	}
}

// TestDriftAgreesOverTheCheckout is AC-11 over the tree the gate actually
// judges. A fixture holds one claim; this holds every document the project
// publishes, and the registry a shipped binary carries.
func TestDriftAgreesOverTheCheckout(t *testing.T) {
	root := repoRoot(t)

	scriptOut, scriptCode := runScript(t, "doc_drift", "--root", root)
	commandOut, commandCode := runDriftCommand(root)

	if scriptCode != commandCode {
		t.Errorf("the script exited %d and the command exited %d\nscript:\n%s\ncommand:\n%s",
			scriptCode, commandCode, scriptOut, commandOut)
	}
	if got, want := stripANSI(commandOut), stripANSI(scriptOut); got != want {
		t.Errorf("the reports differ over the checkout:\nscript:\n%s\ncommand:\n%s", want, got)
	}
}

// TestContractScriptAndCommandAgree is AC-11 for the YANG/handler contract.
//
// The rows are compared as a SET rather than in order: the script's own table
// changes between runs (see TestScriptStillSortsOnOneKeyAlone), so an ordered
// comparison would fail against a port that is right. Everything else -- the
// counts, the verdict, every orphan section, every row -- is compared exactly.
func TestContractScriptAndCommandAgree(t *testing.T) {
	root := repoRoot(t)

	scriptOut, scriptCode := runScript(t, "commands")
	result, err := docvalid.Validate(root)
	if err != nil {
		t.Fatalf("the command failed over the checkout: %v", err)
	}
	commandCode := 0
	if !result.Valid {
		commandCode = 1
	}

	if scriptCode != commandCode {
		t.Errorf("the script exited %d and the command exited %d", scriptCode, commandCode)
	}
	if diff := lineSetDiff(scriptOut, result.Text()); diff != "" {
		t.Errorf("the reports hold different lines:\n%s", diff)
	}
	if result.Total == 0 {
		t.Fatal("the command found no YANG command, so this comparison proves nothing")
	}
}

// TestContractJSONAgrees compares the answer `make ze-command-contract-check-json`
// publishes, field by field, so a caller reading that JSON keeps reading it.
func TestContractJSONAgrees(t *testing.T) {
	scriptOut, _ := runScript(t, "commands", "--json")

	var fromScript docvalid.ValidationResult
	if err := json.Unmarshal([]byte(scriptOut), &fromScript); err != nil {
		t.Fatalf("the script did not answer JSON: %v\n%s", err, scriptOut)
	}

	fromCommand, err := docvalid.Validate(repoRoot(t))
	if err != nil {
		t.Fatalf("the command failed over the checkout: %v", err)
	}

	// The command's own JSON, read back through the same struct, so the
	// comparison covers the field NAMES as well as the values.
	raw, err := json.Marshal(fromCommand)
	if err != nil {
		t.Fatalf("marshal the command's answer: %v", err)
	}
	var roundTripped docvalid.ValidationResult
	if err := json.Unmarshal(raw, &roundTripped); err != nil {
		t.Fatalf("the command did not answer JSON: %v", err)
	}

	sortEntries(fromScript.YANGCommands)
	sortEntries(roundTripped.YANGCommands)

	if fromScript.Total != roundTripped.Total ||
		fromScript.TotalHandlers != roundTripped.TotalHandlers ||
		fromScript.TotalLocal != roundTripped.TotalLocal ||
		fromScript.Valid != roundTripped.Valid {
		t.Errorf("the counts differ: script %+v, command %+v",
			[]any{fromScript.Total, fromScript.TotalHandlers, fromScript.TotalLocal, fromScript.Valid},
			[]any{roundTripped.Total, roundTripped.TotalHandlers, roundTripped.TotalLocal, roundTripped.Valid})
	}
	if diff := lineSetDiff(entryLines(fromScript.YANGCommands), entryLines(roundTripped.YANGCommands)); diff != "" {
		t.Errorf("the command tables differ:\n%s", diff)
	}
	for _, pair := range []struct {
		name           string
		script, ported []string
	}{
		{"handlers", fromScript.Handlers, roundTripped.Handlers},
		{"local-handlers", fromScript.LocalHandlers, roundTripped.LocalHandlers},
		{"orphan-handlers", fromScript.OrphanHandlers, roundTripped.OrphanHandlers},
		{"orphan-local-handlers", fromScript.OrphanLocalHandlers, roundTripped.OrphanLocalHandlers},
		{"skipped-handlers", fromScript.SkippedHandlers, roundTripped.SkippedHandlers},
	} {
		if strings.Join(pair.script, "|") != strings.Join(pair.ported, "|") {
			t.Errorf("%s differ:\nscript: %v\ncommand: %v", pair.name, pair.script, pair.ported)
		}
	}
}

func sortEntries(entries []docvalid.CommandEntry) {
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].WireMethod != entries[j].WireMethod {
			return entries[i].WireMethod < entries[j].WireMethod
		}
		return entries[i].YANGPath < entries[j].YANGPath
	})
}

func entryLines(entries []docvalid.CommandEntry) string {
	var out strings.Builder
	for _, e := range entries {
		out.WriteString(e.WireMethod + " | " + e.YANGPath + " | " + e.Module + "\n")
	}
	return out.String()
}

// TestGeneratorWritesTheSameTree is AC-11 for the WRITING gate. An output
// comparison alone would pass a generator that printed the right line and wrote
// the wrong bytes, so the two resulting trees are compared file by file.
func TestGeneratorWritesTheSameTree(t *testing.T) {
	scriptTree, commandTree := t.TempDir(), t.TempDir()
	for _, tree := range []string{scriptTree, commandTree} {
		// The directory the table lives in must exist, and a stale table is
		// what a rerun of this gate normally overwrites.
		write(t, tree, "docs/features/pipe-operators.generated.md", "# stale\n")
	}

	scriptOut, scriptCode := runScript(t, "doc_drift", "--root", scriptTree, "--write-generated")
	written, err := docvalid.WriteGenerated(commandTree)
	if err != nil {
		t.Fatalf("the command failed to write: %v", err)
	}
	commandOut, commandCode := written.Text(), 0

	if scriptCode != commandCode {
		t.Errorf("the script exited %d and the command exited %d", scriptCode, commandCode)
	}
	if commandOut != scriptOut {
		t.Errorf("the reports differ:\nscript: %q\ncommand: %q", scriptOut, commandOut)
	}
	if diff := treeDiff(t, scriptTree, commandTree); diff != "" {
		t.Errorf("the two runs left different trees behind: %s", diff)
	}
	if body := readFile(t, filepath.Join(commandTree, "docs", "features", "pipe-operators.generated.md")); strings.Contains(body, "stale") {
		t.Error("the generator did not overwrite the stale table, so this case proves nothing")
	}
}

// TestPortReportsTheFileTheScriptCountsBlind is the one behavior the port does
// NOT preserve, asserted in the direction of the fix.
//
// The script's os.Open failure answers a zero count and no finding, so a
// document that understates the tree agrees with a scan that never happened. A
// dangling symbolic link reaches it without a permission change, so root cannot
// defeat the fixture.
func TestPortReportsTheFileTheScriptCountsBlind(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "internal/z/real_test.go", "package z\n\nfunc TestA()\n")
	if err := os.Symlink(filepath.Join(dir, "internal", "z", "no-such-file"),
		filepath.Join(dir, "internal", "z", "gone_test.go")); err != nil {
		t.Fatalf("create the dangling link: %v", err)
	}
	write(t, dir, "README.md", "# Ze\n\n1 unit tests in the tree\n")

	scriptOut, scriptCode := runScript(t, "doc_drift", "--root", dir)
	if scriptCode != 0 || !strings.Contains(stripANSI(scriptOut), "No documentation drift detected") {
		t.Fatalf("the script no longer counts blind here, so this test has nothing left to say: exit %d\n%s",
			scriptCode, scriptOut)
	}

	commandOut, commandCode := runDriftCommand(dir)
	if commandCode == 0 {
		t.Errorf("the command passed a tree holding a file it could not read:\n%s", commandOut)
	}
	if !strings.Contains(stripANSI(commandOut), "could not be read") {
		t.Errorf("the command did not say the file was unreadable:\n%s", commandOut)
	}
}

// TestScriptStillSortsOnOneKeyAlone pins the second defect the port fixed: the
// script's command table is not byte stable.
//
// sort.Slice is not stable, one wire method is reached from two YANG paths, and
// the rows arrive from a map walk, so two runs of the same gate over the same
// tree print the table in different orders. Measured 2026-08-26: five runs, four
// orders, six to eight lines apart. The assertion is on the SCRIPT's source, so
// it reddens the day somebody fixes the script -- and the answer then is to
// delete this file with the script it compares against.
func TestScriptStillSortsOnOneKeyAlone(t *testing.T) {
	source := readFile(t, filepath.Join(repoRoot(t), "scripts", "docvalid", "commands.go"))
	const oneKey = "return commands[i].WireMethod < commands[j].WireMethod\n\t})"
	if !strings.Contains(source, oneKey) {
		t.Fatal("the script no longer sorts on the wire method alone; delete this test with the script")
	}
}

// TestPortSortsTheTableTotally is the same fact from the other side: the port
// answers one order, whatever the map walk produced.
func TestPortSortsTheTableTotally(t *testing.T) {
	root := repoRoot(t)
	first, err := docvalid.Validate(root)
	if err != nil {
		t.Fatalf("validate: %v", err)
	}
	second, err := docvalid.Validate(root)
	if err != nil {
		t.Fatalf("validate a second time: %v", err)
	}
	if entryLines(first.YANGCommands) != entryLines(second.YANGCommands) {
		t.Error("the command answered two different tables over one unchanged tree")
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
	keys := make([]string, 0, len(counts))
	for line := range counts {
		keys = append(keys, line)
	}
	sort.Strings(keys)
	for _, line := range keys {
		switch {
		case counts[line] > 0:
			out.WriteString("  only the script: " + line + "\n")
		case counts[line] < 0:
			out.WriteString("  only the command: " + line + "\n")
		}
	}
	return out.String()
}

// treeDiff answers the files two trees disagree about, or "" when every file
// matches byte for byte.
func treeDiff(t *testing.T, left, right string) string {
	t.Helper()
	leftFiles, rightFiles := digest(t, left), digest(t, right)
	var out strings.Builder
	seen := map[string]bool{}
	for _, files := range []map[string]string{leftFiles, rightFiles} {
		for rel := range files {
			if seen[rel] {
				continue
			}
			seen[rel] = true
			if leftFiles[rel] != rightFiles[rel] {
				out.WriteString("  " + rel + "\n")
			}
		}
	}
	return out.String()
}

// digest answers every file of a tree, by path relative to it.
func digest(t *testing.T, root string) map[string]string {
	t.Helper()
	files := map[string]string{}
	err := filepath.WalkDir(root, func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr != nil {
			return relErr
		}
		files[rel] = readFile(t, path)
		return nil
	})
	if err != nil {
		t.Fatalf("walk %s: %v", root, err)
	}
	return files
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	body, err := os.ReadFile(path) //nolint:gosec // a path this test itself built
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(body)
}

// write writes one fixture file, creating the directories above it.
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
