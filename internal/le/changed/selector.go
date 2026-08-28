// Design: docs/architecture/testing/verify-freshness-scope.md -- one selector for the change set
//
// Action: le changed scope [print <packages|tags|both>] [depth <n>]
//
//	[paths-from <file>] [drop-log <file>]
//
// The selector answers two questions about one changed file set: which Go
// packages must be retested, and which build-tag features the change can reach.
// Both answers come from one walk, so no consumer can hold two of them that
// disagree.
//
// Two properties decide the shape of the code below.
//
// The reverse import graph is built WITH every feature tag on. cmd/ze/hub
// imports internal/component/ssh only from files carrying //go:build ze_ssh, so
// an untagged graph reports that package as having no importers at all, and an
// SSH change then retests nothing in the hub.
//
// A non-Go path is classified by the packages that read it, never by the
// directory it sits in. Hook settings, rule pages, and manifests are inputs to
// tooling tests rather than members of their own directories.
//
// A kind no rule names does NOT widen the whole run, and the reason is that the
// package answer drives two Go-only stages, ze-lint-changed and
// ze-unit-test-changed. A non-Go file cannot change what a Go package compiles
// to. It can only change what a Go TEST does, and then only if that test reads
// it, so the answer is the packages that could plausibly read it: the package
// the path sits in, or the tooling packages when it sits in no package. Every
// stage that judges the file's own content -- the doc gates, ze-rfc-check, the
// functional suites -- is a separate stage that runs unconditionally and reads
// none of this. Widening on every unnamed kind was measured instead: one
// modified hook script, then one plan file, made the whole answer ./... on a
// tree a session had merely dirtied, so the rule could never fire in practice.
// The residual risk is a non-Go file read by a Go test sitting neither beside
// it nor in a tooling package, and unclassifiedDirs names every such path on
// stderr so that gap stays visible (ai/rules/repo-maintenance.md refuses a
// silent cap).
//
// Three inputs still widen to every package, because each one changes what the
// Go packages themselves compile to: go.mod, go.sum and vendor/ (dependencyMoved).
//
// A fourth widens for a different reason. The change set is the working tree
// PLUS everything committed since the last proven commit, so with no proven
// commit the second term is every commit in history rather than none of them
// (greenBaseline). A tree with no green point behind it is a tree where nothing
// is proven, and the wide answer clears itself on the first passing verify.
//
// A tracked Go directory the unit tag set never compiles widens only when the
// wide answer buys a check the narrow one does not. The module root buys none.
// ./... compiles it no more than the narrow answer does, so the wide answer runs
// the same checks and pays for the whole tree. uncompiledTreeReaders maps the
// module root to what does read it.
//
// The gokrazy module cache never reaches that question: packageDirsFor answers
// it by path. cmd/ze-installer stopped being one of them on 2026-08-24. The
// repository's flavor-aware lint selects it only when the whole tree is named,
// so the wide answer is the only answer that reports on the initrd's PID 1.
//
// The walk stops at depth 2. Measured over 646 packages, the full transitive
// closure selects a third of the tree for one edit under internal/core, while
// depth 2 recovers 85% of it and depth 3 lands within 3% of the closure. The
// spec records that decision; this program states on stderr what the bound
// dropped, so a later spec can judge depth 1 on data rather than on hope.
package changed

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"go/build/constraint"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	// featureManifestPath is the single source of truth for compile-out-able
	// features. It is read on every run and never copied: a generated second
	// copy would be a second source of truth needing its own staleness gate
	// (ai/rules/plugins.md).
	featureManifestPath = "feature-gates.txt"

	docValidationPackage = "internal/le/docvalid"
	verifyRunnerPackage  = "internal/le/verifyengine"
	checksPackage        = "internal/le"
	hookRuntimePackage   = "internal/le/hookruntime"
	rfcPackage           = "internal/le/rfc"
	rulesPackage         = "internal/le/rules"
	specPackage          = "internal/le/specstatus"
	workflowPackage      = "internal/le/workflowcheck"
	docCheckPackage      = "internal/le/doccheck"

	// ciRunnerPackage holds the functional-test runner. ci_fixture_test.go walks
	// every committed .ci and fails on a BGP frame whose Length field disagrees
	// with its byte count, and TestCIPeerBlockCorpusParses
	// (peer_block_directive_test.go) parses the whole corpus and is deliberately
	// fatal rather than skipping when the tree moves.
	ciRunnerPackage = "internal/test/runner"

	// editorRunnerPackage holds the editor-test harness. runner_test.go walks
	// test/editor for .et files and requires the corpus to be non-empty.
	editorRunnerPackage = "internal/component/cli/testing"

	// gokrazyModuleCache is a tracked third-party module cache with its own
	// go.mod. No package here compiles it, and every tree-walking check names it
	// in a skip list.
	gokrazyModuleCache = "gokrazy/modcache"

	// rfcCorpusPrefix holds the RFC files that suite reads at run time:
	// enrolled.txt, short/*.md, extraction/*.json, drain-budget.txt.
	rfcCorpusPrefix = "rfc/"

	// vendorPrefix is third-party code. It is never linted or tested here, and a
	// change to it means a dependency moved, which reaches the whole tree.
	vendorPrefix = "vendor/"

	// moduleFilePath and moduleSumPath name the module graph itself. They are
	// matched exactly, so a nested module's own go.mod (examples/plugin/go) is
	// not read as a move of this module's dependencies.
	moduleFilePath = "go.mod"
	moduleSumPath  = "go.sum"

	// externalPluginExample is a SEPARATE module (module example/acme-monitor,
	// with a replace back to this one), so go list ./... never reports it and no
	// package here compiles it.
	externalPluginExample = "examples/plugin/go"

	// defaultDepth is the reverse-dependency bound. See the file header.
	defaultDepth = 2

	// everyPackage is the fail-open package answer. go test, go build and
	// golangci-lint all accept it.
	everyPackage = "./..."

	// rootPackage is the answer for a change to a file in the module root.
	rootPackage = "."

	// goListDeadline bounds the one graph build. It measures 2.6s on the current
	// tree; the bound exists so a wedged toolchain fails rather than hangs.
	goListDeadline = 120 * time.Second

	// gitDeadline bounds each git query that collects the changed paths.
	gitDeadline = 60 * time.Second
)

// pathRule maps one kind of non-Go path to the Go packages that READ it.
//
// A kind is named by where the path lives and how it ends. prefix is matched
// against the whole repo-relative path, so it names a directory (ai/) or a file
// at the module root (feature-gates.txt). An empty prefix or suffix matches
// anything.
//
// dirs holds the packages whose tests consume the kind, and every row here
// names at least one. "This kind is read by nothing" is a claim about a whole
// TREE rather than about a suffix, so it is written as a branch in
// packageDirsFor beside the tree it judges, where the evidence for it sits.
// A kind no rule names is the third case, and it seeds its readers rather than
// widening (unclassifiedDirs).
type pathRule struct {
	prefix string
	suffix string
	dirs   []string
}

// toolingPackages are the Go packages whose tests read repository content
// instead of compiling it.
var toolingPackages = []string{docValidationPackage, verifyRunnerPackage, checksPackage}

// treeWalkingPackages read every tracked source file.
var treeWalkingPackages = []string{checksPackage}

// nonGoPathRules maps each declarative repository input to the Go packages that
// read it.
var nonGoPathRules = []pathRule{
	{prefix: ".claude/settings", suffix: ".json", dirs: []string{hookRuntimePackage}},
	{prefix: testTreePrefix, suffix: ".ci", dirs: []string{ciRunnerPackage, docValidationPackage, checksPackage}},
	{prefix: testTreePrefix, suffix: ".et", dirs: []string{editorRunnerPackage}},
	{prefix: testTreePrefix, suffix: ".wb", dirs: []string{"internal/component/web/testing"}},
	{prefix: "ai/", suffix: markdownSuffix, dirs: []string{rulesPackage, docCheckPackage}},
	{prefix: "plan/", suffix: markdownSuffix, dirs: []string{specPackage, docCheckPackage}},
	{prefix: "docs/", suffix: markdownSuffix, dirs: []string{docValidationPackage, docCheckPackage}},
	{prefix: ".github/", suffix: ".yml", dirs: []string{workflowPackage}},
	{prefix: rfcCorpusPrefix, dirs: []string{rfcPackage}},
}

// printMode says which answer goes to stdout.
type printMode string

const (
	printPackages printMode = "packages"
	printTags     printMode = "tags"
	printBoth     printMode = "both"
)

// selectorOptions is the parsed command line.
type selectorOptions struct {
	mode      printMode
	depth     int
	pathsFrom string
	dropLog   string
}

// featureGate is one manifest row: a build tag, and the package it gates.
type featureGate struct {
	tag string
	pkg string
}

// packageGraph is the first-party import graph, built with every feature tag on.
type packageGraph struct {
	// dirOf maps an import path to its repo-relative directory.
	dirOf map[string]string
	// pathOf maps a repo-relative directory to its import path.
	pathOf map[string]string
	// importers maps an import path to the packages importing it, the imports of
	// their test files included. A test-only importer is a package whose tests
	// exercise the changed code, which is what a retest set exists to find.
	importers map[string][]string
}

// changeSet is the selector's answer.
type changeSet struct {
	// packages holds ./-prefixed package directories, sorted and unique, or the
	// single entry everyPackage when the selector failed open.
	packages []string
	// tags holds the feature tags the change can reach, sorted and unique.
	tags []string
}

// resolveSelector is the whole native selector. It returns 0 for every answer,
// including every fail-open answer. Only a caller error and a missing manifest
// return non-zero.
func (s Scope) resolveSelector(args []string) (ScopeReport, int) {
	opts, err := parseSelectorFlags(args)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "verify-scope: %v\n", err)
		return ScopeReport{}, 2
	}

	root := canonicalRoot(s.Root)

	gates, err := loadFeatureGates(root)
	if err != nil {
		// The manifest is the one input with no safe substitute. Without it the
		// selector cannot name a feature at all, so it refuses rather than
		// answering with a tag set it invented.
		_, _ = fmt.Fprintf(os.Stderr, "verify-scope: %v\n", err)
		return ScopeReport{}, 2
	}
	everyTag := manifestTags(gates)

	paths, err := changedPaths(root, opts.pathsFrom)
	if err != nil {
		if errors.Is(err, errNoGreenBaseline) {
			// Its own message already names the condition and the file, and it is
			// a widening reason rather than a broken query.
			return emitReport(opts, failOpen(everyTag), err)
		}
		return emitReport(opts, failOpen(everyTag), fmt.Errorf("the changed paths could not be read (%w)", err))
	}

	seeds, widen := classifyPaths(root, paths)
	if widen != nil {
		return emitReport(opts, failOpen(everyTag), widen)
	}
	if len(seeds) == 0 {
		// Every path either seeds a package, seeds nothing by a rule that says
		// so, or widens the run. An empty seed list therefore means the change
		// set held nothing a Go package compiles or reads -- an empty set, or a
		// set of paths under examples/plugin/go and gokrazy/modcache, the two
		// trees this module neither compiles nor walks. That is the one state in
		// which selecting no package is the honest answer, and it is still said
		// out loud (ai/rules/evidence.md -- a zero value must never be a
		// valid-looking answer nobody announced).
		_, _ = fmt.Fprintln(os.Stderr, "verify-scope: no changed path is compiled or read by a Go package, so no package is selected")
		return emitReport(opts, changeSet{}, nil)
	}

	graph, err := loadPackageGraph(root, everyTag)
	if err != nil {
		return emitReport(opts, failOpen(everyTag), fmt.Errorf("the import graph could not be built (%w)", err))
	}

	selected, widen := seedPackages(graph, seeds)
	if widen != nil {
		return emitReport(opts, failOpen(everyTag), widen)
	}
	answer := changeSet{
		packages: expandPackages(graph, selected, opts),
		tags:     reachedTags(root, paths, seeds, gates, everyTag),
	}
	return emitReport(opts, answer, nil)
}

// canonicalRoot resolves root to the path go list itself reports.
//
// go list answers each package's Dir through the kernel's real working
// directory, which macOS resolves through /var and /tmp to their /private
// targets. t.TempDir() and most CI checkouts hand back the symlinked form, so
// loadPackageGraph's cmd.Dir carries that form while go list's own output is
// already resolved. parsePackageGraph then computes filepath.Rel(root, dir) to
// recover each package's repo-relative path, and a root that still carries the
// symlink shares no prefix with a resolved dir at all: every package maps to a
// path outside the tree, seedPackages then reports every seed as unknown to go
// list, and the selector fails open to everyPackage on a checkout that was
// never actually unclassifiable. Resolving once here, before root reaches any
// of the selector's path arithmetic, keeps every comparison on go list's own
// footing. A root the filesystem cannot resolve is returned unchanged: the
// read that follows names that failure on its own path rather than one this
// function invented.
func canonicalRoot(root string) string {
	resolved, resolveErr := filepath.EvalSymlinks(root)
	if resolveErr != nil {
		return root
	}
	return resolved
}

func parseSelectorFlags(args []string) (selectorOptions, error) {
	opts := selectorOptions{mode: printPackages, depth: defaultDepth}
	for _, arg := range args {
		name, value, found := strings.Cut(arg, "=")
		if !found {
			return opts, fmt.Errorf("unknown argument %q (want --print=, --depth=, --paths-from= or --drop-log=)", arg)
		}
		switch name {
		case "--print":
			mode := printMode(value)
			if !knownPrintMode(mode) {
				return opts, fmt.Errorf("--print wants packages, tags or both, got %q", value)
			}
			opts.mode = mode
		case "--depth":
			depth, err := strconv.Atoi(value)
			if err != nil {
				return opts, fmt.Errorf("--depth wants a whole number, got %q", value)
			}
			// Depth 0 is the one value with a real failure mode: it retests the
			// edited package and no importer of it, so a behavior change in a
			// core package would be judged by that package's own tests alone.
			if depth < 1 {
				return opts, fmt.Errorf("--depth must be 1 or more, got %d: depth 0 retests no importer at all", depth)
			}
			opts.depth = depth
		case "--paths-from":
			opts.pathsFrom = value
		case "--drop-log":
			opts.dropLog = value
		default:
			return opts, fmt.Errorf("unknown flag %q", name)
		}
	}
	return opts, nil
}

func knownPrintMode(mode printMode) bool {
	switch mode {
	case printPackages, printTags, printBoth:
		return true
	}
	return false
}

// loadFeatureGates reads featureManifestPath into its "<tag> <pkg>" rows. Every
// feature consumer reads this one manifest, so no consumer can hold a stale
// copy of it.
func loadFeatureGates(root string) ([]featureGate, error) {
	file, err := os.Open(filepath.Join(root, featureManifestPath)) //nolint:gosec // the feature manifest is a tracked path under the repository root
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", featureManifestPath, err)
	}
	defer func() { _ = file.Close() }()

	var gates []featureGate
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "#") {
			continue
		}
		fields := strings.Fields(line)
		if len(fields) < 2 {
			return nil, fmt.Errorf("%s: malformed line %q (want \"<tag> <pkg>\")", featureManifestPath, line)
		}
		gates = append(gates, featureGate{tag: fields[0], pkg: path.Clean(fields[1])})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", featureManifestPath, err)
	}
	if len(gates) == 0 {
		return nil, fmt.Errorf("%s declares no feature gate", featureManifestPath)
	}
	return gates, nil
}

// manifestTags returns every tag the manifest declares, sorted and unique.
func manifestTags(gates []featureGate) []string {
	seen := map[string]bool{}
	tags := make([]string, 0, len(gates))
	for _, gate := range gates {
		if seen[gate.tag] {
			continue
		}
		seen[gate.tag] = true
		tags = append(tags, gate.tag)
	}
	sort.Strings(tags)
	return tags
}

// tagForPackage returns the feature tag gating a repo-relative directory, or the
// empty string when the directory is always-on. The longest matching manifest
// prefix wins, so a gated sub-package under an always-on parent resolves to its
// own tag. A package beneath a gated package is dropped with that package.
func tagForPackage(dir string, gates []featureGate) string {
	tag := ""
	longest := 0
	for _, gate := range gates {
		if !underPackage(dir, gate.pkg) {
			continue
		}
		if len(gate.pkg) <= longest {
			continue
		}
		longest = len(gate.pkg)
		tag = gate.tag
	}
	return tag
}

// underPackage says whether dir is pkg itself or a directory inside it.
func underPackage(dir, pkg string) bool {
	if dir == pkg {
		return true
	}
	if !strings.HasPrefix(dir, pkg) {
		return false
	}
	// Require the path boundary, so internal/component/ssh does not match a
	// sibling named internal/component/sshkeys.
	return dir[len(pkg)] == '/'
}

// changedPaths returns the repo-relative paths the selector must classify. With
// --paths-from it reads them from a file, one per line, which is how a test and
// any caller holding its own list drives the selector. Without it, git answers.
func changedPaths(root, pathsFrom string) ([]string, error) {
	if pathsFrom == "" {
		return gitChangedPaths(root)
	}
	body, err := os.ReadFile(pathsFrom) //nolint:gosec // pathsFrom is the operator-named path list this action takes as its input
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", pathsFrom, err)
	}
	var paths []string
	for line := range strings.SplitSeq(string(body), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		paths = append(paths, line)
	}
	return paths, nil
}

// gitChangedPaths collects the unstaged, staged, untracked, and
// committed-since-the-last-green paths.
//
// Every query uses -z, so a path holding a space, a quote or a newline arrives
// verbatim instead of in git's quoted form. No pathspec filters the queries: a
// path kind the selector does not know MUST reach the classifier, or the
// fail-open branch could never run outside a test (ai/rules/evidence.md -- a
// guard on a path the traffic does not take does not exist).
func gitChangedPaths(root string) ([]string, error) {
	queries := make([][]string, 0, 4)
	queries = append(queries,
		[]string{gitDiff, gitNameOnly, "-z"},
		[]string{gitDiff, "--cached", gitNameOnly, "-z"},
		[]string{"ls-files", "--others", "--exclude-standard", "-z"},
	)
	baseline, err := greenBaseline(root)
	if err != nil {
		return nil, err
	}
	// A commit that has left the working-tree diff is invisible to the three
	// queries above, so a scoped verify on a clean tree would test nothing in the
	// package that commit changed.
	queries = append(queries, []string{gitDiff, gitNameOnly, "-z", baseline, "HEAD"})

	seen := map[string]bool{}
	var paths []string
	for _, query := range queries {
		out, err := runGit(root, query...)
		if err != nil {
			return nil, err
		}
		for entry := range strings.SplitSeq(out, "\x00") {
			if entry == "" {
				continue
			}
			if seen[entry] {
				continue
			}
			seen[entry] = true
			paths = append(paths, entry)
		}
	}
	return paths, nil
}

// errNoGreenBaseline says that no verify run has recorded a commit it proved.
// The caller widens on it, so it is a widening reason rather than a failure.
var errNoGreenBaseline = errors.New("no verify run has recorded a green commit")

// greenBaseline returns the commit recorded by the last PASSING verify. When no
// such commit exists it returns errNoGreenBaseline naming the condition, and the
// caller widens the run to every package.
//
// Earlier scoped selection dropped the committed-since term when no green
// baseline existed. Without a proven commit, every commit in history is
// unverified, so a clean tree would select nothing and report green over code
// no stage ran. A scoped green is commit evidence, so that narrow answer could
// otherwise be laundered into a commit.
//
// The wide answer clears itself: the first passing verify writes exit=0 and a
// SHA, and every later run narrows again.
func greenBaseline(root string) (string, error) {
	statusPath := os.Getenv("ZE_VERIFY_STATUS_FILE")
	if statusPath == "" {
		statusPath = filepath.Join("tmp", "ze-verify.status")
	}
	// An ABSOLUTE override is used as it stands. `filepath.Join(root, "/abs")`
	// yields `<root>/abs`, which no file answers to, so the read would fail and
	// the run would widen on a status file the operator did supply.
	if !filepath.IsAbs(statusPath) {
		statusPath = filepath.Join(root, statusPath)
	}
	body, err := os.ReadFile(statusPath) //nolint:gosec // statusPath is the tracked verify-status file, or the operator override this action accepts
	if err != nil {
		return "", fmt.Errorf("%w: %s cannot be read (%w)", errNoGreenBaseline, statusPath, err)
	}
	exit, sha := "", ""
	for line := range strings.SplitSeq(string(body), "\n") {
		switch {
		case strings.HasPrefix(line, "exit="):
			exit = strings.TrimSpace(strings.TrimPrefix(line, "exit="))
		case strings.HasPrefix(line, "git_sha="):
			sha = strings.TrimSpace(strings.TrimPrefix(line, "git_sha="))
		}
	}
	if exit != "0" {
		return "", fmt.Errorf("%w: %s records exit=%s over the commit it names", errNoGreenBaseline, statusPath, exit)
	}
	if sha == "" {
		return "", fmt.Errorf("%w: %s records no git_sha", errNoGreenBaseline, statusPath)
	}
	if sha == "unknown" {
		return "", fmt.Errorf("%w: %s records git_sha=unknown", errNoGreenBaseline, statusPath)
	}
	kind, err := runGit(root, "cat-file", "-t", sha)
	if err != nil {
		return "", fmt.Errorf("%w: %s records git_sha=%s, which this repository does not hold", errNoGreenBaseline, statusPath, sha)
	}
	if strings.TrimSpace(kind) != "commit" {
		return "", fmt.Errorf("%w: %s records git_sha=%s, which is a %s rather than a commit", errNoGreenBaseline, statusPath, sha, strings.TrimSpace(kind))
	}
	return sha, nil
}

func runGit(root string, args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), gitDeadline)
	defer cancel()

	cmd := exec.CommandContext(ctx, "git", args...) //nolint:gosec // args are the fixed git queries this file builds, never operator text
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w", strings.Join(args, " "), err)
	}
	return string(out), nil
}

// classifyPaths maps each changed path to the package directory that must be
// retested. It returns the seed directories, and a non-nil reason when the set
// cannot be narrowed at all.
//
// That reason is the fail-open branch, and three things reach it: the module
// graph moving, a path make cannot carry as one word, and a seed whose directory
// is gone. Refusing the run instead would block work on a new file kind, and
// under-selection is the failure that ships a defect, so the wide answer is the
// safe one (ai/rules/evidence.md -- a guard fails closed or says something).
//
// A path no rule names does not reach it. It seeds unclassifiedDirs and is named
// on stderr, which is the same guarantee written the other way round: the answer
// is never silently narrow, and the operator holds the path that would need a
// rule.
func classifyPaths(root string, paths []string) ([]string, error) {
	seen := map[string]bool{}
	var seeds []string
	for _, changed := range paths {
		changed = filepath.ToSlash(changed)
		if err := dependencyMoved(changed); err != nil {
			return nil, err
		}

		dirs, classified := packageDirsFor(changed)
		if !classified {
			dirs = unclassifiedDirs(root, changed)
			// Naming the path is what keeps the residual risk visible: a non-Go
			// file read by a Go test that sits neither beside it nor in a
			// tooling package is a missing rule, and this line is the evidence
			// the next reader needs to write one.
			_, _ = fmt.Fprintf(os.Stderr, "verify-scope: no rule names %s, so it seeds %s\n", changed, packageWords(dirs))
		}
		for _, dir := range dirs {
			if !dirExists(root, dir) {
				// A deleted file leaves no directory behind. Its importers still
				// owe a retest, and no package name survives to name them with.
				return nil, fmt.Errorf("%s needs the package %s, which is not a directory on disk", changed, packageWord(dir))
			}
			if seen[dir] {
				continue
			}
			seen[dir] = true
			seeds = append(seeds, dir)
		}
	}
	sort.Strings(seeds)
	return seeds, nil
}

// dependencyMoved reports the one class of change that reaches every package:
// the module graph itself. A vendored file is never ours to lint or test, and
// go.mod or go.sum changing means a dependency moved, so every package that
// compiles against it is reachable. The caller widens the run and says so.
func dependencyMoved(changed string) error {
	if strings.HasPrefix(changed, vendorPrefix) || changed == moduleFilePath || changed == moduleSumPath {
		return fmt.Errorf("%s changed, so a dependency moved", changed)
	}
	return nil
}

// packageDirsFor returns the repo-relative package directories a changed path
// seeds, and whether a rule named its kind at all. It has three answers, and no
// two of them may be confused:
//
//   - one or more directories, classified: the packages that compile or read the
//     path. A Go file seeds its own directory, and the module root is the empty
//     string. A non-Go kind seeds the tooling packages nonGoPathRules names.
//   - no directory, classified: the path is in a tree this module neither
//     compiles nor walks, so it seeds nothing (examples/plugin/go and
//     gokrazy/modcache, each a separate module).
//   - not classified: no rule names the kind, and the caller answers with
//     unclassifiedDirs and names the path on stderr.
func packageDirsFor(changed string) ([]string, bool) {
	// The example plugin is its own module, so its .go files belong to no
	// package go list ./... reports, and no Go package here compiles or reads
	// it: the only references in this module are two doc comments, in
	// internal/test/cli/cmd_plugin_external.go and pkg/plugin/sdk/sdk.go. No
	// native action or workflow builds the module, so it seeds nothing. Adding
	// an unrelated package would run tests that cannot fail when the example
	// breaks.
	if underPackage(changed, externalPluginExample) {
		return nil, true
	}
	// The gokrazy module cache is third-party code with its own go.mod, and every
	// repository tree walker skips it. No Go test here reads it, so it seeds
	// nothing.
	if underPackage(changed, gokrazyModuleCache) {
		return nil, true
	}
	if strings.HasSuffix(changed, ".go") {
		dir := path.Dir(changed)
		if dir == rootPackage {
			return []string{""}, true
		}
		return []string{dir}, true
	}
	for _, rule := range nonGoPathRules {
		if strings.HasPrefix(changed, rule.prefix) && strings.HasSuffix(changed, rule.suffix) {
			return rule.dirs, true
		}
	}
	return nil, false
}

// unclassifiedDirs is the answer for a kind no rule names. A path sitting in a
// directory that holds Go source is a fixture of that package, read by the tests
// beside it. A path sitting in no package is read, if by anything here, by the
// tooling packages.
//
// Neither answer is the whole tree, and the file header states why that is safe:
// this list drives two Go-only stages, and a non-Go file changes what a Go TEST
// does rather than what a package compiles to.
func unclassifiedDirs(root, changed string) []string {
	dir := path.Dir(changed)
	if dir == rootPackage {
		// A file at the module root sits beside tools.go, which is gated by
		// //go:build tools and has no tests. The repository root is not a
		// package a fixture sits beside.
		return toolingPackages
	}
	if goPackageDir(root, dir) {
		return []string{dir}
	}
	return toolingPackages
}

// goPackageDir says whether dir holds Go source, which is what makes a file
// beside it a plausible fixture for that package's own tests. A directory that
// no longer exists (a deleted path) holds none.
func goPackageDir(root, dir string) bool {
	entries, err := os.ReadDir(filepath.Join(root, filepath.FromSlash(dir)))
	if err != nil {
		return false
	}
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".go") {
			return true
		}
	}
	return false
}

func dirExists(root, dir string) bool {
	info, err := os.Stat(filepath.Join(root, filepath.FromSlash(dir)))
	if err != nil {
		return false
	}
	return info.IsDir()
}

// loadPackageGraph builds the first-party import graph in ONE go list run, with
// ze_core and every feature tag the manifest declares.
//
// One run costs 2.6s on the current tree against 2.9s for an untagged run. A
// per-tag loop was measured at 94.6s over 37 tag sets, which is over three times
// the whole 30s budget the spec sets for the selector. The single run is
// therefore the only affordable shape, not an optimisation.
func loadPackageGraph(root string, featureTags []string) (*packageGraph, error) {
	tags := make([]string, 0, len(featureTags)+1)
	tags = append(tags, "ze_core")
	tags = append(tags, featureTags...)

	const format = "{{.ImportPath}}\t{{.Dir}}\t" +
		"{{range .Imports}}{{.}} {{end}}{{range .TestImports}}{{.}} {{end}}{{range .XTestImports}}{{.}} {{end}}"

	ctx, cancel := context.WithTimeout(context.Background(), goListDeadline)
	defer cancel()

	cmd := exec.CommandContext(ctx, "go", "list", "-e", "-tags", strings.Join(tags, ","), "-f", format, "./...") //nolint:gosec // the only variable is the build-tag list read from the tracked feature manifest
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0")
	out, err := cmd.Output()
	if err != nil {
		if exit, ok := errors.AsType[*exec.ExitError](err); ok {
			return nil, fmt.Errorf("go list: %w: %s", err, strings.TrimSpace(string(exit.Stderr)))
		}
		return nil, fmt.Errorf("go list: %w", err)
	}
	return parsePackageGraph(root, string(out))
}

// parsePackageGraph reads the tab-separated go list records into the graph. The
// three fields are the import path, the directory, and the imports of the
// package and of its tests.
func parsePackageGraph(root, listing string) (*packageGraph, error) {
	graph := &packageGraph{
		dirOf:     map[string]string{},
		pathOf:    map[string]string{},
		importers: map[string][]string{},
	}
	imports := map[string][]string{}
	for line := range strings.SplitSeq(listing, "\n") {
		if line == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 3 {
			return nil, fmt.Errorf("go list wrote a record with %d fields, want 3: %q", len(fields), line)
		}
		importPath, dir := fields[0], fields[1]
		rel, err := filepath.Rel(root, dir)
		if err != nil {
			return nil, fmt.Errorf("place %s under the repository root: %w", importPath, err)
		}
		rel = filepath.ToSlash(rel)
		if rel == rootPackage {
			rel = ""
		}
		graph.dirOf[importPath] = rel
		graph.pathOf[rel] = importPath
		imports[importPath] = strings.Fields(fields[2])
	}
	if len(graph.dirOf) == 0 {
		return nil, errors.New("go list reported no package at all")
	}

	// An import naming no listed package is standard library or vendored
	// third-party code, and neither has first-party importers to retest.
	for importPath, imported := range imports {
		for _, dependency := range imported {
			if _, ok := graph.dirOf[dependency]; !ok {
				continue
			}
			graph.importers[dependency] = append(graph.importers[dependency], importPath)
		}
	}
	return graph, nil
}

// uncompiledTreeReaders answers virtual package roots whose Go files no unit
// flavor compiles, and reports whether dir is one of them.
//
// This table takes precedence over go list. Go 1.27 can report the module root
// as "." even when tools.go is excluded by its tools build tag. Treating that
// answer as compiled would select a package no unit or lint flavor loads. The
// narrow answer instead names the tree-walking checks that consume tools.go.
//
// A rule belongs here only when the wide answer buys nothing. ./... under the
// unit tag set does not compile the module root either, so widening pays for the
// whole tree without adding coverage.
//
// cmd/ze-installer had a rule here until 2026-08-24 and lost it, because the
// wide answer stopped being equal. The flavor-aware lint selects that package
// only from a whole-tree scope, so the narrow answer reports on nothing. An
// unruled seed widens, which is the answer the initrd's PID 1 needs.
func uncompiledTreeReaders(dir string) ([]string, bool) {
	if dir == "" {
		// The module root holds tools.go alone (//go:build tools), which pins the
		// tool imports go mod vendor follows. Repository tree checks read it.
		// Nothing compiles it, and no lint flavor selects it either.
		return treeWalkingPackages, true
	}
	return nil, false
}

// seedPackages turns seed directories into import paths. A declared virtual
// root is mapped to its readers first. Other directories must occur in the Go
// package graph or the selector fails closed.
func seedPackages(graph *packageGraph, seeds []string) ([]string, error) {
	var packages []string
	for _, dir := range seeds {
		if readers, ruled := uncompiledTreeReaders(dir); ruled {
			for _, reader := range readers {
				readerPath, ok := graph.pathOf[reader]
				if !ok {
					return nil, fmt.Errorf("%s is read by %s, which go list does not report either", packageWord(dir), packageWord(reader))
				}
				packages = append(packages, readerPath)
			}
			continue
		}
		if importPath, ok := graph.pathOf[dir]; ok {
			packages = append(packages, importPath)
			continue
		}
		return nil, fmt.Errorf("%s is not a package go list reports, so its importers cannot be found", dir)
	}
	return packages, nil
}

// expandPackages walks the reverse graph to the configured depth and returns the
// ./-prefixed answer. It reports on stderr what depth 1 would have missed and
// what the bound dropped, so the depth stays a measured decision rather than a
// silent cap (ai/rules/repo-maintenance.md).
func expandPackages(graph *packageGraph, seeds []string, opts selectorOptions) []string {
	atDepth := reverseWalk(graph, seeds, opts.depth)
	atOne := reverseWalk(graph, seeds, 1)
	closure := reverseWalk(graph, seeds, len(graph.dirOf))

	gainedOverOne := difference(atDepth, atOne)
	droppedFromClosure := difference(closure, atDepth)
	_, _ = fmt.Fprintf(os.Stderr, "verify-scope: depth %d selected %d packages from %d changed"+
		" (depth 1 selects %d, the closure selects %d);"+
		" %d packages beyond depth %d are not selected\n",
		opts.depth, len(atDepth), len(seeds), len(atOne), len(closure), len(droppedFromClosure), opts.depth)
	writeDropLog(opts, gainedOverOne, droppedFromClosure)

	packages := make([]string, 0, len(atDepth))
	for importPath := range atDepth {
		packages = append(packages, packageWord(graph.dirOf[importPath]))
	}
	sort.Strings(packages)
	return packages
}

// reverseWalk returns every package reachable from the seeds by following
// importers, up to depth levels. Depth 1 is the seeds plus their direct
// importers. The walk is bounded by depth AND by the visited set, so a cycle in
// the graph cannot extend it.
func reverseWalk(graph *packageGraph, seeds []string, depth int) map[string]bool {
	visited := map[string]bool{}
	frontier := make([]string, 0, len(seeds))
	for _, seed := range seeds {
		if visited[seed] {
			continue
		}
		visited[seed] = true
		frontier = append(frontier, seed)
	}
	for range depth {
		var next []string
		for _, importPath := range frontier {
			for _, importer := range graph.importers[importPath] {
				if visited[importer] {
					continue
				}
				visited[importer] = true
				next = append(next, importer)
			}
		}
		if len(next) == 0 {
			break
		}
		frontier = next
	}
	return visited
}

// difference returns the members of wide that narrow does not hold, sorted.
func difference(wide, narrow map[string]bool) []string {
	var only []string
	for member := range wide {
		if narrow[member] {
			continue
		}
		only = append(only, member)
	}
	sort.Strings(only)
	return only
}

// writeDropLog records the two measurements a later spec needs to judge the
// depth: what depth 1 would have missed, and what the bound dropped from the
// closure. --drop-log=- writes them to stderr.
func writeDropLog(opts selectorOptions, gainedOverOne, droppedFromClosure []string) {
	if opts.dropLog == "" {
		return
	}
	var body strings.Builder
	_, _ = body.WriteString("# selected-at-depth-")
	_, _ = body.WriteString(strconv.Itoa(opts.depth))
	_, _ = body.WriteString("-but-not-at-depth-1\n")
	writeDropSection(&body, gainedOverOne)
	_, _ = body.WriteString("# dropped-beyond-depth-")
	_, _ = body.WriteString(strconv.Itoa(opts.depth))
	_, _ = body.WriteString("\n")
	writeDropSection(&body, droppedFromClosure)

	if opts.dropLog == "-" {
		_, _ = fmt.Fprint(os.Stderr, body.String())
		return
	}
	if err := os.WriteFile(opts.dropLog, []byte(body.String()), 0o600); err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "verify-scope: the drop log %s could not be written: %v\n", opts.dropLog, err)
	}
}

func writeDropSection(body *strings.Builder, importPaths []string) {
	for _, importPath := range importPaths {
		_, _ = body.WriteString(importPath)
		_, _ = body.WriteString("\n")
	}
}

// reachedTags returns the feature tags the change can reach. Two facts put a tag
// in the answer, and a build combination is judged when either one names it.
//
// The first is the tag GATING a changed package. That answer comes from the
// changed packages and not from their importers, and feature-gates.txt is what
// makes it correct: nothing always-on imports a gated package, and every
// importer of one is itself compiled only when that tag is on. A
// build without the tag therefore compiles nothing that can see the change. An
// always-on changed package has no such shield and reaches every feature.
//
// The second is a tag a changed FILE NEGATES. The argument above is about what a
// build DROPS, and a file constrained !ze_T is compiled only by the builds that
// dropped T: a file reading `ze_web && !ze_ssh` compiles in without_ze_ssh and
// in no other row of the matrix, so an answer naming ze_web alone would subtract
// the only build that can see it. The negated tags are therefore unioned in.
//
// A changed Go file that cannot be read, or whose constraint the compiler would
// refuse, widens to every feature. Its negations are exactly what cannot be
// known, and a guard that cannot read its input must not return a valid-looking
// narrow answer (ai/rules/evidence.md).
func reachedTags(root string, paths, seeds []string, gates []featureGate, everyTag []string) []string {
	seen := map[string]bool{}
	for _, dir := range seeds {
		tag := tagForPackage(dir, gates)
		if tag == "" {
			return everyTag
		}
		seen[tag] = true
	}

	declared := make(map[string]bool, len(everyTag))
	for _, tag := range everyTag {
		declared[tag] = true
	}
	for _, changed := range paths {
		changed = filepath.ToSlash(changed)
		if !strings.HasSuffix(changed, ".go") {
			continue
		}
		negated, err := negatedTags(root, changed)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "verify-scope: %v, so every feature is selected\n", err)
			return everyTag
		}
		for _, tag := range negated {
			// A negated tag the manifest does not declare gates no package and
			// owns no matrix row, so it names no build to judge.
			if declared[tag] {
				seen[tag] = true
			}
		}
	}

	tags := make([]string, 0, len(seen))
	for tag := range seen {
		tags = append(tags, tag)
	}
	sort.Strings(tags)
	return tags
}

// negatedTags returns the build tags a Go file's //go:build line negates, at any
// depth of the expression. A file with no constraint negates nothing.
//
// Every tag under a NOT is collected, so !(ze_a && ze_b) yields both: the file
// compiles in a build that dropped either one, and naming both is what keeps
// those builds in the matrix.
func negatedTags(root, changed string) ([]string, error) {
	file, err := os.Open(filepath.Join(root, filepath.FromSlash(changed))) //nolint:gosec // changed names a file inside the tracked checkout under root
	if err != nil {
		return nil, fmt.Errorf("the build constraint of %s could not be read (%w)", changed, err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "package ") {
			// The constraint block ends at the package clause, which every Go
			// file has and which no build line may follow.
			break
		}
		if !constraint.IsGoBuild(line) {
			continue
		}
		expr, parseErr := constraint.Parse(line)
		if parseErr != nil {
			return nil, fmt.Errorf("the build constraint of %s does not parse (%w)", changed, parseErr)
		}
		return tagsUnderNot(expr, nil), nil
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("the build constraint of %s could not be read (%w)", changed, err)
	}
	return nil, nil
}

// tagsUnderNot walks expr and collects the tags a negation covers. Inside a NOT
// it takes every tag of the subtree, because dropping any one of them can make
// that subtree false and the file compile.
func tagsUnderNot(expr constraint.Expr, into []string) []string {
	switch node := expr.(type) {
	case *constraint.NotExpr:
		return everyTagIn(node.X, into)
	case *constraint.AndExpr:
		return tagsUnderNot(node.Y, tagsUnderNot(node.X, into))
	case *constraint.OrExpr:
		return tagsUnderNot(node.Y, tagsUnderNot(node.X, into))
	}
	return into
}

// everyTagIn collects every tag expr mentions, whatever the operators around it.
func everyTagIn(expr constraint.Expr, into []string) []string {
	switch node := expr.(type) {
	case *constraint.TagExpr:
		return append(into, node.Tag)
	case *constraint.NotExpr:
		return everyTagIn(node.X, into)
	case *constraint.AndExpr:
		return everyTagIn(node.Y, everyTagIn(node.X, into))
	case *constraint.OrExpr:
		return everyTagIn(node.Y, everyTagIn(node.X, into))
	}
	return into
}

// failOpen is the widest answer: every package and every feature.
func failOpen(everyTag []string) changeSet {
	return changeSet{packages: []string{everyPackage}, tags: everyTag}
}

// packageWord renders a repo-relative directory as the ./-prefixed word the make
// recipes consume. The module root is ".".
func packageWord(dir string) string {
	if dir == "" {
		return rootPackage
	}
	// path.Join would clean the "./" away, and the recipes need the prefix: a
	// bare cmd/ze reads as an import path rather than a directory pattern.
	return "./" + dir
}

// packageWords renders a seed list for an operator message. An empty list is
// said in words, so "seeds nothing" never reads as a truncated line.
func packageWords(dirs []string) string {
	if len(dirs) == 0 {
		return "no package"
	}
	words := make([]string, 0, len(dirs))
	for _, dir := range dirs {
		words = append(words, packageWord(dir))
	}
	return strings.Join(words, " ")
}

// emitReport turns the selector answer into the command's structured payload.
// A widening reason is stated on stderr and never changes the exit code.
func emitReport(opts selectorOptions, answer changeSet, widen error) (ScopeReport, int) {
	report := ScopeReport{
		Packages: answer.packages,
		Tags:     answer.tags,
		Print:    string(opts.mode),
	}
	if widen != nil {
		_, _ = fmt.Fprintf(os.Stderr, "verify-scope: %v, so every package and every feature is selected\n", widen)
		report.Widened = true
		report.Reason = widen.Error()
	}
	return report, 0
}

// The repeated git argument and path fragments this selector matches on.
const (
	gitNameOnly    = "--name-only"
	markdownSuffix = ".md"
	testTreePrefix = "test/"
)

// gitDiff is the git verb every changed-path query starts with.
const (
	gitDiff = "diff"
)
