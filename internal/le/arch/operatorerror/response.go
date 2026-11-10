// Design: docs/architecture/cli/error-surface.md -- "The operator error", the error answer's one route
//
// Overview: operatorerror.go -- the boundary set and the return-site statuses
//
// response.go finds the refusals a handler answers rather than returns. A
// command handler may refuse with plugin.Response{Status: plugin.StatusError,
// Error: <text>} and a nil Go error: Response.Error is a plain string on the
// wire, and it is what the operator reads, so it owes the same three parts a
// returned error does. The one route an operator error takes into it is
// plugin.OperatorErrorResponse, whose parameter is *opererr.Error.

package archoperatorerror

import (
	"go/ast"
	"go/token"
	"go/types"
)

// errorMember is the name of both the Response field an answer's refusal
// text sits in and the error method that renders an error value's text.
const errorMember = "Error"

// responseSites answers the error answers one handler body builds: each
// plugin.Response literal whose Status is plugin.StatusError, and each call of
// the declared route. A literal or call inside a nested function literal
// belongs to that literal, as a return statement there does.
func responseSites(target handlerTarget) []Site {
	var sites []Site
	ast.Inspect(target.body, func(node ast.Node) bool {
		switch expression := node.(type) {
		case *ast.FuncLit:
			return false
		case *ast.CallExpr:
			if target.file.refersTo(expression.Fun, pluginPath, responseErrorRoute) {
				sites = append(sites, responseSite(target, expression.Pos(), StatusTyped, types.ExprString(expression)))
			}
		case *ast.CompositeLit:
			site, isSite := classifyResponse(target, expression)
			if isSite {
				sites = append(sites, site)
			}
		}
		return true
	})
	return sites
}

// responseSite builds the site of one error answer.
func responseSite(target handlerTarget, at token.Pos, status Status, expr string) Site {
	return Site{
		File:    target.file.path,
		Line:    target.file.index.fset.Position(at).Line,
		Form:    FormResponse,
		Handler: target.name,
		Status:  status,
		Expr:    expr,
	}
}

// classifyResponse answers the site one composite literal makes, and false
// for a literal that is not a plugin.Response or whose Status is not the
// StatusError constant. A Status computed at run time is not read: the scan
// cannot tell whether that answer refuses.
func classifyResponse(target handlerTarget, literal *ast.CompositeLit) (Site, bool) {
	file := target.file
	if !file.refersTo(literal.Type, pluginPath, responseType) {
		return Site{}, false
	}
	var status, message ast.Expr
	at := literal.Lbrace
	for _, element := range literal.Elts {
		field, isField := element.(*ast.KeyValueExpr)
		if !isField {
			continue
		}
		key, isKey := field.Key.(*ast.Ident)
		if !isKey {
			continue
		}
		if key.Name == "Status" {
			status = field.Value
		}
		if key.Name == errorMember {
			message = field.Value
			at = field.Pos()
		}
	}
	if status == nil {
		return Site{}, false
	}
	if !file.refersTo(status, pluginPath, responseErrorStatus) {
		return Site{}, false
	}
	if message == nil {
		return responseSite(target, at, StatusPlain, "(no Error field)"), true
	}
	return responseSite(target, at, classifyMessage(file, message), "Error: "+types.ExprString(message)), true
}

// classifyMessage answers what the text in a Response's Error field is.
//
// Pass-through is text another producer wrote: the Error() of an error value
// whose type the syntax does not show (a callee's error, a sentinel), or the
// Error field of another answer being forwarded. The producer of that value
// is where its fix belongs, as a returned error variable's is.
//
// Plain is text this site wrote: a literal, a constant, a formatted or built
// string, the Error() of an errors.New or fmt.Errorf, and the Error() of an
// opererr value too, because that text reaches the answer by hand rather than
// through plugin.OperatorErrorResponse, the one declared route. Typed is only
// that route, which responseSites reads as a call.
func classifyMessage(file *sourceFile, message ast.Expr) Status {
	switch node := ast.Unparen(message).(type) {
	case *ast.SelectorExpr:
		if node.Sel.Name == errorMember {
			return StatusPassThrough
		}
		return StatusPlain
	case *ast.CallExpr:
		return classifyErrorText(file, node)
	default:
		return StatusPlain
	}
}

// classifyErrorText answers what a call in a Response's Error field is: the
// Error() method of an error value is pass-through unless this site built
// that value (an errors.New, a fmt.Errorf, an opererr call), and every other
// call (fmt.Sprintf, a text buffer's String) is text the site wrote.
func classifyErrorText(file *sourceFile, call *ast.CallExpr) Status {
	selector, isSelector := call.Fun.(*ast.SelectorExpr)
	if !isSelector {
		return StatusPlain
	}
	if selector.Sel.Name != errorMember {
		return StatusPlain
	}
	if len(call.Args) != 0 {
		return StatusPlain
	}
	if classifyError(file, selector.X) != StatusPassThrough {
		return StatusPlain
	}
	return StatusPassThrough
}
