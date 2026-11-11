// Design: docs/contributing/ze-go-style.md -- a returned refusal carries the miss.
// Related: internal/component/plugin/types.go -- OperatorErrorResponse's contract.
package archrefusebydefault

import (
	"go/ast"
	"strconv"
)

// operatorResponseImports resolves the canonical response package's local
// aliases. A same-spelled helper from any other path carries no such proof.
func operatorResponseImports(file *ast.File) map[string]bool {
	names := map[string]bool{}
	for _, spec := range file.Imports {
		path, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			continue
		}
		if path != "github.com/ze-software/ze/internal/component/plugin" {
			continue
		}
		name := "plugin"
		if spec.Name != nil {
			name = spec.Name.Name
		}
		if name == "." {
			continue
		}
		if name == "_" {
			continue
		}
		names[name] = true
	}
	return names
}

// operatorErrorResponse recognizes only a direct canonical typed refusal
// beside its nil transport error. The helper always returns StatusError and
// renders its operator error; its nil argument is a programmer panic, never
// a successful response. A local binding hides an import even in a closure.
func (s *fileScan) operatorErrorResponse(ret *ast.ReturnStmt) bool {
	if len(ret.Results) != 2 {
		return false
	}
	if !isNil(ret.Results[1]) {
		return false
	}
	call, isCall := ast.Unparen(ret.Results[0]).(*ast.CallExpr)
	if !isCall {
		return false
	}
	if len(call.Args) != 1 {
		return false
	}
	if call.Ellipsis.IsValid() {
		return false
	}
	helper, isSelector := ast.Unparen(call.Fun).(*ast.SelectorExpr)
	if !isSelector {
		return false
	}
	if helper.Sel.Name != "OperatorErrorResponse" {
		return false
	}
	alias, isAlias := ast.Unparen(helper.X).(*ast.Ident)
	if !isAlias {
		return false
	}
	if alias.Obj != nil {
		return false
	}
	return s.operatorImports[alias.Name]
}
