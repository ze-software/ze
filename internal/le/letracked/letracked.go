// Design: docs/architecture/testing/tracked-build-gate.md -- checks whether le works from what GIT holds
//
// Package letracked checks whether `le` still works when built from the commit
// instead of the working tree.
//
// THE FAILURE THAT CAUSED THIS CHECK OCCURRED. On 2026-08-25, a clean archive
// of the committed development tools failed to load all 21 areas because their
// imports named uncommitted callees. Every forwarding Make target failed at
// HEAD while the working tree ran correctly.
//
// No individual commit was incorrect. Another session held a per-commit file,
// which delayed the commit ADDING the callee. The commit USING it had no such
// dependency. Each commit was verified against the working tree.
//
// WHAT A COMPILED le CHANGES. A Go `le` cannot have this import failure. A
// caller committed without its callee does not compile. In addition,
// ze-repository-tracked-build-check already compiles the complete committed tree
// in every shipped flavor (internal/le/trackedbuild). Thus, the exact Python
// failure cannot occur, but two other failures can occur:
//
//   - A tool package is committed and registers a command, but
//     internal/le/register.go does not blank-import it.
//   - A tool has an init() that panics or registers under an unexpected name. A
//     compile-only check cannot detect this because it occurs at startup.
//
// This gate builds the cmd/ze le personality from the commit, runs it, and
// compares the composition imports, registering packages, and help inventory.

package letracked

import (
	"bytes"
	"context"
	"errors"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	"github.com/ze-software/ze/internal/le/featuretags"
)

// leTree is the import-path prefix every le tool package sits under.
const leTree = "github.com/ze-software/ze/internal/le/"

// registerPath is the sole composition root, relative to the checkout.
const registerPath = "internal/le/register.go"

// toolsDir is where the tool packages live, relative to the checkout.
const toolsDir = "internal/le"

// registerCall is the source text that makes a package a tool.
// Each call joins the shared local-data registry.
const registerCall = "leroot.Register("

// DefaultDeadline bounds the whole run. A wedged `git archive` or `go build`
// would otherwise stall a post-commit check with no limit at all.
const DefaultDeadline = 10 * time.Minute

// commandsHeading is the section of le's help page that lists its own root
// commands. dispatch.go prints only the commands le OWNS under it, so the
// binary's answer needs no filtering here.
const commandsHeading = "Commands:"

// Broken is one finding: a package, and what is wrong with it.
type Broken struct {
	Package string `json:"package"`
	Detail  string `json:"detail"`
}

// Verdict is what the committed tree said.
type Verdict struct {
	Rev    string `json:"rev"`
	Commit string `json:"commit,omitempty"`
	// Areas is how many tool packages the commit wires into le.
	Areas int `json:"areas"`
	// Commands is what the binary built from the commit actually offers.
	Commands []string `json:"commands,omitempty"`
	// Broken is the row set. One key holds rows, so the row operators act on
	// the findings.
	Broken []Broken `json:"broken,omitempty"`
	OK     bool     `json:"ok"`
}

// Text renders the verdict for a person, in the shape the script printed.
func (v Verdict) Text() string {
	var tb textbuf.Buffer
	tb.Str("==> loading every le area from ").Str(v.Rev).Byte('\n')

	if v.OK {
		return tb.Str("  ").Int(int64(v.Areas)).Str(" area(s) load from ").Str(v.Rev).Byte('\n').String()
	}

	for _, one := range v.Broken {
		tb.Str("  BROKEN  ").Str(one.Package).Str(": ").Str(one.Detail).Byte('\n')
	}
	tb.Byte('\n')
	tb.Int(int64(len(v.Broken))).Str(" of ").Int(int64(v.Areas)).
		Str(" area(s) do not load from ").Str(v.Rev).Str(".\n")
	tb.Str("The working tree is not the tree anybody else gets. A file present here\n")
	return tb.Str("and absent from the commit is exactly what this finds.\n").String()
}

// toolImports returns the sorted short names of the tool packages that the
// composition root blank-imports. Named imports support the root handler and
// are not composition entries.
func toolImports(src []byte) ([]string, error) {
	file, err := parser.ParseFile(token.NewFileSet(), registerPath, src, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}

	names := make([]string, 0, len(file.Imports))
	for _, spec := range file.Imports {
		if spec.Name == nil || spec.Name.Name != "_" {
			continue
		}
		path, unquoteErr := strconv.Unquote(spec.Path.Value)
		if unquoteErr != nil {
			return nil, unquoteErr
		}
		if !strings.HasPrefix(path, leTree) {
			var tb textbuf.Buffer
			return nil, errors.New(tb.Str(registerPath).Str(" imports ").Str(path).
				Str(", which is not an le tool package").String())
		}
		names = append(names, strings.TrimPrefix(path, leTree))
	}
	sort.Strings(names)
	return names, nil
}

// registeringPackages returns the sorted short names of all tool packages under
// root that join the shared local-data registry.
//
// The test checks for the CALL, not a naming convention. A package gets a
// command by creating it. A directory that contains only helpers is not a tool.
func registeringPackages(root string) ([]string, error) {
	base := filepath.Join(root, toolsDir)
	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, err
	}

	var names []string
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		registers, checkErr := packageRegisters(filepath.Join(base, entry.Name()))
		if checkErr != nil {
			return nil, checkErr
		}
		if registers {
			names = append(names, entry.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// packageRegisters reports whether any non-test file of one package calls
// leroot.Register.
func packageRegisters(dir string) (bool, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false, err
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		raw, readErr := os.ReadFile(filepath.Join(dir, name)) //nolint:gosec // a build tool reads the tree it was pointed at
		if readErr != nil {
			return false, readErr
		}
		if bytes.Contains(raw, []byte(registerCall)) {
			return true, nil
		}
	}
	return false, nil
}

// commandNames returns the root commands in the order in which the help page
// lists them.
//
// The parser reads the page from the "Commands:" heading to the first blank line
// after it. The usage line above has the same indentation, but it has a separate
// heading. Therefore, the parser does not identify that line as a command.
func commandNames(page string) []string {
	var names []string
	inside := false

	for line := range strings.SplitSeq(page, "\n") {
		if strings.TrimSpace(line) == commandsHeading {
			inside = true
			continue
		}
		if !inside {
			continue
		}
		if strings.TrimSpace(line) == "" {
			break
		}
		if fields := strings.Fields(line); len(fields) > 0 {
			names = append(names, fields[0])
		}
	}
	return names
}

// Compare returns all disagreements between three lists. The lists contain
// imported packages, registered packages, and commands that the built binary
// offers.
//
// An EMPTY commit is a finding, not a clean result. If a tree wires nothing,
// each comparison below has nothing to compare and would report no difference.
// This check must report that condition as breakage.
func Compare(imported, registering, offered []string) []Broken {
	if len(imported) == 0 && len(registering) == 0 {
		return []Broken{{
			Package: registerPath,
			Detail:  "the commit wires no tool into le at all",
		}}
	}

	importedSet := setOf(imported)
	registeringSet := setOf(registering)

	// wired contains the packages expected to produce a command. Each package is
	// imported by the composition root AND registers a command. The check counts
	// against this population instead of the import list. This gives one finding
	// for each disagreement and does not also report an unwired tool as a missing
	// command.
	wired := 0
	for _, name := range imported {
		if registeringSet[name] {
			wired++
		}
	}

	var broken []Broken
	for _, name := range registering {
		if !importedSet[name] {
			broken = append(broken, Broken{
				Package: name,
				Detail:  "registers a command and internal/le/register.go does not import it",
			})
		}
	}
	for _, name := range imported {
		if !registeringSet[name] {
			broken = append(broken, Broken{
				Package: name,
				Detail:  "is imported by internal/le/register.go and registers no command",
			})
		}
	}

	// Compare the binary's result with the wiring that produced it. Use a count
	// instead of a name comparison. A tool's package name and command name are
	// different words. Without a third table, only the count can be derived from
	// both sides, and no one maintains such a table.
	if len(offered) != wired {
		var tb textbuf.Buffer
		broken = append(broken, Broken{
			Package: "le",
			Detail: tb.Str("offers ").Int(int64(len(offered))).Str(" command(s) and ").
				Str(registerPath).Str(" wires ").Int(int64(wired)).Str(" tool package(s)").String(),
		})
	}
	return broken
}

// setOf answers a membership test over a name list.
func setOf(names []string) map[string]bool {
	set := make(map[string]bool, len(names))
	for _, name := range names {
		set[name] = true
	}
	return set
}

// Run judges one commit and returns the verdict and exit code.
//
// The three codes have separate meanings. Code 0 means le works for the commit.
// Code 1 means it does not work. Code 2 means the run failed to judge the
// commit. A killed build and a broken commit require investigation of different
// causes.
func Run(ctx context.Context, repo, rev string) (Verdict, int, error) {
	verdict := Verdict{Rev: rev}

	commit, err := resolveRev(ctx, repo, rev)
	if err != nil {
		return verdict, 2, err
	}
	verdict.Commit = commit

	dest, err := os.MkdirTemp("", "le-tracked-")
	if err != nil {
		return verdict, 2, err
	}
	defer os.RemoveAll(dest) //nolint:errcheck // a scratch tree, removed on every exit path

	if err := extract(ctx, repo, commit, dest); err != nil {
		return verdict, 2, err
	}

	src, err := os.ReadFile(filepath.Join(dest, registerPath)) //nolint:gosec // the extracted tree, at a fixed path
	if err != nil {
		var tb textbuf.Buffer
		return verdict, 2, errors.New(tb.Str(rev).Str(" holds no ").Str(registerPath).String())
	}
	imported, err := toolImports(src)
	if err != nil {
		return verdict, 2, err
	}
	verdict.Areas = len(imported)

	registering, err := registeringPackages(dest)
	if err != nil {
		return verdict, 2, err
	}

	offered, err := buildAndList(ctx, dest)
	if err != nil {
		return verdict, 2, err
	}
	verdict.Commands = offered

	verdict.Broken = Compare(imported, registering, offered)
	if len(verdict.Broken) > 0 {
		return verdict, 1, nil
	}
	verdict.OK = true
	return verdict, 0, nil
}

// resolveRev turns a commit-ish into the sha the run is judging, so the verdict
// names a commit rather than a moving name.
func resolveRev(ctx context.Context, repo, rev string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", "rev-parse", rev) //nolint:gosec // rev is a commit-ish the operator named; git resolves it or refuses
	cmd.Dir = repo
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// extract materializes the commit into dest.
//
// `git archive` rather than a worktree: it reads the object store and writes
// nothing into .git/. A worktree registration outlives a killed run and then
// needs pruning by hand, and several sessions share this checkout.
func extract(ctx context.Context, repo, commit, dest string) error {
	archive := exec.CommandContext(ctx, "git", "archive", "--format=tar", commit) //nolint:gosec // commit is the sha rev-parse answered
	archive.Dir = repo

	var tarball bytes.Buffer
	var complaint bytes.Buffer
	archive.Stdout = &tarball
	archive.Stderr = &complaint
	if err := archive.Run(); err != nil {
		var tb textbuf.Buffer
		return errors.New(tb.Str("git archive: ").Str(strings.TrimSpace(complaint.String())).String())
	}

	untar := exec.CommandContext(ctx, "tar", "-x", "-C", dest) //nolint:gosec // dest is the scratch tree this run created
	untar.Stdin = &tarball
	untar.Stderr = &complaint
	if err := untar.Run(); err != nil {
		var tb textbuf.Buffer
		return errors.New(tb.Str("tar: ").Str(strings.TrimSpace(complaint.String())).String())
	}
	return nil
}

// buildAndList builds the cmd/ze le personality from the extracted tree, runs
// it, and returns the commands that it offers.
//
// The binary must run. A tool can have an init() that panics or can register
// under an unexpected name. It can compile successfully but fail or be absent
// at startup. A check that only compiles the tree cannot detect either problem.
func buildAndList(ctx context.Context, dest string) ([]string, error) {
	probeDir, err := os.MkdirTemp(dest, ".le-tracked-probe-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(probeDir) //nolint:errcheck // the extracted tree is scratch space

	binary := filepath.Join(probeDir, "le")

	tags, err := featuretags.DaemonBuildTags(dest, "ze_le")
	if err != nil {
		return nil, err
	}
	build := exec.CommandContext(ctx, "go", "build", "-tags", tags, "-o", binary, "./cmd/ze") //nolint:gosec // binary is inside the scratch tree
	build.Dir = dest
	build.Env = append(os.Environ(), "GOFLAGS=-mod=vendor", "CGO_ENABLED=0")

	var complaint bytes.Buffer
	build.Stderr = &complaint
	if err := build.Run(); err != nil {
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str("go build ./cmd/ze le personality: ").Str(strings.TrimSpace(complaint.String())).String())
	}

	// `help` writes the page to stderr and exits 0, which is what makes it the
	// cheapest proof that every init() ran.
	page := exec.CommandContext(ctx, binary, "help") //nolint:gosec // the binary this run just built, at a path it chose
	page.Dir = dest

	var printed bytes.Buffer
	page.Stdout = &printed
	page.Stderr = &printed
	if err := page.Run(); err != nil {
		var tb textbuf.Buffer
		return nil, errors.New(tb.Str("le help: ").Err(err).Str(": ").
			Str(strings.TrimSpace(printed.String())).String())
	}
	return commandNames(printed.String()), nil
}
