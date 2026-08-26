// Design: docs/architecture/cli/command-namespacing.md -- "-" means stdin/stdout via internal/core/cliio
//
// Package dashstdio enforces the invariant that a command must NOT read or
// write a USER-SUPPLIED path with a raw os call: it must route through
// internal/core/cliio so the "-" token resolves to stdin/stdout. The convention
// was stated twice and enforced nowhere, so about 34 command sites drifted;
// this gate is the lasting deliverable.
//
// It scans the CLI-facing trees (NOT internal/core, where cliio lives) for the
// os path primitives -- os.ReadFile, os.Open, os.Create, os.WriteFile,
// os.OpenFile -- and flags a call whose PATH ARGUMENT is CLI-tainted.
// "CLI-tainted" is computed by a light per-package dataflow, so a derived path
// (a draft/backup name, a filepath.Join, a strftime expansion) is NOT flagged
// and the gate stays low-noise (the accepted tradeoff over full dataflow
// analysis):
//
//   - a direct CLI-arg expression: fs.Arg(n) / <flags>.Args()[i] / os.Args[i] /
//     flag.Arg(n) / a dereferenced flag pointer *fooFlag;
//   - a local variable whose assignment(s) are CLI-tainted (alias chains);
//   - a range variable over a CLI-arg slice;
//   - a function parameter that ANY same-package call passes a tainted argument
//     to (propagated to a fixpoint) -- this catches funnel helpers like
//     openReader.
//
// This is a LINT HEURISTIC, not a soundness proof. It errs toward false
// negatives:
//   - Taint does NOT flow through arbitrary expressions (concat, slicing,
//     function RETURN values) or struct fields, so a laundered path
//     (`p := resolve(fs.Arg(0)); os.ReadFile(p)` or `c.path = fs.Arg(0);
//     os.ReadFile(c.path)`) is missed. A fixed derived path is therefore safe.
//   - Reads through an interface/storage abstraction
//     (`store.ReadFile(fs.Arg(0))`) are NOT flagged: they are not `os.*` calls,
//     and the config CLI resolves "-" at the CLI edge before delegating real
//     names to the store. A NEW command that reads a CLI path via a store
//     method thus escapes this gate -- the boundary is deliberate, not an
//     oversight.
//
// False positives are very rare but not provably zero: isCLIArgExpr matches
// `X.Arg(n)`/`X.Args()` by METHOD NAME, so an unrelated method named Arg/Args
// fed to an os path call would be flagged (allowlist it if that ever happens).
//
// A genuine raw-os site that must stay (a path that can never be "-": a device
// node, an internally-derived name the taint cannot see) is exempted either by
// a per-file entry in fileAllowlist (whole-subsystem) or, preferably, by a
// precise inline "cliio:allow <reason>" marker on the call's line.

package dashstdio

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// scanRoots are the trees that carry operator-facing CLI commands.
// internal/core is deliberately outside this walk: it is where cliio (the
// helper) lives, and its os calls ARE the sanctioned implementation.
var scanRoots = []string{
	"internal/component", "internal/plugins", "internal/analyze", "internal/mrt",
	"internal/perf", "internal/appliance", "internal/test", "internal/chaos",
	"cmd/ze",
}

// fileAllowlist maps a repo-relative .go path to the reason its raw os path
// call on a CLI-tainted argument is legitimate (a path that can never be "-").
// Add an entry ONLY after confirming the call cannot and should not accept
// stdin/stdout.
var fileAllowlist = map[string]string{}

// osPathFuncs are the os functions whose first argument is a filesystem path.
var osPathFuncs = map[string]bool{
	"ReadFile": true, "Open": true, "Create": true, "WriteFile": true, "OpenFile": true,
}

// allowMarker exempts one call site whose path can never be "-". The reason
// must follow it on the same line, and it can share the line with a
// linter-suppression comment.
const allowMarker = "cliio:allow"

// scanFloor is the least non-test Go files the walk must read before the gate
// believes it saw the tree. This checkout carried 3457 on 2026-08-26, so the
// floor fires on a tree that was never read rather than on one that shrank.
const scanFloor = 500

// Check walks tree's CLI-facing packages and answers every raw os path call on
// a user-supplied path.
//
// floor is a parameter rather than a constant because a fixture tree holds a
// handful of files: le passes scanFloor and a test passes 0.
func Check(tree string, floor int) (Findings, error) {
	roots := make([]string, 0, len(scanRoots))
	for _, rel := range scanRoots {
		roots = append(roots, filepath.Join(tree, filepath.FromSlash(rel)))
	}
	return scan(tree, roots, floor)
}

// scan groups every non-test Go file under roots by package directory and
// analyses each package on its own, which is the unit the taint fixpoint runs
// over.
func scan(tree string, roots []string, floor int) (Findings, error) {
	fset := token.NewFileSet()
	byDir := map[string][]string{}
	read := 0

	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			read++
			dir := filepath.Dir(path)
			byDir[dir] = append(byDir[dir], path)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	if read < floor {
		return nil, fmt.Errorf("the walk read %d non-test Go files under %s, below the floor of %d: this tree was not read", read, tree, floor)
	}

	dirs := make([]string, 0, len(byDir))
	for dir := range byDir {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)

	var all Findings
	for _, dir := range dirs {
		found, err := analyzePackage(fset, tree, byDir[dir])
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

// sourceFile is one parsed file of a package, with the text every finding's
// code line is cut from.
type sourceFile struct {
	rel   string
	file  *ast.File
	lines []string
	funcs []*ast.FuncDecl
}

// analyzePackage parses every non-test Go file of one package, propagates CLI
// taint through same-package function parameters to a fixpoint, then flags os
// path primitives on a tainted argument.
func analyzePackage(fset *token.FileSet, tree string, paths []string) (Findings, error) {
	var files []sourceFile
	funcDecls := map[string][]*ast.FuncDecl{} // name -> package-level decls
	flagVarsByFn := map[*ast.FuncDecl]map[string]bool{}

	for _, path := range paths {
		src, err := os.ReadFile(path) //nolint:gosec // repository path
		if err != nil {
			return nil, err
		}
		parsed, err := parser.ParseFile(fset, path, src, 0)
		if err != nil {
			return nil, fmt.Errorf("parse %s: %w", path, err)
		}
		relPath, err := filepath.Rel(tree, path)
		if err != nil {
			return nil, err
		}

		// The text is kept from the ONE read above. The script read each file a
		// second time for its code lines and dropped that read's error, so a
		// file that became unreadable between the two passes yielded findings
		// with a blank code column.
		current := sourceFile{
			rel:   filepath.ToSlash(relPath),
			file:  parsed,
			lines: strings.Split(string(src), "\n"),
		}
		for _, decl := range parsed.Decls {
			funcDecl, ok := decl.(*ast.FuncDecl)
			if !ok || funcDecl.Body == nil {
				continue
			}
			current.funcs = append(current.funcs, funcDecl)
			flagVarsByFn[funcDecl] = collectFlagVars(funcDecl)
			if name := packageLevelName(funcDecl); name != "" {
				funcDecls[name] = append(funcDecls[name], funcDecl)
			}
		}
		files = append(files, current)
	}

	taintedParams := propagateTaint(files, funcDecls, flagVarsByFn)
	return flagCalls(fset, files, taintedParams, flagVarsByFn), nil
}

// propagateTaint runs the same-package fixpoint: a parameter any call passes a
// tainted argument to becomes tainted itself, which is what catches a funnel
// helper two hops from the CLI edge.
func propagateTaint(
	files []sourceFile,
	funcDecls map[string][]*ast.FuncDecl,
	flagVarsByFn map[*ast.FuncDecl]map[string]bool,
) map[*ast.FuncDecl]map[int]bool {
	taintedParams := map[*ast.FuncDecl]map[int]bool{}

	for {
		changed := false
		for _, current := range files {
			for _, fn := range current.funcs {
				tainted := computeTainted(fn, taintedParams[fn], flagVarsByFn[fn])
				flagVars := flagVarsByFn[fn]
				ast.Inspect(fn, func(node ast.Node) bool {
					call, ok := node.(*ast.CallExpr)
					if !ok {
						return true
					}
					callee := identName(call.Fun) // package-level call by bare name
					if callee == "" {
						return true
					}
					for _, target := range funcDecls[callee] {
						for i, arg := range call.Args {
							if !exprIsTainted(arg, tainted, flagVars) {
								continue
							}
							if taintedParams[target] == nil {
								taintedParams[target] = map[int]bool{}
							}
							if taintedParams[target][i] {
								continue
							}
							taintedParams[target][i] = true
							changed = true
						}
					}
					return true
				})
			}
		}
		if !changed {
			return taintedParams
		}
	}
}

// flagCalls is the final pass: every os path primitive whose first argument is
// tainted, less the allowlisted files and the marked lines.
func flagCalls(
	fset *token.FileSet,
	files []sourceFile,
	taintedParams map[*ast.FuncDecl]map[int]bool,
	flagVarsByFn map[*ast.FuncDecl]map[string]bool,
) Findings {
	var out Findings
	for _, current := range files {
		if _, ok := fileAllowlist[current.rel]; ok {
			continue
		}
		osNames := osImportNames(current.file)
		for _, fn := range current.funcs {
			tainted := computeTainted(fn, taintedParams[fn], flagVarsByFn[fn])
			flagVars := flagVarsByFn[fn]
			ast.Inspect(fn, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || !osNames[identName(selector.X)] || !osPathFuncs[selector.Sel.Name] || len(call.Args) == 0 {
					return true
				}
				if !exprIsTainted(call.Args[0], tainted, flagVars) {
					return true
				}

				line := fset.Position(call.Pos()).Line
				code := ""
				if line >= 1 && line <= len(current.lines) {
					code = strings.TrimSpace(current.lines[line-1])
				}
				if strings.Contains(code, allowMarker) {
					return true
				}
				out = append(out, Finding{File: current.rel, Line: line, Fn: selector.Sel.Name, Code: code})
				return true
			})
		}
	}
	return out
}

// identName answers the name of an identifier expression, and the empty string
// for anything else.
func identName(expr ast.Expr) string {
	if ident, ok := expr.(*ast.Ident); ok {
		return ident.Name
	}
	return ""
}

// packageLevelName answers the declared name of a package-level function, and
// the empty string for a method. A method is analyzed for local dataflow but
// not for cross-call parameter propagation, which is keyed by bare name.
func packageLevelName(decl *ast.FuncDecl) string {
	if decl.Recv != nil {
		return ""
	}
	return decl.Name.Name
}

// isCLIArgExpr reports whether expr is a direct read of a CLI argument:
// fs.Arg(n), <flags>.Args()[i], flag.Arg(n)/Args(), os.Args[i], or a deref of a
// flag variable.
func isCLIArgExpr(expr ast.Expr, flagVars map[string]bool) bool {
	switch typed := expr.(type) {
	case *ast.CallExpr:
		if selector, ok := typed.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "Arg" {
			return true // fs.Arg(n) / flag.Arg(n)
		}
	case *ast.IndexExpr:
		return isCLIArgSlice(typed.X)
	case *ast.StarExpr:
		return flagVars[identName(typed.X)]
	}
	return false
}

// isCLIArgSlice reports whether expr is a CLI-argument slice: fs.Args(),
// flag.Args(), or os.Args.
func isCLIArgSlice(expr ast.Expr) bool {
	switch typed := expr.(type) {
	case *ast.CallExpr:
		if selector, ok := typed.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "Args" {
			return true // fs.Args() / flag.Args()
		}
	case *ast.SelectorExpr:
		if identName(typed.X) == "os" && typed.Sel.Name == "Args" {
			return true // os.Args
		}
	}
	return false
}

// isFlagConstructor reports whether call is a flag constructor returning a
// pointer whose deref is a CLI value: fs.String/Bool/Int/... or
// flag.String/....
func isFlagConstructor(call *ast.CallExpr) bool {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	switch selector.Sel.Name {
	case "String", "Bool", "Int", "Int64", "Uint", "Uint64", "Float64", "Duration":
		return true
	}
	return false
}

// collectFlagVars finds local identifiers bound to a flag constructor, so a
// later deref of them counts as a CLI value.
func collectFlagVars(fn *ast.FuncDecl) map[string]bool {
	flagVars := map[string]bool{}
	ast.Inspect(fn, func(node ast.Node) bool {
		assign, ok := node.(*ast.AssignStmt)
		if !ok {
			return true
		}
		for i, rhs := range assign.Rhs {
			call, ok := rhs.(*ast.CallExpr)
			if !ok || !isFlagConstructor(call) || i >= len(assign.Lhs) {
				continue
			}
			if name := identName(assign.Lhs[i]); name != "" {
				flagVars[name] = true
			}
		}
		return true
	})
	return flagVars
}

// exprIsTainted tests whether expr carries a CLI path given the current tainted
// set. It deliberately does NOT descend into binary ops, slices, or arbitrary
// calls, so a derived path (p + ".draft", p[1:], filepath.Join(...)) is not
// tainted.
func exprIsTainted(expr ast.Expr, tainted, flagVars map[string]bool) bool {
	if isCLIArgExpr(expr, flagVars) {
		return true
	}
	return tainted[identName(expr)]
}

// computeTainted answers the set of local identifiers in fn that carry a CLI
// path, seeded with fn's tainted parameters. It iterates to a local fixpoint so
// alias chains resolve regardless of statement order.
func computeTainted(fn *ast.FuncDecl, seedParams map[int]bool, flagVars map[string]bool) map[string]bool {
	tainted := map[string]bool{}
	if fn.Type.Params != nil && len(seedParams) > 0 {
		index := 0
		for _, field := range fn.Type.Params.List {
			for _, name := range field.Names {
				if seedParams[index] {
					tainted[name.Name] = true
				}
				index++
			}
		}
	}

	for {
		changed := false
		ast.Inspect(fn, func(node ast.Node) bool {
			switch statement := node.(type) {
			case *ast.AssignStmt:
				// x := rhs / x = rhs: taint x when rhs is CLI-tainted. Only 1:1
				// assignments alias a path; multi-value (x, y := f()) is
				// skipped.
				if len(statement.Lhs) != len(statement.Rhs) {
					return true
				}
				for i := range statement.Lhs {
					name := identName(statement.Lhs[i])
					if name == "" || tainted[name] || !exprIsTainted(statement.Rhs[i], tainted, flagVars) {
						continue
					}
					tainted[name] = true
					changed = true
				}
			case *ast.RangeStmt:
				// for _, v := range <cli-arg-slice>: taint the value variable.
				if statement.Value == nil || !isCLIArgSlice(statement.X) {
					return true
				}
				name := identName(statement.Value)
				if name == "" || tainted[name] {
					return true
				}
				tainted[name] = true
				changed = true
			}
			return true
		})
		if !changed {
			return tainted
		}
	}
}

// osImportNames resolves the local names of the "os" import, honoring aliases.
func osImportNames(file *ast.File) map[string]bool {
	names := map[string]bool{}
	for _, imported := range file.Imports {
		if strings.Trim(imported.Path.Value, `"`) != "os" {
			continue
		}
		if imported.Name != nil {
			names[imported.Name.Name] = true
			continue
		}
		names["os"] = true
	}
	return names
}
