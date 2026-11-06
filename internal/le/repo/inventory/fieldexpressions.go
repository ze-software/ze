// Design: docs/architecture/plugin/feature-gates.md -- metadata preserves lexical ownership.
package repoinventory

import "go/ast"

// fieldTarget follows at most 64 immutable owner edges and shares the expression
// nesting bound. It projects keyed retained struct literals only; pointer writes,
// unkeyed layout, dynamic receivers and omitted fields do not invent metadata.
func (e *expression) fieldTarget(origin sourceExpr, selector *ast.SelectorExpr) (sourceExpr, error) {
	if e.depth == 64 {
		return sourceExpr{}, e.problem(origin, "field ownership exceeds 64 declaration steps")
	}
	e.depth++
	defer func() { e.depth-- }()
	source := sourceExpr{value: selector.X, file: origin.file, pkg: origin.pkg}
	for range 64 {
		if literal, found := source.value.(*ast.CompositeLit); found {
			structure := false
			if _, anonymous := literal.Type.(*ast.StructType); anonymous {
				structure = true
			}
			if definition, _, retained := e.concreteType(sourceExpr{value: literal.Type, file: source.file, pkg: source.pkg}); retained {
				_, structure = definition.Type.(*ast.StructType)
			}
			if !structure {
				return sourceExpr{}, e.problem(origin, "field ownership is not a retained struct")
			}
			for _, element := range literal.Elts {
				field, keyed := element.(*ast.KeyValueExpr)
				if !keyed {
					return sourceExpr{}, e.problem(origin, "unkeyed struct field ownership is unsupported")
				}
				name, identifier := field.Key.(*ast.Ident)
				if !identifier {
					return sourceExpr{}, e.problem(origin, "struct field ownership is unresolved")
				}
				if name.Name == selector.Sel.Name {
					return sourceExpr{value: field.Value, file: source.file, pkg: source.pkg}, nil
				}
			}
			return sourceExpr{}, e.problem(origin, "struct field ownership has no explicit initializer")
		}
		if parenthesized, found := source.value.(*ast.ParenExpr); found {
			source.value = parenthesized.X
			continue
		}
		if err := e.callableImmutable(source); err != nil {
			return sourceExpr{}, e.problem(origin, "field ownership is not immutable: "+err.Error())
		}
		next, err := e.target(source)
		if err != nil {
			return sourceExpr{}, e.problem(origin, "field ownership is unresolved: "+err.Error())
		}
		source = next
	}
	return sourceExpr{}, e.problem(origin, "field ownership exceeds 64 declaration steps")
}
