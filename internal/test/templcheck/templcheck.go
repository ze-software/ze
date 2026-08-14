// Design: (none -- test utility, no architecture doc)

// Package templcheck reads the components templ generates and requires every
// parameter to be a type the compiler checks a field name against.
//
// This is the guard behind AC-8 of plan/spec-web-templ-migration.md. Porting
// markup to templ buys nothing on its own: an unchecked map key stays unchecked
// inside a templ component, and html/template's failure survives the port.
// ExecuteTemplate on a map[string]any returns no error for a key the markup
// misspells. It renders an empty value and reports success, so a renamed field
// gives a blank panel and no log line.
//
// The check is fail-closed. A parameter this package cannot resolve to a
// struct, a scalar or templ.Component is reported. "I did not see it" and "it
// is safe" are different answers.
//
// A struct is walked, field by field, embedded fields included. A struct that
// wraps the map defeats the whole check one dereference in. It is also the
// cheapest port of a package whose handlers build map[string]any today.
package templcheck

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/core/textbuf"
)

// builtinScalars are the predeclared types a component CAN take. None of them
// carries a field name, so the markup reads no name the compiler misses. error
// is here because its method set is checked like any interface's.
var builtinScalars = map[string]bool{
	"bool": true, "byte": true, "complex64": true, "complex128": true,
	"error": true, "float32": true, "float64": true, "int": true,
	"int8": true, "int16": true, "int32": true, "int64": true, "rune": true,
	"string": true, "uint": true, "uint8": true, "uint16": true,
	"uint32": true, "uint64": true, "uintptr": true,
}

// reasonMap, reasonAny, reasonForeign and reasonUndeclared are the four ways a
// parameter fails. Each names what the type resolved to.
const (
	reasonMap        = "a map, whose keys the compiler never checks"
	reasonAny        = "an empty interface, which carries no field name at all"
	reasonForeign    = "declared in another package, so this check cannot see whether it is a map"
	reasonUndeclared = "a name this package does not declare, so this check cannot see whether it is a map"
	reasonUnknown    = "a type this check does not recognize"
)

// Report returns one line per problem in dir. An empty result means every
// component takes a type the compiler checks, and that the walk read want of
// them. The count is what stops the check passing over an empty set.
//
// Named types are resolved through the package's own type declarations, so
// `type viewData map[string]any` is reported as the map it is. Every .go file in
// dir supplies those declarations, test files included.
func Report(dir string, want int) ([]string, error) {
	fset := token.NewFileSet()

	decls, err := typeDecls(fset, dir)
	if err != nil {
		return nil, err
	}

	files, err := generatedFiles(dir)
	if err != nil {
		return nil, err
	}

	var (
		lines     []string
		inspected int
		tb        textbuf.Buffer
	)

	for _, file := range files {
		parsed, parseErr := parseFile(fset, file)
		if parseErr != nil {
			return nil, parseErr
		}

		for _, decl := range parsed.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || !returnsComponent(fn) {
				continue
			}

			inspected++

			for _, param := range fn.Type.Params.List {
				reason := unresolvedReason(param.Type, decls, map[string]bool{}, false)
				if reason == "" {
					continue
				}

				tb.Reset()
				tb.Str(fn.Name.Name).Str(" in ").Str(filepath.Base(file)).
					Str(" takes ").Str(paramName(param)).Str(", which is ").Str(reason)
				lines = append(lines, tb.String())
			}
		}
	}

	if inspected != want {
		tb.Reset()
		tb.Str("inspected ").Int(int64(inspected)).Str(" components in ").Str(dir).
			Str(", expected ").Int(int64(want)).
			Str("; update the count when the component set changes")
		lines = append(lines, tb.String())
	}

	return lines, nil
}

// AssertTyped fails t for every line Report returns.
func AssertTyped(t *testing.T, dir string, want int) {
	t.Helper()

	lines, err := Report(dir, want)
	if err != nil {
		t.Fatalf("read the generated components in %s: %v", dir, err)
	}

	for _, line := range lines {
		t.Error(line)
	}
}

// generatedFiles returns the templ output in dir. A directory that cannot be
// read is an error, so a wrong path never reads as a clean pass.
func generatedFiles(dir string) ([]string, error) {
	if _, err := os.Stat(dir); err != nil {
		return nil, err
	}

	return filepath.Glob(filepath.Join(dir, "*_templ.go"))
}

// typeDecls maps each type name the package declares to the expression on the
// right of its declaration.
func typeDecls(fset *token.FileSet, dir string) (map[string]ast.Expr, error) {
	files, err := filepath.Glob(filepath.Join(dir, "*.go"))
	if err != nil {
		return nil, err
	}

	decls := make(map[string]ast.Expr)

	for _, file := range files {
		parsed, parseErr := parseFile(fset, file)
		if parseErr != nil {
			return nil, parseErr
		}

		for _, decl := range parsed.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}

			for _, spec := range gen.Specs {
				if ts, isType := spec.(*ast.TypeSpec); isType {
					decls[ts.Name.Name] = ts.Type
				}
			}
		}
	}

	return decls, nil
}

// parseFile reads and parses one Go file.
func parseFile(fset *token.FileSet, path string) (*ast.File, error) {
	src, err := os.ReadFile(path) //nolint:gosec // path from a glob of the directory under test
	if err != nil {
		return nil, err
	}

	return parser.ParseFile(fset, path, src, 0)
}

// returnsComponent reports whether a function returns exactly one
// templ.Component, which is what templ generates for each component.
func returnsComponent(fn *ast.FuncDecl) bool {
	if fn.Type.Results == nil || len(fn.Type.Results.List) != 1 {
		return false
	}

	return isTemplComponent(fn.Type.Results.List[0].Type)
}

// isTemplComponent reports whether an expression spells templ.Component.
func isTemplComponent(e ast.Expr) bool {
	sel, ok := e.(*ast.SelectorExpr)
	if !ok {
		return false
	}

	pkg, ok := sel.X.(*ast.Ident)

	return ok && pkg.Name == "templ" && sel.Sel.Name == "Component"
}

// unresolvedReason returns why a parameter type is not one the compiler checks,
// or an empty string when it is. seen holds the named types already followed, so
// a recursive declaration stops. inField says the expression was reached through
// a struct field, which changes the verdict on a type from another package.
func unresolvedReason(e ast.Expr, decls map[string]ast.Expr, seen map[string]bool, inField bool) string {
	switch t := e.(type) {
	case *ast.MapType:
		return reasonMap
	case *ast.ArrayType:
		return unresolvedReason(t.Elt, decls, seen, inField)
	case *ast.Ellipsis:
		return unresolvedReason(t.Elt, decls, seen, inField)
	case *ast.StarExpr:
		return unresolvedReason(t.X, decls, seen, inField)
	case *ast.ParenExpr:
		return unresolvedReason(t.X, decls, seen, inField)
	case *ast.InterfaceType:
		if t.Methods == nil || len(t.Methods.List) == 0 {
			return reasonAny
		}

		return ""
	case *ast.StructType:
		return structReason(t, decls, seen)
	case *ast.FuncType, *ast.ChanType:
		// Neither carries a field the markup reads.
		return ""
	case *ast.SelectorExpr:
		return selectorReason(t, inField)
	case *ast.Ident:
		return identReason(t, decls, seen, inField)
	}

	return reasonUnknown
}

// structReason walks a struct's fields and returns the first that reaches a type
// carrying no compiler-checked name. Wrapping the map is no escape:
// `struct{ Data map[string]any }` leaves `v.Data["title"]` as unchecked as
// `v["title"]`. That wrapper is the cheapest port of a package that builds
// map[string]any today.
//
// An embedded field is walked like a named one, so a map reached through the
// embedded type is refused too.
func structReason(st *ast.StructType, decls map[string]ast.Expr, seen map[string]bool) string {
	if st.Fields == nil {
		return ""
	}

	for _, field := range st.Fields.List {
		reason := unresolvedReason(field.Type, decls, seen, true)
		if reason == "" {
			continue
		}

		var tb textbuf.Buffer

		tb.Str("a struct whose field ").Str(fieldLabel(field)).Str(" is ").Str(reason)

		return tb.String()
	}

	return ""
}

// selectorReason judges a type from another package.
//
// A PARAMETER of a foreign type is refused: the parameter type IS the view
// model, and this check cannot read a declaration it never parsed.
//
// A FIELD of a foreign type is accepted. The markup reaches it through a name
// the compiler checks, and refusing it would refuse template.HTML, which the web
// view models carry today. The limit that leaves is a named map declared in
// another package and held in a field. No package in ze has that shape, and
// seeing it needs the type checker rather than this walk over one directory.
func selectorReason(sel *ast.SelectorExpr, inField bool) string {
	if isTemplComponent(sel) || inField {
		return ""
	}

	return reasonForeign
}

// fieldLabel names a field for the report. An embedded field is named by its
// type, which is how the markup reaches it.
func fieldLabel(field *ast.Field) string {
	if len(field.Names) > 0 {
		names := make([]string, 0, len(field.Names))
		for _, n := range field.Names {
			names = append(names, n.Name)
		}

		return strings.Join(names, ", ")
	}

	return embeddedLabel(field.Type)
}

// embeddedLabel reads the name of an embedded type.
func embeddedLabel(e ast.Expr) string {
	switch t := e.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.StarExpr:
		return embeddedLabel(t.X)
	case *ast.SelectorExpr:
		return t.Sel.Name
	}

	return "an embedded type"
}

// identReason resolves a bare name: a predeclared scalar, the empty interface
// any, or a type the package declares.
func identReason(id *ast.Ident, decls map[string]ast.Expr, seen map[string]bool, inField bool) string {
	if id.Name == "any" {
		return reasonAny
	}

	if builtinScalars[id.Name] {
		return ""
	}

	if seen[id.Name] {
		return ""
	}

	underlying, ok := decls[id.Name]
	if !ok {
		return reasonUndeclared
	}

	seen[id.Name] = true

	return unresolvedReason(underlying, decls, seen, inField)
}

// paramName renders a parameter's names.
func paramName(param *ast.Field) string {
	names := make([]string, 0, len(param.Names))
	for _, n := range param.Names {
		names = append(names, n.Name)
	}

	if len(names) == 0 {
		return "an unnamed parameter"
	}

	return strings.Join(names, ", ")
}
