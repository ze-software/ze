// Design: docs/architecture/cli/command-namespacing.md -- CLI command grammar gate
//
// cli_grammar walks the compile-time YANG command tree (every built-in command,
// including plugin -cmd modules) and checks each command against the reverse-
// engineered grammar rules R1-R8 (internal/component/command/grammar), plus the
// static R3 check that no `--flag` appears in any .yang file. Category-exempt
// commands (bridge / wire-protocol / editor) are skipped and counted.
//
// This is Feeder 1 of the grammar gate (ai/rules/cli-grammar.md). The plugin
// registration check (validateCommandName) is Feeder 2; the in-process runtime
// guard (TestRuntimeBuiltinSurfaceGrammar / TestRegistrationRejectsBadGrammar in
// internal/component/plugin/server) is Feeder 3.
//
// Usage:   go run scripts/checks/cli_grammar.go [--json]
// Called by: make ze-cli-grammar-check
//
//go:build ignore

package main

import (
	"bufio"
	"encoding/json"
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

	// Blank imports trigger init() registrations for every command surface.
	// Mirrors scripts/docvalid/commands.go so BuildCommandTree sees all modules.
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

type result struct {
	Findings     []grammar.Finding `json:"findings"`
	FlagInYANG   []flagHit         `json:"flag-in-yang"`
	DemoLaunch   []demoLaunchHit   `json:"demo-launch"`
	Exempt       map[string]int    `json:"exempt-by-category"`
	Checked      int               `json:"commands-checked"`
	DemoScripts  int               `json:"demo-scripts-checked"`
	RootsChecked int               `json:"roots-checked"`
	RootExempt   int               `json:"root-namespace-exempt"`
	TreeExempt   int               `json:"tree-namespace-exempt"`
	PendingSplit int               `json:"pending-namespace-split"`
	Valid        bool              `json:"valid"`
}

// pendingNamespaceSplit lists command paths that violate R9 (sibling namespace
// collision, ai/rules/cli-grammar.md "Compound Token vs Namespace Split") but are
// already shipped and are scheduled for the agreed rename migration (split the
// hyphenated member into `namespace member`). They are tracked debt, NOT a permanent
// category exemption (those are structural, keyed on wire method, in
// grammar.ExemptCategory). Each entry is suppressed from the blocking findings and
// counted into PendingSplit so it is reported, never silent. Remove an entry the moment
// its command is migrated (new path + deprecated alias). A NEW R9 collision that is not
// in this list fails the gate, which is the point: the debt only shrinks.
// pendingNamespaceSplit holds shipped commands that violate R9 (sibling namespace
// collision) but whose rename to the two-token form is not yet done. It is EMPTY: the
// cli-hyphen-namespace-split migration (2026-07-13) split every one. Any NEW R9 finding
// therefore fails the gate outright, which is the point. To stage a future split
// migration, list the command path here (value true) so the gate reports it as debt
// (PendingSplit) without blocking, then delete the entry when the command is renamed.
var pendingNamespaceSplit = map[string]bool{}

// rootNamespaceExempt is the root feeder's relief valve: a root whose hyphen is a
// genuine indivisible name even though its left segment happens to match a YANG
// verb or container (the R9 "as-set with no as sibling" case, applied to roots).
// It mirrors pendingNamespaceSplit (YANG tree) and grammar.ExemptCategory (wire
// methods) so the root feeder is not the one enforcement path with no escape.
// EMPTY today: every hyphenated root was split. Add an entry (value true) ONLY
// for a demonstrably-indivisible compound, with a one-line reason, so a
// legitimate future root (route-refresh, prefix-list, next-hop, as-path, ...) is
// not a hard false positive that forces editing gate source.
var rootNamespaceExempt = map[string]bool{}

// treeNamespaceExempt is the YANG-tree feeder's relief valve, the exact counterpart of
// rootNamespaceExempt: a tree token whose hyphen is a genuine indivisible name even
// though its left segment happens to also exist as a sibling at that level (R9 test 2,
// "keep the hyphen for a protocol / LSA / object name", beating R9 test 3, "a shared
// prefix is not proof of a namespace"). Without it the tree feeder is the one
// enforcement path with no escape for a real compound: pendingNamespaceSplit means
// "scheduled for rename", which is the wrong claim to record about a name that is
// staying, and grammar.ExemptCategory is keyed on wire-method namespace
// (bridge/wire-protocol/editor), which a plain `ze-show:` command never matches.
//
// Add an entry ONLY for a demonstrably-indivisible compound, with a one-line reason.
// Entries are counted into TreeExempt and printed, so an exemption is reported, never
// silent. A NEW R9 collision that is not listed here still fails the gate.
//
// Note CheckSiblings tests only the LEFT hyphen segment, so which side the shared word
// falls on decides whether a name is flagged at all: `summary` / `asbr-summary` and
// `external` / `nssa-external` are the same "two distinct LSA types share a word"
// situation as `router` / `router-information` and are silently fine. That positional
// asymmetry is why this valve exists.
var treeNamespaceExempt = map[string]bool{
	// RFC 7770 names one object, the Router Information (RI) LSA -- OSPFv2 opaque
	// type 4, OSPFv3 function code 12. The `router` sibling is the Router-LSA
	// (Type 1), a different LSA type that shares the word by accident and owns
	// nothing under it; `show ospf database router information` would file an
	// Opaque LSA under Type 1. ai/rules/cli-grammar.md R9 test 2 lists
	// `router-information` by name as a keep-the-hyphen LSA name.
	"show ospf database router-information":      true,
	"show ospf ipv6 database router-information": true,
}

type flagHit struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Text string `json:"text"`
}

// demoLaunchHit is a `ze ...` invocation in a checked-in demo script whose
// position-1 token is not a command the dispatcher accepts.
type demoLaunchHit struct {
	File  string `json:"file"`
	Line  int    `json:"line"`
	Token string `json:"token"`
}

// heredocOpen matches a heredoc redirect (`<<EOF`, `<<-'EOF'`, `<< "EOF"`) so
// the demo scan can skip the prose body that follows.
var heredocOpen = regexp.MustCompile(`<<-?\s*(["']?[A-Za-z_][A-Za-z0-9_]*["']?)`)

// zeGlobalFlagWithValue lists the global flags that consume the following token,
// so the scan does not mistake a flag VALUE for the command word. Mirrors the
// value-consuming cases of zeFlags parsing in cmd/ze/ze_core_dispatch.go.
var zeGlobalFlagWithValue = map[string]bool{
	"--plugin":    true,
	"--web":       true,
	"--mcp":       true,
	"--mcp-token": true,
	"--pprof":     true,
	"-f":          true,
}

func main() {
	jsonOut := false
	for _, a := range os.Args[1:] {
		if a == "--json" {
			jsonOut = true
		}
	}

	res := run()

	if jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(res)
	} else {
		printResult(res)
	}
	if !res.Valid {
		os.Exit(1)
	}
}

func run() result {
	loader, err := yang.DefaultLoader()
	if err != nil {
		fmt.Fprintln(os.Stderr, "cli-grammar: load YANG:", err)
		os.Exit(2)
	}
	tree := yang.BuildCommandTree(loader)

	res := result{Exempt: map[string]int{}}
	walk(tree, "", &res)

	res.FlagInYANG = flagInYANG()

	// Feeder 4: the root namespace (ai/rules/cli-grammar.md). Root handlers are
	// registered outside the YANG command tree, so the walk above never sees them;
	// a hyphenated root whose left segment names a namespace on another surface
	// (a YANG verb or object container) would sit undetected -- exactly how
	// traffic-control / isis-decode / ospf-decode / update-serve stayed green for
	// a whole migration. Enumerate the registered roots from source and check them
	// against the cross-surface namespace set (verbs + containers).
	namespaces := map[string]bool{}
	for verb := range tree.Children {
		namespaces[verb] = true
	}
	for name := range yangContainerNames() {
		namespaces[name] = true
	}
	roots := registeredRootNames()
	res.RootsChecked = len(roots)
	for _, f := range grammar.CheckRootNamespace(roots, namespaces) {
		if rootNamespaceExempt[f.Command] {
			res.RootExempt++
			continue
		}
		res.Findings = append(res.Findings, f)
	}

	// Feeder 5: the repo's own call sites. `-` is the stdin sentinel that
	// survives in zeDispatch beside the verbs and roots.
	accepted := map[string]bool{"-": true}
	for verb := range tree.Children {
		accepted[verb] = true
	}
	for _, name := range roots {
		accepted[name] = true
	}
	res.DemoLaunch, res.DemoScripts = demoLaunchHits(accepted)

	sort.Slice(res.Findings, func(i, j int) bool {
		if res.Findings[i].Command != res.Findings[j].Command {
			return res.Findings[i].Command < res.Findings[j].Command
		}
		return res.Findings[i].Rule < res.Findings[j].Rule
	})
	res.Valid = len(res.Findings) == 0 && len(res.FlagInYANG) == 0 && len(res.DemoLaunch) == 0
	return res
}

func walk(node *command.Node, prefix string, res *result) {
	if node == nil {
		return
	}
	// R9: sibling namespace collision, checked once per level over this node's children.
	names := make([]string, 0, len(node.Children))
	for name := range node.Children {
		names = append(names, name)
	}
	for _, f := range grammar.CheckSiblings(prefix, names) {
		if pendingNamespaceSplit[f.Command] {
			res.PendingSplit++
			continue
		}
		if treeNamespaceExempt[f.Command] {
			res.TreeExempt++
			continue
		}
		res.Findings = append(res.Findings, f)
	}
	for name, child := range node.Children {
		path := name
		if prefix != "" {
			var tb textbuf.Buffer
			path = tb.Str(prefix).Byte(' ').Str(name).String()
		}
		if child.WireMethod != "" {
			if cat, ok := grammar.ExemptCategory(child.WireMethod); ok {
				res.Exempt[cat]++
			} else {
				res.Checked++
				res.Findings = append(res.Findings, grammar.CheckName(path)...)
				res.Findings = append(res.Findings, grammar.CheckNode(path, child)...)
			}
		}
		walk(child, path, res)
	}
}

// flagRe matches a CLI flag spelling (`--foo`) in a YANG file.
var flagRe = regexp.MustCompile(`--[a-z]`)

// flagInYANG scans every .yang file under internal/ for `--flag` spellings, which
// must never appear in the command model (R3). urn:/http/xml lines are namespace
// noise, not command grammar.
func flagInYANG() []flagHit {
	var hits []flagHit
	_ = filepath.Walk("internal", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".yang") {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		ln := 0
		for sc.Scan() {
			ln++
			line := sc.Text()
			if !flagRe.MatchString(line) {
				continue
			}
			if strings.Contains(line, "urn:") || strings.Contains(line, "http") || strings.Contains(line, "xml") {
				continue
			}
			hits = append(hits, flagHit{File: path, Line: ln, Text: strings.TrimSpace(line)})
		}
		_ = sc.Err() // best-effort scan; a read error just yields fewer hits
		return nil
	})
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].File != hits[j].File {
			return hits[i].File < hits[j].File
		}
		return hits[i].Line < hits[j].Line
	})
	return hits
}

// demoLaunchHits reports every `ze <token>` invocation in the checked-in demo
// scripts whose token is not in `accepted`.
//
// Feeder 5 of the grammar gate. The other feeders check how commands are
// DECLARED; this one checks the repo's own CALL SITES, because nothing else
// does: `make ze-verify` never executes demos/terminal (they need Docker + VHS,
// they run from mk/terminal-demo.mk at release time and from the website
// workflow). When `ze <config-file>` was removed in favour of
// `ze start <config-file>`, thirteen demo scripts kept the dead form and the
// Deploy website job failed at "Generate terminal media" on every push for four
// days before anyone read the log.
func demoLaunchHits(accepted map[string]bool) ([]demoLaunchHit, int) {
	files, _ := filepath.Glob("demos/terminal/*/*.sh")
	top, _ := filepath.Glob("demos/terminal/*.sh")
	files = append(files, top...)
	sort.Strings(files)

	var hits []demoLaunchHit
	for _, path := range files {
		fh, err := os.Open(path)
		if err != nil {
			continue
		}
		scanner := bufio.NewScanner(fh)
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
			if m := heredocOpen.FindStringSubmatch(text); m != nil {
				heredoc = strings.Trim(m[1], `"'`)
				continue
			}
			if strings.HasPrefix(trimmed, "#") {
				continue
			}
			fields := strings.Fields(text)
			for i, f := range fields {
				if f != "ze" {
					continue
				}
				token := ""
				for j := i + 1; j < len(fields); j++ {
					w := fields[j]
					if zeGlobalFlagWithValue[w] {
						j++ // this flag's value, never the command word
						continue
					}
					if strings.HasPrefix(w, "-") && w != "-" {
						continue // value-less global flag
					}
					token = w
					break
				}
				// No token (a bare `ze`), a shell redirect, or a variable the
				// static scan cannot resolve: not a launch-form claim.
				if token == "" || strings.HasPrefix(token, ">") || strings.HasPrefix(token, "$") {
					continue
				}
				if accepted[token] {
					continue
				}
				hits = append(hits, demoLaunchHit{File: path, Line: line, Token: token})
			}
		}
		if err := scanner.Err(); err != nil {
			fmt.Fprintf(os.Stderr, "cli-grammar: read %s: %v\n", path, err)
			os.Exit(2) // fail closed: an unread script is an unchecked call site
		}
		fh.Close() //nolint:errcheck // read-only scan
	}
	return hits, len(files)
}

// registeredRootNames returns the STRING-LITERAL first argument of every
// registry.RegisterRootHandler / MustRegisterRootHandler / RegisterRoot call
// across cmd/ze and internal/. Root handlers register in package main (cmd/ze)
// or in internal owner packages and never reach the YANG-tree walk, so the gate
// enumerates them from source -- build-tag independent, mirroring
// scripts/checks/command_ownership.go.
//
// LIMITATION (shared with command_ownership.go): this is a static AST scan, so a
// root registered with a NON-LITERAL name -- e.g. a variable passed through a
// one-line helper like internal/test/cli's registerRoot(name, ...) -- is not
// resolved and is not checked. Every real `ze` CLI root is registered with a
// literal name (grep confirms), so all of them are covered; the invisible cases
// today are the `ze-test <suite>` roots (SectionTest, a different binary and
// surface). If a future `ze` root is added through a name-variable wrapper, it
// escapes this feeder -- a `_test.go` asserting the real roots stay literal, or
// resolving the wrapper's literal arg, would close that gap.
//
// Local metas (RegisterLocalMeta) are deliberately excluded: `update serve` is a
// two-token local path, not a compound root.
func registeredRootNames() []string {
	var names []string
	for _, dir := range []string{"cmd/ze", "internal"} {
		_ = filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			forEachRegistryRootName(path, func(name string) { names = append(names, name) })
			return nil
		})
	}
	return names
}

// forEachRegistryRootName invokes fn with the first string-literal argument of
// every registry.RegisterRoot / (Must)RegisterRootHandler call in the file.
func forEachRegistryRootName(path string, fn func(name string)) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return
	}
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return true
		}
		pkg, ok := sel.X.(*ast.Ident)
		if !ok || (pkg.Name != "registry" && pkg.Name != "cmdregistry") {
			return true
		}
		switch sel.Sel.Name {
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
		if name, err := strconv.Unquote(lit.Value); err == nil {
			fn(name)
		}
		return true
	})
}

// containerRe matches a top-of-line YANG `container <name> {` declaration.
var containerRe = regexp.MustCompile(`^\s*container\s+([a-z][a-z0-9-]*)\s*\{`)

// yangContainerNames returns every YANG container token name under internal/.
// These are the object namespaces (traffic, isis, ospf, bgp, ...) that a root
// command's left hyphen-segment must not silently shadow. Scanning source rather
// than the built command tree keeps this independent of which feature build tags
// are on when the gate runs (isis/ospf command modules are feature-gated).
func yangContainerNames() map[string]bool {
	names := map[string]bool{}
	_ = filepath.Walk("internal", func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() || !strings.HasSuffix(path, ".yang") {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer f.Close()
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if m := containerRe.FindStringSubmatch(sc.Text()); m != nil {
				names[m[1]] = true
			}
		}
		_ = sc.Err() // best-effort scan; a read error just yields fewer names
		return nil
	})
	return names
}

func printResult(r result) {
	fmt.Fprintf(os.Stdout, "# CLI Grammar Gate\n\n")
	fmt.Fprintf(os.Stdout, "Commands checked: %d\n", r.Checked)
	fmt.Fprintf(os.Stdout, "Roots checked: %d\n", r.RootsChecked)
	if r.RootExempt > 0 {
		fmt.Fprintf(os.Stdout, "Root namespace-exempt (indivisible compounds): %d\n", r.RootExempt)
	}
	if r.TreeExempt > 0 {
		fmt.Fprintf(os.Stdout, "Tree namespace-exempt (indivisible compounds): %d\n", r.TreeExempt)
	}
	cats := make([]string, 0, len(r.Exempt))
	for c := range r.Exempt {
		cats = append(cats, c)
	}
	sort.Strings(cats)
	for _, c := range cats {
		fmt.Fprintf(os.Stdout, "Exempt (%s): %d\n", c, r.Exempt[c])
	}
	if r.PendingSplit > 0 {
		fmt.Fprintf(os.Stdout, "Pending namespace-split (R9 debt, tracked for rename migration): %d\n", r.PendingSplit)
	}
	fmt.Fprintln(os.Stdout)

	if len(r.Findings) > 0 {
		fmt.Fprintf(os.Stdout, "## Grammar violations (%d)\n\n", len(r.Findings))
		for _, f := range r.Findings {
			fmt.Fprintf(os.Stdout, "  [%s] %s\n        %s\n", f.Rule, f.Command, f.Message)
		}
		fmt.Fprintln(os.Stdout)
	}
	if len(r.FlagInYANG) > 0 {
		fmt.Fprintf(os.Stdout, "## --flag in YANG (%d)\n\n", len(r.FlagInYANG))
		for _, h := range r.FlagInYANG {
			fmt.Fprintf(os.Stdout, "  %s:%d  %s\n", h.File, h.Line, h.Text)
		}
		fmt.Fprintln(os.Stdout)
	}

	if len(r.DemoLaunch) > 0 {
		fmt.Fprintf(os.Stdout, "## Dead launch form in demo scripts (%d)\n\n", len(r.DemoLaunch))
		for _, h := range r.DemoLaunch {
			fmt.Fprintf(os.Stdout, "  %s:%d  `ze %s` -- %q is not a verb or a registered root\n", h.File, h.Line, h.Token, h.Token)
		}
		fmt.Fprintln(os.Stdout)
	}

	if r.Valid {
		fmt.Fprintln(os.Stdout, "cli-grammar: OK")
	} else {
		fmt.Fprintf(os.Stdout, "cli-grammar: FAILED (%d grammar, %d flag-in-yang, %d demo-launch)\n",
			len(r.Findings), len(r.FlagInYANG), len(r.DemoLaunch))
	}
}
