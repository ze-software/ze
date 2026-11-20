// Design: docs/architecture/third-party-credits.md -- what a carrying package declares
// Overview: check.go -- the refusals these declarations answer for

package credits

import (
	"bytes"
	"errors"
	"go/ast"
	"go/parser"
	"go/token"
	"path"
	"strconv"
	"strings"

	"github.com/ze-software/ze/internal/le/deps"
)

// presence is one credits.DeclarePresent call found in Go source.
type presence struct {
	// work is the literal name the call declares.
	work string
	// pkg is the import path of the package holding the call.
	pkg string
	// at is the file and line of the call, for a refusal.
	at string
}

// declarePresent is the function a carrying package calls.
const declarePresent = "DeclarePresent"

// presenceDeclarations reads every DeclarePresent call of the Ze module's
// non-test Go files with the Go parser. A file that does not mention the
// credits import path is not parsed: it cannot call the package. A call whose
// argument is not a string literal is answered as a refusal, because the
// check cannot read which work it declares.
func presenceDeclarations(root, module string, files []string) ([]presence, []Refusal, error) {
	importPath := module + "/" + creditsPackage
	quoted := []byte(strconv.Quote(importPath))
	var found []presence
	var refusals []Refusal
	for _, rel := range files {
		if strings.HasSuffix(rel, "_test.go") {
			continue
		}
		if path.Dir(rel) == creditsPackage {
			continue
		}
		judged, err := deps.Judged(root, rel)
		if err != nil {
			return nil, nil, err
		}
		if !judged {
			continue
		}
		src, err := readRel(root, rel)
		if err != nil {
			return nil, nil, err
		}
		if !bytes.Contains(src, quoted) {
			continue
		}
		calls, problems, err := fileDeclarations(rel, src, importPath, module)
		if err != nil {
			return nil, nil, err
		}
		found = append(found, calls...)
		refusals = append(refusals, problems...)
	}
	return found, refusals, nil
}

// fileDeclarations answers the DeclarePresent calls of one parsed file.
func fileDeclarations(rel string, src []byte, importPath, module string) ([]presence, []Refusal, error) {
	files := token.NewFileSet()
	file, err := parser.ParseFile(files, rel, src, parser.SkipObjectResolution)
	if err != nil {
		return nil, nil, errors.New("parse " + rel + ": " + err.Error())
	}
	local, imported := localName(file, importPath)
	if !imported {
		return nil, nil, nil
	}
	pkg := module + "/" + path.Dir(rel)
	var found []presence
	var refusals []Refusal
	ast.Inspect(file, func(node ast.Node) bool {
		call, isCall := node.(*ast.CallExpr)
		if !isCall {
			return true
		}
		if !callsDeclarePresent(call, local) {
			return true
		}
		at := rel + ":" + strconv.Itoa(files.Position(call.Pos()).Line)
		name, literal := literalArgument(call)
		if !literal {
			refusals = append(refusals, Refusal{
				Work:    at,
				Problem: "calls credits.DeclarePresent with an argument that is not one string literal",
				Why:     "the check reads which work a package declares from its source, and cannot read a computed name",
				Edit:    "pass the work's name, as website/data/dependencies.json spells it, as a string literal",
				Fix:     updateCommand,
			})
			return true
		}
		found = append(found, presence{work: name, pkg: pkg, at: at})
		return true
	})
	return found, refusals, nil
}

// localName answers the name file uses for importPath: its alias, "credits"
// when it has none, "" for a dot import, and false when the file does not
// import it or imports it blank.
func localName(file *ast.File, importPath string) (string, bool) {
	for _, spec := range file.Imports {
		value, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		if value != importPath {
			continue
		}
		if spec.Name == nil {
			return path.Base(importPath), true
		}
		switch spec.Name.Name {
		case "_":
			return "", false
		case ".":
			return "", true
		}
		return spec.Name.Name, true
	}
	return "", false
}

// callsDeclarePresent answers whether call is local.DeclarePresent(...), or
// the bare DeclarePresent(...) of a dot import (local "").
func callsDeclarePresent(call *ast.CallExpr, local string) bool {
	if local == "" {
		ident, bare := call.Fun.(*ast.Ident)
		return bare && ident.Name == declarePresent
	}
	selector, selected := call.Fun.(*ast.SelectorExpr)
	if !selected {
		return false
	}
	owner, named := selector.X.(*ast.Ident)
	if !named {
		return false
	}
	return owner.Name == local && selector.Sel.Name == declarePresent
}

// literalArgument answers the call's one string-literal argument.
func literalArgument(call *ast.CallExpr) (string, bool) {
	if len(call.Args) != 1 {
		return "", false
	}
	literal, isLiteral := call.Args[0].(*ast.BasicLit)
	if !isLiteral {
		return "", false
	}
	if literal.Kind != token.STRING {
		return "", false
	}
	value, err := strconv.Unquote(literal.Value)
	if err != nil {
		return "", false
	}
	return value, true
}
