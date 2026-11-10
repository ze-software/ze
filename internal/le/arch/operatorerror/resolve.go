// Design: docs/architecture/core-design.md -- registration; the operator boundary set is derived from it
//
// Overview: operatorerror.go -- the boundary set and the return-site statuses
//
// resolve.go is the package index the scan reads and the resolution from a
// registration's handler expression to the function body it names. It reads
// syntax only, with no type checker: a type check of the whole module under
// every feature tag costs minutes, and the registration shapes name their
// handler in a form the syntax answers (a function, a method value, a
// literal, a factory returning a literal). A handler it cannot follow is an
// Unresolved row, never a handler with no plain error.

package archoperatorerror

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	pathpkg "path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// productRoots are the trees whose code ships. internal/le and internal/test
// are development tooling and test fakes: no operator reads their refusals.
var productRoots = []string{"cmd", "internal", "pkg"}

// excludedDirs are the subtrees under productRoots that do not ship.
var excludedDirs = []string{"internal/le", "internal/test"}

// overlayFile replaces one file's working-tree bytes for a scan: a commit's
// carried blob, or its parent's. present false means the side does not hold
// the file.
type overlayFile struct {
	text    string
	present bool
}

// packageIndex is every parsed shipped file, by directory. It is built once
// per scan and only read after; it is not safe for concurrent use while
// indexTree builds it.
type packageIndex struct {
	fset *token.FileSet
	dirs map[string]*packageDir
	// order is every directory, sorted, so a scan answers in a fixed order.
	order []string
}

// packageDir is one package's declarations.
type packageDir struct {
	name    string
	files   []*sourceFile
	funcs   map[string][]funcRef
	methods map[string][]funcRef
	values  map[string]valueRef
}

// sourceFile is one parsed file and the import names it binds.
type sourceFile struct {
	index *packageIndex
	// path is relative to the tree, slash-separated; dir is its directory.
	path    string
	dir     string
	ast     *ast.File
	imports map[string]string
}

// funcRef is a declared function or method.
type funcRef struct {
	file *sourceFile
	decl *ast.FuncDecl
}

// valueRef is a package-level var or const with one value expression.
type valueRef struct {
	file *sourceFile
	expr ast.Expr
}

// handlerTarget is one function body a registration made a boundary.
type handlerTarget struct {
	file *sourceFile
	name string
	typ  *ast.FuncType
	body *ast.BlockStmt
}

// handlerExpr is the expression a registration names its handler with.
type handlerExpr struct {
	file      *sourceFile
	expr      ast.Expr
	enclosing string
}

// foundRegistration is one registration and the handler expressions it names.
type foundRegistration struct {
	registration Registration
	handlers     []handlerExpr
}

// indexTree parses every shipped Go file under tree's product roots, with
// overlay replacing the bytes of the paths it names.
func indexTree(tree string, overlay map[string]overlayFile) (*packageIndex, error) {
	paths, err := shippedPaths(tree)
	if err != nil {
		return nil, err
	}
	for path, file := range overlay {
		if !shipped(path) {
			continue
		}
		if !file.present {
			continue
		}
		if slices.Contains(paths, path) {
			continue
		}
		paths = append(paths, path)
	}
	slices.Sort(paths)

	index := &packageIndex{fset: token.NewFileSet(), dirs: make(map[string]*packageDir)}
	for _, path := range paths {
		var source any
		if replaced, isOverlaid := overlay[path]; isOverlaid {
			if !replaced.present {
				continue
			}
			source = replaced.text
		}
		parsed, err := parser.ParseFile(index.fset, filepath.Join(tree, filepath.FromSlash(path)), source, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		index.add(path, parsed)
	}
	for _, dir := range index.order {
		for _, file := range index.dirs[dir].files {
			file.imports = index.importNames(file.ast)
		}
	}
	return index, nil
}

// shippedPaths walks tree's product roots for the files shipped answers true.
// A root the tree does not hold is skipped: a fixture tree holds one or two.
func shippedPaths(tree string) ([]string, error) {
	var paths []string
	for _, root := range productRoots {
		err := filepath.WalkDir(filepath.Join(tree, root), func(full string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			rel, err := filepath.Rel(tree, full)
			if err != nil {
				return err
			}
			rel = filepath.ToSlash(rel)
			if entry.IsDir() {
				if skippedDir(rel) {
					return filepath.SkipDir
				}
				return nil
			}
			if shipped(rel) {
				paths = append(paths, rel)
			}
			return nil
		})
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
	}
	return paths, nil
}

// skippedDir reports whether a directory holds no shipped code.
func skippedDir(rel string) bool {
	for segment := range strings.SplitSeq(rel, "/") {
		if segment == "vendor" {
			return true
		}
		if segment == "testdata" {
			return true
		}
		if strings.HasPrefix(segment, ".") {
			return true
		}
	}
	for _, excluded := range excludedDirs {
		if rel == excluded {
			return true
		}
		if strings.HasPrefix(rel, excluded+"/") {
			return true
		}
	}
	return false
}

// shipped reports whether a tree-relative path is shipped Go source.
func shipped(rel string) bool {
	if !strings.HasSuffix(rel, ".go") {
		return false
	}
	if strings.HasSuffix(rel, "_test.go") {
		return false
	}
	if skippedDir(pathpkg.Dir(rel)) {
		return false
	}
	for _, root := range productRoots {
		if strings.HasPrefix(rel, root+"/") {
			return true
		}
	}
	return false
}

// add files one parsed file under its directory and indexes its declarations.
func (index *packageIndex) add(path string, parsed *ast.File) {
	dir := pathpkg.Dir(path)
	pkg, known := index.dirs[dir]
	if !known {
		pkg = &packageDir{
			name:    parsed.Name.Name,
			funcs:   make(map[string][]funcRef),
			methods: make(map[string][]funcRef),
			values:  make(map[string]valueRef),
		}
		index.dirs[dir] = pkg
		index.order = append(index.order, dir)
	}
	file := &sourceFile{index: index, path: path, dir: dir, ast: parsed}
	pkg.files = append(pkg.files, file)
	for _, decl := range parsed.Decls {
		switch declaration := decl.(type) {
		case *ast.FuncDecl:
			ref := funcRef{file: file, decl: declaration}
			if declaration.Recv == nil {
				pkg.funcs[declaration.Name.Name] = append(pkg.funcs[declaration.Name.Name], ref)
				continue
			}
			pkg.methods[declaration.Name.Name] = append(pkg.methods[declaration.Name.Name], ref)
		case *ast.GenDecl:
			for _, spec := range declaration.Specs {
				value, isValue := spec.(*ast.ValueSpec)
				if !isValue {
					continue
				}
				if len(value.Values) != len(value.Names) {
					continue
				}
				for position, name := range value.Names {
					pkg.values[name.Name] = valueRef{file: file, expr: value.Values[position]}
				}
			}
		}
	}
}

// importNames answers the name each import of a file binds. An unaliased
// import binds its package clause name, read from the index where the tree
// holds the package, and the path's last element otherwise (the standard
// library). Blank and dot imports bind no name a selector can use.
func (index *packageIndex) importNames(parsed *ast.File) map[string]string {
	names := make(map[string]string, len(parsed.Imports))
	for _, spec := range parsed.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		if spec.Name != nil {
			names[spec.Name.Name] = path
			continue
		}
		names[index.packageName(path)] = path
	}
	return names
}

// packageName answers the package clause name of an import path.
func (index *packageIndex) packageName(path string) string {
	rel, inModule := strings.CutPrefix(path, modulePath+"/")
	if inModule {
		if pkg, known := index.dirs[rel]; known {
			return pkg.name
		}
	}
	return pathpkg.Base(path)
}

// packagePath answers a file's own import path.
func (file *sourceFile) packagePath() string { return modulePath + "/" + file.dir }

// importsAs reports whether expr is an identifier naming the import of path.
func (file *sourceFile) importsAs(expr ast.Expr, path string) bool {
	ident, isIdent := expr.(*ast.Ident)
	return isIdent && file.imports[ident.Name] == path
}

// refersTo reports whether expr names the declaration name of the package at
// path: `name` inside that package, or `alias.name` where alias imports it.
func (file *sourceFile) refersTo(expr ast.Expr, path, name string) bool {
	switch node := ast.Unparen(expr).(type) {
	case *ast.Ident:
		if node.Name != name {
			return false
		}
		return file.packagePath() == path
	case *ast.SelectorExpr:
		if node.Sel.Name != name {
			return false
		}
		return file.importsAs(node.X, path)
	default:
		return false
	}
}

// registrations answers every registration site in the index, in file order.
func (index *packageIndex) registrations() []foundRegistration {
	var found []foundRegistration
	for _, dir := range index.order {
		for _, file := range index.dirs[dir].files {
			found = append(found, file.registrations()...)
		}
	}
	return found
}

// registrations answers the registration sites in one file.
func (file *sourceFile) registrations() []foundRegistration {
	var found []foundRegistration
	for _, decl := range file.ast.Decls {
		enclosing := packageScope
		if funcDecl, isFunc := decl.(*ast.FuncDecl); isFunc {
			enclosing = funcName(funcDecl)
		}
		ast.Inspect(decl, func(node ast.Node) bool {
			switch expression := node.(type) {
			case *ast.CompositeLit:
				found = append(found, file.literalRegistrations(expression, enclosing)...)
			case *ast.CallExpr:
				if registration, isRegistration := file.callRegistration(expression, enclosing); isRegistration {
					found = append(found, registration)
				}
			}
			return true
		})
	}
	return found
}

// literalKind is the registration struct a composite literal builds.
type literalKind uint8

// The struct types a registration literal is built from.
const (
	literalNone literalKind = iota
	literalRPC
	literalRegistration
)

// literalType answers which registration struct a type expression names.
func (file *sourceFile) literalType(typ ast.Expr) literalKind {
	if star, isStar := typ.(*ast.StarExpr); isStar {
		typ = star.X
	}
	if file.refersTo(typ, serverPath, "RPCRegistration") {
		return literalRPC
	}
	if file.refersTo(typ, registryPath, "Registration") {
		return literalRegistration
	}
	return literalNone
}

// literalRegistrations answers the registrations one composite literal
// builds: the literal itself when its type is a registration struct, and the
// elements whose type a slice or array literal elides.
func (file *sourceFile) literalRegistrations(literal *ast.CompositeLit, enclosing string) []foundRegistration {
	if literal.Type == nil {
		// An elided element type is judged by the slice literal that holds it.
		return nil
	}
	if kind := file.literalType(literal.Type); kind != literalNone {
		return file.fieldRegistrations(literal, kind, enclosing)
	}
	array, isArray := literal.Type.(*ast.ArrayType)
	if isArray {
		return file.elidedRegistrations(literal, file.literalType(array.Elt), enclosing)
	}
	// Any other literal (a map, a struct of another type) registers nothing.
	return nil
}

// elidedRegistrations reads the elements of a slice or array literal whose
// element type is a registration struct and is elided in each element.
func (file *sourceFile) elidedRegistrations(literal *ast.CompositeLit, kind literalKind, enclosing string) []foundRegistration {
	var found []foundRegistration
	if kind == literalNone {
		return found
	}
	for _, element := range literal.Elts {
		if unary, isUnary := element.(*ast.UnaryExpr); isUnary {
			element = unary.X
		}
		inner, isLiteral := element.(*ast.CompositeLit)
		if !isLiteral {
			continue
		}
		if inner.Type != nil {
			continue
		}
		found = append(found, file.fieldRegistrations(inner, kind, enclosing)...)
	}
	return found
}

// fieldRegistrations reads the handler fields of one registration literal.
// Unkeyed fields are not read: no registration in the tree spells them, and
// go vet refuses an unkeyed literal of an imported struct.
func (file *sourceFile) fieldRegistrations(literal *ast.CompositeLit, kind literalKind, enclosing string) []foundRegistration {
	fields := make(map[string]ast.Expr, len(literal.Elts))
	for _, element := range literal.Elts {
		pair, isPair := element.(*ast.KeyValueExpr)
		if !isPair {
			continue
		}
		key, isKey := pair.Key.(*ast.Ident)
		if !isKey {
			continue
		}
		fields[key.Name] = pair.Value
	}
	line := file.index.fset.Position(literal.Lbrace).Line
	var found []foundRegistration
	if kind == literalRPC {
		if handler, named := fields["Handler"]; named {
			if !isNilIdent(handler) {
				registration := Registration{File: file.path, Line: line, Kind: KindRPC, WireMethod: file.stringValue(fields["WireMethod"])}
				if registration.WireMethod == "" {
					registration.WirePrefix = file.wirePrefix(fields["WireMethod"])
				}
				found = append(found, foundRegistration{registration: registration, handlers: []handlerExpr{{file: file, expr: handler, enclosing: enclosing}}})
			}
		}
		return found
	}
	if verifier, named := fields["InProcessConfigVerifier"]; named {
		if !isNilIdent(verifier) {
			found = append(found, foundRegistration{
				registration: Registration{File: file.path, Line: line, Kind: KindConfigVerifier},
				handlers:     []handlerExpr{{file: file, expr: verifier, enclosing: enclosing}},
			})
		}
	}
	if handlers, named := fields["RPCHandlers"]; named {
		found = append(found, foundRegistration{
			registration: Registration{File: file.path, Line: line, Kind: KindPluginRPC},
			handlers:     file.mapHandlers(handlers, enclosing),
		})
	}
	return found
}

// callRegistration answers the registration a call makes, if it is one.
func (file *sourceFile) callRegistration(call *ast.CallExpr, enclosing string) (foundRegistration, bool) {
	line := file.index.fset.Position(call.Lparen).Line
	one := func(kind Kind, position int) (foundRegistration, bool) {
		if len(call.Args) <= position {
			return foundRegistration{}, false
		}
		return foundRegistration{
			registration: Registration{File: file.path, Line: line, Kind: kind},
			handlers:     []handlerExpr{{file: file, expr: call.Args[position], enclosing: enclosing}},
		}, true
	}
	if file.refersTo(call.Fun, serverPath, "RegisterStreamingHandler") {
		return one(KindStreaming, 2)
	}
	if file.refersTo(call.Fun, serverPath, "RegisterPluginStreamingHandler") {
		return one(KindStreaming, 3)
	}
	if file.refersTo(call.Fun, registryPath, "AddRPCHandlers") {
		if len(call.Args) != 1 {
			return foundRegistration{}, false
		}
		return foundRegistration{
			registration: Registration{File: file.path, Line: line, Kind: KindPluginRPC},
			handlers:     file.mapHandlers(call.Args[0], enclosing),
		}, true
	}
	selector, isSelector := call.Fun.(*ast.SelectorExpr)
	if !isSelector {
		return foundRegistration{}, false
	}
	if len(call.Args) != 1 {
		return foundRegistration{}, false
	}
	// The SDK's two callbacks are matched by method name: pkg/plugin/sdk
	// declares the only methods of those names in the module, and a syntax
	// scan cannot read the receiver's type.
	if selector.Sel.Name == "OnConfigVerify" {
		return one(KindSDKConfigVerify, 0)
	}
	if selector.Sel.Name == "OnExecuteCommand" {
		return one(KindSDKCommand, 0)
	}
	return foundRegistration{}, false
}

// mapHandlers answers the handler values of a method-to-handler map: a map
// literal, a package variable holding one, or a call to a function that
// returns one. A map the syntax cannot follow is answered as one handler
// expression the resolution then reports unresolved.
func (file *sourceFile) mapHandlers(expr ast.Expr, enclosing string) []handlerExpr {
	switch node := ast.Unparen(expr).(type) {
	case *ast.CompositeLit:
		var handlers []handlerExpr
		for _, element := range node.Elts {
			pair, isPair := element.(*ast.KeyValueExpr)
			if !isPair {
				continue
			}
			handlers = append(handlers, handlerExpr{file: file, expr: pair.Value, enclosing: enclosing})
		}
		return handlers
	case *ast.Ident:
		if value, known := file.index.dirs[file.dir].values[node.Name]; known {
			return value.file.mapHandlers(value.expr, packageScope)
		}
	case *ast.CallExpr:
		var handlers []handlerExpr
		for _, ref := range file.index.functions(file, node.Fun) {
			for _, result := range topLevelResults(ref.decl.Body) {
				literal, isLiteral := result.(*ast.CompositeLit)
				if !isLiteral {
					continue
				}
				handlers = append(handlers, ref.file.mapHandlers(literal, funcName(ref.decl))...)
			}
		}
		if len(handlers) > 0 {
			return handlers
		}
	}
	return []handlerExpr{{file: file, expr: expr, enclosing: enclosing}}
}

// resolveHandler answers the function bodies a handler expression names, or
// none and the reason the syntax could not follow it.
func (index *packageIndex) resolveHandler(handler handlerExpr) ([]handlerTarget, string) {
	file := handler.file
	switch node := ast.Unparen(handler.expr).(type) {
	case *ast.FuncLit:
		return []handlerTarget{{file: file, name: handler.enclosing + " (func literal)", typ: node.Type, body: node.Body}}, ""
	case *ast.Ident, *ast.SelectorExpr, *ast.IndexExpr:
		refs := index.functions(file, node)
		if len(refs) > 0 {
			return declTargets(refs), ""
		}
		if literal, isLiteral := index.literalValue(file, node); isLiteral {
			return []handlerTarget{{file: literal.file, name: types.ExprString(node) + " (func literal)", typ: literal.lit.Type, body: literal.lit.Body}}, ""
		}
		return nil, "names " + types.ExprString(node) + ", which is no function, method or function-valued variable the package declares"
	case *ast.CallExpr:
		var targets []handlerTarget
		for _, ref := range index.functions(file, node.Fun) {
			for _, result := range topLevelResults(ref.decl.Body) {
				literal, isLiteral := result.(*ast.FuncLit)
				if !isLiteral {
					continue
				}
				targets = append(targets, handlerTarget{file: ref.file, name: funcName(ref.decl) + " (returned func literal)", typ: literal.Type, body: literal.Body})
			}
		}
		if len(targets) > 0 {
			return targets, ""
		}
		return nil, "is a call whose result is not a function literal the called function returns"
	default:
		return nil, "is an expression the syntax scan does not follow"
	}
}

// declTargets turns declared functions into handler targets.
func declTargets(refs []funcRef) []handlerTarget {
	targets := make([]handlerTarget, 0, len(refs))
	for _, ref := range refs {
		if ref.decl.Body == nil {
			continue
		}
		targets = append(targets, handlerTarget{file: ref.file, name: funcName(ref.decl), typ: ref.decl.Type, body: ref.decl.Body})
	}
	return targets
}

// functions answers the declarations an expression names as a function: a
// function of the file's package, a function of an imported package, or a
// method of the file's package by name. Several answers are several build
// variants (fd_linux.go and fd_other.go) or several receivers holding the
// method name; each is a boundary, because the syntax cannot tell which one
// the registration reaches.
func (index *packageIndex) functions(file *sourceFile, expr ast.Expr) []funcRef {
	switch node := ast.Unparen(expr).(type) {
	case *ast.IndexExpr:
		return index.functions(file, node.X)
	case *ast.Ident:
		return index.dirs[file.dir].funcs[node.Name]
	case *ast.SelectorExpr:
		if ident, isIdent := node.X.(*ast.Ident); isIdent {
			if path, imported := file.imports[ident.Name]; imported {
				return index.importedFuncs(path, node.Sel.Name)
			}
		}
		return index.dirs[file.dir].methods[node.Sel.Name]
	default:
		return nil
	}
}

// importedFuncs answers the functions named name in the package at path. An
// import outside the tree (the standard library, a vendored module) declares
// no handler body the scan can read, which resolveHandler reports as an
// Unresolved row with its reason.
func (index *packageIndex) importedFuncs(path, name string) []funcRef {
	var refs []funcRef
	if rel, inModule := strings.CutPrefix(path, modulePath+"/"); inModule {
		if pkg, known := index.dirs[rel]; known {
			refs = pkg.funcs[name]
		}
	}
	return refs
}

// literalRef is a function literal a package variable holds.
type literalRef struct {
	file *sourceFile
	lit  *ast.FuncLit
}

// literalValue answers the function literal a package-level variable of the
// file's package holds, for `var handleX = func(...) ...`.
func (index *packageIndex) literalValue(file *sourceFile, expr ast.Expr) (literalRef, bool) {
	ident, isIdent := expr.(*ast.Ident)
	if !isIdent {
		return literalRef{}, false
	}
	value, known := index.dirs[file.dir].values[ident.Name]
	if !known {
		return literalRef{}, false
	}
	literal, isLiteral := ast.Unparen(value.expr).(*ast.FuncLit)
	if !isLiteral {
		return literalRef{}, false
	}
	return literalRef{file: value.file, lit: literal}, true
}

// topLevelResults answers the result expressions of a body's own return
// statements, not those of a function literal nested in it.
func topLevelResults(body *ast.BlockStmt) []ast.Expr {
	if body == nil {
		return nil
	}
	var results []ast.Expr
	ast.Inspect(body, func(node ast.Node) bool {
		switch statement := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			results = append(results, statement.Results...)
		}
		return true
	})
	return results
}

// stringValue answers a string literal or a constant naming one, and "" for
// any other expression: a computed wire method, which Registration.WireMethod
// documents as "".
func (file *sourceFile) stringValue(expr ast.Expr) string {
	text := ""
	switch node := ast.Unparen(expr).(type) {
	case *ast.BasicLit:
		if node.Kind == token.STRING {
			if unquoted, err := strconv.Unquote(node.Value); err == nil {
				text = unquoted
			}
		}
	case *ast.Ident:
		if value, known := file.index.dirs[file.dir].values[node.Name]; known {
			text = value.file.stringValue(value.expr)
		}
	case *ast.SelectorExpr:
		if value, known := file.importedValue(node); known {
			text = value.file.stringValue(value.expr)
		}
	}
	return text
}

// wirePrefix answers the literal head of a concatenation, `"ze-host:show-"`
// for `"ze-host:show-" + section`, and "" for any expression no literal
// leads. The recursion follows the left operand of a left-associative `+`
// chain, so it is bounded by the expression's own depth.
func (file *sourceFile) wirePrefix(expr ast.Expr) string {
	prefix := ""
	binary, isBinary := ast.Unparen(expr).(*ast.BinaryExpr)
	if isBinary && binary.Op == token.ADD {
		prefix = file.stringValue(binary.X)
		if prefix == "" {
			prefix = file.wirePrefix(binary.X)
		}
	}
	return prefix
}

// importedValue answers the package-level value `alias.Name` names in a
// package the tree holds.
func (file *sourceFile) importedValue(selector *ast.SelectorExpr) (valueRef, bool) {
	ident, isIdent := selector.X.(*ast.Ident)
	if isIdent {
		if rel, inModule := strings.CutPrefix(file.imports[ident.Name], modulePath+"/"); inModule {
			if pkg, known := file.index.dirs[rel]; known {
				value, declared := pkg.values[selector.Sel.Name]
				return value, declared
			}
		}
	}
	return valueRef{}, false
}

// isNilIdent reports whether expr is the identifier nil.
func isNilIdent(expr ast.Expr) bool {
	ident, isIdent := ast.Unparen(expr).(*ast.Ident)
	return isIdent && ident.Name == "nil"
}

// packageScope names the enclosing function of a registration at package
// level, which has no declaration of its own.
const packageScope = "(package scope)"

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
