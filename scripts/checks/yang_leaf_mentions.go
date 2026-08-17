// Design: docs/architecture/config/yang-config-design.md -- config leaf consumption
// Related: internal/component/config/claims -- the blocking root-claim gate this complements
//
// yang_leaf_mentions reports YANG config leaves that the owning plugin package
// never names.
//
// The blocking gate (internal/component/plugin/all/config_claims_test.go) proves
// every config subtree is DELIVERED to a plugin. It cannot prove the plugin
// READS what it receives: plugins hand-parse map[string]any with string-literal
// keys, so delivery and consumption are separate facts. A leaf added to a YANG
// module without the matching parse line is accepted, delivered, and ignored.
//
// The signal is a heuristic. A leaf name is "mentioned" when its kebab spelling
// appears as a string literal in the owning package or its yang/ sibling. A key
// built at run time, a name shared with an unrelated string, or a leaf read
// through a shared helper all break it. So this check REPORTS and always exits
// 0. It is deliberately absent from every verify stage; the ratchet that keeps
// it honest is its self-test.
//
// Usage:   CGO_ENABLED=0 go run scripts/checks/yang_leaf_mentions.go [--json] [--selftest]
// Called by: make ze-yang-leaf-mentions-report (advisory; not a verify stage)
//
//go:build ignore

package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// leafRE matches a `leaf <name> {` or `leaf-list <name> {` statement.
var leafRE = regexp.MustCompile(`(?m)^[\t ]*leaf(?:-list)?[\t ]+([a-z0-9][a-z0-9-]*)[\t ]*\{`)

// containerRE matches the statements that open a named level in the data tree.
var containerRE = regexp.MustCompile(`(?m)^[\t ]*(?:container|list|grouping)[\t ]+([a-z0-9][a-z0-9-]*)[\t ]*\{`)

// keyRE matches a list's `key "name";` statement. A key leaf carries the list
// entry's name, which arrives as the MAP KEY in the delivered config, so no
// plugin ever names it as a literal. Reporting one is always noise.
var keyRE = regexp.MustCompile(`(?m)^[\t ]*key[\t ]+"([^"]+)"`)

type finding struct {
	Module  string `json:"module"`
	Package string `json:"package"`
	Leaf    string `json:"leaf"`
	Path    string `json:"path"`
}

type report struct {
	Modules  int       `json:"modules"`
	Leaves   int       `json:"leaves"`
	Findings []finding `json:"findings"`
}

func main() {
	jsonOut := flag.Bool("json", false, "emit the report as JSON")
	selftest := flag.Bool("selftest", false, "run the built-in fixture and exit")
	flag.Parse()

	if *selftest {
		if err := runSelftest(); err != nil {
			fmt.Fprintf(os.Stderr, "yang-leaf-mentions: selftest FAILED: %v\n", err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stdout, "yang-leaf-mentions: selftest OK")
		return
	}

	rep, err := scanTree(".")
	if err != nil {
		fmt.Fprintf(os.Stderr, "yang-leaf-mentions: %v\n", err)
		os.Exit(1)
	}

	if *jsonOut {
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(rep); err != nil {
			fmt.Fprintf(os.Stderr, "yang-leaf-mentions: encode: %v\n", err)
			os.Exit(1)
		}
		return
	}

	fmt.Fprintf(os.Stdout, "yang-leaf-mentions: %d config modules, %d leaves, %d never named by the owning package\n",
		rep.Modules, rep.Leaves, len(rep.Findings))
	fmt.Fprintln(os.Stdout, "ADVISORY. A finding is a candidate to read, not a defect: a key built at")
	fmt.Fprintln(os.Stdout, "run time, or read through a shared helper, is reported the same way.")
	for _, f := range rep.Findings {
		fmt.Fprintf(os.Stdout, "  %-28s %-46s %s\n", f.Module, f.Path, f.Package)
	}
}

// scanTree walks every config-schema module under internal/ and reports the
// leaves the owning package never names.
//
// Owner resolution is structural, never a table: scripts/codegen/yang_glue.go
// discovers a module by its `<pkg>/yang/` directory, so the owning package is
// that directory's parent.
func scanTree(root string) (report, error) {
	var rep report

	yangDirs, err := discoverYangDirs(root)
	if err != nil {
		return rep, err
	}

	for _, dir := range yangDirs {
		owner := filepath.Dir(dir)
		literals, err := packageLiterals(owner)
		if err != nil {
			return rep, err
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			return rep, err
		}
		for _, e := range entries {
			if e.IsDir() || !strings.HasSuffix(e.Name(), "-conf.yang") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, e.Name())) //nolint:gosec // repository path
			if err != nil {
				return rep, err
			}
			module := strings.TrimSuffix(e.Name(), ".yang")
			leaves := parseLeaves(string(data))
			rep.Modules++
			rep.Leaves += len(leaves)
			for _, lf := range leaves {
				if literals[lf.name] {
					continue
				}
				rep.Findings = append(rep.Findings, finding{
					Module:  module,
					Package: filepath.ToSlash(owner),
					Leaf:    lf.name,
					Path:    lf.path,
				})
			}
		}
	}

	sort.Slice(rep.Findings, func(i, j int) bool {
		if rep.Findings[i].Module != rep.Findings[j].Module {
			return rep.Findings[i].Module < rep.Findings[j].Module
		}
		return rep.Findings[i].Path < rep.Findings[j].Path
	})
	return rep, nil
}

// discoverYangDirs returns every `yang/` directory under internal/ that holds a
// config-schema module.
func discoverYangDirs(root string) ([]string, error) {
	var dirs []string
	base := filepath.Join(root, "internal")
	err := filepath.WalkDir(base, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() || d.Name() != "yang" {
			return nil
		}
		entries, readErr := os.ReadDir(path)
		if readErr != nil {
			return readErr
		}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), "-conf.yang") {
				dirs = append(dirs, path)
				break
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(dirs)
	return dirs, nil
}

type yangLeaf struct {
	name string
	path string
}

type markerKind int

const (
	markerLeaf markerKind = iota
	markerDir
	markerKey
)

type yangMarker struct {
	pos  int
	kind markerKind
	name string
}

type yangLevel struct {
	name  string
	depth int
	keys  map[string]bool
}

// parseLeaves returns the config leaves of a YANG module with the container
// path each sits under.
//
// The module text is read rather than the resolved schema, because this check
// runs from `go run` with no build tags and no registry. A resolved walk would
// need the module's package linked in, which is exactly what disappears under a
// reduced tag set.
func parseLeaves(text string) []yangLeaf {
	var markers []yangMarker
	for _, m := range containerRE.FindAllStringSubmatchIndex(text, -1) {
		markers = append(markers, yangMarker{pos: m[0], kind: markerDir, name: text[m[2]:m[3]]})
	}
	for _, m := range leafRE.FindAllStringSubmatchIndex(text, -1) {
		markers = append(markers, yangMarker{pos: m[0], kind: markerLeaf, name: text[m[2]:m[3]]})
	}
	for _, m := range keyRE.FindAllStringSubmatchIndex(text, -1) {
		markers = append(markers, yangMarker{pos: m[0], kind: markerKey, name: text[m[2]:m[3]]})
	}
	sort.Slice(markers, func(i, j int) bool { return markers[i].pos < markers[j].pos })

	// Brace depth gives the enclosing containers. It is counted over the raw
	// text between markers, which is enough for a name path: a wrong path only
	// changes the label on a finding, never whether it is one.
	var stack []yangLevel
	var out []yangLeaf
	depth, prev := 0, 0
	seen := make(map[string]bool)

	for _, mk := range markers {
		depth += strings.Count(text[prev:mk.pos], "{") - strings.Count(text[prev:mk.pos], "}")
		prev = mk.pos
		for len(stack) > 0 && stack[len(stack)-1].depth >= depth {
			stack = stack[:len(stack)-1]
		}
		switch mk.kind {
		case markerDir:
			stack = append(stack, yangLevel{name: mk.name, depth: depth, keys: map[string]bool{}})
			continue
		case markerKey:
			if len(stack) > 0 {
				for _, k := range strings.Fields(mk.name) {
					stack[len(stack)-1].keys[k] = true
				}
			}
			continue
		case markerLeaf:
		}
		if len(stack) > 0 && stack[len(stack)-1].keys[mk.name] {
			continue // list key leaf: its value is the entry name, never a literal
		}
		parts := make([]string, 0, len(stack)+1)
		for _, s := range stack {
			parts = append(parts, s.name)
		}
		parts = append(parts, mk.name)
		path := strings.Join(parts, "/")
		if seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, yangLeaf{name: mk.name, path: path})
	}
	return out
}

// packageLiterals returns every string literal in the owner directory's
// non-test Go files, plus the parts of every literal that holds a separator. A
// leaf read as `cfg["hold-time"]` and a leaf named inside a path constant
// "bgp/hold-time" both count as mentions.
//
// The walk is RECURSIVE. A component keeps its config parsing in a subpackage
// (BGP parses in internal/component/bgp/config, not in internal/component/bgp),
// so a shallow scan reported every BGP leaf as unread: 235 findings, of which
// the whole BGP block was noise.
func packageLiterals(owner string) (map[string]bool, error) {
	out := make(map[string]bool)
	fset := token.NewFileSet()

	err := filepath.WalkDir(owner, func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return nil // a file this parser cannot read hides nothing it could have found
		}
		ast.Inspect(file, func(n ast.Node) bool {
			lit, ok := n.(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				return true
			}
			value := strings.Trim(lit.Value, "`\"")
			out[value] = true
			for _, part := range strings.FieldsFunc(value, isLiteralSeparator) {
				out[part] = true
			}
			return true
		})
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// isLiteralSeparator splits a literal into the names it can carry. The quote
// characters matter: a config field read through a struct tag appears in the
// AST as the backquoted literal `json:"add-path"`, so without them the whole
// mrt module read as unconsumed.
func isLiteralSeparator(r rune) bool {
	switch r {
	case '/', '.', ' ', ':', ',', '=', '"', '\'', '`', '{', '}':
		return true
	default:
		return false
	}
}

const fixtureYANG = `module ze-fixture-conf {
    container fixture {
        leaf read-leaf { type string; }
        leaf never-read-leaf { type string; }
        container inner {
            leaf-list also-read { type string; }
        }
    }
}`

const fixtureGo = `package fixture

func extract(m map[string]any) (string, []string) {
	v, _ := m["read-leaf"].(string)
	path := "fixture/inner/also-read"
	_ = path
	return v, nil
}`

// runSelftest scans a fixture whose answer is known.
//
// Two properties are asserted, and the second is the one that matters: an
// always-report check and an always-silent check both look like a working
// heuristic until something is known to be consumed.
func runSelftest() error {
	dir, err := os.MkdirTemp("", "yang-leaf-mentions-selftest")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir) //nolint:errcheck // temp fixture

	owner := filepath.Join(dir, "internal", "plugins", "fixture")
	yangDir := filepath.Join(owner, "yang")
	if err := os.MkdirAll(yangDir, 0o750); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(yangDir, "ze-fixture-conf.yang"), []byte(fixtureYANG), 0o600); err != nil {
		return err
	}
	if err := os.WriteFile(filepath.Join(owner, "config.go"), []byte(fixtureGo), 0o600); err != nil {
		return err
	}

	rep, err := scanTree(dir)
	if err != nil {
		return err
	}
	if rep.Modules != 1 {
		return fmt.Errorf("want 1 fixture module scanned, got %d: discovery no longer finds a <pkg>/yang/<stem>-conf.yang", rep.Modules)
	}
	if rep.Leaves != 3 {
		return fmt.Errorf("want 3 fixture leaves parsed, got %d: the leaf parser no longer matches leaf and leaf-list statements", rep.Leaves)
	}

	got := make(map[string]string, len(rep.Findings))
	for _, f := range rep.Findings {
		got[f.Leaf] = f.Path
	}
	path, reported := got["never-read-leaf"]
	if !reported {
		return fmt.Errorf("the unread fixture leaf was not reported: findings %+v", rep.Findings)
	}
	if path != "fixture/never-read-leaf" {
		return fmt.Errorf("want leaf path %q, got %q", "fixture/never-read-leaf", path)
	}
	if _, ok := got["read-leaf"]; ok {
		return fmt.Errorf("a leaf named by a map key literal was reported: findings %+v", rep.Findings)
	}
	if _, ok := got["also-read"]; ok {
		return fmt.Errorf("a leaf named inside a path literal was reported: findings %+v", rep.Findings)
	}
	if len(rep.Findings) != 1 {
		return fmt.Errorf("want exactly 1 finding over the fixture, got %+v", rep.Findings)
	}
	return nil
}
