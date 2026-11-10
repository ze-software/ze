// Design: docs/contributing/error-inventory.md -- the inventory of user-visible message sites
//
// Detail: writers.go -- the writers whose message this judges
// Related: scan.go -- the index names are resolved against
//
// classify.go reads, from the syntax, how the message a writer is handed was
// built. The writers take a string or a plain error, so the operator error
// type is found where the argument was made: an opererr.New or opererr.Wrap
// call, a name bound to one, a parameter or variable declared
// *opererr.Error, or a function of the tree that answers only operator
// errors. Its text reaches the writer through .Error(), a format, a
// concatenation or a textbuf chain; .Problem(), .Why() and .Fix() each drop
// the rest, so they do not carry it.
//
// "Built by opererr" means what archoperatorerror's builtByOpererr means: an
// opererr.New or Wrap call, never an opererr.Error composite literal, which
// is the zero operator error with no fix. One narrowing the message rule
// needs is added: an opererr.New or Wrap whose fix argument is the empty
// string literal names no fix, and is not typed here.

package archerrorinventory

import (
	"go/ast"
	"go/token"
	"slices"
	"strconv"
	"strings"
)

// opererrPath is the operator error type's package.
const opererrPath = modulePath + "/internal/core/opererr"

// The opererr constructors, and the position of each one's fix argument.
const (
	opererrNew      = "New"
	opererrNewFix   = 2
	opererrWrap     = "Wrap"
	opererrWrapFix  = 3
	opererrType     = "Error"
	errorMethod     = "Error"
	errorResultType = "error"
)

// resolveDepthMax bounds how many names, calls and operands one judgement
// follows. Every step reads source the tree's authors wrote, and a cycle of
// names (a = b, b = a) would otherwise never end.
const resolveDepthMax = 16

// scope is where an expression is read: its file and the function that
// encloses it, nil at package level.
type scope struct {
	index *treeIndex
	file  *sourceFile
	fn    *ast.FuncDecl
}

// classify answers the status of a writer site whose message is the
// expressions message: typed when one of them carries the operator error,
// else plain-names-command when their literal text names a command, else
// plain.
func (sc scope) classify(message []ast.Expr) Status {
	for _, expr := range message {
		if sc.carries(expr, 0) {
			return StatusTyped
		}
	}
	if namesCommand(sc.messageText(message)) {
		return StatusPlainNamesCommand
	}
	return StatusPlain
}

// carries reports whether expr is, or is text that holds, an operator error
// with its fix: the error itself, its Error() text, or a call, format or
// concatenation one of whose operands carries it.
func (sc scope) carries(expr ast.Expr, depth int) bool {
	if depth > resolveDepthMax {
		return false
	}
	if sc.isOperatorError(expr, depth) {
		return true
	}
	switch node := ast.Unparen(expr).(type) {
	case *ast.CallExpr:
		return sc.callCarries(node, depth)
	case *ast.BinaryExpr:
		return sc.carries(node.X, depth+1) || sc.carries(node.Y, depth+1)
	case *ast.Ident:
		bound, found := sc.binding(node)
		if !found {
			return false
		}
		if bound.value == nil {
			return false
		}
		return bound.scope.carries(bound.value, depth+1)
	default:
		return false
	}
}

// callCarries reports whether a call's answer holds an operator error's
// text. A method of the operator error carries it only when it is Error():
// Problem, Why and Fix each answer one part.
func (sc scope) callCarries(call *ast.CallExpr, depth int) bool {
	if selector, isSelector := ast.Unparen(call.Fun).(*ast.SelectorExpr); isSelector {
		if sc.isOperatorError(selector.X, depth+1) {
			return selector.Sel.Name == errorMethod
		}
		if !sc.isImport(selector.X) {
			if sc.carries(selector.X, depth+1) {
				return true
			}
		}
	}
	for _, arg := range call.Args {
		if sc.carries(arg, depth+1) {
			return true
		}
	}
	return false
}

// isOperatorError reports whether expr's value is an *opererr.Error that
// names a fix.
func (sc scope) isOperatorError(expr ast.Expr, depth int) bool {
	if depth > resolveDepthMax {
		return false
	}
	switch node := ast.Unparen(expr).(type) {
	case *ast.CallExpr:
		return sc.callIsOperatorError(node, depth)
	case *ast.Ident:
		bound, found := sc.binding(node)
		if !found {
			return false
		}
		if bound.typ != nil {
			return bound.scope.file.isOperatorErrorType(bound.typ)
		}
		if bound.value == nil {
			return false
		}
		return bound.scope.isOperatorError(bound.value, depth+1)
	default:
		return false
	}
}

// callIsOperatorError reports whether a call answers an operator error: an
// opererr constructor handed a fix that is not the empty literal, or a
// function of the tree that answers only operator errors.
func (sc scope) callIsOperatorError(call *ast.CallExpr, depth int) bool {
	if sc.file.refersTo(call.Fun, opererrPath, opererrNew) {
		return fixArgument(call, opererrNewFix)
	}
	if sc.file.refersTo(call.Fun, opererrPath, opererrWrap) {
		return fixArgument(call, opererrWrapFix)
	}
	ref, found := sc.function(call.Fun)
	if !found {
		return false
	}
	return sc.index.answersOperatorError(ref, depth+1)
}

// fixArgument reports whether a constructor call's fix argument, at index
// at, can name a fix: present, and not the empty string literal.
func fixArgument(call *ast.CallExpr, at int) bool {
	if at >= len(call.Args) {
		return false
	}
	literal, isLiteral := ast.Unparen(call.Args[at]).(*ast.BasicLit)
	if !isLiteral {
		return true
	}
	if literal.Kind != token.STRING {
		return true
	}
	text, err := strconv.Unquote(literal.Value)
	if err != nil {
		return false
	}
	return text != ""
}

// function answers the package-level function of the tree a call's callee
// names: unqualified in the file's own package, or through an import.
func (sc scope) function(fun ast.Expr) (funcRef, bool) {
	switch node := ast.Unparen(fun).(type) {
	case *ast.Ident:
		ref, found := sc.index.funcs[sc.file.dir][node.Name]
		return ref, found
	case *ast.SelectorExpr:
		ident, isIdent := node.X.(*ast.Ident)
		if !isIdent {
			return funcRef{}, false
		}
		dir, inTree := cutModule(sc.file.imports[ident.Name])
		if !inTree {
			return funcRef{}, false
		}
		ref, found := sc.index.funcs[dir][node.Sel.Name]
		return ref, found
	default:
		return funcRef{}, false
	}
}

// answersOperatorError reports whether every answer a function gives is an
// operator error: its first result is declared *opererr.Error, or it is
// declared error and each of its returns hands back nil or an operator
// error, at least one the latter. A function already being judged answers
// false, which ends a recursion through itself.
func (index *treeIndex) answersOperatorError(ref funcRef, depth int) bool {
	if answer, judged := index.typedFuncs[ref.decl]; judged {
		return answer
	}
	results := ref.decl.Type.Results
	if results == nil {
		return false
	}
	if len(results.List) == 0 {
		return false
	}
	first := results.List[0].Type
	if ref.file.isOperatorErrorType(first) {
		return true
	}
	if !isErrorType(first) {
		return false
	}
	if ref.decl.Body == nil {
		return false
	}
	index.typedFuncs[ref.decl] = false
	answer := scope{index: index, file: ref.file, fn: ref.decl}.returnsOnlyOperatorErrors(depth)
	index.typedFuncs[ref.decl] = answer
	return answer
}

// returnsOnlyOperatorErrors reports whether each return of the scope's
// function, outside its function literals, answers nil or an operator error
// first, and at least one answers an operator error.
func (sc scope) returnsOnlyOperatorErrors(depth int) bool {
	typed, other := 0, 0
	ast.Inspect(sc.fn.Body, func(node ast.Node) bool {
		switch statement := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			if len(statement.Results) == 0 {
				other++
				return false
			}
			result := statement.Results[0]
			if isNil(result) {
				return false
			}
			if sc.isOperatorError(result, depth+1) {
				typed++
				return false
			}
			other++
			return false
		}
		return true
	})
	if other > 0 {
		return false
	}
	return typed > 0
}

// isOperatorErrorType reports whether typ, written in file, is
// *opererr.Error.
func (file *sourceFile) isOperatorErrorType(typ ast.Expr) bool {
	star, isStar := typ.(*ast.StarExpr)
	if !isStar {
		return false
	}
	return file.refersTo(star.X, opererrPath, opererrType)
}

// isErrorType reports whether typ is the predeclared error interface.
func isErrorType(typ ast.Expr) bool {
	ident, isIdent := typ.(*ast.Ident)
	if !isIdent {
		return false
	}
	return ident.Name == errorResultType
}

// isNil reports whether expr is the predeclared nil.
func isNil(expr ast.Expr) bool {
	ident, isIdent := ast.Unparen(expr).(*ast.Ident)
	if !isIdent {
		return false
	}
	return ident.Name == "nil"
}

// isImport reports whether expr names one of the file's imports.
func (sc scope) isImport(expr ast.Expr) bool {
	ident, isIdent := expr.(*ast.Ident)
	if !isIdent {
		return false
	}
	_, imported := sc.file.imports[ident.Name]
	return imported
}

// nameBinding is what a name is bound to where it is used: its declared
// type, its value, or both, and the scope the value is read in.
type nameBinding struct {
	typ   ast.Expr
	value ast.Expr
	scope scope
}

// binding answers the declaration a name in the scope's function refers
// to: the last parameter, variable or assignment of that name before the
// use, or the package constant of that name. Shadowing in a nested block is
// not modeled, so the declaration nearest before the use wins.
func (sc scope) binding(use *ast.Ident) (nameBinding, bool) {
	if sc.fn != nil {
		if found, isLocal := sc.localBinding(use); isLocal {
			return found, true
		}
	}
	value, isConstant := sc.index.constants[sc.file.dir][use.Name]
	if !isConstant {
		return nameBinding{}, false
	}
	return nameBinding{value: value, scope: scope{index: sc.index, file: sc.file}}, true
}

// localBinding answers the nearest declaration of use's name before it in
// the scope's function.
func (sc scope) localBinding(use *ast.Ident) (nameBinding, bool) {
	var found nameBinding
	nearest := token.NoPos
	consider := func(name *ast.Ident, typ, value ast.Expr) {
		if name.Name != use.Name {
			return
		}
		if name.Pos() >= use.Pos() {
			return
		}
		if name.Pos() < nearest {
			return
		}
		nearest = name.Pos()
		found = nameBinding{typ: typ, value: value, scope: sc}
	}
	ast.Inspect(sc.fn, func(node ast.Node) bool {
		switch declaration := node.(type) {
		case *ast.Field:
			for _, name := range declaration.Names {
				consider(name, declaration.Type, nil)
			}
		case *ast.ValueSpec:
			for i, name := range declaration.Names {
				var value ast.Expr
				if i < len(declaration.Values) {
					value = declaration.Values[i]
				}
				consider(name, declaration.Type, value)
			}
		case *ast.AssignStmt:
			for i, lhs := range declaration.Lhs {
				name, isName := lhs.(*ast.Ident)
				if !isName {
					continue
				}
				var value ast.Expr
				if len(declaration.Rhs) == len(declaration.Lhs) {
					value = declaration.Rhs[i]
				}
				consider(name, nil, value)
			}
		case *ast.RangeStmt:
			for _, expr := range []ast.Expr{declaration.Key, declaration.Value} {
				if name, isName := expr.(*ast.Ident); isName {
					consider(name, nil, nil)
				}
			}
		}
		return true
	})
	return found, nearest != token.NoPos
}

// messageText answers the literal text of a message: each expression's
// string fragments joined as written, the expressions joined by a space.
func (sc scope) messageText(message []ast.Expr) string {
	parts := make([]string, 0, len(message))
	for _, expr := range message {
		var fragments []string
		sc.fragments(expr, 0, &fragments)
		if text := strings.Join(fragments, ""); text != "" {
			parts = append(parts, text)
		}
	}
	return strings.Join(parts, " ")
}

// fragments appends the string literals expr is built from, in source
// order: through a concatenation, a call's receiver chain and arguments (a
// format, a textbuf chain), a name's bound value, and for a name declared
// with no value (a textbuf.Buffer), the method chains written on it before
// the use.
func (sc scope) fragments(expr ast.Expr, depth int, out *[]string) {
	if depth > resolveDepthMax {
		return
	}
	switch node := ast.Unparen(expr).(type) {
	case *ast.BasicLit:
		if node.Kind != token.STRING {
			return
		}
		if text, err := strconv.Unquote(node.Value); err == nil {
			*out = append(*out, text)
		}
	case *ast.BinaryExpr:
		sc.fragments(node.X, depth+1, out)
		sc.fragments(node.Y, depth+1, out)
	case *ast.CallExpr:
		if selector, isSelector := ast.Unparen(node.Fun).(*ast.SelectorExpr); isSelector {
			if !sc.isImport(selector.X) {
				sc.fragments(selector.X, depth+1, out)
			}
		}
		for _, arg := range node.Args {
			sc.fragments(arg, depth+1, out)
		}
	case *ast.Ident:
		sc.identFragments(node, depth, out)
	}
}

// identFragments appends the fragments a name's binding is built from.
func (sc scope) identFragments(use *ast.Ident, depth int, out *[]string) {
	found, isBound := sc.binding(use)
	if !isBound {
		return
	}
	if found.value != nil {
		found.scope.fragments(found.value, depth+1, out)
		return
	}
	if sc.fn == nil {
		return
	}
	if sc.fn.Body == nil {
		return
	}
	for _, statement := range sc.fn.Body.List {
		sc.chainFragments(statement, use, depth, out)
	}
}

// chainFragments appends the fragments of every call statement before use,
// in statement, whose receiver chain starts at use's name: the text a
// textbuf.Buffer was given one statement at a time.
func (sc scope) chainFragments(statement ast.Stmt, use *ast.Ident, depth int, out *[]string) {
	ast.Inspect(statement, func(node ast.Node) bool {
		if _, isLiteral := node.(*ast.FuncLit); isLiteral {
			return false
		}
		expression, isExpression := node.(*ast.ExprStmt)
		if !isExpression {
			return true
		}
		if expression.Pos() >= use.Pos() {
			return false
		}
		call, isCall := expression.X.(*ast.CallExpr)
		if !isCall {
			return false
		}
		root, rooted := chainRoot(call)
		if !rooted {
			return false
		}
		if root.Name != use.Name {
			return false
		}
		sc.chainArgFragments(call, depth+1, out)
		return false
	})
}

// chainArgFragments appends the fragments of the arguments of every call in
// a method chain, root first, without resolving the chain's root name. The
// root is the name identFragments is resolving: walking it again would walk
// every earlier statement once more for each call, which grows exponentially
// with the number of writes to one buffer. The walk descends one receiver per
// step, so it is bounded by the expression's own depth.
func (sc scope) chainArgFragments(call *ast.CallExpr, depth int, out *[]string) {
	var calls []*ast.CallExpr
	var expr ast.Expr = call
	for {
		node, isCall := ast.Unparen(expr).(*ast.CallExpr)
		if !isCall {
			break
		}
		calls = append(calls, node)
		selector, isSelector := ast.Unparen(node.Fun).(*ast.SelectorExpr)
		if !isSelector {
			break
		}
		expr = selector.X
	}
	for _, chained := range slices.Backward(calls) {
		for _, arg := range chained.Args {
			sc.fragments(arg, depth, out)
		}
	}
}

// chainRoot answers the name a method chain starts at: x in x.A().B(c).
// The walk descends one receiver per step, so it is bounded by the
// expression's own depth.
func chainRoot(call *ast.CallExpr) (*ast.Ident, bool) {
	var expr ast.Expr = call
	for {
		switch node := ast.Unparen(expr).(type) {
		case *ast.CallExpr:
			expr = node.Fun
		case *ast.SelectorExpr:
			expr = node.X
		case *ast.Ident:
			return node, true
		default:
			return nil, false
		}
	}
}

// namesCommand reports whether a message's literal text names a command an
// operator could run: "ze " or "./le " at a word start followed by a
// lowercase word, or a config statement ("set ", "delete ") followed by a
// lowercase word at the start of the text or right after a quote or
// backtick. A config statement elsewhere is prose ("is not set yet"). The
// rule only sorts the worklist: no tool can prove the command fixes the
// problem, so plain-names-command never meets the message rule.
func namesCommand(text string) bool {
	for at := range len(text) {
		if !wordStart(text, at) {
			continue
		}
		rest := text[at:]
		if commandFollows(rest, "ze ") {
			return true
		}
		if commandFollows(rest, "./le ") {
			return true
		}
		if !statementStart(text, at) {
			continue
		}
		if commandFollows(rest, "set ") {
			return true
		}
		if commandFollows(rest, "delete ") {
			return true
		}
	}
	return false
}

// commandFollows reports whether rest opens with prefix and a lowercase
// word after it.
func commandFollows(rest, prefix string) bool {
	after, found := strings.CutPrefix(rest, prefix)
	if !found {
		return false
	}
	if after == "" {
		return false
	}
	return after[0] >= 'a' && after[0] <= 'z'
}

// wordStart reports whether a word may start at text[at]: the text's start,
// or a byte before it that cannot be part of a word, a path or a flag.
func wordStart(text string, at int) bool {
	if at == 0 {
		return true
	}
	previous := text[at-1]
	switch {
	case previous >= 'a' && previous <= 'z':
		return false
	case previous >= 'A' && previous <= 'Z':
		return false
	case previous >= '0' && previous <= '9':
		return false
	}
	return !strings.ContainsRune("_-./", rune(previous))
}

// statementStart reports whether a config statement may start at
// text[at]: the text's start, or right after a quote or backtick.
func statementStart(text string, at int) bool {
	if at == 0 {
		return true
	}
	return strings.ContainsRune("`'\"", rune(text[at-1]))
}
