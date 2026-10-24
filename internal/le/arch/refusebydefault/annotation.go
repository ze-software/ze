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
// an identifier the file holds, or a name behind one of its imports.

package archrefusebydefault

import (
	"go/ast"
	"go/token"
	pathpkg "path"
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

// annotations answers every annotation in a file, each judged against the
// names the file can see.
func annotations(fset *token.FileSet, file *ast.File) []Annotation {
	var found []Annotation
	visible := visibleNames(file)
	for _, group := range file.Comments {
		for _, comment := range group.List {
			text := strings.TrimSpace(strings.TrimPrefix(comment.Text, "//"))
			rest, isAnnotation := strings.CutPrefix(text, annotationTag)
			if !isAnnotation {
				continue
			}
			annotation := Annotation{Line: fset.Position(comment.Slash).Line}
			annotation.Guard, annotation.Problem = parseGuard(rest, visible)
			found = append(found, annotation)
		}
	}
	return found
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
	return "", strconv.Quote(guard) + " names nothing this file holds or imports, so the guard cannot be found"
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
