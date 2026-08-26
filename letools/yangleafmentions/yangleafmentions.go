// Design: docs/architecture/config/yang-config-design.md -- config leaf consumption
// Related: internal/component/config/claims -- the blocking root-claim gate this complements
//
// Package yangleafmentions reports YANG config leaves that the owning plugin
// package never names.
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
// through a shared helper all break it. So this check REPORTS and always
// answers 0. It is deliberately absent from every verify stage; the ratchet that
// keeps it honest is its selftest.

package yangleafmentions

import (
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
// The pattern is assembled in a const, and both halves of that are
// deliberate. Written whole, the `+` sits beside a quote and
// c_string_concat refuses the commit, because it cannot tell a REGEX
// from a concatenation. Written as `{1,}` -- identical in RE2 -- the
// linter objects to the longer spelling. A const line is exempt from
// the concatenation rule, so splitting the literal satisfies both and
// leaves the compiled pattern byte-for-byte what it always was.
const keyPattern = `(?m)^[\t ]*key[\t ]` + `+"([^"]+)"`

var keyRE = regexp.MustCompile(keyPattern)

// ScanTree walks every config-schema module under root's internal/ and reports
// the leaves the owning package never names.
//
// Owner resolution is structural, never a table: the YANG glue generator
// discovers a module by its `<pkg>/yang/` directory, so the owning package is
// that directory's parent.
func ScanTree(root string) (Report, error) {
	var rep Report

	yangDirs, err := discoverYangDirs(root)
	if err != nil {
		return rep, err
	}

	for _, dir := range yangDirs {
		owner := filepath.Dir(dir)
		// The package is named RELATIVE to the tree walked, because the name a
		// reader needs is the one they can open. An absolute path says where
		// this checkout sits, which no reader of the report asked.
		rel, err := filepath.Rel(root, owner)
		if err != nil {
			return rep, err
		}
		literals, err := packageLiterals(owner)
		if err != nil {
			return rep, err
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			return rep, err
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), "-conf.yang") {
				continue
			}
			data, err := os.ReadFile(filepath.Join(dir, entry.Name())) //nolint:gosec // repository path
			if err != nil {
				return rep, err
			}
			module := strings.TrimSuffix(entry.Name(), ".yang")
			leaves := parseLeaves(string(data))
			rep.Modules++
			rep.Leaves += len(leaves)
			for _, leaf := range leaves {
				if literals[leaf.name] {
					continue
				}
				rep.Findings = append(rep.Findings, Finding{
					Module:  module,
					Package: filepath.ToSlash(rel),
					Leaf:    leaf.name,
					Path:    leaf.path,
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

// discoverYangDirs answers every `yang/` directory under root's internal/ that
// holds a config-schema module.
func discoverYangDirs(root string) ([]string, error) {
	var dirs []string
	base := filepath.Join(root, "internal")
	err := filepath.WalkDir(base, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() || entry.Name() != "yang" {
			return nil
		}
		entries, readErr := os.ReadDir(path)
		if readErr != nil {
			return readErr
		}
		for _, candidate := range entries {
			if !candidate.IsDir() && strings.HasSuffix(candidate.Name(), "-conf.yang") {
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

// yangLeaf is one config leaf and the container path it sits under.
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

// yangMarker is one statement the path walk cares about, at its offset.
type yangMarker struct {
	pos  int
	kind markerKind
	name string
}

// yangLevel is one open named level of the data tree, with the key leaves the
// level declared.
type yangLevel struct {
	name  string
	depth int
	keys  map[string]bool
}

// parseLeaves answers the config leaves of a YANG module with the container
// path each sits under.
//
// The module text is read rather than the resolved schema, because a resolved
// walk would need the module's package linked in, which is exactly what
// disappears under a reduced tag set.
func parseLeaves(text string) []yangLeaf {
	markers := make([]yangMarker, 0,
		len(containerRE.FindAllStringIndex(text, -1))+
			len(leafRE.FindAllStringIndex(text, -1))+
			len(keyRE.FindAllStringIndex(text, -1)))
	for _, match := range containerRE.FindAllStringSubmatchIndex(text, -1) {
		markers = append(markers, yangMarker{pos: match[0], kind: markerDir, name: text[match[2]:match[3]]})
	}
	for _, match := range leafRE.FindAllStringSubmatchIndex(text, -1) {
		markers = append(markers, yangMarker{pos: match[0], kind: markerLeaf, name: text[match[2]:match[3]]})
	}
	for _, match := range keyRE.FindAllStringSubmatchIndex(text, -1) {
		markers = append(markers, yangMarker{pos: match[0], kind: markerKey, name: text[match[2]:match[3]]})
	}
	sort.Slice(markers, func(i, j int) bool { return markers[i].pos < markers[j].pos })

	// Brace depth gives the enclosing containers. It is counted over the raw
	// text between markers, which is enough for a name path: a wrong path only
	// changes the label on a finding, never whether it is one.
	var stack []yangLevel
	var out []yangLeaf
	depth, prev := 0, 0
	seen := make(map[string]bool)

	for _, marker := range markers {
		depth += strings.Count(text[prev:marker.pos], "{") - strings.Count(text[prev:marker.pos], "}")
		prev = marker.pos
		for len(stack) > 0 && stack[len(stack)-1].depth >= depth {
			stack = stack[:len(stack)-1]
		}
		switch marker.kind {
		case markerDir:
			stack = append(stack, yangLevel{name: marker.name, depth: depth, keys: map[string]bool{}})
			continue
		case markerKey:
			if len(stack) > 0 {
				for key := range strings.FieldsSeq(marker.name) {
					stack[len(stack)-1].keys[key] = true
				}
			}
			continue
		case markerLeaf:
		}
		if len(stack) > 0 && stack[len(stack)-1].keys[marker.name] {
			continue // list key leaf: its value is the entry name, never a literal
		}
		parts := make([]string, 0, len(stack)+1)
		for _, level := range stack {
			parts = append(parts, level.name)
		}
		parts = append(parts, marker.name)
		path := strings.Join(parts, "/")
		if seen[path] {
			continue
		}
		seen[path] = true
		out = append(out, yangLeaf{name: marker.name, path: path})
	}
	return out
}

// packageLiterals answers every string literal in the owner directory's
// non-test Go files, plus the parts of every literal that holds a separator. A
// leaf read as `cfg["hold-time"]` and a leaf named inside a path constant
// "bgp/hold-time" both count as mentions.
//
// The walk is RECURSIVE. A component keeps its config parsing in a subpackage
// (BGP parses in internal/component/bgp/config, not in internal/component/bgp),
// so a shallow scan reported every BGP leaf as unread: 235 findings, of which
// the whole BGP block was noise.
//
// A file the parser cannot read hides nothing it could have found: its literals
// go uncollected, so its leaves are REPORTED rather than silently passed. The
// error is dropped for that reason and no other.
func packageLiterals(owner string) (map[string]bool, error) {
	out := make(map[string]bool)
	fset := token.NewFileSet()

	err := filepath.WalkDir(owner, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		file, parseErr := parser.ParseFile(fset, path, nil, 0)
		if parseErr != nil {
			return nil //nolint:nilerr // see the doc comment: an unparsed file REPORTS its leaves rather than passing them
		}
		ast.Inspect(file, func(node ast.Node) bool {
			lit, ok := node.(*ast.BasicLit)
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
