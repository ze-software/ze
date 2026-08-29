// Design: docs/architecture/cli/command-namespacing.md -- CLI command grammar gate
//
// Package cligrammar walks the compile-time YANG command tree (every built-in
// command, including plugin -cmd modules) and checks each command against the
// reverse-engineered grammar rules R1-R8
// (internal/component/command/grammar), plus the static R3 check that no
// `--flag` appears in any .yang file. Category-exempt commands (bridge /
// wire-protocol / editor) are skipped and counted.
//
// This is Feeder 1 of the grammar gate. The plugin registration check
// (validateCommandName) is Feeder 2; the in-process runtime guard
// (TestRuntimeBuiltinSurfaceGrammar / TestRegistrationRejectsBadGrammar in
// internal/component/plugin/server) is Feeder 3. Feeder 4 is the root
// namespace and Feeder 5 is the repository's own call sites in terminal-demo
// definitions. Feeder 6 is le's own command surface, which the first five
// never saw: le registers into neither the YANG tree nor the wire methods, so
// its roots drifted into hyphenating an object to its member exactly as ze's
// roots once did.
//
// Every population this gate reads is FLOORED and every read error is
// answered. The retired implementation discarded both: it walked `internal`,
// `cmd/ze` and `demos/terminal` relative to the working directory, skipped a
// file it could not open, ignored a scanner error, and ignored a Go file it
// could not parse. Run anywhere but a checkout, or over a tree holding one
// unreadable file, it printed `cli-grammar: OK` having checked less than it
// claimed.

package cligrammar

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	// Blank imports trigger init() registrations for every command surface, so
	// BuildCommandTree sees all modules.
	_ "github.com/ze-software/ze/internal/component/plugin/all"

	_ "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/cache/yang"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/commit/yang"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/monitor/yang"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/peer/yang"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/raw/yang"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/rib/yang"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/cmd/update/yang"
	_ "github.com/ze-software/ze/internal/component/bgp/plugins/route_refresh/yang"

	"github.com/ze-software/ze/internal/component/command"
	"github.com/ze-software/ze/internal/component/command/grammar"
	"github.com/ze-software/ze/internal/component/config/yang"
	"github.com/ze-software/ze/internal/core/textbuf"
)

// pendingNamespaceSplit holds shipped commands that violate R9 (sibling
// namespace collision) but whose rename to the two-token form is not yet done.
// It is EMPTY: the cli-hyphen-namespace-split migration (2026-07-13) split
// every one. Any NEW R9 finding therefore fails the gate outright, which is the
// point. To stage a future split migration, list the command path here (value
// true) so the gate reports it as debt (PendingSplit) without blocking, then
// delete the entry when the command is renamed.
//
// These are tracked debt, NOT a permanent category exemption (those are
// structural, keyed on wire method, in grammar.ExemptCategory).
var pendingNamespaceSplit = map[string]bool{}

// rootNamespaceExempt is the root feeder's relief valve: a root whose hyphen is
// a genuine indivisible name even though its left segment happens to match a
// YANG verb or container (the R9 "as-set with no as sibling" case, applied to
// roots). It mirrors pendingNamespaceSplit (YANG tree) and
// grammar.ExemptCategory (wire methods) so the root feeder is not the one
// enforcement path with no escape.
//
// EMPTY today: every hyphenated root was split. Add an entry (value true) ONLY
// for a demonstrably-indivisible compound, with a one-line reason, so a
// legitimate future root (route-refresh, prefix-list, next-hop, as-path, ...)
// is not a hard false positive that forces editing gate source.
var rootNamespaceExempt = map[string]bool{}

// leNamespaceExempt is feeder 6's relief valve, the counterpart of
// rootNamespaceExempt for le's own surface. An entry says a hyphenated le
// command is one indivisible name even though its left segment looks like an
// object.
//
// The six test-* commands are the standing entry, and the reason is the rule's
// own trap: a shared prefix is not a namespace. test-unit and test-chaos run
// suites, test-health generates a page, test-weakened gates a ledger,
// test-sensitivity gates a count and test-helper produces fixtures. They are
// five kinds of thing sharing a word, as flow-export and flow-recent are, and
// there is no `le test` object holding them. Splitting them would also promise
// a namespace that does not hold the real suites, which are `functional`,
// `integration`, `deployment`, `qemu`, `fuzz`, `mutation` and `stress-repro`.
var leNamespaceExempt = map[string]bool{
	"test-chaos":       true,
	"test-health":      true,
	"test-helper":      true,
	"test-sensitivity": true,
	"test-unit":        true,
	"test-weakened":    true,
}

// leNamespaces answers the words that name an object on le's surface, so the
// root check can tell `verify-lint` (an object and its member) from
// `dash-stdio` (one name that happens to hold a hyphen).
//
// A word is an object when it is a registered command in its own right, as
// `verify`, `site` and `repository` are, or when two or more commands share it
// as their first hyphen segment, as `config` and `plugin` are. Deriving both
// from the live registry is what keeps the gate honest: a hand-kept list of
// namespaces would be the thing that goes stale.
func leNamespaces(roots []string) map[string]bool {
	registered := make(map[string]bool, len(roots))
	shared := make(map[string]int, len(roots))
	for _, name := range roots {
		registered[name] = true
		if left, _, found := strings.Cut(name, "-"); found {
			shared[left]++
		}
	}

	namespaces := make(map[string]bool, len(shared))
	for left, count := range shared {
		if count > 1 || registered[left] {
			namespaces[left] = true
		}
	}
	return namespaces
}

// treeNamespaceExempt is the YANG-tree feeder's relief valve, the exact
// counterpart of rootNamespaceExempt: a tree token whose hyphen is a genuine
// indivisible name even though its left segment happens to also exist as a
// sibling at that level (R9 test 2, "keep the hyphen for a protocol / LSA /
// object name", beating R9 test 3, "a shared prefix is not proof of a
// namespace"). Without it the tree feeder is the one enforcement path with no
// escape for a real compound: pendingNamespaceSplit means "scheduled for
// rename", which is the wrong claim to record about a name that is staying, and
// grammar.ExemptCategory is keyed on wire-method namespace
// (bridge/wire-protocol/editor), which a plain `ze-show:` command never
// matches.
//
// Add an entry ONLY for a demonstrably-indivisible compound, with a one-line
// reason. Entries are counted into TreeExempt and printed, so an exemption is
// reported, never silent. A NEW R9 collision that is not listed here still
// fails the gate.
//
// Note CheckSiblings tests only the LEFT hyphen segment, so which side the
// shared word falls on decides whether a name is flagged at all: `summary` /
// `asbr-summary` and `external` / `nssa-external` are the same "two distinct
// LSA types share a word" situation as `router` / `router-information` and are
// silently fine. That positional asymmetry is why this valve exists.
var treeNamespaceExempt = map[string]bool{
	// RFC 7770 names one object, the Router Information (RI) LSA -- OSPFv2 opaque
	// type 4, OSPFv3 function code 12. The `router` sibling is the Router-LSA
	// (Type 1), a different LSA type that shares the word by accident and owns
	// nothing under it; `show ospf database router information` would file an
	// Opaque LSA under Type 1. The CLI rule's R9 test 2 lists
	// `router-information` by name as a keep-the-hyphen LSA name.
	"show ospf database router-information":      true,
	"show ospf ipv6 database router-information": true,
}

// heredocOpen matches a heredoc redirect (`<<EOF`, `<<-'EOF'`, `<< "EOF"`) so
// the demo scan can skip the prose body that follows.
var heredocOpen = regexp.MustCompile(`<<-?\s*(["']?[A-Za-z_][A-Za-z0-9_]*["']?)`)

// zeGlobalFlagWithValue lists the global flags that consume the following
// token, so the scan does not mistake a flag VALUE for the command word.
// Mirrors the value-consuming cases of zeFlags parsing in
// cmd/ze/ze_core_dispatch.go.
var zeGlobalFlagWithValue = map[string]bool{
	"--plugin":    true,
	"--web":       true,
	"--mcp":       true,
	"--mcp-token": true,
	"--pprof":     true,
	"-f":          true,
}

// flagRE matches a CLI flag spelling (`--foo`) in a YANG file.
var flagRE = regexp.MustCompile(`--[a-z]`)

// containerRE matches a top-of-line YANG `container <name> {` declaration.
var containerRE = regexp.MustCompile(`^\s*container\s+([a-z][a-z0-9-]*)\s*\{`)

// Floor is the least each population may hold before the gate refuses to
// believe it read the tree.
//
// A gate that walks a directory tree can go green two ways: it found no
// violation, or it found no FILE. The script this replaces could not tell those
// apart, so a run from the wrong directory, or over a checkout missing a
// directory, printed OK. Each floor is set well under the count this checkout
// carries, so it fires on a tree that was never read rather than on a tree that
// shrank.
type Floor struct {
	// YANGFiles is the least .yang files the R3 scan must see under internal/.
	YANGFiles int
	// Roots is the least registered root handlers the AST scan must resolve.
	Roots int
	// DemoScripts is the least checked-in demo definitions the call-site scan
	// must read.
	DemoScripts int
	// LeRoots is the least registered le commands the registry scan must hold.
	LeRoots int
}

// DefaultFloor is what le passes. The counts on 2026-08-26 were 217 .yang
// files, 40 roots, and 19 terminal-demo definitions.
var DefaultFloor = Floor{YANGFiles: 100, Roots: 20, DemoScripts: 10, LeRoots: 60}

// Check walks every feeder of the grammar gate over tree and answers what it
// found.
//
// The error is a population it could not read or one that fell under its floor,
// which is a different fact from a grammar violation and never renders as a
// clean page.
// Check runs every feeder over one checkout.
//
// leRoots is the le command surface, passed in rather than read from the
// registry here. This package cannot import le's composition root, which
// blank-imports it, so a registry read from inside this package sees only
// whatever the calling binary happens to have linked: one command in its own
// test binary, and eighty-six in le. Taking the population as an argument is
// what makes the floor mean the same thing in both.
func Check(tree string, floor Floor, leRoots []string) (Result, error) {
	loader, err := yang.DefaultLoader()
	if err != nil {
		return Result{}, fmt.Errorf("load YANG: %w", err)
	}
	commandTree := yang.BuildCommandTree(loader)

	result := Result{Exempt: map[string]int{}}
	walk(commandTree, "", &result)

	flags, yangFiles, err := flagInYANG(tree)
	if err != nil {
		return Result{}, err
	}
	if yangFiles < floor.YANGFiles {
		return Result{}, fmt.Errorf("the R3 scan read %d .yang files under %s, below the floor of %d: this tree was not read", yangFiles, tree, floor.YANGFiles)
	}
	result.FlagInYANG = flags

	// Feeder 4: the root namespace. Root handlers are registered outside the
	// YANG command tree, so the walk above never sees them; a hyphenated root
	// whose left segment names a namespace on another surface (a YANG verb or
	// object container) would sit undetected -- exactly how traffic-control /
	// isis-decode / ospf-decode / update-serve stayed green for a whole
	// migration. Enumerate the registered roots from source and check them
	// against the cross-surface namespace set (verbs + containers).
	namespaces := map[string]bool{}
	for verb := range commandTree.Children {
		namespaces[verb] = true
	}
	containers, err := yangContainerNames(tree)
	if err != nil {
		return Result{}, err
	}
	for name := range containers {
		namespaces[name] = true
	}

	roots, err := registeredRootNames(tree)
	if err != nil {
		return Result{}, err
	}
	if len(roots) < floor.Roots {
		return Result{}, fmt.Errorf("the root scan resolved %d registered roots under %s, below the floor of %d: this tree was not read", len(roots), tree, floor.Roots)
	}
	result.RootsChecked = len(roots)
	for _, finding := range grammar.CheckRootNamespace(roots, namespaces) {
		if rootNamespaceExempt[finding.Command] {
			result.RootExempt++
			continue
		}
		result.Findings = append(result.Findings, finding)
	}

	// Feeder 5: the repository's own call sites. `-` is the stdin sentinel that
	// survives in zeDispatch beside the verbs and roots.
	accepted := map[string]bool{"-": true}
	for verb := range commandTree.Children {
		accepted[verb] = true
	}
	for _, name := range roots {
		accepted[name] = true
	}
	launches, scripts, err := demoLaunchHits(tree, accepted)
	if err != nil {
		return Result{}, err
	}
	if scripts < floor.DemoScripts {
		return Result{}, fmt.Errorf("the call-site scan read %d demo scripts under %s, below the floor of %d: this tree was not read", scripts, tree, floor.DemoScripts)
	}
	result.DemoLaunch, result.DemoScripts = launches, scripts

	// Feeder 6: le's own root surface. The four feeders above read ze's YANG
	// command tree, and le registers into neither the tree nor the wire
	// methods, so nothing had ever checked it. That is the same hole
	// root-namespace-grammar.md records for ze's roots, one surface along.
	if len(leRoots) < floor.LeRoots {
		return Result{}, fmt.Errorf("the le scan resolved %d registered commands, below the floor of %d: this registry was not read", len(leRoots), floor.LeRoots)
	}
	result.LeRootsChecked = len(leRoots)
	for _, finding := range grammar.CheckRootNamespace(leRoots, leNamespaces(leRoots)) {
		if leNamespaceExempt[finding.Command] {
			result.LeExempt++
			continue
		}
		result.Findings = append(result.Findings, finding)
	}

	sort.Slice(result.Findings, func(i, j int) bool {
		if result.Findings[i].Command != result.Findings[j].Command {
			return result.Findings[i].Command < result.Findings[j].Command
		}
		return result.Findings[i].Rule < result.Findings[j].Rule
	})
	result.Valid = len(result.Findings) == 0 && len(result.FlagInYANG) == 0 && len(result.DemoLaunch) == 0
	return result, nil
}

// walk checks one level of the command tree and descends.
func walk(node *command.Node, prefix string, result *Result) {
	if node == nil {
		return
	}
	// R9: sibling namespace collision, checked once per level over this node's
	// children.
	names := make([]string, 0, len(node.Children))
	for name := range node.Children {
		names = append(names, name)
	}
	for _, finding := range grammar.CheckSiblings(prefix, names) {
		if pendingNamespaceSplit[finding.Command] {
			result.PendingSplit++
			continue
		}
		if treeNamespaceExempt[finding.Command] {
			result.TreeExempt++
			continue
		}
		result.Findings = append(result.Findings, finding)
	}
	for name, child := range node.Children {
		path := name
		if prefix != "" {
			var tb textbuf.Buffer
			path = tb.Str(prefix).Byte(' ').Str(name).String()
		}
		if child.WireMethod != "" {
			if category, ok := grammar.ExemptCategory(child.WireMethod); ok {
				result.Exempt[category]++
			} else {
				result.Checked++
				result.Findings = append(result.Findings, grammar.CheckName(path)...)
				result.Findings = append(result.Findings, grammar.CheckNode(path, child)...)
			}
		}
		walk(child, path, result)
	}
}

// flagInYANG scans every .yang file under tree's internal/ for `--flag`
// spellings, which must never appear in the command model (R3). urn:/http/xml
// lines are namespace noise, not command grammar. It answers the hits and the
// number of files it read, which is what the floor is measured against.
func flagInYANG(tree string) ([]FlagHit, int, error) {
	var hits []FlagHit
	read := 0
	err := walkYANG(tree, func(rel string, scanner *bufio.Scanner) error {
		read++
		for line := 1; scanner.Scan(); line++ {
			text := scanner.Text()
			if !flagRE.MatchString(text) {
				continue
			}
			if strings.Contains(text, "urn:") || strings.Contains(text, "http") || strings.Contains(text, "xml") {
				continue
			}
			hits = append(hits, FlagHit{File: rel, Line: line, Text: strings.TrimSpace(text)})
		}
		return scanner.Err()
	})
	if err != nil {
		return nil, 0, err
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].File != hits[j].File {
			return hits[i].File < hits[j].File
		}
		return hits[i].Line < hits[j].Line
	})
	return hits, read, nil
}

// yangContainerNames answers every YANG container token name under tree's
// internal/. These are the object namespaces (traffic, isis, ospf, bgp, ...)
// that a root command's left hyphen-segment must not silently shadow. Scanning
// source rather than the built command tree keeps this independent of which
// feature build tags are on when the gate runs (isis/ospf command modules are
// feature-gated).
func yangContainerNames(tree string) (map[string]bool, error) {
	names := map[string]bool{}
	err := walkYANG(tree, func(_ string, scanner *bufio.Scanner) error {
		for scanner.Scan() {
			if match := containerRE.FindStringSubmatch(scanner.Text()); match != nil {
				names[match[1]] = true
			}
		}
		return scanner.Err()
	})
	if err != nil {
		return nil, err
	}
	return names, nil
}

// walkYANG calls read for every .yang file under tree's internal/, handing it
// the repository-relative path and an open scanner.
//
// Every error reaches the caller. A file that cannot be opened, a walk that
// cannot descend and a line too long to scan each mean the gate read less than
// its population, and the script this replaces answered all three by carrying
// on with fewer hits.
func walkYANG(tree string, read func(rel string, scanner *bufio.Scanner) error) error {
	base := filepath.Join(tree, "internal")
	return filepath.WalkDir(base, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && entry.Name() == "testdata" {
			return filepath.SkipDir
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".yang") {
			return nil
		}
		rel, relErr := filepath.Rel(tree, path)
		if relErr != nil {
			return relErr
		}
		file, openErr := os.Open(path) //nolint:gosec // repository path
		if openErr != nil {
			return openErr
		}
		defer file.Close() //nolint:errcheck // read-only scan
		return read(filepath.ToSlash(rel), bufio.NewScanner(file))
	})
}

// demoLaunchHits reports every `ze <token>` invocation in the checked-in
// terminal-demo definitions whose token is not in accepted, and the number of
// definitions it read.
//
// Feeder 5 of the grammar gate. The other feeders check how commands are
// declared; this one checks the repository's own call sites. Terminal demos
// need Docker and run from `./le terminal-demo`, so the pre-commit gate does not
// execute them. When `ze <config-file>` was removed in favor of
// `ze start <config-file>`, thirteen demos kept the dead form and the deploy
// workflow stayed red for four days.
func demoLaunchHits(tree string, accepted map[string]bool) ([]DemoLaunchHit, int, error) {
	var files []string
	for _, pattern := range []string{
		filepath.Join(tree, "demos", "terminal", "*", "*.tape"),
		filepath.Join(tree, "demos", "terminal", "*", "*.cjs"),
		filepath.Join(tree, "demos", "terminal", "*.tape"),
	} {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			return nil, 0, err
		}
		files = append(files, matches...)
	}
	sort.Strings(files)

	var hits []DemoLaunchHit
	for _, path := range files {
		rel, relErr := filepath.Rel(tree, path)
		if relErr != nil {
			return nil, 0, relErr
		}
		found, scanErr := scanDemoScript(path, filepath.ToSlash(rel), accepted)
		if scanErr != nil {
			// An unread definition is an unchecked call site, which is the
			// reason this feeder exists.
			return nil, 0, scanErr
		}
		hits = append(hits, found...)
	}
	return hits, len(files), nil
}

// scanDemoScript reports every dead launch form in one demo definition.
func scanDemoScript(path, rel string, accepted map[string]bool) ([]DemoLaunchHit, error) {
	file, err := os.Open(path) //nolint:gosec // repository path
	if err != nil {
		return nil, err
	}
	defer file.Close() //nolint:errcheck // read-only scan

	var hits []DemoLaunchHit
	scanner := bufio.NewScanner(file)
	heredoc := "" // terminator while inside a heredoc body, "" otherwise
	for line := 1; scanner.Scan(); line++ {
		text := scanner.Text()
		trimmed := strings.TrimSpace(text)
		// Heredoc bodies are prose, not commands: cards.sh narrates "a fresh
		// ZeFS database with ze init, list and validate ...". Tokenising that
		// yields `init,` and a false positive.
		if heredoc != "" {
			if trimmed == heredoc {
				heredoc = ""
			}
			continue
		}
		if match := heredocOpen.FindStringSubmatch(text); match != nil {
			heredoc = strings.Trim(match[1], `"'`)
			continue
		}
		if strings.HasPrefix(trimmed, "#") {
			continue
		}
		command := text
		if rest, ok := strings.CutPrefix(trimmed, "Type "); ok {
			quoted := strings.TrimSpace(rest)
			unquoted, unquoteErr := strconv.Unquote(quoted)
			if unquoteErr != nil {
				return nil, fmt.Errorf("read %s:%d: invalid Type command: %w", rel, line, unquoteErr)
			}
			command = unquoted
		}
		for _, token := range launchTokens(strings.Fields(command)) {
			if accepted[token] {
				continue
			}
			hits = append(hits, DemoLaunchHit{File: rel, Line: line, Token: token})
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", rel, err)
	}
	return hits, nil
}

// launchTokens answers the command word of every `ze ...` invocation on one
// line. A bare `ze`, a shell redirect, and a variable the static scan cannot
// resolve are each no launch-form claim and answer nothing.
func launchTokens(fields []string) []string {
	var tokens []string
	for i, field := range fields {
		if field != "ze" {
			continue
		}
		token := ""
		for j := i + 1; j < len(fields); j++ {
			word := fields[j]
			if zeGlobalFlagWithValue[word] {
				j++ // this flag's value, never the command word
				continue
			}
			if strings.HasPrefix(word, "-") && word != "-" {
				continue // value-less global flag
			}
			token = word
			break
		}
		if token == "" || strings.HasPrefix(token, ">") || strings.HasPrefix(token, "$") ||
			token == "|" || token == "||" || token == "&&" || token == ";" {
			continue
		}
		tokens = append(tokens, token)
	}
	return tokens
}

// registeredRootNames answers the STRING-LITERAL first argument of every
// registry.RegisterRootHandler / MustRegisterRootHandler / RegisterRoot call
// across tree's cmd/ze and internal/. Root handlers register in package main
// (cmd/ze) or in internal owner packages and never reach the YANG-tree walk, so
// the gate enumerates them from source -- build-tag independent, mirroring the
// command-ownership gate.
//
// LIMITATION (shared with internal/le/commandownership): this is a static AST scan,
// so a root registered with a NON-LITERAL name -- e.g. a variable passed through
// a one-line helper like internal/test/cli's registerRoot(name, ...) -- is not
// resolved and is not checked. Every real `ze` CLI root is registered with a
// literal name, so all of them are covered; the invisible cases today are the
// `ze-test <suite>` roots (SectionTest, a different binary and surface). If a
// future `ze` root is added through a name-variable wrapper, it escapes this
// feeder.
//
// Local metas (RegisterLocalMeta) are deliberately excluded: `update serve` is a
// two-token local path, not a compound root.
func registeredRootNames(tree string) ([]string, error) {
	var names []string
	for _, dir := range []string{filepath.Join("cmd", "ze"), "internal"} {
		err := filepath.WalkDir(filepath.Join(tree, dir), func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() && entry.Name() == "testdata" {
				return filepath.SkipDir
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			found, parseErr := rootNamesIn(path)
			if parseErr != nil {
				return parseErr
			}
			names = append(names, found...)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return names, nil
}

// rootNamesIn answers the first string-literal argument of every
// registry.RegisterRoot / (Must)RegisterRootHandler call in one file.
//
// A file the parser cannot read is an ERROR rather than an empty answer: a root
// this scan never resolved is a root the namespace feeder never checked, and
// the script this replaces returned silently on exactly that path.
func rootNamesIn(path string) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	var names []string
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := selector.X.(*ast.Ident)
		if !ok || (pkg.Name != "registry" && pkg.Name != "cmdregistry") {
			return true
		}
		switch selector.Sel.Name {
		case "RegisterRootHandler", "MustRegisterRootHandler", "RegisterRoot":
		default:
			return true
		}
		if len(call.Args) == 0 {
			return true
		}
		lit, ok := call.Args[0].(*ast.BasicLit)
		if !ok || lit.Kind != token.STRING {
			return true
		}
		if name, unquoteErr := strconv.Unquote(lit.Value); unquoteErr == nil {
			names = append(names, name)
		}
		return true
	})
	return names, nil
}
