// Design: docs/architecture/plugin/feature-gates.md -- bounded canonical family-name declaration forms.
// Related: expressions.go -- retained immutable expression resolution.
package repoinventory

import (
	"go/ast"
	"go/token"
)

// familyName resolves the declared canonical name returned by family.MustRegister,
// never the host registry. The retained constructor's AFI/SAFI name arguments are
// the same two values that core/family joins for Family.String.
func (e *expression) familyName(source sourceExpr) (string, error) {
	for range 64 {
		call, ok := source.value.(*ast.CallExpr)
		if ok {
			selector, selected := call.Fun.(*ast.SelectorExpr)
			if !selected {
				return "", e.problem(source, "family name has no declared constructor")
			}
			owner, named := selector.X.(*ast.Ident)
			if !named {
				return "", e.problem(source, "family constructor ownership is unresolved")
			}
			if source.pkg.imports[source.file][owner.Name] != "internal/core/family" {
				return "", e.problem(source, "family name requires a retained family.MustRegister declaration")
			}
			if selector.Sel.Name != "MustRegister" {
				return "", e.problem(source, "family name requires a retained family.MustRegister declaration")
			}
			if len(call.Args) != 4 {
				return "", e.problem(source, "family.MustRegister requires AFI, SAFI and both canonical names")
			}
			afi, err := e.text(sourceExpr{value: call.Args[2], file: source.file, pkg: source.pkg})
			if err != nil {
				return "", err
			}
			safi, err := e.text(sourceExpr{value: call.Args[3], file: source.file, pkg: source.pkg})
			if err != nil {
				return "", err
			}
			if afi == "" {
				return "", e.problem(source, "family AFI name is empty")
			}
			if safi == "" {
				return "", e.problem(source, "family SAFI name is empty")
			}
			return afi + "/" + safi, nil
		}
		if err := e.immutable(source); err != nil {
			return "", err
		}
		resolved, err := e.target(source)
		if err != nil {
			return "", err
		}
		source = resolved
	}
	return "", e.problem(source, "family constructor exceeds 64 declaration steps")
}

func (e *expression) stringSliceHelper(function sourceFunction, source sourceExpr) ([]string, error) {
	body := function.decl.Body
	if body == nil {
		return nil, e.problem(source, "slice helper has no retained Go body")
	}
	if len(body.List) == 1 {
		returned, ok := body.List[0].(*ast.ReturnStmt)
		if ok && len(returned.Results) == 1 {
			return e.texts(sourceExpr{value: returned.Results[0], file: function.file, pkg: function.pkg})
		}
	}
	return e.familyProjection(function, source)
}

// familyProjection accepts only the finite make(len(set)), range set,
// names[i]=family.String(), return names form used by FlowSpec. Every other
// statement or binding shape refuses rather than interpreting arbitrary Go.
func (e *expression) familyProjection(function sourceFunction, source sourceExpr) ([]string, error) {
	problem := func() ([]string, error) {
		return nil, e.problem(source, "slice helper is not an immutable family-name projection")
	}
	body := function.decl.Body.List
	if len(body) != 3 {
		return problem()
	}
	allocation, ok := body[0].(*ast.AssignStmt)
	if !ok {
		return problem()
	}
	if allocation.Tok != token.DEFINE {
		return problem()
	}
	if len(allocation.Lhs) != 1 {
		return problem()
	}
	if len(allocation.Rhs) != 1 {
		return problem()
	}
	names, ok := allocation.Lhs[0].(*ast.Ident)
	if !ok {
		return problem()
	}
	makeCall, ok := allocation.Rhs[0].(*ast.CallExpr)
	if !ok {
		return problem()
	}
	makeName, ok := makeCall.Fun.(*ast.Ident)
	if !ok {
		return problem()
	}
	if makeName.Name != "make" {
		return problem()
	}
	if len(makeCall.Args) != 2 {
		return problem()
	}
	slice, ok := makeCall.Args[0].(*ast.ArrayType)
	if !ok {
		return problem()
	}
	element, ok := slice.Elt.(*ast.Ident)
	if !ok {
		return problem()
	}
	if slice.Len != nil {
		return problem()
	}
	if element.Name != "string" {
		return problem()
	}
	length, ok := makeCall.Args[1].(*ast.CallExpr)
	if !ok {
		return problem()
	}
	lenName, ok := length.Fun.(*ast.Ident)
	if !ok {
		return problem()
	}
	if lenName.Name != "len" {
		return problem()
	}
	if len(length.Args) != 1 {
		return problem()
	}
	set, ok := length.Args[0].(*ast.Ident)
	if !ok {
		return problem()
	}
	iteration, ok := body[1].(*ast.RangeStmt)
	if !ok {
		return problem()
	}
	if iteration.Tok != token.DEFINE {
		return problem()
	}
	if len(iteration.Body.List) != 1 {
		return problem()
	}
	rangedSet, ok := iteration.X.(*ast.Ident)
	if !ok {
		return problem()
	}
	if !sameBinding(set, rangedSet) {
		return problem()
	}
	index, ok := iteration.Key.(*ast.Ident)
	if !ok {
		return problem()
	}
	family, ok := iteration.Value.(*ast.Ident)
	if !ok {
		return problem()
	}
	assignment, ok := iteration.Body.List[0].(*ast.AssignStmt)
	if !ok {
		return problem()
	}
	if assignment.Tok != token.ASSIGN {
		return problem()
	}
	if len(assignment.Lhs) != 1 {
		return problem()
	}
	if len(assignment.Rhs) != 1 {
		return problem()
	}
	target, ok := assignment.Lhs[0].(*ast.IndexExpr)
	if !ok {
		return problem()
	}
	targetNames, ok := target.X.(*ast.Ident)
	if !ok {
		return problem()
	}
	targetIndex, ok := target.Index.(*ast.Ident)
	if !ok {
		return problem()
	}
	if !sameBinding(names, targetNames) {
		return problem()
	}
	if !sameBinding(index, targetIndex) {
		return problem()
	}
	conversion, ok := assignment.Rhs[0].(*ast.CallExpr)
	if !ok {
		return problem()
	}
	method, ok := conversion.Fun.(*ast.SelectorExpr)
	if !ok {
		return problem()
	}
	receiver, ok := method.X.(*ast.Ident)
	if !ok {
		return problem()
	}
	if len(conversion.Args) != 0 {
		return problem()
	}
	if method.Sel.Name != "String" {
		return problem()
	}
	if !sameBinding(family, receiver) {
		return problem()
	}
	returned, ok := body[2].(*ast.ReturnStmt)
	if !ok {
		return problem()
	}
	if len(returned.Results) != 1 {
		return problem()
	}
	resultName, ok := returned.Results[0].(*ast.Ident)
	if !ok {
		return problem()
	}
	if !sameBinding(names, resultName) {
		return problem()
	}
	collection := sourceExpr{value: set, file: function.file, pkg: function.pkg}
	for range 64 {
		if literal, ok := collection.value.(*ast.CompositeLit); ok {
			result := make([]string, 0, len(literal.Elts))
			for _, value := range literal.Elts {
				name, err := e.familyName(sourceExpr{value: value, file: collection.file, pkg: collection.pkg})
				if err != nil {
					return nil, err
				}
				result = append(result, name)
			}
			return result, nil
		}
		if err := e.immutable(collection); err != nil {
			return nil, err
		}
		resolved, err := e.target(collection)
		if err != nil {
			return nil, err
		}
		collection = resolved
	}
	return nil, e.problem(source, "family projection exceeds 64 declaration steps")
}

func sameBinding(a, b *ast.Ident) bool {
	return a.Name == b.Name && a.Obj == b.Obj
}
