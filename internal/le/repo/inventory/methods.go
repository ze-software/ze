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
// literal included. A pointer method also resolves on an addressable value
// receiver, which Go calls on its address, and a method promoted through
// embedded fields resolves on the embedded field (methodReceiver).
// Value-method fallback, dynamic interfaces and generic method sets remain
// unresolved. A caller that executes the method MUST also execute the
// receiver expression, because it need not be a name.
func (e *expression) method(source sourceExpr) (sourceFunction, bool) {
	return e.methodAt(source, 0)
}

// methodAt is method at its caller's valueType step count, so a method chain
// stays inside valueType's 64-step bound.
func (e *expression) methodAt(source sourceExpr, depth int) (sourceFunction, bool) {
	call, found := e.methodReceiver(source, depth)
	return call.function, found
}

// methodCall is a resolved method selector x.m: the method's declaration, the
// receiver Go binds to it, and whether Go binds that receiver's address.
type methodCall struct {
	function sourceFunction
	// receiver is x for a method x's type declares itself, and the embedded
	// field x.E (x.E1.E2 one level deeper) for a promoted one, because a
	// promoted method's receiver is the embedded field it was promoted from.
	// The selector is built by promotedMethod: its field name is the embedded
	// field's implicit name, its positions those of the call's selector.
	receiver sourceExpr
	// addressed answers whether Go calls the method on &receiver. Go
	// specification, "Calls": "If x is addressable and &x's method set
	// contains m, x.m() is shorthand for (&x).m()."
	addressed bool
}

// methodReceiver is methodAt that also answers the receiver Go binds, and
// whether it is implicitly addressed. A caller that binds the receiver MUST
// bind that receiver, not x, and when it is addressed MUST bind its address,
// not its value, so a write through the pointer reaches the storage.
//
// A selector T itself declares a member for (a method, a field, or an
// embedded field's implicit name) is resolved at depth zero only, as
// "Selectors" says the shallowest member wins; a selector with no such
// member is looked up among the promoted methods (promotedMethod).
func (e *expression) methodReceiver(source sourceExpr, depth int) (methodCall, bool) {
	selector, ok := source.value.(*ast.SelectorExpr)
	if !ok {
		return methodCall{}, false
	}
	receiver := sourceExpr{value: selector.X, file: source.file, pkg: source.pkg}
	datatype, found := e.valueType(receiver, depth+1)
	if !found {
		return methodCall{}, false
	}
	pointer, indirect := datatype.value.(*ast.StarExpr)
	if indirect {
		datatype.value = pointer.X
	}
	definition, owner, found := e.concreteType(datatype)
	if !found {
		return methodCall{}, false
	}
	methods, known := declaredMethods(definition, owner, selector.Sel.Name)
	if !known {
		return methodCall{}, false
	}
	if len(methods) == 0 {
		if declaresField(definition, selector.Sel.Name) {
			return methodCall{}, false
		}
		addressable := indirect || e.addressable(receiver)
		return e.promotedMethod(receiver, selector, definition, owner, addressable)
	}
	if len(methods) != 1 {
		return methodCall{}, false
	}
	function := methods[0]
	_, receiverIndirect := function.decl.Recv.List[0].Type.(*ast.StarExpr)
	addressed := false
	if receiverIndirect != indirect {
		// A value method on a pointer receiver stays unresolved.
		if indirect {
			return methodCall{}, false
		}
		if !e.addressable(receiver) {
			return methodCall{}, false
		}
		addressed = true
	}
	// Bodyless methods are not covered by the package-function assembly
	// certificate. They must retain unresolved execution ownership.
	if function.decl.Body == nil {
		return methodCall{}, false
	}
	return methodCall{function: function, receiver: receiver, addressed: addressed}, true
}

// addressable admits the receivers Go can take the address of without a type
// check: a variable, in parentheses or not. A field, element or dereference
// receiver stays unresolved. The recursion follows source parentheses only,
// so the source's own nesting bounds it.
func (e *expression) addressable(source sourceExpr) bool {
	switch operand := source.value.(type) {
	case *ast.ParenExpr:
		return e.addressable(sourceExpr{value: operand.X, file: source.file, pkg: source.pkg})
	case *ast.Ident:
		binding := operand.Obj
		if binding == nil {
			binding = callablePackageBinding(source.pkg, operand.Name)
		}
		if binding == nil {
			return false
		}
		return binding.Kind == ast.Var
	}
	return false
}

// parameterType answers the type a declared field has inside its function.
// Go specification, "Passing arguments to ... parameters": "If f is variadic
// with a final parameter p of type ...T, then within f the type of p is
// equivalent to type []T." So a variadic parameter's ...T answers []T, and
// gopls's `for i, arg := range args[1:]` over `args ...string` ranges a
// []string. Every reader of a parameter's type takes it from here, so no
// type walk meets the ...T spelling, which only a signature carries. Any
// other field answers the type it declares.
func parameterType(field *ast.Field) ast.Expr {
	if variadic, ok := field.Type.(*ast.Ellipsis); ok {
		return &ast.ArrayType{Lbrack: variadic.Ellipsis, Elt: variadic.Elt}
	}
	return field.Type
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
		if predeclaredIdentifier(callee, "make") {
			// Go specification, "Making slices, maps and channels": "The
			// built-in function make takes a type T, which must be a slice,
			// map or channel type, or a type parameter, optionally followed by
			// a type-specific list of expressions." It returns a value of
			// type T, so make(map[string]*T) is that map type.
			if len(value.Args) == 0 {
				return sourceExpr{}, false
			}
			return sourceExpr{value: value.Args[0], file: source.file, pkg: source.pkg}, true
		}
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
	case *ast.SelectorExpr:
		return e.fieldType(source, value, depth)
	case *ast.SliceExpr:
		return e.sliceType(source, value, depth)
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
		// A write never changes a variable's static type. Go specification,
		// "Assignment statements": "each value must be assignable to the type
		// of the operand to which it is assigned". So a written variable keeps
		// the type its declaration gives it, and no write check applies here.
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
			return sourceExpr{value: parameterType(declaration), file: source.file, pkg: source.pkg}, true
		case *ast.ValueSpec:
			if declaration.Type != nil {
				return sourceExpr{value: declaration.Type, file: source.file, pkg: source.pkg}, true
			}
			for index, name := range declaration.Names {
				if name.Obj != binding {
					continue
				}
				if len(declaration.Names) != len(declaration.Values) {
					return e.tupleType(source, declaration.Values, len(declaration.Names), index, depth+1)
				}
				return e.valueType(sourceExpr{value: declaration.Values[index], file: source.file, pkg: source.pkg}, depth+1)
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
			for index, left := range declaration.Lhs {
				name, ok := left.(*ast.Ident)
				if !ok {
					continue
				}
				if name.Obj != binding {
					continue
				}
				if len(declaration.Lhs) != len(declaration.Rhs) {
					return e.tupleType(source, declaration.Rhs, len(declaration.Lhs), index, depth+1)
				}
				return e.valueType(sourceExpr{value: declaration.Rhs[index], file: source.file, pkg: source.pkg}, depth+1)
			}
		}
	}
	return sourceExpr{}, false
}

// tupleType answers the static type of the operand at position among
// operands, declared or assigned from right, the one multi-valued call on
// the right-hand side. Go specification, "Assignment statements": "The
// number of operands on the left hand side must match the number of values.
// For instance, if f is a function returning two values, x, y = f() assigns
// the first value to x and the second to y." Operand i has the type of the
// callee's i-th declared result, a result field with several names counting
// once per name. The callee resolves as valueType's call arm resolves one: a
// declared function or a resolved method, never generic. A callee that does
// not resolve, a result count that differs from operands, and the comma-ok
// forms (a type assertion, a map index, a receive) refuse.
func (e *expression) tupleType(source sourceExpr, right []ast.Expr, operands, position, depth int) (sourceExpr, bool) {
	if len(right) != 1 {
		return sourceExpr{}, false
	}
	call, isCall := ast.Unparen(right[0]).(*ast.CallExpr)
	if !isCall {
		return sourceExpr{}, false
	}
	callee := sourceExpr{value: call.Fun, file: source.file, pkg: source.pkg}
	function, resolved := e.declaredFunction(callee)
	if !resolved {
		function, resolved = e.methodAt(callee, depth+1)
	}
	if !resolved {
		return sourceExpr{}, false
	}
	if function.decl.Type.TypeParams.NumFields() != 0 {
		return sourceExpr{}, false
	}
	if function.decl.Type.Results.NumFields() != operands {
		return sourceExpr{}, false
	}
	first := 0
	for _, result := range function.decl.Type.Results.List {
		count := max(len(result.Names), 1)
		if position < first+count {
			return sourceExpr{value: result.Type, file: function.file, pkg: function.pkg}, true
		}
		first += count
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

// fieldType answers the static type of a field selector x.f. Go
// specification, "Selectors": "For a value x of type T or *T where T is not a
// pointer or interface type, x.f denotes the field or method at the shallowest
// depth in T where there is such an f." Only depth zero is answered: a direct
// field of a retained, non-generic struct type, through at most one pointer.
// An imported name, a method, an embedded or promoted field, and a holder
// whose type is unknown or not such a struct refuse.
func (e *expression) fieldType(source sourceExpr, selector *ast.SelectorExpr, depth int) (sourceExpr, bool) {
	if _, imported := e.callableImport(source, selector); imported {
		return sourceExpr{}, false
	}
	holder, found := e.valueType(sourceExpr{value: selector.X, file: source.file, pkg: source.pkg}, depth+1)
	if !found {
		return sourceExpr{}, false
	}
	if pointer, indirect := holder.value.(*ast.StarExpr); indirect {
		holder.value = pointer.X
	}
	definition, owner, found := e.concreteType(holder)
	if !found {
		return sourceExpr{}, false
	}
	structure, isStruct := definition.Type.(*ast.StructType)
	if !isStruct {
		return sourceExpr{}, false
	}
	file, declared := declaringFile(owner, definition)
	if !declared {
		return sourceExpr{}, false
	}
	for _, field := range structure.Fields.List {
		for _, name := range field.Names {
			if name.Name == selector.Sel.Name {
				return sourceExpr{value: field.Type, file: file, pkg: owner}, true
			}
		}
	}
	return sourceExpr{}, false
}

// sliceType answers the static type of a slice expression a[low:high]. Go
// specification, "Slice expressions": "Except for untyped strings, if the
// sliced operand is a string or slice, the result of the slice operation is a
// non-constant value of the same type as the operand." and "If the sliced
// operand is an array, it must be addressable and the result of the slice
// operation is a slice with the same element type as the array." A pointer to
// an array slices the array it points to. An operand whose type is unknown,
// or of any other kind, refuses.
func (e *expression) sliceType(source sourceExpr, slice *ast.SliceExpr, depth int) (sourceExpr, bool) {
	operand, found := e.valueType(sourceExpr{value: slice.X, file: source.file, pkg: source.pkg}, depth+1)
	if !found {
		return sourceExpr{}, false
	}
	shape, found := e.underlyingType(operand, depth+1)
	if !found {
		return sourceExpr{}, false
	}
	if pointer, indirect := shape.value.(*ast.StarExpr); indirect {
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
		return sourceExpr{value: &ast.ArrayType{Lbrack: array.Lbrack, Elt: array.Elt}, file: shape.file, pkg: shape.pkg}, true
	}
	switch kind := shape.value.(type) {
	case *ast.ArrayType:
		if kind.Len == nil {
			return operand, true
		}
		return sourceExpr{value: &ast.ArrayType{Lbrack: kind.Lbrack, Elt: kind.Elt}, file: shape.file, pkg: shape.pkg}, true
	case *ast.Ident:
		basic, isBasic := basicType(shape)
		if !isBasic {
			return sourceExpr{}, false
		}
		if basic.Info()&types.IsString == 0 {
			return sourceExpr{}, false
		}
		return operand, true
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
