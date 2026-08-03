// Design: plan/learned (cli-dash-stdio) -- "-" means stdin/stdout via internal/core/cliio
//
// cli_dash_stdio enforces the invariant that a command must NOT read or write a
// USER-SUPPLIED path with a raw os call: it must route through internal/core/cliio
// so the "-" token resolves to stdin/stdout (ai/rules/cli.md). The
// convention was stated twice and enforced nowhere, so ~34 command sites drifted;
// this gate is the lasting deliverable.
//
// It scans the CLI-facing trees (NOT internal/core, where cliio lives) for the os
// path primitives -- os.ReadFile, os.Open, os.Create, os.WriteFile, os.OpenFile --
// and flags a call whose PATH ARGUMENT is CLI-tainted. "CLI-tainted" is computed by
// a light per-package dataflow, so a derived path (a draft/backup name, a
// filepath.Join, a strftime expansion) is NOT flagged and the gate stays low-noise
// (the accepted tradeoff over full dataflow analysis):
//
//   - a direct CLI-arg expression: fs.Arg(n) / <flags>.Args()[i] / os.Args[i] /
//     flag.Arg(n) / a dereferenced flag pointer *fooFlag;
//   - a local variable whose assignment(s) are CLI-tainted (alias chains);
//   - a range variable over a CLI-arg slice;
//   - a function parameter that ANY same-package call passes a tainted argument to
//     (propagated to a fixpoint) -- this catches funnel helpers like openReader.
//
// This is a LINT HEURISTIC, not a soundness proof. It errs toward false negatives:
//   - Taint does NOT flow through arbitrary expressions (concat, slicing, function
//     RETURN values) or struct fields, so a laundered path (`p := resolve(fs.Arg(0));
//     os.ReadFile(p)` or `c.path = fs.Arg(0); os.ReadFile(c.path)`) is missed. A
//     fixed derived path is therefore safe.
//   - Reads through an interface/storage abstraction (`store.ReadFile(fs.Arg(0))`)
//     are NOT flagged: they are not `os.*` calls, and the config CLI resolves "-"
//     at the CLI edge before delegating real names to the store. A NEW command that
//     reads a CLI path via a store method thus escapes this gate -- the boundary is
//     deliberate, not an oversight.
// False positives are very rare but not provably zero: `isCLIArgExpr` matches
// `X.Arg(n)`/`X.Args()` by METHOD NAME, so an unrelated method named Arg/Args fed
// to an os path call would be flagged (allowlist it if that ever happens).
//
// A genuine raw-os site that must stay (a path that can never be "-": a device
// node, an internally-derived name the taint cannot see) is exempted either by a
// per-file entry in fileAllowlist (whole-subsystem) or, preferably, by a precise
// inline "cliio:allow <reason>" marker on the call's line.
//
// Usage:     go run scripts/checks/cli_dash_stdio.go [--json|--selftest]
// Called by: make ze-dash-stdio-check (wired into ze-verify via
//            scripts/status/verify_run.go) and scripts/checks/cli_dash_stdio_test.go
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
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// scanRoots are the trees that carry operator-facing CLI commands. internal/core
// is deliberately excluded: it is where cliio (the helper) lives, and its os calls
// ARE the sanctioned implementation.
var scanRoots = []string{
	"internal/component", "internal/plugins", "internal/analyze", "internal/mrt",
	"internal/perf", "internal/appliance", "internal/test", "internal/chaos",
	"cmd/ze",
}

// fileAllowlist maps a repo-relative .go path to the reason its raw os path call
// on a CLI-tainted argument is legitimate (a path that can never be "-"). Add an
// entry ONLY after confirming the call cannot and should not accept stdin/stdout.
var fileAllowlist = map[string]string{}

// osPathFuncs are the os functions whose first argument is a filesystem path.
var osPathFuncs = map[string]bool{
	"ReadFile": true, "Open": true, "Create": true, "WriteFile": true, "OpenFile": true,
}

type finding struct {
	File string `json:"file"`
	Line int    `json:"line"`
	Fn   string `json:"fn"`
	Code string `json:"code"`
}

func identName(e ast.Expr) string {
	if id, ok := e.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// funcName returns the declared name for a package-level function, or "" for a
// method (methods are analysed for local dataflow but not for cross-call param
// propagation, which is keyed by bare name).
func funcName(fd *ast.FuncDecl) string {
	if fd.Recv != nil {
		return ""
	}
	return fd.Name.Name
}

// isCLIArgExpr reports whether e is a direct read of a CLI argument: fs.Arg(n),
// <flags>.Args()[i], flag.Arg(n)/Args(), os.Args[i], or a *deref of a flag var.
func isCLIArgExpr(e ast.Expr, flagVars map[string]bool) bool {
	switch x := e.(type) {
	case *ast.CallExpr:
		if sel, ok := x.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Arg" {
			return true // fs.Arg(n) / flag.Arg(n)
		}
	case *ast.IndexExpr:
		return isCLIArgSlice(x.X)
	case *ast.StarExpr:
		return flagVars[identName(x.X)]
	}
	return false
}

// isCLIArgSlice reports whether e is a CLI-argument slice: fs.Args(), flag.Args(),
// or os.Args.
func isCLIArgSlice(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.CallExpr:
		if sel, ok := x.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Args" {
			return true // fs.Args() / flag.Args()
		}
	case *ast.SelectorExpr:
		if identName(x.X) == "os" && x.Sel.Name == "Args" {
			return true // os.Args
		}
	}
	return false
}

// flagCtor reports whether call is a flag constructor returning a pointer whose
// deref is a CLI value: fs.String/Bool/Int/... or flag.String/....
func flagCtor(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	switch sel.Sel.Name {
	case "String", "Bool", "Int", "Int64", "Uint", "Uint64", "Float64", "Duration":
		return true
	}
	return false
}

// collectFlagVars finds local identifiers bound to a flag constructor, so a later
// *deref of them counts as a CLI value.
func collectFlagVars(fn *ast.FuncDecl) map[string]bool {
	flagVars := map[string]bool{}
	ast.Inspect(fn, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, rhs := range as.Rhs {
			call, ok := rhs.(*ast.CallExpr)
			if !ok || !flagCtor(call) || i >= len(as.Lhs) {
				continue
			}
			if name := identName(as.Lhs[i]); name != "" {
				flagVars[name] = true
			}
		}
		return true
	})
	return flagVars
}

// exprIsTainted tests whether e carries a CLI path given the current tainted set.
// It deliberately does NOT descend into binary ops, slices, or arbitrary calls, so
// a derived path (p + ".draft", p[1:], filepath.Join(...)) is not tainted.
func exprIsTainted(e ast.Expr, tainted, flagVars map[string]bool) bool {
	if isCLIArgExpr(e, flagVars) {
		return true
	}
	return tainted[identName(e)]
}

// computeTainted returns the set of local identifiers in fn that carry a CLI path,
// seeded with fn's tainted parameters. It iterates to a local fixpoint so alias
// chains resolve regardless of statement order.
func computeTainted(fn *ast.FuncDecl, seedParams map[int]bool, flagVars map[string]bool) map[string]bool {
	tainted := map[string]bool{}
	if fn.Type.Params != nil && len(seedParams) > 0 {
		idx := 0
		for _, field := range fn.Type.Params.List {
			for _, name := range field.Names {
				if seedParams[idx] {
					tainted[name.Name] = true
				}
				idx++
			}
		}
	}

	for {
		changed := false
		ast.Inspect(fn, func(n ast.Node) bool {
			switch s := n.(type) {
			case *ast.AssignStmt:
				// x := rhs / x = rhs : taint x if rhs is CLI-tainted. Only 1:1
				// assignments alias a path; multi-value (x, y := f()) is skipped.
				if len(s.Lhs) == len(s.Rhs) {
					for i := range s.Lhs {
						name := identName(s.Lhs[i])
						if name != "" && !tainted[name] && exprIsTainted(s.Rhs[i], tainted, flagVars) {
							tainted[name] = true
							changed = true
						}
					}
				}
			case *ast.RangeStmt:
				// for _, v := range <cli-arg-slice> : taint the value var.
				if s.Value != nil && isCLIArgSlice(s.X) {
					if name := identName(s.Value); name != "" && !tainted[name] {
						tainted[name] = true
						changed = true
					}
				}
			}
			return true
		})
		if !changed {
			break
		}
	}
	return tainted
}

// osImportNames resolves the local name(s) of the "os" import, honouring aliases.
func osImportNames(f *ast.File) map[string]bool {
	names := map[string]bool{}
	for _, imp := range f.Imports {
		if strings.Trim(imp.Path.Value, `"`) != "os" {
			continue
		}
		if imp.Name != nil {
			names[imp.Name.Name] = true
		} else {
			names["os"] = true
		}
	}
	return names
}

type fileInfo struct {
	path  string
	rel   string
	f     *ast.File
	funcs []*ast.FuncDecl
}

// analyzePkg parses every non-test .go file in a package, propagates CLI taint
// through same-package function parameters to a fixpoint, then flags os path
// primitives on a tainted argument.
func analyzePkg(fset *token.FileSet, paths []string) ([]finding, error) {
	var files []fileInfo
	funcDecls := map[string][]*ast.FuncDecl{} // name -> package-level decls
	flagVarsByFn := map[*ast.FuncDecl]map[string]bool{}

	for _, p := range paths {
		src, err := os.ReadFile(p)
		if err != nil {
			return nil, err
		}
		f, err := parser.ParseFile(fset, p, src, 0)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", p, err)
		}
		fi := fileInfo{path: p, rel: filepath.ToSlash(p), f: f}
		for _, decl := range f.Decls {
			if fd, ok := decl.(*ast.FuncDecl); ok && fd.Body != nil {
				fi.funcs = append(fi.funcs, fd)
				flagVarsByFn[fd] = collectFlagVars(fd)
				if name := funcName(fd); name != "" {
					funcDecls[name] = append(funcDecls[name], fd)
				}
			}
		}
		files = append(files, fi)
	}

	// Fixpoint: propagate taint into same-package function parameters.
	taintedParams := map[*ast.FuncDecl]map[int]bool{}
	for {
		changed := false
		for _, fi := range files {
			for _, fn := range fi.funcs {
				tainted := computeTainted(fn, taintedParams[fn], flagVarsByFn[fn])
				fv := flagVarsByFn[fn]
				ast.Inspect(fn, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok {
						return true
					}
					callee := identName(call.Fun) // package-level call by bare name
					if callee == "" {
						return true
					}
					for _, target := range funcDecls[callee] {
						for i, arg := range call.Args {
							if !exprIsTainted(arg, tainted, fv) {
								continue
							}
							if taintedParams[target] == nil {
								taintedParams[target] = map[int]bool{}
							}
							if !taintedParams[target][i] {
								taintedParams[target][i] = true
								changed = true
							}
						}
					}
					return true
				})
			}
		}
		if !changed {
			break
		}
	}

	// Final pass: flag os path primitives on a tainted first argument.
	var out []finding
	for _, fi := range files {
		if _, ok := fileAllowlist[fi.rel]; ok {
			continue
		}
		src, _ := os.ReadFile(fi.path)
		lines := strings.Split(string(src), "\n")
		osNames := osImportNames(fi.f)
		for _, fn := range fi.funcs {
			tainted := computeTainted(fn, taintedParams[fn], flagVarsByFn[fn])
			fv := flagVarsByFn[fn]
			ast.Inspect(fn, func(n ast.Node) bool {
				call, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || !osNames[identName(sel.X)] || !osPathFuncs[sel.Sel.Name] || len(call.Args) == 0 {
					return true
				}
				if !exprIsTainted(call.Args[0], tainted, fv) {
					return true
				}
				pos := fset.Position(call.Pos())
				code := ""
				if pos.Line >= 1 && pos.Line <= len(lines) {
					code = strings.TrimSpace(lines[pos.Line-1])
				}
				// A precise, self-documenting exemption for a site whose path can
				// never be "-" (e.g. an O_EXCL atomic create where "-" is rejected
				// upstream). The reason must follow the "cliio:allow" marker on the
				// same line; it can share the line with a linter-suppression comment.
				if strings.Contains(code, "cliio:allow") {
					return true
				}
				out = append(out, finding{File: fi.rel, Line: pos.Line, Fn: sel.Sel.Name, Code: code})
				return true
			})
		}
	}
	return out, nil
}

func scan(roots []string) ([]finding, error) {
	var all []finding
	fset := token.NewFileSet()
	byDir := map[string][]string{} // package dir -> non-test .go files
	for _, root := range roots {
		err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() || !strings.HasSuffix(p, ".go") || strings.HasSuffix(p, "_test.go") {
				return nil
			}
			dir := filepath.Dir(p)
			byDir[dir] = append(byDir[dir], p)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	dirs := make([]string, 0, len(byDir))
	for d := range byDir {
		dirs = append(dirs, d)
	}
	sort.Strings(dirs)
	for _, dir := range dirs {
		found, err := analyzePkg(fset, byDir[dir])
		if err != nil {
			return nil, err
		}
		all = append(all, found...)
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

	findings, err := scan(scanRoots)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cli-dash-stdio: %v\n", err)
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
		fmt.Fprintf(os.Stderr, "cli-dash-stdio: %d raw os call(s) on a user-supplied path (must use internal/core/cliio so \"-\" means stdin/stdout):\n", len(findings))
		for _, f := range findings {
			fmt.Fprintf(os.Stderr, "  %s:%d (os.%s): %s\n", f.File, f.Line, f.Fn, f.Code)
		}
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "A filename-accepting command must read/write through internal/core/cliio")
		fmt.Fprintln(os.Stderr, "(ReadFile/OpenReader/Create/WriteFile) so \"-\" resolves to stdin/stdout. If this")
		fmt.Fprintln(os.Stderr, "path can never be \"-\" (a device node, an internally-derived name), add an")
		fmt.Fprintln(os.Stderr, "allowlist entry with a reason in scripts/checks/cli_dash_stdio.go.")
		os.Exit(1)
	}

	fmt.Println("cli-dash-stdio: OK")
}

// runSelftest builds isolated fixtures reproducing the pre-migration violation
// shapes (each MUST be flagged) and the legitimate shapes (each MUST NOT be
// flagged), then scans them. It proves the detector fires -- the R-2 requirement
// that the gate demonstrably catches the bug it was written for, not merely passes
// on the migrated tree.
func runSelftest() int {
	dir, err := os.MkdirTemp("", "cli-dash-stdio-selftest-*")
	if err != nil {
		fmt.Fprintf(os.Stderr, "cli-dash-stdio selftest: %v\n", err)
		return 2
	}
	defer func() { _ = os.RemoveAll(dir) }()

	write := func(rel, content string) {
		full := filepath.Join(dir, filepath.FromSlash(rel))
		if mkErr := os.MkdirAll(filepath.Dir(full), 0o755); mkErr != nil {
			panic(mkErr)
		}
		if wErr := os.WriteFile(full, []byte(content), 0o644); wErr != nil {
			panic(wErr)
		}
	}

	// Each fixture lives in its own subdir (its own package) so names never clash.
	write("direct/x.go", fxDirect)
	write("alias/x.go", fxAlias)
	write("rangef/x.go", fxRange)
	write("flagderef/x.go", fxFlagDeref)
	write("funnel/x.go", fxFunnel)
	write("twohop/x.go", fxTwoHop)
	write("aliasedos/x.go", fxAliasedOS)
	write("osargs/x.go", fxOSArgs)
	write("writef/x.go", fxWrite)
	write("derived/x.go", fxDerived)
	write("helper/x.go", fxHelper)
	write("joined/x.go", fxJoined)
	write("store/x.go", fxStore)
	write("allowmarker/x.go", fxAllowMarker)

	findings, err := scan([]string{dir})
	if err != nil {
		fmt.Fprintf(os.Stderr, "cli-dash-stdio selftest scan: %v\n", err)
		return 2
	}
	count := map[string]int{}
	for _, f := range findings {
		for _, sub := range []string{"direct", "alias", "rangef", "flagderef", "funnel", "twohop", "aliasedos", "osargs", "writef", "derived", "helper", "joined", "store", "allowmarker"} {
			if strings.HasSuffix(f.File, path.Join(sub, "x.go")) {
				count[sub]++
			}
		}
	}

	var failed []string
	mustFlag := func(sub, msg string) {
		if count[sub] == 0 {
			failed = append(failed, msg)
		}
	}
	mustNotFlag := func(sub, msg string) {
		if count[sub] != 0 {
			failed = append(failed, msg)
		}
	}
	mustFlag("direct", "direct fs.Arg read not flagged")
	mustFlag("alias", "aliased CLI-arg read not flagged")
	mustFlag("rangef", "range-over-args read not flagged")
	mustFlag("flagderef", "flag-pointer deref write not flagged")
	mustFlag("funnel", "funnel-parameter read not flagged")
	mustFlag("twohop", "two-hop funnel-parameter read not flagged")
	mustFlag("aliasedos", "aliased os import (import fsys \"os\") read not flagged")
	mustFlag("osargs", "os.Args index read not flagged")
	mustFlag("writef", "fs.Arg write not flagged")
	mustNotFlag("derived", "derived (sliced) path wrongly flagged")
	mustNotFlag("helper", "cliio call wrongly flagged")
	mustNotFlag("joined", "filepath.Join path wrongly flagged")
	mustNotFlag("store", "store.ReadFile (storage abstraction) wrongly flagged")
	mustNotFlag("allowmarker", "//cliio:allow-marked site wrongly flagged")

	if len(failed) > 0 {
		fmt.Fprintln(os.Stderr, "cli-dash-stdio selftest FAILED:")
		for _, m := range failed {
			fmt.Fprintf(os.Stderr, "  %s\n", m)
		}
		return 1
	}
	fmt.Println("cli-dash-stdio selftest OK")
	return 0
}

// Fixtures are isolated Go sources fed to the scanner; they reproduce the exact
// shapes the gate must (or must not) flag. Kept as consts so the checker's own
// source contains no bare CLI-arg raw-os call for the guard-of-the-guard to trip on.

const fxDirect = `package p
import "os"
func cmd(fs argset) {
	if _, err := os.ReadFile(fs.Arg(0)); err != nil {
		return
	}
}
type argset struct{}
func (argset) Arg(int) string { return "" }
`

const fxAlias = `package p
import "os"
func cmd(fs argset) {
	pth := fs.Arg(0)
	if _, err := os.ReadFile(pth); err != nil {
		return
	}
}
type argset struct{}
func (argset) Arg(int) string { return "" }
`

const fxRange = `package p
import "os"
func cmd(fs argset) {
	for _, pth := range fs.Args() {
		if _, err := os.ReadFile(pth); err != nil {
			return
		}
	}
}
type argset struct{}
func (argset) Args() []string { return nil }
`

const fxFlagDeref = `package p
import "os"
func cmd(fs argset) {
	out := fs.String("o", "", "")
	if _, err := os.Create(*out); err != nil {
		return
	}
}
type argset struct{}
func (argset) String(a, b, c string) *string { return nil }
`

const fxFunnel = `package p
import "os"
func load(pth string) {
	if _, err := os.Open(pth); err != nil {
		return
	}
}
func cmd(fs argset) { load(fs.Arg(0)) }
type argset struct{}
func (argset) Arg(int) string { return "" }
`

const fxTwoHop = `package p
import "os"
func inner(pth string) {
	if _, err := os.Open(pth); err != nil {
		return
	}
}
func outer(pth string) { inner(pth) }
func cmd(fs argset) { outer(fs.Arg(0)) }
type argset struct{}
func (argset) Arg(int) string { return "" }
`

const fxAliasedOS = `package p
import fsys "os"
func cmd(fs argset) {
	if _, err := fsys.ReadFile(fs.Arg(0)); err != nil {
		return
	}
}
type argset struct{}
func (argset) Arg(int) string { return "" }
`

const fxOSArgs = `package p
import "os"
func cmd() {
	if _, err := os.ReadFile(os.Args[1]); err != nil {
		return
	}
}
`

const fxWrite = `package p
import "os"
func cmd(fs argset, data []byte) {
	if err := os.WriteFile(fs.Arg(1), data, 0o600); err != nil {
		return
	}
}
type argset struct{}
func (argset) Arg(int) string { return "" }
`

const fxDerived = `package p
import "os"
func cmd(fs argset) {
	pth := fs.Arg(0)
	derived := pth[1:]
	if _, err := os.ReadFile(derived); err != nil {
		return
	}
}
type argset struct{}
func (argset) Arg(int) string { return "" }
`

const fxHelper = `package p
func cmd(fs argset) {
	if _, err := cliio.ReadFile(fs.Arg(0)); err != nil {
		return
	}
}
type argset struct{}
func (argset) Arg(int) string { return "" }
`

const fxJoined = `package p
import (
	"os"
	"path/filepath"
)
func cmd(dir string) {
	if _, err := os.ReadFile(filepath.Join(dir, "x")); err != nil {
		return
	}
}
`

const fxStore = `package p
func cmd(store backend, fs argset) {
	if _, err := store.ReadFile(fs.Arg(0)); err != nil {
		return
	}
}
type backend interface{ ReadFile(string) ([]byte, error) }
type argset struct{}
func (argset) Arg(int) string { return "" }
`

const fxAllowMarker = `package p
import "os"
func cmd(fs argset) {
	if _, err := os.ReadFile(fs.Arg(0)); err != nil { //cliio:allow reason: never "-"
		return
	}
}
type argset struct{}
func (argset) Arg(int) string { return "" }
`
