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
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	// Blank imports trigger init() registrations for every command surface.
	// Mirrors scripts/docvalid/commands.go so BuildCommandTree sees all modules.
	_ "codeberg.org/thomas-mangin/ze/internal/component/plugin/all"

	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/cache/yang"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/commit/yang"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/monitor/yang"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/peer/yang"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/raw/yang"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/rib/yang"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/cmd/update/yang"
	_ "codeberg.org/thomas-mangin/ze/internal/component/bgp/plugins/route_refresh/yang"

	"codeberg.org/thomas-mangin/ze/internal/component/command"
	"codeberg.org/thomas-mangin/ze/internal/component/command/grammar"
	"codeberg.org/thomas-mangin/ze/internal/component/config/yang"
	"codeberg.org/thomas-mangin/ze/internal/core/textbuf"
)

type result struct {
	Findings     []grammar.Finding `json:"findings"`
	FlagInYANG   []flagHit         `json:"flag-in-yang"`
	Exempt       map[string]int    `json:"exempt-by-category"`
	Checked      int               `json:"commands-checked"`
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

type flagHit struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Text string `json:"text"`
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

	sort.Slice(res.Findings, func(i, j int) bool {
		if res.Findings[i].Command != res.Findings[j].Command {
			return res.Findings[i].Command < res.Findings[j].Command
		}
		return res.Findings[i].Rule < res.Findings[j].Rule
	})
	res.Valid = len(res.Findings) == 0 && len(res.FlagInYANG) == 0
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

func printResult(r result) {
	fmt.Fprintf(os.Stdout, "# CLI Grammar Gate\n\n")
	fmt.Fprintf(os.Stdout, "Commands checked: %d\n", r.Checked)
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

	if r.Valid {
		fmt.Fprintln(os.Stdout, "cli-grammar: OK")
	} else {
		fmt.Fprintf(os.Stdout, "cli-grammar: FAILED (%d grammar, %d flag-in-yang)\n", len(r.Findings), len(r.FlagInYANG))
	}
}
