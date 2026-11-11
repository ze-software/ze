// Design: docs/architecture/doctor-and-health-checks.md -- the doctor-coverage gate
//
// Overview: sites.go -- the site shapes and the code each one names
//
// wrappers.go finds the functions that write a log line for their caller, and
// tells which expressions the source shows are strings. Both read syntax only.
//
// A wrapper is a function or method whose last two parameters are a string
// message and a variadic list of attributes, and whose body passes those two
// on, the message alone or concatenated with a prefix or suffix, to a Warn or
// Error log call, or to another wrapper:
//
//	func warnLimited(log *slog.Logger, last *time.Time, msg string, attrs ...any) {
//		...
//		log.Warn(msg, attrs...)
//	}
//
// The line the operator reads is the wrapper's caller's, and so is the `code`
// attribute that names its doctor code: each call of a wrapper is a site of the
// wrapper's level, and the forwarding call inside the wrapper is none. The
// wrapper is found from its body, never from its name. A name is matched as Go
// scopes it, since the scan holds no types: a function by its package and name,
// a method by its name within its package, and an exported method by its name
// anywhere in the tree.

package archdoctorcoverage

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
)

// wrapper is the log call a wrapper forwards to: its level, and the index of
// the message among a call's arguments (a method's receiver is no argument).
type wrapper struct {
	kind      Kind
	messageAt int
}

// funcRef names a function or method by its tree directory and its name.
type funcRef struct {
	dir  string
	name string
}

// wrapperSet is every wrapper in the tree and every forwarding call inside
// one. It is built once per scan (findWrappers) and only read after.
type wrapperSet struct {
	funcs    map[funcRef]wrapper
	methods  map[funcRef]wrapper
	exported map[string]wrapper
	forwards map[*ast.CallExpr]bool
}

// wrapperCandidate is a function whose parameters could make it a wrapper:
// the message parameter's name, the variadic parameter's name, and the
// message's index among a call's arguments.
type wrapperCandidate struct {
	file      *sourceFile
	decl      *ast.FuncDecl
	message   string
	variadic  string
	messageAt int
}

// findWrappers answers the tree's wrappers. A wrapper may forward to another,
// so each pass adds the candidates whose body forwards to a log call or to a
// wrapper already found, and the passes stop when one adds none: a pass that
// does not stop adds a candidate, so there are at most as many passes as
// candidates.
func (index *treeIndex) findWrappers() wrapperSet {
	set := wrapperSet{
		funcs:    make(map[funcRef]wrapper),
		methods:  make(map[funcRef]wrapper),
		exported: make(map[string]wrapper),
		forwards: make(map[*ast.CallExpr]bool),
	}
	var candidates []wrapperCandidate
	for _, file := range index.files {
		for _, decl := range file.ast.Decls {
			if candidate, isCandidate := candidateOf(file, decl); isCandidate {
				candidates = append(candidates, candidate)
			}
		}
	}
	for added := true; added; {
		added = false
		remaining := candidates[:0]
		for _, candidate := range candidates {
			calls, forwarded := index.forwardsOf(&set, candidate)
			if len(calls) == 0 {
				remaining = append(remaining, candidate)
				continue
			}
			set.add(candidate, forwarded)
			for _, call := range calls {
				set.forwards[call] = true
			}
			added = true
		}
		candidates = remaining
	}
	return set
}

// candidateOf answers the wrapper candidate a declaration is: a function with
// a body whose last parameter is variadic and named, and whose one before it
// is a named string.
func candidateOf(file *sourceFile, decl ast.Decl) (wrapperCandidate, bool) {
	function, isFunc := decl.(*ast.FuncDecl)
	if !isFunc {
		return wrapperCandidate{}, false
	}
	if function.Body == nil {
		return wrapperCandidate{}, false
	}
	var names []string
	var typed []ast.Expr
	for _, field := range function.Type.Params.List {
		for _, name := range field.Names {
			names = append(names, name.Name)
			typed = append(typed, field.Type)
		}
	}
	last := len(names) - 1
	if last < 1 {
		return wrapperCandidate{}, false
	}
	if _, isVariadic := typed[last].(*ast.Ellipsis); !isVariadic {
		return wrapperCandidate{}, false
	}
	if !isStringType(typed[last-1]) {
		return wrapperCandidate{}, false
	}
	if names[last] == "_" {
		return wrapperCandidate{}, false
	}
	if names[last-1] == "_" {
		return wrapperCandidate{}, false
	}
	return wrapperCandidate{file: file, decl: function, message: names[last-1], variadic: names[last], messageAt: last - 1}, true
}

// forwardsOf answers the calls in a candidate's body that pass on its message
// and attribute parameters, alone and in that order, to a log call of a
// spelled level or to a wrapper the set holds, and the first one's level:
// the level the wrapper's callers are judged at. Every forwarding call is
// answered, so a wrapper that forwards on two branches leaves neither a site.
func (index *treeIndex) forwardsOf(set *wrapperSet, candidate wrapperCandidate) ([]*ast.CallExpr, Kind) {
	var calls []*ast.CallExpr
	kind := KindUnspecified
	ast.Inspect(candidate.decl.Body, func(node ast.Node) bool {
		call, isCall := node.(*ast.CallExpr)
		if !isCall {
			return true
		}
		forwarded, isLog := index.logCallShape(set, candidate.file, call)
		if !isLog {
			return true
		}
		if !passesOn(call, forwarded.messageAt, candidate.message, candidate.variadic) {
			return true
		}
		if len(calls) == 0 {
			kind = forwarded.kind
		}
		calls = append(calls, call)
		return true
	})
	return calls, kind
}

// passesOn reports whether a call's arguments from messageAt on are the
// parameter message, alone or concatenated with a prefix or suffix, and the
// spread of the parameter variadic (dhcp v6's warnOutage prefixes its
// caller's message: "iface dhcp v6: "+msg).
func passesOn(call *ast.CallExpr, messageAt int, message, variadic string) bool {
	if len(call.Args) != messageAt+2 {
		return false
	}
	if !call.Ellipsis.IsValid() {
		return false
	}
	if !carries(call.Args[messageAt], message, 0) {
		return false
	}
	return isIdentNamed(call.Args[messageAt+1], variadic)
}

// carries reports whether expr is the name message, or a concatenation one
// of whose operands carries it.
func carries(expr ast.Expr, message string, depth int) bool {
	if depth > codeDepthMax {
		return false
	}
	switch node := expr.(type) {
	case *ast.Ident:
		return node.Name == message
	case *ast.ParenExpr:
		return carries(node.X, message, depth+1)
	case *ast.BinaryExpr:
		if node.Op != token.ADD {
			return false
		}
		if carries(node.X, message, depth+1) {
			return true
		}
		return carries(node.Y, message, depth+1)
	}
	return false
}

// isIdentNamed reports whether expr is the bare name name.
func isIdentNamed(expr ast.Expr, name string) bool {
	ident, isIdent := expr.(*ast.Ident)
	if !isIdent {
		return false
	}
	return ident.Name == name
}

// logCallShape answers the log call a call is, by shape alone: a wrapper the
// set holds, the package logger's Warn or Error forms, or a value's Warn,
// Error, or Log form of a spelled level.
func (index *treeIndex) logCallShape(set *wrapperSet, file *sourceFile, call *ast.CallExpr) (wrapper, bool) {
	if forwarded, isWrapper := set.of(file, call); isWrapper {
		return forwarded, true
	}
	selector, isSelector := call.Fun.(*ast.SelectorExpr)
	if !isSelector {
		return wrapper{}, false
	}
	if receiver, isIdent := selector.X.(*ast.Ident); isIdent {
		if path, imported := file.imports[receiver.Name]; imported {
			if path != slogPath {
				return wrapper{}, false
			}
			kind, messageAt := logShape(selector.Sel.Name)
			return wrapper{kind: kind, messageAt: messageAt}, kind != KindUnspecified
		}
	}
	kind, messageAt := index.methodShape(file, selector.Sel.Name, call.Args)
	return wrapper{kind: kind, messageAt: messageAt}, kind != KindUnspecified
}

// add records a candidate as a wrapper of level kind.
func (set *wrapperSet) add(candidate wrapperCandidate, kind Kind) {
	found := wrapper{kind: kind, messageAt: candidate.messageAt}
	ref := funcRef{dir: candidate.file.dir, name: candidate.decl.Name.Name}
	if candidate.decl.Recv == nil {
		set.funcs[ref] = found
		return
	}
	set.methods[ref] = found
	if candidate.decl.Name.IsExported() {
		set.exported[ref.name] = found
	}
}

// of answers the wrapper a call calls: a function of the file's package by
// its bare name, an exported function of another package of the tree through
// its import name, or a method by its name (see the file comment).
func (set *wrapperSet) of(file *sourceFile, call *ast.CallExpr) (wrapper, bool) {
	switch callee := call.Fun.(type) {
	case *ast.Ident:
		found, isWrapper := set.funcs[funcRef{dir: file.dir, name: callee.Name}]
		return found, isWrapper
	case *ast.SelectorExpr:
		if receiver, isIdent := callee.X.(*ast.Ident); isIdent {
			if path, imported := file.imports[receiver.Name]; imported {
				dir, inTree := cutModule(path)
				if !inTree {
					return wrapper{}, false
				}
				found, isWrapper := set.funcs[funcRef{dir: dir, name: callee.Sel.Name}]
				return found, isWrapper
			}
		}
		// A method named as a log call is judged by its own shape
		// (methodShape): a wrapper method named Warn or Error must not set
		// the message index of every other value's Warn or Error.
		if logMethodName(callee.Sel.Name) {
			return wrapper{}, false
		}
		if found, isWrapper := set.methods[funcRef{dir: file.dir, name: callee.Sel.Name}]; isWrapper {
			return found, true
		}
		found, isWrapper := set.exported[callee.Sel.Name]
		return found, isWrapper
	}
	return wrapper{}, false
}

// logMethodName reports whether name is a method name methodShape reads: a
// Warn or Error form, Log or LogAttrs.
func logMethodName(name string) bool {
	if kind, _ := logShape(name); kind != KindUnspecified {
		return true
	}
	return name == "Log" || name == "LogAttrs"
}

// stringValued reports whether the source shows expr is a string: a constant
// or a literal (resolve), a concatenation with a string operand, an error's or
// an operator error's text, a String() call, a fmt.Sprint form, or a name an
// enclosing function in scope (innermost last) or the file's package declares
// as one. Anything else, an error value included, answers false.
func (index *treeIndex) stringValued(file *sourceFile, scope []ast.Node, expr ast.Expr, depth int) bool {
	if depth > codeDepthMax {
		return false
	}
	if _, isString := index.resolve(file, expr, 0); isString {
		return true
	}
	switch node := expr.(type) {
	case *ast.ParenExpr:
		return index.stringValued(file, scope, node.X, depth+1)
	case *ast.BinaryExpr:
		if node.Op != token.ADD {
			return false
		}
		// Go adds only operands of one type, so one string operand makes the
		// sum a string.
		if index.stringValued(file, scope, node.X, depth+1) {
			return true
		}
		return index.stringValued(file, scope, node.Y, depth+1)
	case *ast.CallExpr:
		return stringCall(file, node)
	case *ast.Ident:
		return index.stringName(file, scope, node.Name, depth)
	}
	return false
}

// stringCall reports whether a call answers a string by its shape: x.Error()
// or x.Problem() (textCall), x.String(), or fmt.Sprint, Sprintf or Sprintln.
func stringCall(file *sourceFile, call *ast.CallExpr) bool {
	if textCall(call) {
		return true
	}
	selector, isSelector := call.Fun.(*ast.SelectorExpr)
	if !isSelector {
		return false
	}
	if selector.Sel.Name == "String" {
		return len(call.Args) == 0
	}
	if !refersTo(file, selector.X, fmtPath) {
		return false
	}
	switch selector.Sel.Name {
	case "Sprint", "Sprintf", "Sprintln":
		return true
	}
	return false
}

// stringName reports whether name is declared a string where the call sees
// it: by the innermost enclosing function that declares it (a parameter or
// result, a var, or a short variable declaration), else by a package-level
// variable of the file's package.
func (index *treeIndex) stringName(file *sourceFile, scope []ast.Node, name string, depth int) bool {
	for i, enclosing := range slices.Backward(scope) {
		var signature *ast.FuncType
		var body *ast.BlockStmt
		switch function := enclosing.(type) {
		case *ast.FuncDecl:
			signature, body = function.Type, function.Body
			if typed, declared := fieldType(function.Recv, name); declared {
				return isStringType(typed)
			}
		case *ast.FuncLit:
			signature, body = function.Type, function.Body
		default:
			// sitesOf puts only functions in scope.
			panic("BUG: a scope entry that is no function")
		}
		if typed, declared := fieldType(signature.Params, name); declared {
			return isStringType(typed)
		}
		if typed, declared := fieldType(signature.Results, name); declared {
			return isStringType(typed)
		}
		typed, value, declared := localDeclaration(body, name)
		if !declared {
			continue
		}
		if typed != nil {
			return isStringType(typed)
		}
		if value == nil {
			return false
		}
		return index.stringValued(file, scope[:i+1], value, depth+1)
	}
	ref, declared := index.variables[file.dir][name]
	if !declared {
		return false
	}
	return index.stringValued(ref.file, nil, ref.expr, depth+1)
}

// fieldType answers the type a field list gives name.
func fieldType(fields *ast.FieldList, name string) (ast.Expr, bool) {
	if fields == nil {
		return nil, false
	}
	for _, field := range fields.List {
		for _, declared := range field.Names {
			if declared.Name == name {
				return field.Type, true
			}
		}
	}
	return nil, false
}

// localDeclaration answers the first declaration of name in a function body,
// outside the function literals it holds: its written type, or the value it
// is declared with, or neither (a range variable, a multi-value assignment),
// which no caller reads as a string.
func localDeclaration(body *ast.BlockStmt, name string) (ast.Expr, ast.Expr, bool) {
	if body == nil {
		return nil, nil, false
	}
	var typed, value ast.Expr
	declared := false
	ast.Inspect(body, func(node ast.Node) bool {
		if declared {
			return false
		}
		switch statement := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.AssignStmt:
			if statement.Tok != token.DEFINE {
				return true
			}
			for i, target := range statement.Lhs {
				if !isIdentNamed(target, name) {
					continue
				}
				declared = true
				if len(statement.Rhs) == len(statement.Lhs) {
					value = statement.Rhs[i]
				}
				return false
			}
		case *ast.RangeStmt:
			// A range variable's type is the element's, which the syntax
			// does not show: declared with no type and no value.
			if statement.Tok != token.DEFINE {
				return true
			}
			declared = isIdentNamed(statement.Key, name)
			if !declared {
				declared = isIdentNamed(statement.Value, name)
			}
			return !declared
		case *ast.ValueSpec:
			for i, declaredName := range statement.Names {
				if declaredName.Name != name {
					continue
				}
				declared = true
				typed = statement.Type
				if i < len(statement.Values) {
					value = statement.Values[i]
				}
				return false
			}
		}
		return true
	})
	return typed, value, declared
}

// isStringType reports whether a type expression is the builtin string.
func isStringType(expr ast.Expr) bool {
	return types.ExprString(expr) == "string"
}
