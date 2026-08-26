// Design: docs/architecture/config/yang-config-design.md -- config value-coercion guard
//
// Package configcoercion enforces an invariant discovered while debugging why
// ddos-detect never fired (session 6503): the plugin config framework delivers
// every YANG leaf value to a plugin's ParseConfig as a JSON STRING ("true",
// "50000"), NOT the native JSON type. A hand-written config parser that coerces
// a config value with a native-type assertion -- a `switch v.(type)` that
// handles bool/int/float64/uint but not string, or a direct `v.(bool)` /
// `v.(float64)` on a config map value -- therefore ALWAYS fails the assertion
// and silently falls back to the leaf's default. For a boolean `enabled` gate
// (ddos/detect, ddos/flowtriq, anomaly/detect, l2tp/pppoe, ...) that disables
// the entire feature with no error anywhere; for numeric leaves the operator's
// value is silently ignored. Every correct parser (trafficusage, iface, ospf,
// isis, firewall, ...) coerces via a helper with a `case string:` arm.
//
// It scans every internal/**/config.go for:
//   1. a type switch (`switch x.(type)`) whose cases include a numeric or bool
//      type but NOT string -- a value coercion that ignores the delivered form;
//   2. a direct type assertion to a numeric or bool type (`v.(bool)`,
//      `v.(float64)`, ...) -- which fails on the string a config value actually
//      is. (The `x.(type)` guard of a type switch has a nil asserted type and
//      is not flagged here.)
//
// A finding is suppressed only for an allowlisted file with a stated reason
// (genuine non-config coercion). New parsers must handle strings; add an
// allowlist entry, with a reason, only for a real non-config use.

package configcoercion

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/ze-software/ze/letools/lepath"
)

// scanRoot is the tree walked for config.go files. Config parsers live under
// internal/ (plugins and components alike).
const scanRoot = "internal"

// allowlist maps a repo-relative config.go path to the reason its native-type
// coercion is legitimate (it does NOT coerce a delivered config value). Add an
// entry only after confirming the assertion/switch operates on non-config data.
var allowlist = map[string]string{}

// numericOrBoolTypes are the asserted type names that a delivered string config
// value can never satisfy.
var numericOrBoolTypes = map[string]bool{
	"bool":    true,
	"int":     true,
	"int8":    true,
	"int16":   true,
	"int32":   true,
	"int64":   true,
	"uint":    true,
	"uint8":   true,
	"uint16":  true,
	"uint32":  true,
	"uint64":  true,
	"float32": true,
	"float64": true,
}

// identName returns the identifier name of a type expression, or "" if the
// expression is not a bare identifier (e.g. a qualified or composite type).
func identName(expr ast.Expr) string {
	if id, ok := expr.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// ScanFile parses one config.go and returns its anti-pattern findings, naming
// each one by the path the caller passed rather than by where the file sits on
// disk. It is exported because the selftest drives it against fixtures, which
// is what proves the AST detection independent of the live tree.
func ScanFile(fset *token.FileSet, path, rel string) (Findings, error) {
	src, err := os.ReadFile(path) //nolint:gosec // the path comes from this tool's own walk
	if err != nil {
		return nil, err
	}
	file, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", rel, err)
	}
	lines := strings.Split(string(src), "\n")
	codeAt := func(pos token.Pos) (int, string) {
		number := fset.Position(pos).Line
		text := ""
		if number >= 1 && number <= len(lines) {
			text = strings.TrimSpace(lines[number-1])
		}
		return number, text
	}

	var out Findings
	ast.Inspect(file, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.TypeSwitchStmt:
			has := map[string]bool{}
			for _, stmt := range typed.Body.List {
				clause, ok := stmt.(*ast.CaseClause)
				if !ok {
					continue
				}
				for _, expr := range clause.List {
					if id := identName(expr); id != "" {
						has[id] = true
					}
				}
			}
			numericOrBool := false
			for id := range has {
				if numericOrBoolTypes[id] {
					numericOrBool = true
					break
				}
			}
			if numericOrBool && !has["string"] {
				number, text := codeAt(typed.Pos())
				out = append(out, Finding{File: rel, Line: number, Kind: KindTypeSwitch, Code: text})
			}
		case *ast.TypeAssertExpr:
			// typed.Type == nil is the `x.(type)` guard of a type switch; skip.
			if typed.Type == nil {
				return true
			}
			if numericOrBoolTypes[identName(typed.Type)] {
				number, text := codeAt(typed.Pos())
				out = append(out, Finding{File: rel, Line: number, Kind: KindTypeAssert, Code: text})
			}
		}
		return true
	})
	return out, nil
}

// Check walks tree for config.go files (excluding _test.go) and collects
// findings from the non-allowlisted ones, sorted by file and line.
func Check(tree string) (Findings, error) {
	var all Findings
	fset := token.NewFileSet()

	root := filepath.Join(tree, scanRoot)
	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || entry.Name() != "config.go" {
			return nil
		}
		relative, relErr := filepath.Rel(tree, path)
		if relErr != nil {
			return relErr
		}
		rel := filepath.ToSlash(relative)
		if _, ok := allowlist[rel]; ok {
			return nil
		}
		found, scanErr := ScanFile(fset, path, rel)
		if scanErr != nil {
			return scanErr
		}
		all = append(all, found...)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(all, func(i, j int) bool {
		if all[i].File != all[j].File {
			return all[i].File < all[j].File
		}
		return all[i].Line < all[j].Line
	})
	return all, nil
}

// runCheck is the `check` action: walk the checkout and answer what it holds.
func runCheck() (any, int) {
	tree, err := lepath.Root()
	if err != nil {
		reportError(err)
		return nil, 2
	}

	findings, err := Check(tree)
	if err != nil {
		// 2 rather than 1: a walk that did not complete is a different fact
		// from a tree holding a coercion bug.
		reportError(err)
		return nil, 2
	}
	if len(findings) > 0 {
		return findings, 1
	}
	return findings, 0
}
