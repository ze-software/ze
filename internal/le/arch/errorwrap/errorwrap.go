// Design: docs/contributing/ze-go-style.md -- an error carries each prefix once, checked on changed code
// Related: format.go -- the segments a format string writes
// Related: resolve.go -- naming the function a wrapped error came from
// Related: funcvalue.go -- the functions a called name can hold
// Related: report.go -- the check's answer
//
// errorwrap.go finds the wrap that says again what its error already says:
//
//	func parseTreeWithYANG(...) (*Tree, *Schema, error) {
//		return nil, nil, fmt.Errorf("parse config: %w", err)
//	}
//	...
//	return fmt.Errorf("reload: parse config: %w", loadErr)
//
// The operator reads `reload: parse config: parse config: line 3`. Each layer
// believes it owns the context, so the fix is a decision about which one
// does, and the gate names both sites so the reader can make it.
//
// A segment is one ": "-separated literal part of the format string before
// %w (format.go). The wrapped operand is traced back through the syntax: to
// the assignment that bound it, to the call that produced it, to that
// callee's return statements, and on through every error a return passes up
// unwrapped, collecting each segment some constructor on the way writes
// (fmt.Errorf, errors.New). A called name that holds a function value is
// followed to the values it can hold, through a function parameter to the
// arguments its callers pass (funcvalue.go). A segment's verbs compare as one
// mark, so `line %d: ` repeated is a repeat. A wrap whose own segment is
// among them is a finding.

package archerrorwrap

import (
	"go/ast"
	"go/token"
	"io/fs"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	repochanged "github.com/ze-software/ze/internal/le/repo/changed"
)

// depthMax bounds how many calls deep one trace follows an error. The
// module's own call chains are the structure walked, which no peer controls;
// the bound keeps a chain the cycle guard misses (two literals passing one
// error back and forth) finite. A segment written deeper is not seen.
const depthMax = 16

// origin is where a segment was written: the file and line of the
// constructor call, and the function that holds it.
type origin struct {
	File string
	Line int
	Fn   string
}

// carried maps each segment an error can hold to the first place a trace met
// it being written.
type carried map[string]origin

// add records segment at its origin, keeping the first origin met.
func (c carried) add(segment string, at origin) {
	if _, held := c[segment]; held {
		return
	}
	c[segment] = at
}

// merge records every segment of other.
func (c carried) merge(other carried) {
	for segment, at := range other {
		c.add(segment, at)
	}
}

// site is one fmt.Errorf call that wraps an error under literal segments of
// its own.
type site struct {
	fn       *funcRef
	call     *ast.CallExpr
	form     format
	line     int
	lastLine int
}

// Check answers the wraps on a changed line that repeat a segment of the
// error they wrap, over every judged Go file the change set names under tree.
//
// Only the call's own lines count, from `fmt.Errorf` to its closing paren:
// an edit elsewhere in the function does not make an old wrap due.
func Check(tree string, changed repochanged.ChangedLines) (CheckReport, error) {
	module, err := modulePath(tree)
	if err != nil {
		return CheckReport{}, err
	}
	return check(tree, module, changed)
}

// check is Check with the module path given, so a fixture tree needs no
// go.mod of its own.
func check(tree, module string, changed repochanged.ChangedLines) (CheckReport, error) {
	paths := make([]string, 0, len(changed))
	for rel := range changed {
		if !judged(rel) {
			continue
		}
		paths = append(paths, rel)
	}
	slices.Sort(paths)
	return judgeFiles(newIndex(tree, module), paths, func(rel string, wrap site) bool {
		return changed.Touches(rel, wrap.line, wrap.lastLine)
	})
}

// Report answers every wrap under roots that repeats a segment of the error it
// wraps, changed or not.
func Report(tree string, roots []string) (CheckReport, error) {
	module, err := modulePath(tree)
	if err != nil {
		return CheckReport{}, err
	}
	return report(tree, module, roots)
}

// report is Report with the module path given.
func report(tree, module string, roots []string) (CheckReport, error) {
	var paths []string
	for _, root := range roots {
		start := filepath.Join(tree, filepath.FromSlash(root))
		walked := filepath.WalkDir(start, func(name string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.IsDir() {
				if name == start {
					return nil
				}
				if skippedDir(entry.Name()) {
					return filepath.SkipDir
				}
				return nil
			}
			rel, relErr := filepath.Rel(tree, name)
			if relErr != nil {
				return relErr
			}
			rel = filepath.ToSlash(rel)
			if judged(rel) {
				paths = append(paths, rel)
			}
			return nil
		})
		if walked != nil {
			return CheckReport{}, walked
		}
	}
	slices.Sort(paths)
	answer, err := judgeFiles(newIndex(tree, module), paths, func(string, site) bool { return true })
	if err != nil {
		return CheckReport{}, err
	}
	answer.Scope = "every Go file under " + strings.Join(roots, ", ")
	return answer, nil
}

// judgeFiles traces every wrap keep selects in paths and answers the report.
func judgeFiles(ix *index, paths []string, keep func(rel string, wrap site) bool) (CheckReport, error) {
	answer := CheckReport{Files: len(paths), Findings: Findings{}}
	for _, rel := range paths {
		pkg, err := ix.packageAt(path.Dir(rel))
		if err != nil {
			return CheckReport{}, err
		}
		file := pkg.file(rel)
		if file == nil {
			return CheckReport{}, &fs.PathError{Op: "judge", Path: rel, Err: fs.ErrNotExist}
		}
		for _, wrap := range ix.sites(pkg, file) {
			if !keep(rel, wrap) {
				continue
			}
			findings, untraced, err := ix.judge(wrap)
			if err != nil {
				return CheckReport{}, err
			}
			answer.Wraps++
			if untraced {
				answer.Untraced++
			}
			answer.Findings = append(answer.Findings, findings...)
		}
	}
	return answer, nil
}

// sites answers every wrap in one file, in source order, each with the
// function that holds it.
func (ix *index) sites(pkg *goPackage, file *sourceFile) []site {
	var found []site
	for _, decl := range file.syntax.Decls {
		fn := &funcRef{name: packageScope, pkg: pkg, file: file, body: &ast.BlockStmt{}}
		if funcDecl, isFunc := decl.(*ast.FuncDecl); isFunc {
			if funcDecl.Body == nil {
				continue
			}
			fn = declaredRef(pkg, file, funcDecl)
		}
		found = ix.collect(fn, decl, found)
	}
	return found
}

// collect appends the wraps under root that fn holds, and those of every
// function literal inside it, each under its own literal.
func (ix *index) collect(fn *funcRef, root ast.Node, found []site) []site {
	ast.Inspect(root, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.FuncLit:
			found = ix.collect(literal(fn, typed, fn.name+" (func literal)"), typed.Body, found)
			return false
		case *ast.CallExpr:
			if wrap, isWrap := ix.wrapSite(fn, typed); isWrap {
				found = append(found, wrap)
			}
		}
		return true
	})
	return found
}

// wrapSite answers call as a site when it is a fmt.Errorf whose literal
// format wraps with %w under at least one segment of its own.
func (ix *index) wrapSite(fn *funcRef, call *ast.CallExpr) (site, bool) {
	if !callsStd(fn.file.syntax, call, "fmt", "Errorf") {
		return site{}, false
	}
	form, parsed := literalFormat(call)
	if !parsed {
		return site{}, false
	}
	if len(form.wrapped) == 0 {
		return site{}, false
	}
	if len(form.outer) == 0 {
		return site{}, false
	}
	return site{
		fn:       fn,
		call:     call,
		form:     form,
		line:     ix.fset.Position(call.Pos()).Line,
		lastLine: ix.fset.Position(call.End()).Line,
	}, true
}

// judge traces one wrap's operands and answers a finding per segment of its
// own they already carry, and whether its operand could not be traced to a
// producer at all.
func (ix *index) judge(wrap site) (Findings, bool, error) {
	tr := &trace{ix: ix, held: carried{}}
	tr.operands(wrap.fn, wrap.call, wrap.form, 0)
	if tr.err != nil {
		return nil, false, tr.err
	}
	var findings Findings
	for _, segment := range wrap.form.outer {
		at, held := tr.held[segment]
		if !held {
			continue
		}
		findings = append(findings, Finding{
			File:       wrap.fn.file.rel,
			Line:       wrap.line,
			Fn:         wrap.fn.name,
			Segment:    shownSegment(segment),
			OriginFile: at.File,
			OriginLine: at.Line,
			OriginFn:   at.Fn,
		})
	}
	return findings, tr.untraced, nil
}

// trace follows one wrapped error to every segment it can carry.
type trace struct {
	ix   *index
	held carried
	// untraced says the wrap's own operand reached a producer the syntax
	// cannot name, or names only in part: an interface method, an error-valued
	// parameter, a function parameter of an exported function or of a literal
	// (funcvalue.go).
	untraced bool
	err      error
}

// operands traces each %w operand of a fmt.Errorf call.
func (tr *trace) operands(fn *funcRef, call *ast.CallExpr, form format, depth int) {
	for _, operand := range form.wrapped {
		position := 1 + operand
		if position >= len(call.Args) {
			continue
		}
		tr.expr(fn, call.Args[position], call.Pos(), depth)
	}
}

// expr traces one error expression evaluated in fn at the position at.
func (tr *trace) expr(fn *funcRef, expr ast.Expr, at token.Pos, depth int) {
	switch typed := ast.Unparen(expr).(type) {
	case *ast.CallExpr:
		tr.call(fn, typed, depth)
	case *ast.Ident:
		if typed.Name == "nil" {
			return
		}
		value, _, owner := lookup(fn, typed.Name, at)
		if value == nil {
			tr.markUntraced(depth)
			return
		}
		tr.expr(owner, value, value.Pos(), depth)
	}
}

// call traces an error built by a constructor, or answered by a callee.
func (tr *trace) call(fn *funcRef, call *ast.CallExpr, depth int) {
	if callsStd(fn.file.syntax, call, "fmt", "Errorf") {
		tr.errorf(fn, call, depth)
		return
	}
	if callsStd(fn.file.syntax, call, "errors", "New") {
		tr.errorsNew(fn, call)
		return
	}
	if depth >= depthMax {
		return
	}
	callees, err := tr.ix.funcValues(fn, call.Fun, call.Pos(), depth)
	if err != nil {
		tr.err = err
		return
	}
	if !callees.complete {
		tr.markUntraced(depth)
	}
	for _, callee := range callees.refs {
		tr.function(callee, depth+1)
	}
}

// markUntraced records an operand the trace could not follow, when it is the
// wrap's own operand rather than one met deeper in a callee.
func (tr *trace) markUntraced(depth int) {
	if depth == 0 {
		tr.untraced = true
	}
}

// errorf records the segments a fmt.Errorf writes and traces what it wraps.
func (tr *trace) errorf(fn *funcRef, call *ast.CallExpr, depth int) {
	form, parsed := literalFormat(call)
	if !parsed {
		return
	}
	at := tr.ix.origin(fn, call.Pos())
	for _, segment := range form.produced() {
		tr.held.add(segment, at)
	}
	tr.operands(fn, call, form, depth)
}

// errorsNew records the segments of an errors.New message.
func (tr *trace) errorsNew(fn *funcRef, call *ast.CallExpr) {
	text, isLiteral := literalString(call)
	if !isLiteral {
		return
	}
	at := tr.ix.origin(fn, call.Pos())
	for _, segment := range messageSegments(text) {
		tr.held.add(segment, at)
	}
}

// function records every segment the errors callee returns can carry,
// reading each body once.
//
// A body already being read higher up the same trace is a cycle and adds
// nothing here; its segments reach the trace through the outer reading.
func (tr *trace) function(callee *funcRef, depth int) {
	if held, read := tr.ix.memo[callee.body]; read {
		tr.held.merge(held)
		return
	}
	if tr.ix.open[callee.body] {
		return
	}
	tr.ix.open[callee.body] = true
	inner := &trace{ix: tr.ix, held: carried{}}
	inner.returns(callee, depth)
	delete(tr.ix.open, callee.body)
	if inner.err != nil {
		tr.err = inner.err
		return
	}
	tr.ix.memo[callee.body] = inner.held
	tr.held.merge(inner.held)
}

// returns traces the last result of every return statement in callee, the
// error position by Go convention, outside nested function literals. A bare
// return of named results is not traced.
func (tr *trace) returns(callee *funcRef, depth int) {
	ast.Inspect(callee.body, func(node ast.Node) bool {
		switch stmt := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			if len(stmt.Results) > 0 {
				tr.expr(callee, stmt.Results[len(stmt.Results)-1], stmt.Pos(), depth)
			}
			return false
		}
		return true
	})
}

// origin answers where a constructor at pos in fn sits.
func (ix *index) origin(fn *funcRef, pos token.Pos) origin {
	return origin{File: fn.file.rel, Line: ix.fset.Position(pos).Line, Fn: fn.name}
}

// literalFormat parses the format of a fmt.Errorf whose format is a string
// literal.
func literalFormat(call *ast.CallExpr) (format, bool) {
	text, isLiteral := literalString(call)
	if !isLiteral {
		return format{}, false
	}
	return parseFormat(text)
}

// literalString answers the first argument of call when it is a string
// literal.
func literalString(call *ast.CallExpr) (string, bool) {
	if len(call.Args) == 0 {
		return "", false
	}
	lit, isLit := call.Args[0].(*ast.BasicLit)
	if !isLit {
		return "", false
	}
	if lit.Kind != token.STRING {
		return "", false
	}
	text, err := strconv.Unquote(lit.Value)
	if err != nil {
		return "", false
	}
	return text, true
}

// judged reports whether a path is shipped Go source this gate reads. Tests,
// vendored modules, fixtures and dot directories are not: a test's error text
// reaches no operator, and the other three are not Ze's code.
func judged(rel string) bool {
	if !strings.HasSuffix(rel, ".go") {
		return false
	}
	if strings.HasSuffix(rel, "_test.go") {
		return false
	}
	for segment := range strings.SplitSeq(rel, "/") {
		if skippedDir(segment) {
			return false
		}
	}
	return true
}

// skippedDir reports whether a directory name holds code this gate does not
// judge.
func skippedDir(name string) bool {
	if name == "vendor" {
		return true
	}
	if name == "testdata" {
		return true
	}
	return strings.HasPrefix(name, ".")
}
