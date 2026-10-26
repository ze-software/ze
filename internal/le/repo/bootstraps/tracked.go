// Design: docs/architecture/testing/tracked-build-gate.md -- checks whether le works from what GIT holds
//
// Package repobootstraps checks whether `le` still works when built from the commit
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
// `le repo compiles` already compiles the complete committed tree
// in every shipped flavor (internal/le/repo/compiles). Thus, the exact Python
// failure cannot occur, but two other failures can occur:
//
//   - A tool package is committed and registers a command, but
//     internal/le/register.go does not import it. A blank import and a named
//     one both run the package's init(), so either one wires the tool.
//   - A tool has an init() that panics or registers under an unexpected name. A
//     compile-only check cannot detect this because it occurs at startup.
//
// This gate builds the cmd/ze le personality from the commit, runs it, and
// compares the composition imports, registering packages, and help inventory.

package repobootstraps

import (
	"bytes"
	"context"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/ze-software/ze/internal/core/textbuf"
	leroot "github.com/ze-software/ze/internal/le/le/root"
	repofeaturetags "github.com/ze-software/ze/internal/le/repo/featuretags"
)

// leTree is the import-path prefix every le tool package sits under.
const leTree = "github.com/ze-software/ze/internal/le/"

// registerPath is the sole composition root, relative to the checkout.
const registerPath = "internal/le/register.go"

// toolsDir is where the tool packages live, relative to the checkout.
const toolsDir = "internal/le"

// leRootPath is the package whose calls put a package on le's root page.
const leRootPath = leTree + "le/root"

// leRootName is the name leRootPath declares in its package clause, so it is
// what a file that imports the package without renaming it calls it by.
const leRootName = "leroot"

// The three leroot calls this check reads. leroot.Register joins the shared
// local-data registry as a tool, and the root page gives that tool one row.
// leroot.RegisterNamespace declares a namespace (`le ai`, `le go`): the page
// prints it as a heading over its members' rows, and a heading is no row.
// leroot.RegisterHidden marks the package's tool as one Ze spawns and a person
// never types (`le mock engine-steps`): it is wired, and the page gives it no
// row.
const (
	registerToolCall      = "Register"
	registerNamespaceCall = "RegisterNamespace"
	registerHiddenCall    = "RegisterHidden"
)

// DefaultDeadline bounds the whole run. A wedged `git archive` or `go build`
// would otherwise stall a post-commit check with no limit at all.
const DefaultDeadline = 10 * time.Minute

// usageTitle is the title of the usage block that opens le's root help page.
// That block shows how to call le and names no command, so listsCommands
// leaves it out of the sections whose rows it counts. dispatch.go prints only
// the commands le OWNS under the command sections, so the binary's answer needs
// no filtering here.
const usageTitle = "Usage"

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

// rootImports is what the composition root imports from the le tree, split by
// the form of the import.
type rootImports struct {
	// blank holds the composition entries. Each one exists only to run its
	// package's init(), so each MUST register a command.
	blank []string
	// named holds the packages the root's own code calls (leroot, lepath,
	// scratch). A named import runs the package's init() exactly as a blank
	// one does, so a named package that registers a command is wired too.
	named []string
}

// toolImports returns the sorted short names of the le packages that the
// composition root imports, blank and named apart. A named import outside the
// le tree (os, strings) supports the root handler and is no tool.
func toolImports(src []byte) (rootImports, error) {
	file, err := parser.ParseFile(token.NewFileSet(), registerPath, src, parser.ImportsOnly)
	if err != nil {
		return rootImports{}, err
	}

	var imports rootImports
	for _, spec := range file.Imports {
		path, unquoteErr := strconv.Unquote(spec.Path.Value)
		if unquoteErr != nil {
			return rootImports{}, unquoteErr
		}
		blank := spec.Name != nil && spec.Name.Name == "_"
		inTree := strings.HasPrefix(path, leTree)
		if blank && !inTree {
			var tb textbuf.Buffer
			return rootImports{}, errors.New(tb.Str(registerPath).Str(" imports ").Str(path).
				Str(", which is not an le tool package").String())
		}
		if !inTree {
			continue
		}
		name := strings.TrimPrefix(path, leTree)
		if blank {
			imports.blank = append(imports.blank, name)
			continue
		}
		imports.named = append(imports.named, name)
	}
	slices.Sort(imports.blank)
	slices.Sort(imports.named)
	return imports, nil
}

// wired returns the sorted short names of the packages the composition root
// links as composition entries: every blank import, and every named import
// whose package registers a tool or declares a namespace. A named support
// package that declares nothing (leroot, lepath) is not a composition entry
// and is left out.
func (r rootImports) wired(declared []string) []string {
	declaredSet := setOf(declared)
	names := slices.Clone(r.blank)
	for _, name := range r.named {
		if declaredSet[name] {
			names = append(names, name)
		}
	}
	slices.Sort(names)
	return names
}

// registrations is what the packages under internal/le declare, split by the
// row each one gives the root page. A package that declares a namespace and
// also registers a tool is a tool: its tool gives the page a row.
type registrations struct {
	// tools are the sorted short names of the packages that register a tool.
	tools []string
	// namespaces are the sorted short names of the packages that declare a
	// namespace and register no tool.
	namespaces []string
	// hidden are the sorted short names of the tools (a subset of tools) whose
	// package marks its command hidden, so the root page gives it no row.
	hidden []string
}

// declared answers every package that calls leroot, tool or namespace, which
// is the population the composition root MUST import.
func (r registrations) declared() []string {
	names := slices.Concat(r.tools, r.namespaces)
	slices.Sort(names)
	return names
}

// declaration is what the files of one package call.
type declaration struct {
	tool      bool
	namespace bool
	hidden    bool
}

// add files one package's declaration under its short name.
func (r *registrations) add(name string, held declaration) {
	if held.tool {
		r.tools = append(r.tools, name)
		if held.hidden {
			r.hidden = append(r.hidden, name)
		}
		return
	}
	if held.namespace {
		r.namespaces = append(r.namespaces, name)
	}
}

// registeringPackages returns what every package under root declares to le.
//
// The test checks for the CALL, not a naming convention. A package gets a
// command by creating it. A directory that contains only helpers is not a tool.
func registeringPackages(root string) (registrations, error) {
	var found registrations
	if err := walkRegistering(filepath.Join(root, toolsDir), "", 1, &found); err != nil {
		return registrations{}, err
	}
	slices.Sort(found.tools)
	slices.Sort(found.namespaces)
	slices.Sort(found.hidden)
	return found, nil
}

// walkRegistering files the declaration of every package below dir, whose
// short name is prefix, down to the grammar's depth.
//
// A package sits at the path its command name predicts, one directory per
// word, so internal/le/go/lint registers `le go lint` and
// internal/le/setup/docker/kernel registers `le setup docker kernel`. An
// object that is also a command of its own registers at each level and is
// named at each. The dispatcher reads at most leroot.CommandWordsMax words, so
// that is the depth the walk stops at, and the recursion is bounded by it. A
// testdata directory, or one named with a leading dot or underscore, is not a
// package to the go tool, so it is not one here.
func walkRegistering(dir, prefix string, depth int, found *registrations) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if !isPackageDir(name) {
			continue
		}
		path := filepath.Join(dir, name)
		short := prefix + name
		held, checkErr := packageDeclares(path)
		if checkErr != nil {
			return checkErr
		}
		found.add(short, held)
		if depth == leroot.CommandWordsMax {
			continue
		}
		if walkErr := walkRegistering(path, short+"/", depth+1, found); walkErr != nil {
			return walkErr
		}
	}
	return nil
}

// isPackageDir reports whether the go tool treats a directory of this name as
// a package directory: it ignores testdata and names opening with a dot or an
// underscore.
func isPackageDir(name string) bool {
	if name == "testdata" {
		return false
	}
	return !strings.HasPrefix(name, ".") && !strings.HasPrefix(name, "_")
}

// packageDeclares reports which leroot calls the non-test files of one
// package make.
//
// It parses the files and reads the calls, rather than searching the text for
// the call's spelling: a string literal or a comment that holds the spelling
// (the panic text in le/root that names the call to fix, the constants in
// this file) calls nothing.
func packageDeclares(dir string) (declaration, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return declaration{}, err
	}
	var held declaration
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		path := filepath.Join(dir, name)
		raw, readErr := os.ReadFile(path) //nolint:gosec // a build tool reads the tree it was pointed at
		if readErr != nil {
			return declaration{}, readErr
		}
		fileHeld, parseErr := fileDeclares(path, raw)
		if parseErr != nil {
			return declaration{}, parseErr
		}
		held.tool = held.tool || fileHeld.tool
		held.namespace = held.namespace || fileHeld.namespace
		held.hidden = held.hidden || fileHeld.hidden
	}
	return held, nil
}

// fileDeclares reports which leroot calls one source file makes. A file that
// does not import leroot makes none.
func fileDeclares(path string, src []byte) (declaration, error) {
	file, err := parser.ParseFile(token.NewFileSet(), path, src, parser.SkipObjectResolution)
	if err != nil {
		return declaration{}, err
	}
	local, err := leRootLocalName(path, file)
	if err != nil {
		return declaration{}, err
	}
	var held declaration
	if local == "" {
		return held, nil
	}
	ast.Inspect(file, func(node ast.Node) bool {
		call, isCall := node.(*ast.CallExpr)
		if !isCall {
			return true
		}
		selector, isSelector := call.Fun.(*ast.SelectorExpr)
		if !isSelector {
			return true
		}
		receiver, isIdent := selector.X.(*ast.Ident)
		if !isIdent {
			return true
		}
		if receiver.Name != local {
			return true
		}
		switch selector.Sel.Name {
		case registerToolCall:
			held.tool = true
		case registerNamespaceCall:
			held.namespace = true
		case registerHiddenCall:
			held.hidden = true
		}
		return true
	})
	return held, nil
}

// leRootLocalName answers the name one file calls leroot by, or "" when the
// file does not import it. A dot import would make the calls unqualified and
// invisible to the selector match, so it is refused rather than read as no
// call.
func leRootLocalName(path string, file *ast.File) (string, error) {
	for _, spec := range file.Imports {
		imported, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return "", err
		}
		if imported != leRootPath {
			continue
		}
		if spec.Name == nil {
			return leRootName, nil
		}
		if spec.Name.Name == "." {
			var tb textbuf.Buffer
			return "", errors.New(tb.Str(path).Str(" dot-imports ").Str(leRootPath).
				Str(", so its calls cannot be read: import it as leroot").String())
		}
		return spec.Name.Name, nil
	}
	return "", nil
}

// commandNames returns the root commands in the order in which the help page
// lists them.
//
// The page prints one section for each command group, then two that list no
// command: the usage block above them, and the launcher block below them,
// whose rows are words the ./le script reads (leroot.LauncherTitle). So the
// parser reads every section and skips those two. The usage line has the same
// indentation as a command line, and its own heading is what separates the
// two.
//
// Reading the sections, rather than one heading by name, is what lets a tracked
// run judge a commit whose page groups its commands differently. That page is
// the only one this parser ever gets: it comes from the binary built from the
// commit, not from this source tree.
func commandNames(page string) []string {
	var names []string
	inside := false

	for line := range strings.SplitSeq(page, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" {
			continue
		}
		if title, heading := sectionTitle(line, trimmed); heading {
			inside = listsCommands(title)
			continue
		}
		if !inside {
			continue
		}
		// An entry starts at column two. A deeper line continues the entry
		// above it, as a wrapped description does, and names no command.
		if !strings.HasPrefix(line, "  ") || strings.HasPrefix(line, "   ") {
			continue
		}
		if fields := strings.Fields(line); len(fields) > 0 {
			names = append(names, fields[0])
		}
	}
	return names
}

// sectionTitle answers the title of the section a page line opens, and false
// when the line opens none. helpfmt writes a heading at column zero as the
// title and a colon, then a space and the section's summary when it has one
// (`Commands:`, `ai: agent tooling...`, `Launcher: words ./le reads...`), and
// every line it lists is indented under it. The title is what precedes the
// first colon.
func sectionTitle(line, trimmed string) (string, bool) {
	if len(line) != len(trimmed) {
		return "", false
	}
	title, _, found := strings.Cut(trimmed, ":")
	return title, found
}

// listsCommands reports whether a section of the root page lists commands:
// every section does except the usage block and the launcher block.
func listsCommands(title string) bool {
	if title == usageTitle {
		return false
	}
	return title != leroot.LauncherTitle
}

// wiredTools counts the packages expected to produce a root-page row: each is
// imported by the composition root AND registers a tool AND does not hide it.
// A namespace package is imported too, and gives the page a heading rather
// than a row, so it is left out. A hidden tool (leroot.RegisterHidden) is
// wired and the page leaves it out, so it is left out too.
func wiredTools(imported, tools, hidden []string) int {
	toolSet := setOf(tools)
	hiddenSet := setOf(hidden)
	wired := 0
	for _, name := range imported {
		if !toolSet[name] {
			continue
		}
		if hiddenSet[name] {
			continue
		}
		wired++
	}
	return wired
}

// Compare returns all disagreements between the packages the composition root
// imports, the packages that register a tool, the packages that declare a
// namespace and no tool, the tools whose package hides them, and the commands
// that the built binary offers.
//
// An EMPTY commit is a finding, not a clean result. If a tree registers no
// tool, each comparison below has nothing to compare and would report no
// difference. This check must report that condition as breakage.
func Compare(imported, tools, namespaces, hidden, offered []string) []Broken {
	if len(tools) == 0 {
		return []Broken{{
			Package: registerPath,
			Detail:  "the commit wires no tool into le at all",
		}}
	}

	importedSet := setOf(imported)
	toolSet := setOf(tools)
	namespaceSet := setOf(namespaces)

	// The count runs over the wired tools instead of the import list. This gives
	// one finding for each disagreement and does not also report an unwired tool
	// as a missing command.
	wired := wiredTools(imported, tools, hidden)

	var broken []Broken
	for _, name := range tools {
		if !importedSet[name] {
			broken = append(broken, Broken{
				Package: name,
				Detail:  "registers a command and internal/le/register.go does not import it",
			})
		}
	}
	for _, name := range namespaces {
		if !importedSet[name] {
			broken = append(broken, Broken{
				Package: name,
				Detail:  "declares a namespace and internal/le/register.go does not import it",
			})
		}
	}
	for _, name := range imported {
		if toolSet[name] {
			continue
		}
		if namespaceSet[name] {
			continue
		}
		broken = append(broken, Broken{
			Package: name,
			Detail:  "is imported by internal/le/register.go and registers no command and no namespace",
		})
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
	rootImported, err := toolImports(src)
	if err != nil {
		return verdict, 2, err
	}

	found, err := registeringPackages(dest)
	if err != nil {
		return verdict, 2, err
	}
	imported := rootImported.wired(found.declared())
	// A hidden tool loads like any other, so it counts as an area that loads.
	verdict.Areas = wiredTools(imported, found.tools, nil)

	offered, err := buildAndList(ctx, dest)
	if err != nil {
		return verdict, 2, err
	}
	verdict.Commands = offered

	verdict.Broken = Compare(imported, found.tools, found.namespaces, found.hidden, offered)
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

	tags, err := repofeaturetags.DaemonBuildTags(dest, "ze_le")
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

	// `help` writes the page and exits 0, which is what makes it the cheapest
	// proof that every init() ran. A commit before the help pages moved to
	// stdout wrote it to stderr, so both streams are read.
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
