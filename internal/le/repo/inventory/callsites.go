// Design: docs/architecture/plugin/feature-gates.md -- membership is reachable registration, not literal census.
// Related: callablealiases.go -- the local closure a call executes; methodpromotion.go -- promoted methods.
package repoinventory

import (
	"go/ast"
	"go/token"
)

// closureBinding answers the function literal that a defining short
// declaration (name := func(...) {...}) binds to name's local variable. A
// reassignment or a reused short declaration of the name is a separate use,
// which the reference census records as indirect.
func closureBinding(name *ast.Ident) (*ast.FuncLit, bool) {
	if name.Obj == nil {
		return nil, false
	}
	if name.Obj.Kind != ast.Var {
		return nil, false
	}
	declaration, declared := name.Obj.Decl.(*ast.AssignStmt)
	if !declared {
		return nil, false
	}
	if declaration.Tok != token.DEFINE {
		return nil, false
	}
	if len(declaration.Lhs) != len(declaration.Rhs) {
		return nil, false
	}
	for index, left := range declaration.Lhs {
		declaring, named := left.(*ast.Ident)
		if !named {
			continue
		}
		if declaring.Obj != name.Obj {
			continue
		}
		literal, closure := declaration.Rhs[index].(*ast.FuncLit)
		return literal, closure
	}
	return nil, false
}

// closureCalledOnly reports whether every use of a local closure's name in
// file, apart from its declaring name, is the callee of a call. A copy, a
// reassignment, a reused short declaration or an address of the name is any
// other use, after which a call through the name may run another function.
func closureCalledOnly(file *ast.File, binding *ast.Object) bool {
	allowed := make(map[*ast.Ident]bool)
	if declaration, declared := binding.Decl.(*ast.AssignStmt); declared {
		for _, left := range declaration.Lhs {
			if name, named := left.(*ast.Ident); named {
				if name.Obj == binding {
					allowed[name] = true
				}
			}
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		call, called := node.(*ast.CallExpr)
		if !called {
			return true
		}
		callee := call.Fun
		for {
			parenthesized, ok := callee.(*ast.ParenExpr)
			if !ok {
				break
			}
			callee = parenthesized.X
		}
		if name, named := callee.(*ast.Ident); named {
			if name.Obj == binding {
				allowed[name] = true
			}
		}
		return true
	})
	only := true
	ast.Inspect(file, func(node ast.Node) bool {
		name, named := node.(*ast.Ident)
		if !named {
			return true
		}
		if name.Obj != binding {
			return true
		}
		if !allowed[name] {
			only = false
		}
		return true
	})
	return only
}

// embeddedTypeName answers the type name an embedded field's type expression
// spells, through pointers and type arguments, and "" when it spells none.
func embeddedTypeName(value ast.Expr) string {
	for range 128 {
		switch datatype := value.(type) {
		case *ast.Ident:
			return datatype.Name
		case *ast.SelectorExpr:
			return datatype.Sel.Name
		case *ast.StarExpr:
			value = datatype.X
		case *ast.IndexExpr:
			value = datatype.X
		case *ast.IndexListExpr:
			value = datatype.X
		default:
			return ""
		}
	}
	return ""
}
