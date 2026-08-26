// The migration's proof for the Python tools of this directory: the script and
// the command answer the same thing.
//
// The scripts under scripts/dev are being replaced by packages under letools/,
// and the two live side by side until the swap. This file is deliberately HERE
// rather than beside the new packages: it is a migration artifact, so the
// commit that deletes the scripts deletes their proof with them.
//
// VALIDATES: spec-le-is-a-ze-binary AC-11 -- over this checkout and over
// fixture trees, each ported script and its command answer the same exit code
// and the same text, and a writing tool leaves the same tree behind.
// PREVENTS: a port that agrees with its script on a fixture and disagrees on
// the checkout. Each of these is a GATE, so a check that quietly stopped firing
// would pass every unit test and change what the repository is told about
// itself.
//
// It also pins the fail-open defects the ports FIXED and the scripts still
// carry. Such a case asserts the SCRIPT still fails open, so it reddens the day
// somebody repairs the script, and the answer then is to delete the case with
// the script it describes.
//
// The file name is parity_python_test.go rather than parity_test.go because the
// Go half of scripts/dev is being ported by a separate step into the same
// package, and two files of one name cannot both exist. Every helper here
// carries a devPy prefix for the same reason.

package main

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ze-software/ze/internal/core/env"
	"github.com/ze-software/ze/letools/archmap"
	"github.com/ze-software/ze/letools/gokrazygosum"
	"github.com/ze-software/ze/letools/leroot"
	"github.com/ze-software/ze/letools/protocolskeleton"
	"github.com/ze-software/ze/letools/workingtree"
)

// devPyTimeout bounds one script run. Every tool compared here reads a tree and
// prints a verdict, which is seconds at most, so a run past this is a hung
// process rather than a slow one.
const devPyTimeout = 120 * time.Second

// devPyResult is what one half of a pair answered.
type devPyResult struct {
	Stdout string
	Stderr string
	Code   int
}

// devPyRunScript runs one scripts/dev Python tool the way its gate runs it, in
// the working directory given, and answers what it printed and exited with.
func devPyRunScript(t *testing.T, script string, args []string, cwd string) devPyResult {
	t.Helper()

	ctx, cancel := context.WithTimeout(t.Context(), devPyTimeout)
	defer cancel()

	argv := append([]string{filepath.Join(devPyRoot(t), "scripts", "dev", script)}, args...)
	cmd := exec.CommandContext(ctx, "python3", argv...) // #nosec G204 -- a tracked script path and a test's own arguments
	cmd.Dir = cwd
	var out, errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut
	err := cmd.Run()

	// An ExitError is the tool's own verdict and is what this file compares.
	// Anything else means the run never happened.
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		t.Fatalf("running %s: %v: %s", script, err, errOut.String())
	}
	return devPyResult{Stdout: out.String(), Stderr: errOut.String(), Code: cmd.ProcessState.ExitCode()}
}

// devPyRunCommand runs one le command through the same path the binary runs it
// through: leroot.Run splits the pipe chain, calls the tool and renders the
// payload. Nothing here reimplements the rendering the comparison is about.
func devPyRunCommand(t *testing.T, name string, answer leroot.Answer, args []string) devPyResult {
	t.Helper()

	var out, errOut bytes.Buffer
	code := leroot.Run(name, answer, args, &out, &errOut)
	return devPyResult{Stdout: out.String(), Stderr: errOut.String(), Code: code}
}

// devPyRoot answers this checkout's root, which is the tree both halves judge
// when no fixture is set, and where the scripts themselves live.
//
// It walks up from the test's own working directory rather than calling
// lepath.Root, deliberately: a fixture case points ZE_REPO_ROOT at a temporary
// tree, and lepath.Root would then answer that tree and the scripts would be
// looked for inside it.
func devPyRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("locating the checkout: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "feature-gates.txt")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("no checkout root above %s", dir)
		}
		dir = parent
	}
}

// devPyTree writes a fixture checkout and tracks every file in it. Both halves
// need a real repository: the script asks git what is tracked, and so does the
// command.
func devPyTree(t *testing.T, files map[string]string) string {
	t.Helper()

	root := t.TempDir()
	for rel, body := range files {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatalf("fixture directory: %v", err)
		}
		if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
			t.Fatalf("fixture file: %v", err)
		}
	}
	for _, argv := range [][]string{{"init", "--quiet"}, {"add", "--all"}} {
		cmd := exec.CommandContext(t.Context(), "git", append([]string{"-C", root}, argv...)...)
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v: %s", argv[0], err, out)
		}
	}
	return root
}

// devPyPointAt makes both halves judge the fixture tree. The command reads
// ZE_REPO_ROOT through env.Get, which caches os.Environ() once per process, so
// the cache is reset here and again when the test ends.
func devPyPointAt(t *testing.T, tree string) {
	t.Helper()

	t.Setenv("ZE_REPO_ROOT", tree)
	env.ResetCache()
	t.Cleanup(env.ResetCache)
}

// devPyAgree fails unless the two halves answered the same exit code and the
// same text. The stream a verdict lands on is compared through the fields the
// caller passes, because a port moves a verdict from stderr to stdout: the
// payload is what `| json` renders, and only a genuine failure reaches stderr.
func devPyAgree(t *testing.T, what string, script, command devPyResult, scriptText, commandText string) {
	t.Helper()

	if script.Code != command.Code {
		t.Errorf("%s: the script exited %d and the command exited %d\nscript:\n%s%s\ncommand:\n%s%s",
			what, script.Code, command.Code, script.Stdout, script.Stderr, command.Stdout, command.Stderr)
	}
	if scriptText != commandText {
		t.Errorf("%s: the two halves print different text\nscript:\n%s\ncommand:\n%s", what, scriptText, commandText)
	}
}

// gosumScript is the tool this group compares.
const gosumScript = "gokrazy_gosum_check.py"

func TestGokrazyGosumBothHalvesAgreeOverTheCheckout(t *testing.T) {
	root := devPyRoot(t)

	script := devPyRunScript(t, gosumScript, nil, root)
	command := devPyRunCommand(t, "gokrazy-gosum", gokrazygosum.Answer, nil)

	devPyAgree(t, "gokrazy-gosum over the checkout", script, command, script.Stdout, command.Stdout)
	if command.Stderr != "" {
		t.Errorf("the command wrote to stderr over a clean checkout: %q", command.Stderr)
	}
	if !strings.Contains(command.Stdout, "builddir go.sum file(s)") {
		t.Errorf("the command checked nothing over the real checkout: %q", command.Stdout)
	}
}

func TestGokrazyGosumBothHalvesReportTheSameConflict(t *testing.T) {
	const (
		rootHash     = "h1:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA="
		builddirHash = "h1:BBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBBB="
	)
	tree := devPyTree(t, map[string]string{
		"go.sum":            "example.com/x v1.0.0 " + rootHash + "\n",
		"go.mod":            "module example.com/fixture\n",
		"feature-gates.txt": "ze_core\n",
		"gokrazy/ze/builddir/example.com/y/go.sum": "example.com/x v1.0.0 " + builddirHash + "\n",
	})
	devPyPointAt(t, tree)

	script := devPyRunScript(t, gosumScript, nil, tree)
	command := devPyRunCommand(t, "gokrazy-gosum", gokrazygosum.Answer, nil)

	// The script writes its conflict list to stderr and the command writes it
	// to stdout: a verdict is the payload in a command, so it reaches the
	// stream `| json` can carry, and only a genuine failure reaches stderr.
	// That is the one deliberate difference, and the text itself is compared
	// byte for byte.
	devPyAgree(t, "gokrazy-gosum over a conflicting tree", script, command, script.Stderr, command.Stdout)
	if script.Code != 1 {
		t.Errorf("the script exited %d over a conflicting tree, want 1", script.Code)
	}
}

func TestGokrazyGosumScriptStillPassesFromASubdirectory(t *testing.T) {
	root := devPyRoot(t)

	// The script reads `git -C . ls-files` and opens `go.sum`, both relative to
	// the working directory. From a subdirectory git answers only that
	// subdirectory's paths, none of which start with the builddir prefix, so
	// the gate reports that it has nothing to check and exits 0 over a tree it
	// never read.
	script := devPyRunScript(t, gosumScript, nil, filepath.Join(root, "scripts"))
	if script.Code != 0 || !strings.Contains(script.Stdout, "nothing to check") {
		t.Fatalf("the script no longer fails open from a subdirectory (exit %d): %s%s\n"+
			"If it was repaired, delete this case with the script and close the row in\n"+
			"plan/journal/gate-excludes-part-of-its-population.md",
			script.Code, script.Stdout, script.Stderr)
	}

	// The command takes the tree from lepath.Root rather than from the working
	// directory, so the same run judges the checkout.
	command := devPyRunCommand(t, "gokrazy-gosum", gokrazygosum.Answer, nil)
	if !strings.Contains(command.Stdout, "builddir go.sum file(s)") {
		t.Errorf("the command checked nothing from a subdirectory: %q", command.Stdout)
	}
}

// workingTreeScript is the tool the cases below compare.
const workingTreeScript = "working_tree_check.py"

// devPyDirty writes untracked files into a fixture repository and points both
// halves at it. Untracked is enough: `--untracked-files=all` reports them, and
// a fixture that had to commit first would need an identity this test has no
// business setting.
func devPyDirty(t *testing.T, paths ...string) string {
	t.Helper()

	tree := devPyTree(t, nil)
	for _, rel := range paths {
		full := filepath.Join(tree, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(full), 0o750); err != nil {
			t.Fatalf("fixture directory: %v", err)
		}
		if err := os.WriteFile(full, []byte("x\n"), 0o600); err != nil {
			t.Fatalf("fixture file: %v", err)
		}
	}
	devPyPointAt(t, tree)
	return tree
}

// The comparison for this tool is over FIXTURE trees rather than over the
// checkout, and that is not a shortcut. Its whole answer is what the working
// tree holds, this checkout is shared by several sessions at once, and a file
// written between the two runs would make the halves disagree about a tree
// neither of them got wrong.
func TestWorkingTreeBothHalvesAgreeOverACleanTree(t *testing.T) {
	tree := devPyDirty(t)

	script := devPyRunScript(t, workingTreeScript, nil, tree)
	command := devPyRunCommand(t, "working-tree", workingtree.Answer, nil)

	devPyAgree(t, "working-tree over a clean tree", script, command, script.Stdout, command.Stdout)
	if strings.TrimSpace(command.Stdout) != "working tree: clean" {
		t.Errorf("a clean tree rendered %q", command.Stdout)
	}
}

func TestWorkingTreeBothHalvesGroupOneAreaTheSameWay(t *testing.T) {
	tree := devPyDirty(t,
		"docs/a.md", "docs/b.md", "docs/c.md", "docs/d.md", "docs/e.md", "docs/f.md")

	script := devPyRunScript(t, workingTreeScript, nil, tree)
	command := devPyRunCommand(t, "working-tree", workingtree.Answer, nil)

	devPyAgree(t, "working-tree over one area", script, command, script.Stdout, command.Stdout)
	if !strings.Contains(command.Stdout, "+2 more") {
		t.Errorf("the sixth file is not accounted for: %q", command.Stdout)
	}
}

func TestWorkingTreeBothHalvesOrderAndAdviseTheSameWay(t *testing.T) {
	tree := devPyDirty(t,
		"docs/a.md", "docs/b.md",
		"ai/rules/x.md", "ai/rules/y.md", "ai/rules/z.md",
		"cmd/le/main.go",
		"Makefile")

	script := devPyRunScript(t, workingTreeScript, nil, tree)
	command := devPyRunCommand(t, "working-tree", workingtree.Answer, nil)

	devPyAgree(t, "working-tree over four areas", script, command, script.Stdout, command.Stdout)
	if !strings.Contains(command.Stdout, "More than one area is in flight") {
		t.Errorf("a four-area tree got no advice: %q", command.Stdout)
	}
}

func TestWorkingTreeBothHalvesFailAtTheSameCeiling(t *testing.T) {
	tree := devPyDirty(t, "docs/a.md", "cmd/le/main.go", "Makefile")

	script := devPyRunScript(t, workingTreeScript, []string{"--max-areas", "2"}, tree)
	command := devPyRunCommand(t, "working-tree", workingtree.Answer, []string{"max-areas", "2"})

	// The one deliberate difference is how the verdict names the ceiling: the
	// script names its own flag, --max-areas, and the command names the
	// keyword a developer types. Everything else is compared byte for byte.
	devPyAgree(t, "working-tree under a ceiling", script, command,
		strings.Replace(script.Stdout, "--max-areas", "max-areas", 1), command.Stdout)
	if script.Code != 1 {
		t.Errorf("the script exited %d past its ceiling, want 1", script.Code)
	}
}

func TestWorkingTreeBothHalvesPassAtTheCeiling(t *testing.T) {
	tree := devPyDirty(t, "docs/a.md", "cmd/le/main.go")

	script := devPyRunScript(t, workingTreeScript, []string{"--max-areas", "2"}, tree)
	command := devPyRunCommand(t, "working-tree", workingtree.Answer, []string{"max-areas", "2"})

	devPyAgree(t, "working-tree at its ceiling", script, command, script.Stdout, command.Stdout)
	if command.Code != 0 {
		t.Errorf("two areas under a ceiling of two exited %d", command.Code)
	}
}

// archMapScript is the tool the cases below compare, and archMapFile is the one
// file it rewrites.
const (
	archMapScript = "arch_map.py"
	archMapFile   = "ai/INSTRUCTIONS.md"
)

// devPyArchTree writes a fixture checkout holding the three source directories
// and an instructions file with the marker pairs, and answers its root. Two
// calls answer two trees with identical content, which is what lets a WRITING
// tool be compared by the bytes it leaves behind.
func devPyArchTree(t *testing.T) string {
	t.Helper()

	var page strings.Builder
	page.WriteString("prose the generator must not touch\n")
	for _, name := range []string{"components", "system-plugins", "bgp-plugins"} {
		page.WriteString("<!-- BEGIN GENERATED: arch-" + name + " (scripts/dev/arch_map.py) -->\n")
		page.WriteString("stale content\n")
		page.WriteString("<!-- END GENERATED: arch-" + name + " -->\n")
	}

	// A file inside each directory is what makes the directory exist: git does
	// not track an empty one, and the fixture must survive being asked for by
	// both halves.
	return devPyTree(t, map[string]string{
		archMapFile: page.String(),
		// The names are deliberately long and hyphenated, so the wrap the two
		// halves must agree about actually happens.
		"internal/component/config-archive-cmd/doc.go":               "package x\n",
		"internal/component/traffic-feature-long/doc.go":             "package x\n",
		"internal/component/bgp/doc.go":                              "package x\n",
		"internal/component/bgp/plugins/filter_aspath_length/doc.go": "package x\n",
		"internal/component/bgp/plugins/redistribute_ingress/doc.go": "package x\n",
		"internal/plugins/flowexport-cmd/doc.go":                     "package x\n",
		"internal/plugins/config-schema/doc.go":                      "package x\n",
	})
}

func TestArchMapBothHalvesAgreeOverTheCheckout(t *testing.T) {
	root := devPyRoot(t)

	script := devPyRunScript(t, archMapScript, []string{"--check"}, root)
	command := devPyRunCommand(t, "arch-map", archmap.Answer, []string{"check"})

	devPyAgree(t, "arch-map check over the checkout", script, command, script.Stdout, command.Stdout)
}

func TestArchMapBothHalvesSeeTheSameStaleFile(t *testing.T) {
	tree := devPyArchTree(t)
	devPyPointAt(t, tree)

	script := devPyRunScript(t, archMapScript, []string{"--check"}, tree)
	command := devPyRunCommand(t, "arch-map", archmap.Answer, []string{"check"})

	// The script writes the staleness verdict to stderr and the command writes
	// it to stdout: a verdict is the payload in a command, so it reaches the
	// stream `| json` can carry. The text is compared byte for byte.
	devPyAgree(t, "arch-map check over a stale tree", script, command, script.Stderr, command.Stdout)
	if script.Code != 1 {
		t.Errorf("the script exited %d over a stale file, want 1", script.Code)
	}
}

// A generator has one more thing to agree about than its page: the BYTES it
// writes. Two halves that print the same verdict and emit different files would
// leave the generated-files check red for whoever ran the other one.
func TestArchMapBothHalvesWriteTheSameFile(t *testing.T) {
	byScript := devPyArchTree(t)
	byCommand := devPyArchTree(t)
	devPyPointAt(t, byCommand)

	script := devPyRunScript(t, archMapScript, nil, byScript)
	command := devPyRunCommand(t, "arch-map", archmap.Answer, []string{"update"})

	devPyAgree(t, "arch-map update", script, command, script.Stdout, command.Stdout)

	written := devPyRead(t, filepath.Join(byScript, filepath.FromSlash(archMapFile)))
	ported := devPyRead(t, filepath.Join(byCommand, filepath.FromSlash(archMapFile)))
	if written != ported {
		t.Errorf("the two halves wrote different files\nscript:\n%s\ncommand:\n%s", written, ported)
	}
	if !strings.Contains(ported, "config-archive-cmd") {
		t.Errorf("the written file carries no directory list:\n%s", ported)
	}
	if strings.Contains(ported, "stale content") {
		t.Error("the write left the old block behind")
	}
	if !strings.Contains(ported, "prose the generator must not touch") {
		t.Error("the write dropped the prose outside the markers")
	}
}

// protocolSkeletonScript is the tool the cases below compare.
const protocolSkeletonScript = "protocol_skeleton_report.py"

// Both halves judge THIS checkout, and neither can be pointed elsewhere: the
// script derives its root from __file__, and the manifest it reads names the
// real protocol directories. So the comparison is over the tree they share.
func TestProtocolSkeletonBothHalvesAgreeOverTheCheckout(t *testing.T) {
	root := devPyRoot(t)

	script := devPyRunScript(t, protocolSkeletonScript, nil, root)
	command := devPyRunCommand(t, "protocol-skeleton", protocolskeleton.Answer, []string{"report"})

	// The one deliberate difference is where the summary sends a reader for
	// the detail. The script has a --verbose flag; the command has one answer
	// and several renderings, so the detail is in the payload and `| json` is
	// what --verbose was.
	devPyAgree(t, "protocol-skeleton report", script, command,
		strings.Replace(script.Stdout, "--verbose for detail", "| json for detail", 1), command.Stdout)
	if command.Code != 0 {
		t.Errorf("the advisory exited %d, and report mode never fails", command.Code)
	}
}

func TestProtocolSkeletonBothSelftestsPass(t *testing.T) {
	root := devPyRoot(t)

	script := devPyRunScript(t, protocolSkeletonScript, []string{"--selftest"}, root)
	command := devPyRunCommand(t, "protocol-skeleton", protocolskeleton.Answer, []string{"selftest"})

	devPyAgree(t, "protocol-skeleton selftest", script, command, script.Stdout, command.Stdout)
	if command.Code != 0 {
		t.Errorf("the selftest exited %d", command.Code)
	}
}

// devPyRead answers a file's whole content, or fails the test.
func devPyRead(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path) // #nosec G304 -- a path this test wrote
	if err != nil {
		t.Fatalf("reading %s: %v", path, err)
	}
	return string(data)
}
