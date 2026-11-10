// Design: docs/contributing/error-inventory.md -- the inventory of user-visible message sites
//
// Detail: writers.go -- the surface writers the scan looks for
// Detail: classify.go -- the status of each writer's message
// Related: inventory.go -- the Row each site becomes
//
// scan.go parses the tree's Go once and lists every call or literal of a
// surface writer (writers.go). It reads no log line, report raise, health
// row, Diagnostic literal or registered handler return: those sites are
// archdoctorcoverage's and archoperatorerror's, and a second scan of them
// would count them twice.

package archerrorinventory

import (
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// modulePath is the import path prefix of the tree's own packages.
const modulePath = "github.com/ze-software/ze"

// parseRoots are the trees whose Go is parsed.
var parseRoots = []string{"cmd", "internal", "pkg"}

// toolingDir is le's tree: only the ./le writers match under it.
const toolingDir = "internal/le"

// excludedDirs are the subtrees under parseRoots whose messages reach no
// user: the functional test harness and its fixtures.
var excludedDirs = []string{"internal/test"}

// sourceFile is one parsed Go file.
type sourceFile struct {
	// path is relative to the tree, slash-separated; dir is its directory.
	path string
	dir  string
	ast  *ast.File
	// imports maps each name the file refers to an import by, to its path.
	imports map[string]string
}

// funcRef is one package-level function declaration and its file.
type funcRef struct {
	decl *ast.FuncDecl
	file *sourceFile
}

// treeIndex is every parsed file and the declarations the classifier
// resolves names against. It is built once per scan and only read after,
// except for typedFuncs, which the classifier fills as it answers; it is not
// safe for concurrent use.
type treeIndex struct {
	fset  *token.FileSet
	files []*sourceFile
	// packageNames maps a directory to the package name its files declare.
	packageNames map[string]string
	// funcs maps a directory to its package-level functions (no receiver).
	funcs map[string]map[string]funcRef
	// constants maps a directory to the value of each constant it declares.
	constants map[string]map[string]ast.Expr
	// findingTypes maps an ./le directory to the struct types it declares
	// whose names end in findingSuffix.
	findingTypes map[string]map[string]bool
	// typedFuncs memoizes answersOperatorError per function; a function
	// still being answered is present with false, which ends a recursion.
	typedFuncs map[*ast.FuncDecl]bool
}

// overlayFile replaces one file's working-tree bytes for a scan: a commit's
// carried text, or a path that side does not hold (present false).
type overlayFile struct {
	text    string
	present bool
}

// writerRows answers one row per surface writer call or literal in the tree
// at root, in file and position order. A path the overlay holds is read from
// it rather than from the working tree (indexTree).
func writerRows(root string, overlay map[string]overlayFile) ([]Row, error) {
	index, err := indexTree(root, overlay)
	if err != nil {
		return nil, err
	}
	rows := []Row{}
	for _, file := range index.files {
		found, err := index.rowsOf(file)
		if err != nil {
			return nil, err
		}
		rows = append(rows, found...)
	}
	return rows, nil
}

// rowsOf answers the writer rows of one file. A writer call's arguments are
// its message, so a writer nested in them (a LocalRefusal inside a stderr
// write) is part of the outer row, never a row of its own.
func (index *treeIndex) rowsOf(file *sourceFile) ([]Row, error) {
	var rows []Row
	var failed error
	for _, decl := range file.ast.Decls {
		fn, _ := decl.(*ast.FuncDecl)
		sc := scope{index: index, file: file, fn: fn}
		ast.Inspect(decl, func(node ast.Node) bool {
			if failed != nil {
				return false
			}
			switch expression := node.(type) {
			case *ast.CallExpr:
				w, isWriter := callWriter(file, expression)
				if !isWriter {
					return true
				}
				row, err := sc.row(w, w.label(), expression, w.message.of(expression.Args))
				if err != nil {
					failed = err
					return false
				}
				rows = append(rows, row)
				return false
			case *ast.CompositeLit:
				found, matched, err := sc.literalRows(expression)
				if err != nil {
					failed = err
					return false
				}
				rows = append(rows, found...)
				return !matched
			}
			return true
		})
	}
	return rows, failed
}

// literalRows answers the rows of a composite literal: the literal itself
// when its type is a writer's, or each element whose type a slice, array or
// map literal of a writer's type elides. matched says whether the literal
// was a writer's, so its elements are not walked again.
func (sc scope) literalRows(lit *ast.CompositeLit) ([]Row, bool, error) {
	if lit.Type == nil {
		return nil, false, nil
	}
	if w, label, isWriter := sc.index.literalWriter(sc.file, lit.Type); isWriter {
		row, err := sc.row(w, label, lit, w.message.of(literalValues(lit)))
		if err != nil {
			return nil, true, err
		}
		return []Row{row}, true, nil
	}
	element := elementType(lit.Type)
	if element == nil {
		return nil, false, nil
	}
	w, label, isWriter := sc.index.literalWriter(sc.file, element)
	if !isWriter {
		return nil, false, nil
	}
	var rows []Row
	for _, value := range literalValues(lit) {
		inner, isLit := ast.Unparen(value).(*ast.CompositeLit)
		if !isLit {
			continue
		}
		if inner.Type != nil {
			continue
		}
		row, err := sc.row(w, label, inner, w.message.of(literalValues(inner)))
		if err != nil {
			return nil, true, err
		}
		rows = append(rows, row)
	}
	return rows, true, nil
}

// elementType answers the element type of a slice, array or map type, or nil.
func elementType(typ ast.Expr) ast.Expr {
	switch node := typ.(type) {
	case *ast.ArrayType:
		return node.Elt
	case *ast.MapType:
		return node.Value
	default:
		return nil
	}
}

// literalValues answers a composite literal's element values, the value of
// each keyed element included.
func literalValues(lit *ast.CompositeLit) []ast.Expr {
	values := make([]ast.Expr, 0, len(lit.Elts))
	for _, element := range lit.Elts {
		if keyed, isKeyed := element.(*ast.KeyValueExpr); isKeyed {
			values = append(values, keyed.Value)
			continue
		}
		values = append(values, element)
	}
	return values
}

// row answers the row of one writer site whose message is the expressions
// message. A writer's site is a refusal, so its pre-rank follows from its
// surface alone.
func (sc scope) row(w *writer, label string, node ast.Node, message []ast.Expr) (Row, error) {
	rank, err := preRankOf(w.surface, SeverityRefusal)
	if err != nil {
		return Row{}, err
	}
	return Row{
		Surface: w.surface,
		File:    sc.file.path,
		Line:    sc.index.fset.Position(node.Pos()).Line,
		EndLine: sc.index.fset.Position(node.End()).Line,
		Package: sc.file.dir,
		Status:  sc.classify(message),
		PreRank: rank,
		Source:  SourceWriter,
		Writer:  label,
		Message: sc.messageText(message),
	}, nil
}

// indexTree parses every Go file under tree's parse roots that is not a test
// and does not sit in an excluded directory. A path the overlay holds is read
// from it: present, its text replaces the working-tree file or adds a file
// the tree does not hold; absent, the file is left out.
func indexTree(tree string, overlay map[string]overlayFile) (*treeIndex, error) {
	paths, err := parsedPaths(tree)
	if err != nil {
		return nil, err
	}
	for path, file := range overlay {
		if !parsed(path) {
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

	index := &treeIndex{
		fset:         token.NewFileSet(),
		packageNames: make(map[string]string),
		funcs:        make(map[string]map[string]funcRef),
		constants:    make(map[string]map[string]ast.Expr),
		findingTypes: make(map[string]map[string]bool),
		typedFuncs:   make(map[*ast.FuncDecl]bool),
	}
	for _, path := range paths {
		var source any
		if replaced, isOverlaid := overlay[path]; isOverlaid {
			if !replaced.present {
				continue
			}
			source = replaced.text
		}
		parsedFile, err := parser.ParseFile(index.fset, filepath.Join(tree, filepath.FromSlash(path)), source, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		file := &sourceFile{path: path, dir: dirOf(path), ast: parsedFile}
		index.files = append(index.files, file)
		index.packageNames[file.dir] = parsedFile.Name.Name
		index.addDeclarations(file)
	}
	for _, file := range index.files {
		file.imports = index.importNames(file.ast)
	}
	return index, nil
}

// addDeclarations records a file's package-level functions, constants and,
// under internal/le, its finding struct types.
func (index *treeIndex) addDeclarations(file *sourceFile) {
	for _, decl := range file.ast.Decls {
		switch node := decl.(type) {
		case *ast.FuncDecl:
			if node.Recv != nil {
				continue
			}
			if index.funcs[file.dir] == nil {
				index.funcs[file.dir] = make(map[string]funcRef)
			}
			index.funcs[file.dir][node.Name.Name] = funcRef{decl: node, file: file}
		case *ast.GenDecl:
			index.addGenDecl(file, node)
		}
	}
}

// addGenDecl records the constants with a value of their own, and the
// finding struct types, one declaration group holds.
func (index *treeIndex) addGenDecl(file *sourceFile, gen *ast.GenDecl) {
	for _, spec := range gen.Specs {
		switch node := spec.(type) {
		case *ast.ValueSpec:
			if gen.Tok != token.CONST {
				continue
			}
			for i, name := range node.Names {
				if i >= len(node.Values) {
					break
				}
				if index.constants[file.dir] == nil {
					index.constants[file.dir] = make(map[string]ast.Expr)
				}
				index.constants[file.dir][name.Name] = node.Values[i]
			}
		case *ast.TypeSpec:
			if !underAny(file.dir, []string{toolingDir}) {
				continue
			}
			if !strings.HasSuffix(node.Name.Name, findingSuffix) {
				continue
			}
			if _, isStruct := node.Type.(*ast.StructType); !isStruct {
				continue
			}
			if index.findingTypes[file.dir] == nil {
				index.findingTypes[file.dir] = make(map[string]bool)
			}
			index.findingTypes[file.dir][node.Name.Name] = true
		}
	}
}

// importNames maps each name a file refers to an import by to its path: the
// alias when it has one, else the package name the tree declares for that
// directory, else the path's last element (the standard library, vendor).
// A blank or dot import gives no name.
func (index *treeIndex) importNames(file *ast.File) map[string]string {
	names := make(map[string]string, len(file.Imports))
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		if spec.Name != nil {
			if spec.Name.Name == "_" || spec.Name.Name == "." {
				continue
			}
			names[spec.Name.Name] = path
			continue
		}
		if dir, inTree := cutModule(path); inTree {
			if name, known := index.packageNames[dir]; known {
				names[name] = path
				continue
			}
		}
		names[path[strings.LastIndexByte(path, '/')+1:]] = path
	}
	return names
}

// packagePath answers the import path of the file's own package.
func (file *sourceFile) packagePath() string { return modulePath + "/" + file.dir }

// importsAs reports whether expr is an identifier naming the import of path.
func (file *sourceFile) importsAs(expr ast.Expr, path string) bool {
	ident, isIdent := expr.(*ast.Ident)
	if !isIdent {
		return false
	}
	return file.imports[ident.Name] == path
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

// isStderr reports whether expr is os.Stderr.
func (file *sourceFile) isStderr(expr ast.Expr) bool {
	return file.refersTo(expr, "os", "Stderr")
}

// cutModule answers the tree directory of an import path inside the module.
func cutModule(path string) (string, bool) {
	return strings.CutPrefix(path, modulePath+"/")
}

// parsedPaths walks tree's parse roots for the Go files indexTree parses,
// sorted. A root the tree does not hold is skipped: a fixture tree holds one
// or two.
func parsedPaths(tree string) ([]string, error) {
	var paths []string
	for _, root := range parseRoots {
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
			if !goSource(rel) {
				return nil
			}
			paths = append(paths, rel)
			return nil
		})
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
	}
	slices.Sort(paths)
	return paths, nil
}

// parsed reports whether a slash-separated path is one parsedPaths walks.
func parsed(rel string) bool {
	if !goSource(rel) {
		return false
	}
	if skippedDir(dirOf(rel)) {
		return false
	}
	for _, root := range parseRoots {
		if strings.HasPrefix(rel, root+"/") {
			return true
		}
	}
	return false
}

// goSource reports whether a path is a Go file that is not a test.
func goSource(rel string) bool {
	if !strings.HasSuffix(rel, ".go") {
		return false
	}
	return !strings.HasSuffix(rel, "_test.go")
}

// skippedDir reports whether a directory holds no message a user reads.
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
	return underAny(rel, excludedDirs)
}

// underAny reports whether rel is one of bases or sits below one, on whole
// path components.
func underAny(rel string, bases []string) bool {
	for _, base := range bases {
		if rel == base {
			return true
		}
		if strings.HasPrefix(rel, base+"/") {
			return true
		}
	}
	return false
}

// dirOf answers the directory of a slash-separated path, "." for a path
// with none.
func dirOf(rel string) string {
	dir, _, found := strings.CutLast(rel, "/")
	if !found {
		return "."
	}
	return dir
}
