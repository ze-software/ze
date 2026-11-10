// Design: docs/architecture/doctor-and-health-checks.md -- the doctor-coverage gate
//
// Overview: doctorcoverage.go -- the site kinds and statuses, and the scan
//
// sites.go decides which calls are problem sites and reads the code each one
// names. The scan reads syntax, not types, so the shapes are:
//
//	slog.Warn(msg, ...), slog.Error(msg, ...)            the package logger
//	x.Warn(msg, ...), x.Error(msg, ...)                  a logger value
//	x.WarnContext(ctx, msg, ...), x.ErrorContext(...)    the context forms
//	x.Log(ctx, slog.LevelWarn|LevelError, msg, ...)      the level forms,
//	x.LogAttrs(ctx, slog.LevelWarn|LevelError, msg, ...) LogAttrs included
//	report.RaiseWarning(source, code, ...)               the report bus
//	report.RaiseError(source, code, ...)
//	health.Register(name, ...), health.RegisterWhen(...) a health row
//
// A call on another imported package (http.Error) is no site. A method call
// on a value is a log call when its message is a string constant, or when the
// arguments after it are attributes: a string key, a slog attribute, or a
// spread (an operator error's LogAttrs). That excludes err.Error(), which
// takes no argument, and a value's Error(err) with nothing after it.
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
	"strconv"
)

// The import paths whose calls are sites.
const (
	slogPath   = "log/slog"
	reportPath = modulePath + "/internal/core/report"
	healthPath = modulePath + "/internal/core/health"
)

// codeKey is the log attribute key a site names its doctor code under.
const codeKey = "code"

// codeDepthMax bounds how many constants a code expression may pass through
// before it reaches its literal: a cycle of constants does not compile, so
// the bound only stops a malformed tree from looping.
const codeDepthMax = 16

// sitesOf answers every problem site in one file, in source order. A call
// nested in another call's arguments is read too.
func (index *treeIndex) sitesOf(file *sourceFile) []Site {
	var sites []Site
	ast.Inspect(file.ast, func(node ast.Node) bool {
		call, isCall := node.(*ast.CallExpr)
		if !isCall {
			return true
		}
		site, isSite := index.siteOf(file, call)
		if !isSite {
			return true
		}
		site.File = file.path
		site.Line = index.fset.Position(call.Pos()).Line
		site.EndLine = index.fset.Position(call.End()).Line
		site.Call = types.ExprString(call.Fun)
		sites = append(sites, site)
		return true
	})
	return sites
}

// siteOf answers the site a call is, if it is one.
func (index *treeIndex) siteOf(file *sourceFile, call *ast.CallExpr) (Site, bool) {
	selector, isSelector := call.Fun.(*ast.SelectorExpr)
	if !isSelector {
		return Site{}, false
	}
	if receiver, isIdent := selector.X.(*ast.Ident); isIdent {
		if path, imported := file.imports[receiver.Name]; imported {
			return index.packageSite(file, path, selector.Sel.Name, call.Args)
		}
	}
	return index.methodSite(file, selector.Sel.Name, call.Args)
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
func (index *treeIndex) methodSite(file *sourceFile, name string, args []ast.Expr) (Site, bool) {
	kind, messageAt := logShape(name)
	if kind == KindUnspecified {
		kind, messageAt = index.levelShape(file, name, args)
	}
	if kind == KindUnspecified {
		return Site{}, false
	}
	if messageAt >= len(args) {
		return Site{}, false
	}
	if !index.looksLikeLog(file, args[messageAt], args[messageAt+1:]) {
		return Site{}, false
	}
	return index.logSite(file, kind, args, messageAt)
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
// a message that is a string constant, a message that is an error's or an
// operator error's text, or attributes after the message.
func (index *treeIndex) looksLikeLog(file *sourceFile, message ast.Expr, rest []ast.Expr) bool {
	if _, isString := index.resolve(file, message, 0); isString {
		return true
	}
	if textCall(message) {
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
