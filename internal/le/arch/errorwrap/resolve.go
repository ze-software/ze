// Design: docs/contributing/ze-go-style.md -- naming the function a wrapped error came from
// Related: errorwrap.go -- the trace that walks what this resolves
// Related: funcvalue.go -- following a function value to its producers
//
// resolve.go reads the module's packages with go/parser, as every other `le
// arch` check does, and names a callee from the syntax alone: a function the
// package declares, a function a module package it imports declares, a
// function literal bound to a local, or a method whose name is unique in the
// package. A name holding a function value is followed further by
// funcvalue.go. Anything else (an interface method, a method name two types
// share) is unnamed, and the caller counts it.

package archerrorwrap

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
)

// packageScope names the enclosing function of a wrap in a package-level
// function literal, which has no declaration of its own.
const packageScope = "(package scope)"

// sourceFile is one parsed Go file, with its path relative to the tree.
type sourceFile struct {
	rel    string
	syntax *ast.File
}

// goPackage is every non-test file of one directory, indexed by what a call
// can name. calls is every call by a bare name in the package, keyed by that
// name, built the first time a parameter is followed to its callers.
type goPackage struct {
	name    string
	files   []*sourceFile
	funcs   map[string]*funcRef
	methods map[string][]*funcRef
	vars    map[string]packageVar
	calls   map[string][]callSite
}

// packageVar is the value a package-level var is declared with.
type packageVar struct {
	file  *sourceFile
	value ast.Expr
}

// funcRef is one function body the trace can read: a declaration or a
// function literal. A literal holds its enclosing function as outer, and
// where it opens, so a name it captures is looked up there.
//
// params is the function's parameter list, nil for a package-scope body.
// callable is the name a call in its package types to reach it: set for a
// plain function declaration, empty for a method or a literal, whose call
// sites the syntax does not name.
type funcRef struct {
	name     string
	pkg      *goPackage
	file     *sourceFile
	body     *ast.BlockStmt
	outer    *funcRef
	opened   token.Pos
	params   *ast.FieldList
	callable string
}

// declaredRef answers a function declaration as a body the trace can read.
func declaredRef(pkg *goPackage, file *sourceFile, decl *ast.FuncDecl) *funcRef {
	ref := &funcRef{name: funcName(decl), pkg: pkg, file: file, body: decl.Body, params: decl.Type.Params}
	if decl.Recv == nil {
		ref.callable = decl.Name.Name
	}
	return ref
}

// resolution is how far a callee was named.
type resolution int

const (
	// resolutionUnspecified is the zero value and never an answer.
	resolutionUnspecified resolution = iota
	// resolutionFound names a body the trace can read.
	resolutionFound
	// resolutionOutside is a function of a package outside the module, whose
	// errors the check does not read.
	resolutionOutside
	// resolutionUnnamed is an in-module callee the syntax cannot name.
	resolutionUnnamed
)

// importKind is what a selector's left-hand name is in a file.
type importKind int

const (
	// importUnspecified is the zero value and never an answer.
	importUnspecified importKind = iota
	// importAbsent says the name is no import of the file.
	importAbsent
	// importExternal is a package outside the module.
	importExternal
	// importModule is a package of the module.
	importModule
)

// index holds every package the trace has read, what each function's errors
// carry once it has been traced, and the bodies each followed parameter can
// hold (funcvalue.go). It is not safe for concurrent use.
type index struct {
	tree      string
	module    string
	fset      *token.FileSet
	packages  map[string]*goPackage
	memo      map[*ast.BlockStmt]carried
	open      map[*ast.BlockStmt]bool
	paramMemo map[paramKey]funcSet
	paramOpen map[paramKey]bool
	// paramCuts counts the parameters a walk met already open higher up.
	paramCuts int
}

// newIndex answers an empty index over tree, whose module path is module.
func newIndex(tree, module string) *index {
	return &index{
		tree:      tree,
		module:    module,
		fset:      token.NewFileSet(),
		packages:  map[string]*goPackage{},
		memo:      map[*ast.BlockStmt]carried{},
		open:      map[*ast.BlockStmt]bool{},
		paramMemo: map[paramKey]funcSet{},
		paramOpen: map[paramKey]bool{},
	}
}

// modulePath reads the module path from tree's go.mod.
func modulePath(tree string) (string, error) {
	data, err := os.ReadFile(filepath.Join(tree, "go.mod"))
	if err != nil {
		return "", fmt.Errorf("read the module path from go.mod: %w", err)
	}
	for line := range strings.Lines(string(data)) {
		fields := strings.Fields(line)
		if len(fields) != 2 {
			continue
		}
		if fields[0] != "module" {
			continue
		}
		return strings.Trim(fields[1], `"`), nil
	}
	return "", fmt.Errorf("%s holds no module line, so no import can be resolved to a directory", filepath.Join(tree, "go.mod"))
}

// packageAt answers the package in the directory rel, relative to the tree,
// reading it the first time.
//
// A file that does not parse is an error rather than a package without it: a
// gate that skips what it cannot read reports a clean tree for it. Every
// non-test file is read whatever its build constraint, so a function declared
// in two tagged files is indexed by its first file in name order.
func (ix *index) packageAt(rel string) (*goPackage, error) {
	if pkg, read := ix.packages[rel]; read {
		return pkg, nil
	}
	entries, err := os.ReadDir(filepath.Join(ix.tree, filepath.FromSlash(rel)))
	if err != nil {
		return nil, fmt.Errorf("read package %s: %w", rel, err)
	}
	pkg := &goPackage{
		funcs:   map[string]*funcRef{},
		methods: map[string][]*funcRef{},
		vars:    map[string]packageVar{},
	}
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() {
			continue
		}
		if !strings.HasSuffix(name, ".go") {
			continue
		}
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		relFile := path.Join(rel, name)
		syntax, parseErr := parser.ParseFile(ix.fset, filepath.Join(ix.tree, filepath.FromSlash(relFile)), nil, parser.SkipObjectResolution)
		if parseErr != nil {
			return nil, parseErr
		}
		file := &sourceFile{rel: relFile, syntax: syntax}
		pkg.name = syntax.Name.Name
		pkg.files = append(pkg.files, file)
		pkg.declare(file)
	}
	ix.packages[rel] = pkg
	return pkg, nil
}

// file answers the parsed file at rel, or nil when the package holds none.
func (pkg *goPackage) file(rel string) *sourceFile {
	for _, file := range pkg.files {
		if file.rel == rel {
			return file
		}
	}
	return nil
}

// declare indexes one file's functions, methods and package-level vars.
func (pkg *goPackage) declare(file *sourceFile) {
	for _, decl := range file.syntax.Decls {
		switch typed := decl.(type) {
		case *ast.FuncDecl:
			pkg.declareFunc(file, typed)
		case *ast.GenDecl:
			pkg.declareVars(file, typed)
		}
	}
}

// declareFunc indexes one function declaration: by name for a function, by
// method name for a method.
func (pkg *goPackage) declareFunc(file *sourceFile, decl *ast.FuncDecl) {
	if decl.Body == nil {
		return
	}
	ref := declaredRef(pkg, file, decl)
	if decl.Recv != nil {
		pkg.methods[decl.Name.Name] = append(pkg.methods[decl.Name.Name], ref)
		return
	}
	if _, held := pkg.funcs[decl.Name.Name]; held {
		return
	}
	pkg.funcs[decl.Name.Name] = ref
}

// declareVars indexes the package-level vars declared with a value.
func (pkg *goPackage) declareVars(file *sourceFile, decl *ast.GenDecl) {
	if decl.Tok != token.VAR {
		return
	}
	for _, spec := range decl.Specs {
		values, isValues := spec.(*ast.ValueSpec)
		if !isValues {
			continue
		}
		if len(values.Values) != len(values.Names) {
			continue
		}
		for position, name := range values.Names {
			pkg.vars[name.Name] = packageVar{file: file, value: values.Values[position]}
		}
	}
}

// moduleDir answers the directory, relative to the tree, of an import path
// inside the module, and false for one outside it.
func (ix *index) moduleDir(importPath string) (string, bool) {
	if importPath == ix.module {
		return ".", true
	}
	rel, inModule := strings.CutPrefix(importPath, ix.module+"/")
	return rel, inModule
}

// resolve names the function a callee expression other than a bare name
// reaches, from its syntax in fn. A bare name is funcValues' to follow.
func (ix *index) resolve(fn *funcRef, fun ast.Expr) (*funcRef, resolution, error) {
	switch callee := ast.Unparen(fun).(type) {
	case *ast.FuncLit:
		return literal(fn, callee, fn.name+" (func literal)"), resolutionFound, nil
	case *ast.SelectorExpr:
		return ix.resolveSelector(fn, callee)
	default:
		return nil, resolutionUnnamed, nil
	}
}

// literal answers a function literal as a body the trace can read, inside
// the function that holds it.
func literal(outer *funcRef, lit *ast.FuncLit, name string) *funcRef {
	return &funcRef{name: name, pkg: outer.pkg, file: outer.file, body: lit.Body, outer: outer, opened: lit.Pos(), params: lit.Type.Params}
}

// resolveSelector names a `x.Name` call: a function of an imported module
// package, or else a method whose name only one type in fn's package declares.
func (ix *index) resolveSelector(fn *funcRef, selector *ast.SelectorExpr) (*funcRef, resolution, error) {
	receiver, isIdent := selector.X.(*ast.Ident)
	if !isIdent {
		ref, how := resolveMethod(fn, selector.Sel.Name)
		return ref, how, nil
	}
	pkg, kind, err := ix.importedPackage(fn.file, receiver.Name)
	if err != nil {
		return nil, resolutionUnspecified, err
	}
	if kind == importExternal {
		return nil, resolutionOutside, nil
	}
	if kind == importModule {
		ref, declared := pkg.funcs[selector.Sel.Name]
		if !declared {
			return nil, resolutionUnnamed, nil
		}
		return ref, resolutionFound, nil
	}
	ref, how := resolveMethod(fn, selector.Sel.Name)
	return ref, how, nil
}

// resolveMethod names a method call by its name alone, which answers only
// when exactly one type in the package declares a method of that name.
func resolveMethod(fn *funcRef, name string) (*funcRef, resolution) {
	methods := fn.pkg.methods[name]
	if len(methods) != 1 {
		return nil, resolutionUnnamed
	}
	return methods[0], resolutionFound
}

// importedPackage answers what name is in file: no import, a package outside
// the module, or a module package, read.
func (ix *index) importedPackage(file *sourceFile, name string) (*goPackage, importKind, error) {
	for _, spec := range file.syntax.Imports {
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return nil, importUnspecified, fmt.Errorf("%s: import %s: %w", file.rel, spec.Path.Value, err)
		}
		rel, inModule := ix.moduleDir(importPath)
		named, err := ix.importNames(spec, importPath, rel, inModule, name)
		if err != nil {
			return nil, importUnspecified, err
		}
		if !named {
			continue
		}
		if !inModule {
			return nil, importExternal, nil
		}
		pkg, err := ix.packageAt(rel)
		if err != nil {
			return nil, importUnspecified, err
		}
		return pkg, importModule, nil
	}
	return nil, importAbsent, nil
}

// importNames reports whether one import spec is the package a file calls
// name. An unaliased module package is read when its directory name differs,
// because its package clause decides the name a caller types.
func (ix *index) importNames(spec *ast.ImportSpec, importPath, rel string, inModule bool, name string) (bool, error) {
	if spec.Name != nil {
		return spec.Name.Name == name, nil
	}
	if path.Base(importPath) == name {
		return true, nil
	}
	if !inModule {
		return false, nil
	}
	pkg, err := ix.packageAt(rel)
	if err != nil {
		return false, err
	}
	return pkg.name == name, nil
}

// lookup answers the value name was last bound to before at, in fn or a
// function it is nested in, which result of that value name holds, and the
// function that bound it; then a package-level var. The result is the
// target's position when one multi-value call answers several targets
// (`a, b := f()`), and 0 otherwise. A name bound nowhere (a parameter, a
// declared function) answers nil.
func lookup(fn *funcRef, name string, at token.Pos) (ast.Expr, int, *funcRef) {
	before := at
	for scope := fn; scope != nil; scope = scope.outer {
		if value, result := lastAssignment(scope.body, name, before); value != nil {
			return value, result, scope
		}
		before = scope.opened
	}
	declared, held := fn.pkg.vars[name]
	if !held {
		return nil, 0, nil
	}
	return declared.value, 0, &funcRef{name: name, pkg: fn.pkg, file: declared.file, body: &ast.BlockStmt{}}
}

// lastAssignment answers the value of the last assignment to name that ENDS
// before before, outside nested function literals.
//
// The end, not the start, is what is compared: in `err = fmt.Errorf("x: %w",
// err)` the operand's own assignment starts before it, and choosing it would
// trace the call into itself. The scan is by source position, so an
// assignment in a branch the wrap does not follow can be chosen; the trace is
// a reading of the syntax, not of the control flow.
func lastAssignment(body *ast.BlockStmt, name string, before token.Pos) (ast.Expr, int) {
	var found ast.Expr
	var foundResult int
	ast.Inspect(body, func(node ast.Node) bool {
		if node == nil {
			return false
		}
		if node.Pos() >= before {
			return false
		}
		switch stmt := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.AssignStmt:
			if stmt.End() > before {
				return true
			}
			if value, result := assignedValue(stmt.Lhs, stmt.Rhs, name); value != nil {
				found, foundResult = value, result
			}
		case *ast.ValueSpec:
			if stmt.End() > before {
				return true
			}
			if value, result := declaredValue(stmt, name); value != nil {
				found, foundResult = value, result
			}
		}
		return true
	})
	return found, foundResult
}

// assignedValue answers the expression an assignment gives name: its own
// right-hand side, or the one multi-value call every target shares, with the
// result it holds.
func assignedValue(targets, values []ast.Expr, name string) (ast.Expr, int) {
	for position, target := range targets {
		ident, isIdent := target.(*ast.Ident)
		if !isIdent {
			continue
		}
		if ident.Name != name {
			continue
		}
		return pickValue(values, len(targets), position)
	}
	return nil, 0
}

// declaredValue answers the expression a var declaration gives name, with the
// result it holds.
func declaredValue(spec *ast.ValueSpec, name string) (ast.Expr, int) {
	for position, ident := range spec.Names {
		if ident.Name != name {
			continue
		}
		return pickValue(spec.Values, len(spec.Names), position)
	}
	return nil, 0
}

// pickValue answers the value of target position out of targets: its own
// expression (result 0) when each target has one, the shared call and the
// position as its result when one call answers them all, and nil for a
// declaration without a value.
func pickValue(values []ast.Expr, targets, position int) (ast.Expr, int) {
	if len(values) == targets {
		return values[position], 0
	}
	if len(values) == 1 {
		return values[0], position
	}
	return nil, 0
}

// callsStd reports whether call is importPath.name, a function of a package
// the file imports.
func callsStd(file *ast.File, call *ast.CallExpr, importPath, name string) bool {
	selector, isSelector := call.Fun.(*ast.SelectorExpr)
	if !isSelector {
		return false
	}
	if selector.Sel.Name != name {
		return false
	}
	receiver, isIdent := selector.X.(*ast.Ident)
	if !isIdent {
		return false
	}
	for _, spec := range file.Imports {
		if spec.Path.Value != strconv.Quote(importPath) {
			continue
		}
		local := path.Base(importPath)
		if spec.Name != nil {
			local = spec.Name.Name
		}
		return local == receiver.Name
	}
	return false
}

// funcName names a declared function, with its receiver type for a method.
func funcName(decl *ast.FuncDecl) string {
	if decl.Recv == nil {
		return decl.Name.Name
	}
	if len(decl.Recv.List) == 0 {
		return decl.Name.Name
	}
	receiver := decl.Recv.List[0].Type
	if star, isStar := receiver.(*ast.StarExpr); isStar {
		receiver = star.X
	}
	if generic, isGeneric := receiver.(*ast.IndexExpr); isGeneric {
		receiver = generic.X
	}
	if generic, isGeneric := receiver.(*ast.IndexListExpr); isGeneric {
		receiver = generic.X
	}
	ident, isIdent := receiver.(*ast.Ident)
	if !isIdent {
		return decl.Name.Name
	}
	return ident.Name + "." + decl.Name.Name
}
