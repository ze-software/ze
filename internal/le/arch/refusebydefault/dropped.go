// Design: docs/contributing/ze-go-style.md -- a zero value is never an answer, an error dropped below WARN
// Overview: refusebydefault.go -- the walk that calls droppedError on each if
//
// dropped.go finds the fifth shape: an error that a branch catches and then
// drops, the function going on or answering a non-error, where the only
// record of the failure is a log line below WARN. Ze logs at WARN by default,
// so the operator never sees that line, and the caller reads the result as a
// success. The MOBIKE route-source lookup failed with EPERM under the SPD
// catch-all, the failure went to a debug line, and Ze never sent
// UPDATE_SA_ADDRESSES (fixed in d6ef7093a8).

package archrefusebydefault

import (
	"go/ast"
	"go/token"
)

// droppedError judges an if that catches an error and drops it: a branch on
// `err != nil` (or the else of `err == nil`) whose only record is a log call
// below WARN, and which then falls through, continues, breaks, or returns
// something other than an error.
//
// The branch hands the error on, and is not refused, when any statement
// other than a quiet log mentions the error, when it records what it met
// (recordsMiss), when it logs at WARN or above, when it panics, or when it
// returns a non-nil error or a bare return a reader cannot judge. A branch
// with no log at all is not this shape: it is silent, not quiet.
func (s *fileScan) droppedError(ifStmt *ast.IfStmt, fnResults results, fn string) {
	body, name, first, last := caughtError(ifStmt)
	if body == nil {
		return
	}
	quiet := []ast.Node{}
	for index, stmt := range body.List {
		if call, isLog := logCall(stmt); isLog {
			if !belowWarn(call) {
				return
			}
			quiet = append(quiet, call)
			continue
		}
		if mentions(stmt, name) {
			return
		}
		if index == len(body.List)-1 {
			if !s.drops(stmt, fnResults) {
				return
			}
			continue
		}
		if leaves(stmt) {
			return
		}
	}
	if len(quiet) == 0 {
		return
	}
	if recordsMiss(body.List, name) {
		return
	}
	s.addHeader(RuleErrorDroppedBelowWarn, fn,
		"the error "+name+" is dropped and its only record is a log line below WARN, which the default level hides; return it, count it, or log it at WARN",
		first, last, quiet...)
}

// caughtError answers the branch that runs when an error is set, the error's
// name, and the header span to anchor on: the if's body for `err != nil`, the
// else block for `err == nil`. It answers a nil body when the if catches no
// error by name.
func caughtError(ifStmt *ast.IfStmt) (body *ast.BlockStmt, name string, first, last token.Pos) {
	binary, isBinary := ast.Unparen(ifStmt.Cond).(*ast.BinaryExpr)
	if !isBinary {
		return nil, "", token.NoPos, token.NoPos
	}
	ident, isIdent := nilComparand(binary).(*ast.Ident)
	if !isIdent {
		return nil, "", token.NoPos, token.NoPos
	}
	if !isErrorName(ident.Name) {
		return nil, "", token.NoPos, token.NoPos
	}
	if binary.Op == token.NEQ {
		return ifStmt.Body, ident.Name, ifStmt.If, ifStmt.Body.Lbrace
	}
	elseBlock, isBlock := ifStmt.Else.(*ast.BlockStmt)
	if !isBlock {
		return nil, "", token.NoPos, token.NoPos
	}
	return elseBlock, ident.Name, elseBlock.Lbrace, elseBlock.Lbrace
}

// logCall answers the call a statement makes when the statement is a slog
// call: `log.Debug(...)`, `logger().InfoContext(ctx, ...)`, `l.Log(ctx,
// level, ...)`. A method named like a level with no message is no log call
// (`ps.Info()` answers a value).
func logCall(stmt ast.Stmt) (*ast.CallExpr, bool) {
	expr, isExpr := stmt.(*ast.ExprStmt)
	if !isExpr {
		return nil, false
	}
	call, isCall := expr.X.(*ast.CallExpr)
	if !isCall {
		return nil, false
	}
	selector, isSelector := call.Fun.(*ast.SelectorExpr)
	if !isSelector {
		return nil, false
	}
	switch selector.Sel.Name {
	case "Debug", "Info", "Warn", "Error":
		return call, len(call.Args) >= 1
	case "DebugContext", "InfoContext", "WarnContext", "ErrorContext":
		return call, len(call.Args) >= 2
	case "Log", "LogAttrs":
		return call, len(call.Args) >= 3
	default:
		return nil, false
	}
}

// belowWarn reports whether a log call writes below WARN. A `Log` whose level
// is not a literal slog.LevelDebug or slog.LevelInfo is read as loud: this
// check cannot see the level, so it does not refuse it.
func belowWarn(call *ast.CallExpr) bool {
	selector, isSelector := call.Fun.(*ast.SelectorExpr)
	if !isSelector {
		return false
	}
	switch selector.Sel.Name {
	case "Debug", "Info", "DebugContext", "InfoContext":
		return true
	case "Log", "LogAttrs":
		switch finalName(call.Args[1]) {
		case "LevelDebug", "LevelInfo":
			return true
		default:
			return false
		}
	default:
		return false
	}
}

// drops reports whether a branch's last statement leaves without the error:
// it falls through, continues, breaks, or returns a value that is no error.
// A return of a non-nil error refuses rather than drops, a bare return in a
// function with an error result answers whatever the named results hold,
// and a panic stops; none of those is this shape.
func (s *fileScan) drops(stmt ast.Stmt, fnResults results) bool {
	switch typed := stmt.(type) {
	case *ast.ReturnStmt:
		if s.operatorErrorResponse(typed) {
			return false
		}
		if !fnResults.errorLast {
			return true
		}
		return answersNilError(typed, fnResults)
	case *ast.BranchStmt:
		return true
	default:
		return !leaves(stmt)
	}
}

// leaves reports whether a statement holds a return or a panic, so the
// branch may hand the failure on in a way this check does not read.
func leaves(stmt ast.Stmt) bool {
	found := false
	ast.Inspect(stmt, func(node ast.Node) bool {
		switch typed := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			found = true
		case *ast.CallExpr:
			if isIdentNamed(typed.Fun, "panic") {
				found = true
			}
		}
		return !found
	})
	return found
}
