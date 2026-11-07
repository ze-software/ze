// Design: docs/architecture/plugin/feature-gates.md -- direct retained concrete methods own their execution.
// Related: callablealiases.go -- normalized calls; registrations.go -- actual method-body traversal.
package repoinventory

import (
	"go/ast"
	"go/token"
	"go/types"
)

// method resolves direct value methods and explicit single-pointer methods on
// any receiver whose static type valueType derives, a call result or a
// literal included. Receiver forms must match; implicit addressing,
// value-method fallback, dynamic interfaces, promotion and generic method sets
// remain unresolved. A caller that executes the method MUST also execute the
// receiver expression, because it need not be a name.
func (e *expression) method(source sourceExpr) (sourceFunction, bool) {
	return e.methodAt(source, 0)
}

// methodAt is method at its caller's valueType step count, so a method chain
// stays inside valueType's 64-step bound.
func (e *expression) methodAt(source sourceExpr, depth int) (sourceFunction, bool) {
	selector, ok := source.value.(*ast.SelectorExpr)
	if !ok {
		return sourceFunction{}, false
	}
	receiver := sourceExpr{value: selector.X, file: source.file, pkg: source.pkg}
	datatype, found := e.valueType(receiver, depth+1)
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
	if depth >= 64 {
		return sourceExpr{}, false
	}
	switch value := source.value.(type) {
	case *ast.ParenExpr:
		return e.valueType(sourceExpr{value: value.X, file: source.file, pkg: source.pkg}, depth+1)
	case *ast.BasicLit, *ast.BinaryExpr:
		// A constant form has its declared type, or the default type of its
		// kind where a variable is declared from it or a range ranges over it
		// (Go specification, "Constants"): x := 3 is an int, and range "ab"
		// ranges a string.
		return e.constantDefault(source, depth)
	case *ast.CompositeLit:
		return sourceExpr{value: value.Type, file: source.file, pkg: source.pkg}, value.Type != nil
	case *ast.FuncLit:
		return sourceExpr{value: value.Type, file: source.file, pkg: source.pkg}, true
	case *ast.UnaryExpr:
		// The static type of &operand is a pointer to the operand's type, so
		// x := &T{} resolves x.Method exactly as var x *T does. The pointer
		// spans the operand's type, in the file that declares that type. Any
		// other unary operator keeps its operand's type, as constantType says.
		if value.Op != token.AND {
			return e.constantDefault(source, depth)
		}
		operand, found := e.valueType(sourceExpr{value: value.X, file: source.file, pkg: source.pkg}, depth+1)
		if !found {
			return sourceExpr{}, false
		}
		return sourceExpr{value: &ast.StarExpr{Star: operand.value.Pos(), X: operand.value}, file: operand.file, pkg: operand.pkg}, true
	case *ast.CallExpr:
		callee := sourceExpr{value: value.Fun, file: source.file, pkg: source.pkg}
		function, found := e.declaredFunction(callee)
		if !found {
			function, found = e.methodAt(callee, depth+1)
		}
		if !found {
			// Go specification, "Conversions": "An explicit conversion is an
			// expression of the form T(x) where T is a type and x is an
			// expression that can be converted to type T."
			if len(value.Args) != 1 {
				return sourceExpr{}, false
			}
			return e.conversionType(sourceExpr{value: value.Fun, file: source.file, pkg: source.pkg})
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
		if _, constant := predeclaredConstant(source, value.Name); constant {
			return e.constantDefault(source, depth)
		}
		if binding == nil {
			return sourceExpr{}, false
		}
		if binding.Kind == ast.Con {
			return e.constantDefault(source, depth)
		}
		if binding.Kind == ast.Fun {
			// A package function's static type is its signature; a generic one
			// has no type until it is instantiated.
			function, found := source.pkg.functions[value.Name]
			if !found {
				return sourceExpr{}, false
			}
			if function.decl.Type.TypeParams.NumFields() != 0 {
				return sourceExpr{}, false
			}
			return sourceExpr{value: function.decl.Type, file: function.file, pkg: function.pkg}, true
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
						return e.rangedType(source, declaration, ranged, value.Name, depth+1)
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

// conversionType answers T for a conversion T(x) whose callee the source
// names as a type: a predeclared basic type, a retained concrete type, a type
// literal, a pointer to a named type, or a parenthesised form of one. Any
// other callee is no type, so it refuses. It strips at most 64 parentheses.
func (e *expression) conversionType(callee sourceExpr) (sourceExpr, bool) {
	for range 64 {
		switch value := callee.value.(type) {
		case *ast.ParenExpr:
			callee.value = value.X
			continue
		case *ast.ArrayType, *ast.MapType, *ast.ChanType, *ast.FuncType, *ast.StructType, *ast.InterfaceType:
			return callee, true
		case *ast.StarExpr:
			// (*T)(x) converts to a pointer type when T names a type.
			if !e.namesType(sourceExpr{value: value.X, file: callee.file, pkg: callee.pkg}) {
				return sourceExpr{}, false
			}
			return callee, true
		case *ast.Ident, *ast.SelectorExpr:
			if !e.namesType(callee) {
				return sourceExpr{}, false
			}
			return callee, true
		}
		return sourceExpr{}, false
	}
	return sourceExpr{}, false
}

// namesType reports whether source names a predeclared basic type or a
// retained concrete type, as concreteType admits one.
func (e *expression) namesType(source sourceExpr) bool {
	if _, basic := basicType(source); basic {
		return true
	}
	_, _, found := e.concreteType(source)
	return found
}

// rangedType answers the static type of the range variable named variable,
// which declaration declares.
// The operand's type comes from valueType, so a package map declared from a
// composite literal, or a call whose single result type is declared, ranges as
// its declared type; an operand whose type is unknown leaves the variable
// unknown. Only a defining range (:=) declares a variable.
func (e *expression) rangedType(source sourceExpr, declaration *ast.AssignStmt, ranged *ast.UnaryExpr, variable string, depth int) (sourceExpr, bool) {
	if declaration.Tok != token.DEFINE {
		return sourceExpr{}, false
	}
	// One defining assignment cannot declare a name twice, so the name alone
	// finds the variable's position; the blank identifier declares nothing.
	position := -1
	for index, left := range declaration.Lhs {
		name, ok := left.(*ast.Ident)
		if !ok {
			continue
		}
		if name.Name == "_" {
			continue
		}
		if name.Name == variable {
			position = index
		}
	}
	if position < 0 {
		return sourceExpr{}, false
	}
	collection, found := e.valueType(sourceExpr{value: ranged.X, file: source.file, pkg: source.pkg}, depth+1)
	if !found {
		return sourceExpr{}, false
	}
	shape, found := e.underlyingType(collection, depth+1)
	if !found {
		return sourceExpr{}, false
	}
	if pointer, indirect := shape.value.(*ast.StarExpr); indirect {
		// Go specification, "For statements with range clause": a pointer to
		// an array ranges as the array does. No other pointer ranges.
		shape, found = e.underlyingType(sourceExpr{value: pointer.X, file: shape.file, pkg: shape.pkg}, depth+1)
		if !found {
			return sourceExpr{}, false
		}
		array, isArray := shape.value.(*ast.ArrayType)
		if !isArray {
			return sourceExpr{}, false
		}
		if array.Len == nil {
			return sourceExpr{}, false
		}
	}
	return rangedElement(collection, shape, position)
}

// underlyingType resolves a retained named type to the type its declaration
// names, in the file that declares it, following at most 64 edges. A
// predeclared basic type and an unnamed type literal are their own underlying
// type. An alias, a generic type or a named chain refuses, as concreteType does.
func (e *expression) underlyingType(source sourceExpr, depth int) (sourceExpr, bool) {
	for ; depth < 64; depth++ {
		switch datatype := source.value.(type) {
		case *ast.ParenExpr:
			source.value = datatype.X
			continue
		case *ast.Ident:
			if _, basic := basicType(source); basic {
				return source, true
			}
		case *ast.SelectorExpr:
			// The exact retained declaration is resolved below.
		default:
			return source, true
		}
		definition, owner, found := e.concreteType(source)
		if !found {
			return sourceExpr{}, false
		}
		file, found := declaringFile(owner, definition)
		if !found {
			return sourceExpr{}, false
		}
		source = sourceExpr{value: definition.Type, file: file, pkg: owner}
	}
	return sourceExpr{}, false
}

// declaringFile answers the retained file whose package scope declares
// definition, so the imports its type expression names resolve in that file.
func declaringFile(owner *sourcePackage, definition *ast.TypeSpec) (*ast.File, bool) {
	for _, file := range owner.files {
		object := file.File.Scope.Lookup(definition.Name.Name)
		if object == nil {
			continue
		}
		if object.Decl == definition {
			return file.File, true
		}
	}
	return nil, false
}

// rangedElement answers the type of the range variable at position (0 for the
// first, 1 for the second) over a collection whose underlying type is shape,
// by the table in the Go specification, "For statements with range clause":
//
//	array, pointer to array, slice  index int        element
//	string                          index int        rune
//	map                             key              element
//	channel                         element          (none)
//	integer n                       the type of n    (none)
//	func(yield func(K, V) bool)     K                V
//
// named is the operand's own type, because an integer range variable has the
// operand's type and not its underlying one. A position the kind has no
// variable for refuses.
func rangedElement(named, shape sourceExpr, position int) (sourceExpr, bool) {
	at := func(value ast.Expr) (sourceExpr, bool) {
		return sourceExpr{value: value, file: shape.file, pkg: shape.pkg}, true
	}
	switch kind := shape.value.(type) {
	case *ast.ArrayType:
		switch position {
		case 0:
			return universeType(shape, "int")
		case 1:
			return at(kind.Elt)
		}
	case *ast.MapType:
		switch position {
		case 0:
			return at(kind.Key)
		case 1:
			return at(kind.Value)
		}
	case *ast.ChanType:
		if position == 0 {
			return at(kind.Value)
		}
	case *ast.FuncType:
		yielded, found := iteratorYields(kind)
		if !found {
			return sourceExpr{}, false
		}
		if position < len(yielded) {
			return at(yielded[position])
		}
	case *ast.Ident:
		basic, found := basicType(shape)
		if !found {
			return sourceExpr{}, false
		}
		if basic.Info()&types.IsString != 0 {
			switch position {
			case 0:
				return universeType(shape, "int")
			case 1:
				return universeType(shape, "rune")
			}
		}
		if basic.Info()&types.IsInteger != 0 {
			if position == 0 {
				return named, true
			}
		}
	}
	return sourceExpr{}, false
}

// iteratorYields answers the parameter types of yield for a range-over-func
// iterator, func(yield func(...) bool) with at most two yielded values, in
// order. Any other function shape is no iterator.
func iteratorYields(iterator *ast.FuncType) ([]ast.Expr, bool) {
	if iterator.Results.NumFields() != 0 {
		return nil, false
	}
	if iterator.Params.NumFields() != 1 {
		return nil, false
	}
	yield, ok := iterator.Params.List[0].Type.(*ast.FuncType)
	if !ok {
		return nil, false
	}
	if yield.Results.NumFields() != 1 {
		return nil, false
	}
	result, ok := yield.Results.List[0].Type.(*ast.Ident)
	if !ok {
		return nil, false
	}
	if result.Name != "bool" {
		return nil, false
	}
	if result.Obj != nil {
		return nil, false
	}
	var yielded []ast.Expr
	for _, field := range yield.Params.List {
		yielded = append(yielded, field.Type)
		for range len(field.Names) - 1 {
			yielded = append(yielded, field.Type)
		}
	}
	if len(yielded) > 2 {
		return nil, false
	}
	return yielded, true
}

// basicType answers the predeclared basic type an identifier names, when no
// retained declaration shadows it.
func basicType(source sourceExpr) (*types.Basic, bool) {
	name, ok := source.value.(*ast.Ident)
	if !ok {
		return nil, false
	}
	if !predeclaredIdentifier(source, name.Name) {
		return nil, false
	}
	builtin, ok := types.Universe.Lookup(name.Name).(*types.TypeName)
	if !ok {
		return nil, false
	}
	basic, ok := builtin.Type().(*types.Basic)
	return basic, ok
}

// universeType names the predeclared type name at the position of at, in its
// file, so a type walk resolves it as the source spelling would. A retained
// declaration that shadows name refuses rather than answer the wrong type.
func universeType(at sourceExpr, name string) (sourceExpr, bool) {
	result := sourceExpr{value: &ast.Ident{NamePos: at.value.Pos(), Name: name}, file: at.file, pkg: at.pkg}
	if !predeclaredIdentifier(result, name) {
		return sourceExpr{}, false
	}
	return result, true
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
