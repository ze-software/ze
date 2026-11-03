// Design: docs/contributing/ze-go-style.md -- a zero another branch relies on is a named guard
// Overview: refusebydefault.go -- the shapes an annotation accepts
//
// annotation.go reads the one escape the check accepts. A zero that another
// branch relies on is a guard, and ai/rules/principles.md requires it to be
// named as one, so the annotation carries the guard's name rather than a bare
// "this is fine":
//
//	// refuse-by-default: guards <Symbol> -- <why the zero is the answer>
//
// It sits on the line of the shape or on the line above it. <Symbol> is an
// identifier or a dotted name, and it must name something the file can see:
// an identifier the file holds, a name behind one of its imports, or a name
// another non-test file of its package declares, where the dependent most
// often lives. It names the dependent, the code that relies on the zero, and never the
// function the shape sits in: that name is always visible, so it would pass
// and record nothing (namesEnclosing).

package archrefusebydefault

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	pathpkg "path"
	"path/filepath"
	"strconv"
	"strings"
)

// annotationTag opens every annotation.
const annotationTag = "refuse-by-default:"

// guardsKeyword precedes the guard's name.
const guardsKeyword = "guards"

// Annotation is one `refuse-by-default:` comment in a source file.
type Annotation struct {
	// Line is the comment's line.
	Line int
	// Guard is the name the annotation gives, "" when it gives none.
	Guard string
	// Problem says why the annotation names no guard, "" when it names one.
	Problem string
}

// Named reports whether the annotation names a guard the file can see.
func (a Annotation) Named() bool { return a.Problem == "" }

// annotations answers every annotation in a file at path, each judged against
// the names the file can see. The package's other files are read only when
// the file holds an annotation, and one that does not parse is an error: a
// guard judged against names the check could not read would be refused
// wrongly, or accepted on a name nobody declares.
func annotations(fset *token.FileSet, file *ast.File, path string) ([]Annotation, error) {
	var found []Annotation
	var texts []string
	for _, group := range file.Comments {
		for _, comment := range group.List {
			text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
			rest, isAnnotation := strings.CutPrefix(text, annotationTag)
			if !isAnnotation {
				continue
			}
			found = append(found, Annotation{Line: fset.Position(comment.Slash).Line})
			texts = append(texts, rest)
		}
	}
	if len(found) == 0 {
		return nil, nil
	}
	visible := visibleNames(file)
	if err := addPackageNames(visible, path, file.Name.Name); err != nil {
		return nil, err
	}
	for index := range found {
		found[index].Guard, found[index].Problem = parseGuard(texts[index], visible)
	}
	return found, nil
}

// addPackageNames adds every top-level name the other non-test files of the
// package in path's directory declare: functions, methods with their
// receiver type, types, variables and constants. A file of another package
// (a `package main` generator beside the code) declares nothing the file can
// see, and neither does a test file, which no shipped dependent can be.
func addPackageNames(visible map[string]bool, path, pkg string) error {
	dir := filepath.Dir(path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	siblingSet := token.NewFileSet()
	for _, entry := range entries {
		if !siblingSource(entry) {
			continue
		}
		sibling := filepath.Join(dir, entry.Name())
		if sibling == filepath.Clean(path) {
			continue
		}
		parsed, err := parser.ParseFile(siblingSet, sibling, nil, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		if parsed.Name.Name != pkg {
			continue
		}
		addDeclaredNames(visible, parsed)
	}
	return nil
}

// siblingSource reports whether a directory entry is a non-test Go file.
func siblingSource(entry os.DirEntry) bool {
	if entry.IsDir() {
		return false
	}
	name := entry.Name()
	if !strings.HasSuffix(name, ".go") {
		return false
	}
	return !strings.HasSuffix(name, "_test.go")
}

// addDeclaredNames adds the top-level names one file declares.
func addDeclaredNames(visible map[string]bool, file *ast.File) {
	for _, decl := range file.Decls {
		switch typed := decl.(type) {
		case *ast.FuncDecl:
			for segment := range strings.SplitSeq(funcName(typed), ".") {
				visible[segment] = true
			}
		case *ast.GenDecl:
			for _, spec := range typed.Specs {
				addSpecNames(visible, spec)
			}
		}
	}
}

// addSpecNames adds the names one type, variable or constant spec declares.
// An import spec declares nothing a sibling file can see.
func addSpecNames(visible map[string]bool, spec ast.Spec) {
	switch typed := spec.(type) {
	case *ast.TypeSpec:
		visible[typed.Name.Name] = true
	case *ast.ValueSpec:
		for _, name := range typed.Names {
			visible[name.Name] = true
		}
	}
}

// parseGuard answers the guard an annotation's text names, or the reason it
// names none.
func parseGuard(text string, visible map[string]bool) (guard, problem string) {
	fields := strings.Fields(text)
	if len(fields) < 2 {
		return "", "the annotation names no guard: write `refuse-by-default: guards <Symbol> -- <why>`"
	}
	if fields[0] != guardsKeyword {
		return "", "the annotation names no guard: it must open with `guards <Symbol>`, not " + strconv.Quote(fields[0])
	}
	guard = fields[1]
	if !isDottedName(guard) {
		return "", strconv.Quote(guard) + " is not a Go name, so it names no guard"
	}
	segments := strings.Split(guard, ".")
	if visible[segments[0]] {
		return guard, ""
	}
	if visible[segments[len(segments)-1]] {
		return guard, ""
	}
	return "", strconv.Quote(guard) + " names nothing this file holds or imports and nothing its package's other files declare, so the guard cannot be found"
}

// namesEnclosing reports whether a guard names the function that encloses the
// shape it annotates, fn being the hit's `Type.Method` or bare function name.
// A method is matched in both spellings, `Type.Method` and the bare `Method`.
func namesEnclosing(guard, fn string) bool {
	if guard == fn {
		return true
	}
	_, method, isMethod := strings.Cut(fn, ".")
	if !isMethod {
		return false
	}
	return guard == method
}

// enclosingProblem says why an annotation that names its own enclosing
// function names no guard, and what to write instead.
func enclosingProblem(guard string) string {
	return strconv.Quote(guard) + " names the function this shape sits in, so it records no guard: " +
		"name the caller or branch that relies on the zero, as in `refuse-by-default: guards <Dependent> -- <why>`"
}

// isDottedName reports whether a word is a Go identifier or a dotted chain of
// them.
func isDottedName(word string) bool {
	for segment := range strings.SplitSeq(word, ".") {
		if !token.IsIdentifier(segment) {
			return false
		}
	}
	return true
}

// visibleNames answers every identifier a file holds and the name each of its
// imports binds.
func visibleNames(file *ast.File) map[string]bool {
	names := map[string]bool{}
	ast.Inspect(file, func(node ast.Node) bool {
		if ident, isIdent := node.(*ast.Ident); isIdent {
			names[ident.Name] = true
		}
		return true
	})
	for _, spec := range file.Imports {
		if spec.Name != nil {
			names[spec.Name.Name] = true
			continue
		}
		importPath, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		names[pathpkg.Base(importPath)] = true
	}
	return names
}

// annotationFor answers the annotation that covers a hit: one on the hit's
// line or on the line above it. ok is false when none does.
func annotationFor(hit Hit, byLine map[int]Annotation) (Annotation, bool) {
	if annotation, ok := byLine[hit.Line]; ok {
		return annotation, true
	}
	annotation, ok := byLine[hit.Line-1]
	return annotation, ok
}
