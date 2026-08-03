// Design: ai/rules/config.md -- config value-coercion guard
//
// config_string_coercion enforces an invariant discovered while debugging why
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
// A finding is suppressed only for an allowlisted file with a stated reason
// (genuine non-config coercion). New parsers must handle strings; add an
// allowlist entry, with a reason, only for a real non-config use.
//
// Usage:     go run scripts/checks/config_string_coercion.go [--json|--selftest]
// Called by: make ze-config-coercion-check (wired into ze-verify via
//            scripts/status/verify_run.go) and
//            scripts/checks/config_string_coercion_test.go
//
//go:build ignore

package main

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
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

type finding struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Kind string `json:"kind"` // "type-switch" | "type-assert"
	Code string `json:"code"`
}

// identName returns the identifier name of a type expression, or "" if the
// expression is not a bare identifier (e.g. a qualified or composite type).
func identName(e ast.Expr) string {
	if id, ok := e.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// scanFile parses one config.go and returns its anti-pattern findings.
func scanFile(fset *token.FileSet, path string) ([]finding, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f, err := parser.ParseFile(fset, path, src, 0)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	rel := filepath.ToSlash(path)
	lines := strings.Split(string(src), "\n")
	codeAt := func(pos token.Pos) (int, string) {
		ln := fset.Position(pos).Line
		txt := ""
		if ln >= 1 && ln <= len(lines) {
			txt = strings.TrimSpace(lines[ln-1])
		}
		return ln, txt
	}

	var out []finding
	ast.Inspect(f, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.TypeSwitchStmt:
			has := map[string]bool{}
			for _, stmt := range node.Body.List {
				cc, ok := stmt.(*ast.CaseClause)
				if !ok {
					continue
				}
				for _, te := range cc.List {
					if name := identName(te); name != "" {
						has[name] = true
					}
				}
			}
			numericOrBool := false
			for name := range has {
				if numericOrBoolTypes[name] {
					numericOrBool = true
					break
				}
			}
			if numericOrBool && !has["string"] {
				ln, txt := codeAt(node.Pos())
				out = append(out, finding{File: rel, Line: ln, Kind: "type-switch", Code: txt})
			}
		case *ast.TypeAssertExpr:
			// node.Type == nil is the `x.(type)` guard of a type switch; skip.
			if node.Type == nil {
				return true
			}
			if numericOrBoolTypes[identName(node.Type)] {
				ln, txt := codeAt(node.Pos())
				out = append(out, finding{File: rel, Line: ln, Kind: "type-assert", Code: txt})
			}
		}
		return true
	})
	return out, nil
}

// scan walks scanRoot for config.go files (excluding _test.go) and collects
// findings from the non-allowlisted ones.
func scan(root string) ([]finding, error) {
	var all []finding
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		if d.Name() != "config.go" {
			return nil
		}
		rel := filepath.ToSlash(path)
		if _, ok := allowlist[rel]; ok {
			return nil
		}
		fs, serr := scanFile(fset, path)
		if serr != nil {
			return serr
		}
		all = append(all, fs...)
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

func main() {
	jsonOut := false
	selftest := false
	for _, a := range os.Args[1:] {
		switch a {
		case "--json":
			jsonOut = true
		case "--selftest":
			selftest = true
		}
	}

	if selftest {
		os.Exit(runSelftest())
	}

	findings, err := scan(scanRoot)
	if err != nil {
		fmt.Fprintf(os.Stderr, "config-string-coercion: %v\n", err)
		os.Exit(2)
	}

	if jsonOut {
		_ = json.NewEncoder(os.Stdout).Encode(findings)
		if len(findings) > 0 {
			os.Exit(1)
		}
		return
	}

	if len(findings) > 0 {
		fmt.Fprintf(os.Stderr, "config-string-coercion: %d native-type config coercion(s) that ignore the delivered string form:\n", len(findings))
		for _, f := range findings {
			fmt.Fprintf(os.Stderr, "  %s:%d (%s): %s\n", f.File, f.Line, f.Kind, f.Code)
		}
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "The config framework delivers every YANG leaf value as a JSON STRING, so a")
		fmt.Fprintln(os.Stderr, "native-type assertion (v.(bool)/v.(float64)) or a numeric/bool type switch with")
		fmt.Fprintln(os.Stderr, "no `case string:` arm always fails -> the leaf silently reverts to its default")
		fmt.Fprintln(os.Stderr, "(a bool `enabled` gate disables the whole feature). Coerce via a helper that")
		fmt.Fprintln(os.Stderr, "accepts the string form (see internal/plugins/trafficusage/config.go cfgBool and")
		fmt.Fprintln(os.Stderr, "the `case string:` arms in toInt/toFloat). Allowlist only genuine non-config uses.")
		os.Exit(1)
	}

	fmt.Println("config-string-coercion: OK")
}

// runSelftest exercises scanFile against isolated temp-dir fixtures so the AST
// detection is proven independent of the live tree.
func runSelftest() int {
	dir, err := os.MkdirTemp("", "config-string-coercion-selftest-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "config-string-coercion selftest: %v\n", err)
		return 2
	}
	defer os.RemoveAll(dir)

	write := func(rel, content string) string {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if mkErr := os.MkdirAll(filepath.Dir(full), 0o755); mkErr != nil {
			panic(mkErr)
		}
		if wErr := os.WriteFile(full, []byte(content), 0o644); wErr != nil {
			panic(wErr)
		}
		return full
	}

	fset := token.NewFileSet()
	countKinds := func(path string) (switches, asserts int) {
		fs, ferr := scanFile(fset, path)
		if ferr != nil {
			panic(ferr)
		}
		for _, f := range fs {
			if f.Kind == "type-switch" {
				switches++
			} else {
				asserts++
			}
		}
		return
	}

	var failed []string
	check := func(cond bool, msg string) {
		if !cond {
			failed = append(failed, msg)
		}
	}

	// 1. Numeric type switch with no string case -- must be flagged.
	buggySwitch := write("buggy_switch/config.go", `package p
func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case float64:
		return int(n), true
	}
	return 0, false
}
`)
	sw, _ := countKinds(buggySwitch)
	check(sw == 1, "numeric type switch without case string: not flagged")

	// 2. Type switch WITH a string case -- must NOT be flagged.
	goodSwitch := write("good_switch/config.go", `package p
import "strconv"
func toInt(v any) (int, bool) {
	switch n := v.(type) {
	case int:
		return n, true
	case string:
		i, _ := strconv.Atoi(n)
		return i, true
	}
	return 0, false
}
`)
	swg, _ := countKinds(goodSwitch)
	check(swg == 0, "string-aware type switch wrongly flagged")

	// 3. Direct v.(bool) assertion -- must be flagged.
	directBool := write("direct_bool/config.go", `package p
func parse(m map[string]any) bool {
	if b, ok := m["enabled"].(bool); ok {
		return b
	}
	return false
}
`)
	_, as := countKinds(directBool)
	check(as == 1, "direct v.(bool) config assertion not flagged")

	// 4. A cfgBool-style helper (bool + string) -- must NOT be flagged.
	goodBool := write("good_bool/config.go", `package p
import "strconv"
func cfgBool(v any) (bool, bool) {
	switch b := v.(type) {
	case bool:
		return b, true
	case string:
		pb, _ := strconv.ParseBool(b)
		return pb, true
	}
	return false, false
}
`)
	swgb, asgb := countKinds(goodBool)
	check(swgb == 0 && asgb == 0, "string-aware cfgBool wrongly flagged")

	if len(failed) > 0 {
		fmt.Fprintln(os.Stderr, "config-string-coercion selftest FAILED:")
		for _, m := range failed {
			fmt.Fprintf(os.Stderr, "  %s\n", m)
		}
		return 1
	}
	fmt.Println("config-string-coercion selftest OK")
	return 0
}
