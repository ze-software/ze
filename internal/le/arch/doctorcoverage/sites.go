// Design: docs/architecture/doctor-and-health-checks.md -- the doctor-coverage gate
//
// Overview: doctorcoverage.go -- the site kinds and statuses, and the scan
// Related: wrappers.go -- the functions that forward a message to Warn or Error
//
// sites.go decides which calls are problem sites and reads the code each one
// names. The scan reads syntax, not types, so the shapes are:
//
//	slog.Warn(msg, ...), slog.Error(msg, ...)            the package logger
//	x.Warn(msg, ...), x.Error(msg, ...)                  a logger value
//	x.WarnContext(ctx, msg, ...), x.ErrorContext(...)    the context forms
//	x.Log(ctx, slog.LevelWarn|LevelError, msg, ...)      the level forms,
//	x.LogAttrs(ctx, slog.LevelWarn|LevelError, msg, ...) LogAttrs included
//	warnLimited(log, last, msg, ...)                     a wrapper's call
//	report.RaiseWarning(source, code, ...)               the report bus
//	report.RaiseError(source, code, ...)
//	health.Register(name, ...), health.RegisterWhen(...) a health row
//	diagnostic.Diagnostic{...}                           a doctor finding,
//	&diagnostic.Diagnostic{...}                          its address taken,
//	[]diagnostic.Diagnostic{{...}}                       elided elements too,
//	rpc.DoctorCheckDiagnostic{...}                       a plugin's finding
//
// A doctor finding is a composite literal whose type is one of findingTypes,
// named through any import name of its package or unqualified inside it. A
// literal with no element is the zero value a function answers beside "no
// finding" (`return diagnostic.Diagnostic{}, false`), so it is no site. Its
// Fix is read from the keyed element `Fix`.
//
// A call on another imported package (http.Error) is no site. A method call
// on a value is a log call when its message is a string the source shows (a
// constant, a concatenation, a string variable, an error's text), or when the
// arguments after it are attributes: a string key, a slog attribute, or a
// spread (an operator error's LogAttrs). That excludes err.Error(), which
// takes no argument, and a value's Error(err) with nothing after it. A call
// of a wrapper, a function whose body forwards its own message and attribute
// parameters to a log call or to another wrapper, is a log call of the
// wrapper's level whatever its name (wrappers.go). A level the source does
// not spell (x.Log(ctx, level, msg)) is no site, and neither is a wrapper
// that forwards one.
//
// A log line names its code as a `code` attribute, the key internal/core/opererr
// passes a caller's code under: `"code", X` among the key-value arguments, or
// slog.String("code", X) (or any slog attribute constructor) anywhere in them,
// nested in an operator error's LogAttrs included.

package archdoctorcoverage

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
	"strconv"
)

// The import paths whose calls are sites.
const (
	slogPath   = "log/slog"
	reportPath = modulePath + "/internal/core/report"
	healthPath = modulePath + "/internal/core/health"

	fmtPath = "fmt"
)

// findingTypes are the types whose composite literal is a doctor finding: the
// diagnostic package's Diagnostic, which an in-tree check answers, and the
// finding a plugin's doctor callback answers, rpc.DoctorCheckDiagnostic, with
// its SDK alias. collectDoctorChecksFrom (internal/component/doctor/cmd)
// carries the plugin finding's Fix to the operator, so both are judged alike.
var findingTypes = []typeRef{
	{dir: "internal/core/diagnostic", name: "Diagnostic"},
	{dir: "pkg/plugin/rpc", name: "DoctorCheckDiagnostic"},
	{dir: "pkg/plugin/sdk", name: "DoctorCheckDiagnostic"},
}

// codeKey is the log attribute key a site names its doctor code under.
const codeKey = "code"

// codeDepthMax bounds how many constants a code expression may pass through
// before it reaches its literal: a cycle of constants does not compile, so
// the bound only stops a malformed tree from looping.
const codeDepthMax = 16

// sitesOf answers every problem site in one file, in source order: its doctor
// findings, and its call sites when callSites holds (the file sits under the
// site roots). A call or literal nested in another's arguments is read too.
func (index *treeIndex) sitesOf(file *sourceFile, callSites bool) []Site {
	var sites []Site
	// path holds every node from the file down to the one inspected, and
	// scope the functions among them, innermost last: a message variable is
	// looked up in the functions that enclose its call (stringValued).
	var path, scope []ast.Node
	ast.Inspect(file.ast, func(node ast.Node) bool {
		if node == nil {
			switch path[len(path)-1].(type) {
			case *ast.FuncDecl, *ast.FuncLit:
				scope = scope[:len(scope)-1]
			}
			path = path[:len(path)-1]
			return false
		}
		path = append(path, node)
		switch node := node.(type) {
		case *ast.FuncDecl, *ast.FuncLit:
			scope = append(scope, node)
		case *ast.CompositeLit:
			sites = append(sites, index.findingsOf(file, node)...)
		case *ast.CallExpr:
			if !callSites {
				return true
			}
			site, isSite := index.siteOf(file, node, scope)
			if !isSite {
				return true
			}
			site.Call = types.ExprString(node.Fun)
			sites = append(sites, index.placed(file, site, node))
		}
		return true
	})
	return sites
}

// placed answers site with its file and the lines node spans.
func (index *treeIndex) placed(file *sourceFile, site Site, node ast.Node) Site {
	site.File = file.path
	site.Line = index.fset.Position(node.Pos()).Line
	site.EndLine = index.fset.Position(node.End()).Line
	return site
}

// findingsOf answers the doctor findings one composite literal is or holds:
// the literal itself when its type is the Diagnostic, or each elided element
// of a slice, array or map literal whose element type is the Diagnostic or a
// pointer to it. An elided element's own literal has no type, so the
// inspection reaching it adds nothing twice.
func (index *treeIndex) findingsOf(file *sourceFile, lit *ast.CompositeLit) []Site {
	if lit.Type == nil {
		return nil
	}
	if isDiagnosticType(file, lit.Type) {
		site, isSite := index.findingSite(file, lit, lit.Type)
		if !isSite {
			return nil
		}
		return []Site{site}
	}
	var element ast.Expr
	switch container := lit.Type.(type) {
	case *ast.ArrayType:
		element = container.Elt
	case *ast.MapType:
		element = container.Value
	default:
		return nil
	}
	if star, isStar := element.(*ast.StarExpr); isStar {
		element = star.X
	}
	if !isDiagnosticType(file, element) {
		return nil
	}
	var sites []Site
	for _, value := range lit.Elts {
		if pair, isPair := value.(*ast.KeyValueExpr); isPair {
			value = pair.Value
		}
		if address, isAddress := value.(*ast.UnaryExpr); isAddress && address.Op == token.AND {
			value = address.X
		}
		elided, isLit := value.(*ast.CompositeLit)
		if !isLit {
			continue
		}
		if elided.Type != nil {
			continue
		}
		if site, isSite := index.findingSite(file, elided, element); isSite {
			sites = append(sites, site)
		}
	}
	return sites
}

// findingSite builds the doctor finding a Diagnostic literal is, typed as
// written. A literal with no element is the zero value, no finding. A Fix
// set to the empty string literal is no Fix; any other expression, a
// constant or a variable included, is a Fix the literal sets. A positional
// literal holds no keyed Fix, so it reads as setting none: write it keyed.
func (index *treeIndex) findingSite(file *sourceFile, lit *ast.CompositeLit, typed ast.Expr) (Site, bool) {
	if len(lit.Elts) == 0 {
		return Site{}, false
	}
	site := Site{Kind: KindDoctorFinding, Call: types.ExprString(typed)}
	for _, element := range lit.Elts {
		pair, isPair := element.(*ast.KeyValueExpr)
		if !isPair {
			continue
		}
		key, isIdent := pair.Key.(*ast.Ident)
		if !isIdent {
			continue
		}
		switch key.Name {
		case "Message":
			site.Message = index.text(file, pair.Value)
		case "Code":
			site.CodeExpr = types.ExprString(pair.Value)
			site.Code, _ = index.resolve(file, pair.Value, 0)
		case "Fix":
			if !emptyStringLiteral(pair.Value) {
				site.FixExpr = types.ExprString(pair.Value)
			}
		}
	}
	return index.placed(file, site, lit), true
}

// isDiagnosticType reports whether a type expression names one of the
// findingTypes: through an import name of its package, or unqualified in a
// file of the package itself.
func isDiagnosticType(file *sourceFile, expr ast.Expr) bool {
	var ref typeRef
	switch typed := expr.(type) {
	case *ast.Ident:
		ref = typeRef{dir: file.dir, name: typed.Name}
	case *ast.SelectorExpr:
		receiver, isIdent := typed.X.(*ast.Ident)
		if !isIdent {
			return false
		}
		dir, inTree := cutModule(file.imports[receiver.Name])
		if !inTree {
			return false
		}
		ref = typeRef{dir: dir, name: typed.Sel.Name}
	default:
		return false
	}
	return slices.Contains(findingTypes, ref)
}

// emptyStringLiteral reports whether expr is a string literal holding nothing:
// "" or “.
func emptyStringLiteral(expr ast.Expr) bool {
	literal, isLiteral := expr.(*ast.BasicLit)
	if !isLiteral {
		return false
	}
	if literal.Kind != token.STRING {
		return false
	}
	value, err := strconv.Unquote(literal.Value)
	if err != nil {
		return false
	}
	return value == ""
}

// siteOf answers the site a call is, if it is one. scope holds the functions
// enclosing the call, innermost last. A wrapper's own forwarding call is no
// site: each call of the wrapper is one, judged for its own code (wrappers.go).
func (index *treeIndex) siteOf(file *sourceFile, call *ast.CallExpr, scope []ast.Node) (Site, bool) {
	if index.wrappers.forwards[call] {
		return Site{}, false
	}
	if forwarded, isWrapper := index.wrappers.of(file, call); isWrapper {
		return index.logSite(file, forwarded.kind, call.Args, forwarded.messageAt)
	}
	selector, isSelector := call.Fun.(*ast.SelectorExpr)
	if !isSelector {
		return Site{}, false
	}
	if receiver, isIdent := selector.X.(*ast.Ident); isIdent {
		if path, imported := file.imports[receiver.Name]; imported {
			return index.packageSite(file, path, selector.Sel.Name, call.Args)
		}
	}
	return index.methodSite(file, selector.Sel.Name, call.Args, scope)
}

// packageSite answers the site a call on an imported package is: the package
// logger, the report bus or the health registry. Any other package's call is
// no site.
func (index *treeIndex) packageSite(file *sourceFile, path, name string, args []ast.Expr) (Site, bool) {
	switch path {
	case slogPath:
		kind, messageAt := logShape(name)
		if kind == KindUnspecified {
			return Site{}, false
		}
		return index.logSite(file, kind, args, messageAt)
	case reportPath:
		return index.reportSite(file, name, args)
	case healthPath:
		return index.healthSite(file, name, args)
	}
	return Site{}, false
}

// methodSite answers the site a method call on a value is: a logger's Warn or
// Error, in any of its forms.
func (index *treeIndex) methodSite(file *sourceFile, name string, args []ast.Expr, scope []ast.Node) (Site, bool) {
	kind, messageAt := index.methodShape(file, name, args)
	if kind == KindUnspecified {
		return Site{}, false
	}
	if messageAt >= len(args) {
		return Site{}, false
	}
	if !index.looksLikeLog(file, args[messageAt], args[messageAt+1:], scope) {
		return Site{}, false
	}
	return index.logSite(file, kind, args, messageAt)
}

// methodShape answers the kind and message index of a method named name: a
// Warn or Error form, or a Log or LogAttrs form whose level the source spells.
func (index *treeIndex) methodShape(file *sourceFile, name string, args []ast.Expr) (Kind, int) {
	kind, messageAt := logShape(name)
	if kind == KindUnspecified {
		return index.levelShape(file, name, args)
	}
	return kind, messageAt
}

// logShape answers the kind of a Warn or Error method and the index of its
// message argument, or KindUnspecified for any other name.
func logShape(name string) (Kind, int) {
	switch name {
	case "Warn":
		return KindLogWarn, 0
	case "Error":
		return KindLogError, 0
	case "WarnContext":
		return KindLogWarn, 1
	case "ErrorContext":
		return KindLogError, 1
	}
	return KindUnspecified, 0
}

// levelShape answers the kind of a Log or LogAttrs call whose level argument
// is slog.LevelWarn or slog.LevelError, and the index of its message. A level
// the source does not spell as one of those two (a variable) is no site.
func (index *treeIndex) levelShape(file *sourceFile, name string, args []ast.Expr) (Kind, int) {
	if name != "Log" && name != "LogAttrs" {
		return KindUnspecified, 0
	}
	if len(args) < 3 {
		return KindUnspecified, 0
	}
	level, isSelector := args[1].(*ast.SelectorExpr)
	if !isSelector {
		return KindUnspecified, 0
	}
	if !refersTo(file, level.X, slogPath) {
		return KindUnspecified, 0
	}
	switch level.Sel.Name {
	case "LevelWarn":
		return KindLogWarn, 2
	case "LevelError":
		return KindLogError, 2
	}
	return KindUnspecified, 0
}

// looksLikeLog reports whether a method call's arguments read as a log call:
// a message the source shows is a string (a constant, a concatenation, a
// string variable of an enclosing function, an error's or an operator error's
// text), or attributes after the message. A value's Error(err) with nothing
// after it is none: err is no string.
func (index *treeIndex) looksLikeLog(file *sourceFile, message ast.Expr, rest []ast.Expr, scope []ast.Node) bool {
	if index.stringValued(file, scope, message, 0) {
		return true
	}
	if len(rest) == 0 {
		return false
	}
	if _, isKey := index.resolve(file, rest[0], 0); isKey {
		return true
	}
	if call, isCall := rest[0].(*ast.CallExpr); isCall {
		if call.Ellipsis.IsValid() {
			return true
		}
		if selector, isSelector := call.Fun.(*ast.SelectorExpr); isSelector {
			return refersTo(file, selector.X, slogPath)
		}
	}
	return false
}

// textCall reports whether an expression is x.Error() or x.Problem(): an
// error's text or an operator error's problem, the messages a log line takes
// when its text is not a literal.
func textCall(expr ast.Expr) bool {
	call, isCall := expr.(*ast.CallExpr)
	if !isCall {
		return false
	}
	if len(call.Args) != 0 {
		return false
	}
	selector, isSelector := call.Fun.(*ast.SelectorExpr)
	if !isSelector {
		return false
	}
	return selector.Sel.Name == "Error" || selector.Sel.Name == "Problem"
}

// logSite builds the site of a log call whose message is args[messageAt].
func (index *treeIndex) logSite(file *sourceFile, kind Kind, args []ast.Expr, messageAt int) (Site, bool) {
	if messageAt >= len(args) {
		return Site{}, false
	}
	site := Site{Kind: kind, Message: index.text(file, args[messageAt])}
	if code := index.codeAttr(file, args[messageAt+1:]); code != nil {
		site.CodeExpr = types.ExprString(code)
		site.Code, _ = index.resolve(file, code, 0)
	}
	return site, true
}

// reportSite builds the site of a report-bus raise: its code is the second
// argument and its message the fourth.
func (index *treeIndex) reportSite(file *sourceFile, name string, args []ast.Expr) (Site, bool) {
	var kind Kind
	switch name {
	case "RaiseWarning":
		kind = KindReportWarning
	case "RaiseError":
		kind = KindReportError
	default:
		return Site{}, false
	}
	if len(args) < 4 {
		return Site{}, false
	}
	site := Site{Kind: kind, Message: index.text(file, args[3]), CodeExpr: types.ExprString(args[1])}
	site.Code, _ = index.resolve(file, args[1], 0)
	return site, true
}

// healthSite builds the site of a health registration: its message is the
// row name.
func (index *treeIndex) healthSite(file *sourceFile, name string, args []ast.Expr) (Site, bool) {
	if name != "Register" && name != "RegisterWhen" {
		return Site{}, false
	}
	if len(args) == 0 {
		return Site{}, false
	}
	return Site{Kind: KindHealthRow, Message: index.text(file, args[0])}, true
}

// codeAttr answers the expression a log call's attributes give the `code`
// key, or nil when they give none. A key-value pair is read at the top level,
// where slog reads it; a slog attribute constructor is read anywhere in the
// arguments, so an operator error's LogAttrs(slog.String("code", X)) counts.
func (index *treeIndex) codeAttr(file *sourceFile, args []ast.Expr) ast.Expr {
	for i := 0; i < len(args); i++ {
		key, isKey := index.resolve(file, args[i], 0)
		if !isKey {
			if code := index.nestedCodeAttr(file, args[i]); code != nil {
				return code
			}
			continue
		}
		if i+1 >= len(args) {
			return nil
		}
		if key == codeKey {
			return args[i+1]
		}
		i++
	}
	return nil
}

// nestedCodeAttr answers the value of the first slog attribute constructor
// under expr whose key is `code`.
func (index *treeIndex) nestedCodeAttr(file *sourceFile, expr ast.Expr) ast.Expr {
	var found ast.Expr
	ast.Inspect(expr, func(node ast.Node) bool {
		if found != nil {
			return false
		}
		call, isCall := node.(*ast.CallExpr)
		if !isCall {
			return true
		}
		selector, isSelector := call.Fun.(*ast.SelectorExpr)
		if !isSelector {
			return true
		}
		if !refersTo(file, selector.X, slogPath) {
			return true
		}
		if len(call.Args) != 2 {
			return true
		}
		if key, isKey := index.resolve(file, call.Args[0], 0); isKey && key == codeKey {
			found = call.Args[1]
			return false
		}
		return true
	})
	return found
}

// text answers an argument as report prints it: the string it resolves to,
// or the expression as written.
func (index *treeIndex) text(file *sourceFile, expr ast.Expr) string {
	if value, isString := index.resolve(file, expr, 0); isString {
		return value
	}
	return types.ExprString(expr)
}

// resolve answers the string a code or message expression holds, when the
// source fixes it: a string literal, a concatenation of them, a constant of
// the file's own package, or an exported constant of another package in the
// tree. Anything else (a variable, a call, a field) answers false: its value
// is known only at run time.
func (index *treeIndex) resolve(file *sourceFile, expr ast.Expr, depth int) (string, bool) {
	if depth > codeDepthMax {
		return "", false
	}
	switch node := expr.(type) {
	case *ast.BasicLit:
		if node.Kind != token.STRING {
			return "", false
		}
		value, err := strconv.Unquote(node.Value)
		if err != nil {
			return "", false
		}
		return value, true
	case *ast.ParenExpr:
		return index.resolve(file, node.X, depth+1)
	case *ast.BinaryExpr:
		if node.Op != token.ADD {
			return "", false
		}
		left, isLeft := index.resolve(file, node.X, depth+1)
		if !isLeft {
			return "", false
		}
		right, isRight := index.resolve(file, node.Y, depth+1)
		if !isRight {
			return "", false
		}
		return left + right, true
	case *ast.Ident:
		return index.resolveConstant(file.dir, node.Name, depth)
	case *ast.SelectorExpr:
		receiver, isIdent := node.X.(*ast.Ident)
		if !isIdent {
			return "", false
		}
		path, imported := file.imports[receiver.Name]
		if !imported {
			return "", false
		}
		dir, inTree := cutModule(path)
		if !inTree {
			return "", false
		}
		return index.resolveConstant(dir, node.Sel.Name, depth)
	}
	return "", false
}

// resolveConstant answers the string the constant name of directory dir holds.
func (index *treeIndex) resolveConstant(dir, name string, depth int) (string, bool) {
	ref, declared := index.constants[dir][name]
	if !declared {
		return "", false
	}
	return index.resolve(ref.file, ref.expr, depth+1)
}

// cutModule answers the tree directory of an import path in this module.
func cutModule(path string) (string, bool) {
	if len(path) <= len(modulePath)+1 {
		return "", false
	}
	if path[:len(modulePath)+1] != modulePath+"/" {
		return "", false
	}
	return path[len(modulePath)+1:], true
}

// refersTo reports whether expr is a name the file imports path by.
func refersTo(file *sourceFile, expr ast.Expr, path string) bool {
	ident, isIdent := expr.(*ast.Ident)
	if !isIdent {
		return false
	}
	return file.imports[ident.Name] == path
}
