// Design: docs/architecture/plugin/feature-gates.md -- direct retained concrete methods own their execution.
// Related: callablealiases.go -- normalized calls; registrations.go -- actual method-body traversal.
package repoinventory

import (
	"go/ast"
	"go/token"
	"go/types"
)

// method resolves direct value methods and explicit single-pointer methods.
// Receiver forms must match; implicit addressing, value-method fallback,
// dynamic interfaces, promotion and generic method sets remain unresolved.
func (e *expression) method(source sourceExpr) (sourceFunction, bool) {
	selector, ok := source.value.(*ast.SelectorExpr)
	if !ok {
		return sourceFunction{}, false
	}
	if _, ok := selector.X.(*ast.Ident); !ok {
		return sourceFunction{}, false
	}
	receiver := sourceExpr{value: selector.X, file: source.file, pkg: source.pkg}
	datatype, found := e.valueType(receiver, 0)
	if !found {
		return sourceFunction{}, false
	}
	pointer, indirect := datatype.value.(*ast.StarExpr)
	if indirect {
		datatype.value = pointer.X
	}
	definition, owner, found := e.concreteType(datatype)
	if !found {
		return sourceFunction{}, false
	}
	var result sourceFunction
	for _, file := range owner.files {
		for _, declaration := range file.File.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if function.Name.Name != selector.Sel.Name {
				continue
			}
			if function.Recv == nil {
				continue
			}
			if len(function.Recv.List) != 1 {
				return sourceFunction{}, false
			}
			receiverType := function.Recv.List[0].Type
			receiverPointer, receiverIndirect := receiverType.(*ast.StarExpr)
			if receiverIndirect != indirect {
				continue
			}
			if receiverIndirect {
				receiverType = receiverPointer.X
			}
			name, ok := receiverType.(*ast.Ident)
			if !ok {
				continue
			}
			binding := name.Obj
			if binding == nil {
				binding = callablePackageBinding(owner, name.Name)
			}
			if binding == nil {
				continue
			}
			if binding.Decl != definition {
				continue
			}
			if result.decl != nil {
				return sourceFunction{}, false
			}
			// Bodyless methods are not covered by the package-function assembly
			// certificate. They must retain unresolved execution ownership.
			if function.Body == nil {
				return sourceFunction{}, false
			}
			result = sourceFunction{decl: function, file: file.File, pkg: owner}
		}
	}
	return result, result.decl != nil
}

// valueType follows at most 64 declaration edges. It derives static types, not
// possible dynamic values; an interface result never becomes its returned literal.
func (e *expression) valueType(source sourceExpr, depth int) (sourceExpr, bool) {
	if depth == 64 {
		return sourceExpr{}, false
	}
	switch value := source.value.(type) {
	case *ast.ParenExpr:
		return e.valueType(sourceExpr{value: value.X, file: source.file, pkg: source.pkg}, depth+1)
	case *ast.CompositeLit:
		return sourceExpr{value: value.Type, file: source.file, pkg: source.pkg}, value.Type != nil
	case *ast.CallExpr:
		function, found := e.declaredFunction(sourceExpr{value: value.Fun, file: source.file, pkg: source.pkg})
		if !found {
			return sourceExpr{}, false
		}
		if function.decl.Type.TypeParams.NumFields() != 0 {
			return sourceExpr{}, false
		}
		if function.decl.Type.Results.NumFields() != 1 {
			return sourceExpr{}, false
		}
		return sourceExpr{value: function.decl.Type.Results.List[0].Type, file: function.file, pkg: function.pkg}, true
	case *ast.Ident:
		binding := value.Obj
		if binding == nil {
			binding = callablePackageBinding(source.pkg, value.Name)
		}
		if binding == nil {
			return sourceExpr{}, false
		}
		if binding.Kind != ast.Var {
			return sourceExpr{}, false
		}
		if err := e.immutable(source); err != nil {
			return sourceExpr{}, false
		}
		// Package declarations may live in a different retained file; imports
		// in their types and initializers belong to that file, not the caller.
		for _, file := range source.pkg.files {
			if file.File.Scope.Lookup(value.Name) == binding {
				source.file = file.File
				break
			}
		}
		switch declaration := binding.Decl.(type) {
		case *ast.Field:
			return sourceExpr{value: declaration.Type, file: source.file, pkg: source.pkg}, true
		case *ast.ValueSpec:
			if declaration.Type != nil {
				return sourceExpr{value: declaration.Type, file: source.file, pkg: source.pkg}, true
			}
			if len(declaration.Names) != len(declaration.Values) {
				return sourceExpr{}, false
			}
			for index, name := range declaration.Names {
				if name.Obj == binding {
					return e.valueType(sourceExpr{value: declaration.Values[index], file: source.file, pkg: source.pkg}, depth+1)
				}
			}
		case *ast.AssignStmt:
			// go/parser binds range variables to a synthetic short assignment
			// with one unary RANGE RHS, not to the RangeStmt itself.
			if len(declaration.Rhs) == 1 {
				if ranged, ok := declaration.Rhs[0].(*ast.UnaryExpr); ok {
					if ranged.Op == token.RANGE {
						if declaration.Tok != token.DEFINE {
							return sourceExpr{}, false
						}
						collection, found := e.valueType(sourceExpr{value: ranged.X, file: source.file, pkg: source.pkg}, depth+1)
						if !found {
							return sourceExpr{}, false
						}
						element, datatype, known := rangedElement(collection)
						if !known {
							return sourceExpr{}, false
						}
						if len(declaration.Lhs) != element+1 {
							return sourceExpr{}, false
						}
						name, ok := declaration.Lhs[element].(*ast.Ident)
						if !ok {
							return sourceExpr{}, false
						}
						if name.Obj != binding {
							return sourceExpr{}, false
						}
						return sourceExpr{value: datatype, file: collection.file, pkg: collection.pkg}, true
					}
				}
			}
			if len(declaration.Lhs) != len(declaration.Rhs) {
				return sourceExpr{}, false
			}
			for index, left := range declaration.Lhs {
				name, ok := left.(*ast.Ident)
				if !ok {
					continue
				}
				if name.Obj == binding {
					return e.valueType(sourceExpr{value: declaration.Rhs[index], file: source.file, pkg: source.pkg}, depth+1)
				}
			}
		}
	}
	return sourceExpr{}, false
}

// rangedElement answers which range binding denotes the element of a
// collection, and its type. Only the second binding of a slice or array range
// is its element, and the key is not; a channel range has one binding, and it
// is the received element.
func rangedElement(collection sourceExpr) (int, ast.Expr, bool) {
	switch kind := collection.value.(type) {
	case *ast.ArrayType:
		return 1, kind.Elt, true
	case *ast.ChanType:
		return 0, kind.Value, true
	}
	return 0, nil, false
}

// concreteType preserves the defining declaration's identity. Only direct
// concrete definitions are admitted; aliases and named underlying chains refuse.
func (e *expression) concreteType(source sourceExpr) (*ast.TypeSpec, *sourcePackage, bool) {
	var binding *ast.Object
	switch value := source.value.(type) {
	case *ast.Ident:
		binding = value.Obj
		if binding == nil {
			binding = callablePackageBinding(source.pkg, value.Name)
		}
	case *ast.SelectorExpr:
		directory, imported := e.callableImport(source, value)
		if !imported {
			return nil, nil, false
		}
		pkg, retained := e.snapshot.packages[directory]
		if !retained {
			return nil, nil, false
		}
		source.pkg = pkg
		binding = callablePackageBinding(pkg, value.Sel.Name)
	}
	if binding == nil {
		return nil, nil, false
	}
	if binding.Kind != ast.Typ {
		return nil, nil, false
	}
	definition, ok := binding.Decl.(*ast.TypeSpec)
	if !ok {
		return nil, nil, false
	}
	if definition.Assign.IsValid() {
		return nil, nil, false
	}
	if definition.TypeParams.NumFields() != 0 {
		return nil, nil, false
	}
	// Method declarations belong to package scope. A local type with the same
	// spelling cannot borrow the package type's method set.
	if callablePackageBinding(source.pkg, definition.Name.Name) != binding {
		return nil, nil, false
	}
	switch underlying := definition.Type.(type) {
	case *ast.StructType, *ast.ArrayType, *ast.MapType, *ast.ChanType, *ast.FuncType:
		return definition, source.pkg, true
	case *ast.Ident:
		if underlying.Obj != nil {
			return nil, nil, false
		}
		if callablePackageBinding(source.pkg, underlying.Name) != nil {
			return nil, nil, false
		}
		builtin, ok := types.Universe.Lookup(underlying.Name).(*types.TypeName)
		if !ok {
			return nil, nil, false
		}
		if _, scalar := builtin.Type().Underlying().(*types.Basic); scalar {
			return definition, source.pkg, true
		}
	}
	return nil, nil, false
}
