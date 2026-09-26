// Design: docs/contributing/ze-go-style.md -- one fact per guard, checked on changed code
//
// compoundguard.go finds the guard that states two facts at once:
//
//	if a || b {
//		return err
//	}
//
// It holds exactly the meaning of two guards in sequence, because || evaluates
// its left operand first and stops at the first true one, and the body leaves
// the enclosing block either way:
//
//	if a {
//		return err
//	}
//	if b {
//		return err
//	}
//
// So the split is always available and never changes behavior, which is what
// lets a gate demand it. Three shapes fall outside that and are not reported:
// an && condition (it splits into NESTED branches, which is a design choice
// rather than a rewrite), an if with an else (the negative case has a body of
// its own), and a body that does not end by leaving (control falls through, so
// a second guard would run the body twice).

package archcompoundguard

import (
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"slices"
	"strings"

	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
)

// Guard is one splittable `||` guard in a source file.
type Guard struct {
	// Line is the line of the `if` keyword.
	Line int
	// LastLine is the line of the body's opening brace, so Line..LastLine is
	// the condition, init statement included.
	LastLine int
	// Fn is the enclosing function, `Type.Method` for a method, and
	// "(package scope)" for a function literal assigned at package level.
	Fn string
}

// packageScope names the enclosing function of a guard in a package-level
// function literal, which has no declaration of its own.
const packageScope = "(package scope)"

// ScanFile answers every splittable `||` guard in one Go source file.
//
// A file that does not parse is an error rather than a file with no guard: a
// gate that skips what it cannot read reports a clean tree for it.
func ScanFile(fset *token.FileSet, path string) ([]Guard, error) {
	file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	if ast.IsGenerated(file) {
		return nil, nil
	}
	var guards []Guard
	for _, decl := range file.Decls {
		fn := packageScope
		if funcDecl, isFunc := decl.(*ast.FuncDecl); isFunc {
			fn = funcName(funcDecl)
		}
		ast.Inspect(decl, func(node ast.Node) bool {
			ifStmt, isIf := node.(*ast.IfStmt)
			if !isIf {
				return true
			}
			if !splittable(ifStmt) {
				return true
			}
			guards = append(guards, Guard{
				Line:     fset.Position(ifStmt.If).Line,
				LastLine: fset.Position(ifStmt.Body.Lbrace).Line,
				Fn:       fn,
			})
			return true
		})
	}
	return guards, nil
}

// splittable reports whether an if statement is a guard of the one shape this
// gate judges: no else, a top-level || condition, and a body that leaves.
func splittable(ifStmt *ast.IfStmt) bool {
	if ifStmt.Else != nil {
		return false
	}
	condition, isBinary := ast.Unparen(ifStmt.Cond).(*ast.BinaryExpr)
	if !isBinary {
		return false
	}
	if condition.Op != token.LOR {
		return false
	}
	return leaves(ifStmt.Body)
}

// leaves reports whether a block's last statement transfers control out of
// it: a return, a continue, a break or a goto.
func leaves(body *ast.BlockStmt) bool {
	if len(body.List) == 0 {
		return false
	}
	switch body.List[len(body.List)-1].(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		// fallthrough cannot end an if body, because the compiler accepts it
		// only as the last statement of a switch case, so every branch here leaves.
		return true
	default:
		return false
	}
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

// Check answers the splittable guards whose condition sits on a changed line,
// over every judged Go file the change set names under tree.
//
// Only the condition's lines count, from the `if` keyword to the body's
// opening brace. A new guard always adds its `if` line, and rewording an old
// condition touches it, but an edit inside an old guard's body does not make
// the condition due.
func Check(tree string, changed repochanged.ChangedLines) (CheckReport, error) {
	paths := make([]string, 0, len(changed))
	for path := range changed {
		if !judged(path) {
			continue
		}
		paths = append(paths, path)
	}
	slices.Sort(paths)

	fset := token.NewFileSet()
	findings := Findings{}
	for _, path := range paths {
		guards, err := ScanFile(fset, filepath.Join(tree, filepath.FromSlash(path)))
		if err != nil {
			return CheckReport{}, err
		}
		for _, guard := range guards {
			if !changed.Touches(path, guard.Line, guard.LastLine) {
				continue
			}
			findings = append(findings, Finding{File: path, Line: guard.Line, Fn: guard.Fn})
		}
	}
	return CheckReport{Files: len(paths), Findings: findings}, nil
}

// judged reports whether a changed path is shipped Go source this gate reads.
// Tests, vendored modules, fixtures and dot directories are not: a test table
// is not a guard a reader simulates, and the other three are not Ze's code.
func judged(path string) bool {
	if !strings.HasSuffix(path, ".go") {
		return false
	}
	if strings.HasSuffix(path, "_test.go") {
		return false
	}
	for segment := range strings.SplitSeq(path, "/") {
		if segment == "vendor" {
			return false
		}
		if segment == "testdata" {
			return false
		}
		if strings.HasPrefix(segment, ".") {
			return false
		}
	}
	return true
}
