// Design: docs/architecture/plugin/feature-gates.md -- a constant has the type the Go specification gives it.
// Related: methods.go -- valueType, which routes every constant form here.
package repoinventory

import (
	"go/ast"
	"go/token"
	"go/types"
	"slices"
)

// constantKind says whether an expression's type is a declared one, and for an
// untyped constant which kind it is. The untyped numeric kinds are declared in
// the order the Go specification ranks them ("Constant expressions": integer,
// rune, floating-point, complex), so an operation on two untyped numeric
// operands takes the later of the two. The zero value is never produced.
type constantKind uint8

const (
	constantUnspecified constantKind = iota
	constantTyped
	untypedBool
	untypedString
	untypedInt
	untypedRune
	untypedFloat
	untypedComplex
)

// numeric reports whether kind is an untyped numeric constant kind.
func (kind constantKind) numeric() bool {
	return kind >= untypedInt
}

// defaultTypeName names the type an untyped constant of kind takes where a
// variable is declared from it or a range ranges over it (Go specification,
// "Constants": "The default type of an untyped constant is bool, rune, int,
// float64, complex128, or string respectively, depending on whether it is a
// boolean, rune, integer, floating-point, complex, or string constant."). A
// typed or unspecified kind has no default type.
func defaultTypeName(kind constantKind) (string, bool) {
	switch kind {
	case untypedBool:
		return types.Typ[types.Bool].Name(), true
	case untypedString:
		return types.Typ[types.String].Name(), true
	case untypedInt:
		return types.Typ[types.Int].Name(), true
	case untypedRune:
		// types.Typ names rune by its alias target, int32; the source spells
		// the default type rune.
		return "rune", true
	case untypedFloat:
		return types.Typ[types.Float64].Name(), true
	case untypedComplex:
		return types.Typ[types.Complex128].Name(), true
	case constantUnspecified, constantTyped:
		return "", false
	}
	return "", false
}

// untypedConstant answers the default type of an untyped constant of kind,
// spelled at the position of at.
func untypedConstant(at sourceExpr, kind constantKind) (sourceExpr, constantKind, bool) {
	name, found := defaultTypeName(kind)
	if !found {
		return sourceExpr{}, constantUnspecified, false
	}
	datatype, found := universeType(at, name)
	if !found {
		return sourceExpr{}, constantUnspecified, false
	}
	return datatype, kind, true
}

// predeclaredConstant answers the kind of a predeclared constant the source
// names when no declaration shadows it, from the universe scope: true and
// false are untyped booleans, and iota is an untyped integer (Go
// specification, "Iota"). nil is no constant and has no default type. Only
// the universe constants' two kinds are listed, so the switch is partial.
func predeclaredConstant(source sourceExpr, name string) (constantKind, bool) {
	if !predeclaredIdentifier(source, name) {
		return constantUnspecified, false
	}
	constant, ok := types.Universe.Lookup(name).(*types.Const)
	if !ok {
		return constantUnspecified, false
	}
	basic, ok := constant.Type().(*types.Basic)
	if !ok {
		return constantUnspecified, false
	}
	// The universe declares constants of these two kinds alone.
	//exhaustive:ignore
	switch basic.Kind() {
	case types.UntypedBool:
		return untypedBool, true
	case types.UntypedInt:
		return untypedInt, true
	}
	return constantUnspecified, false
}

// constantDefault answers valueType for a constant form: a declared type
// where the constant has one, otherwise the default type of its kind.
func (e *expression) constantDefault(source sourceExpr, depth int) (sourceExpr, bool) {
	datatype, _, found := e.constantType(source, depth)
	return datatype, found
}

// constantType answers the type of an expression together with its constant
// kind, so an untyped operand gives way to a typed one as the Go specification
// requires. A form that is not a constant form is typed through valueType and
// counts as typed. It follows at most 64 declaration edges.
func (e *expression) constantType(source sourceExpr, depth int) (sourceExpr, constantKind, bool) {
	if depth >= 64 {
		return sourceExpr{}, constantUnspecified, false
	}
	switch value := source.value.(type) {
	case *ast.ParenExpr:
		return e.constantType(sourceExpr{value: value.X, file: source.file, pkg: source.pkg}, depth+1)
	case *ast.BasicLit:
		// A literal is an untyped constant of the literal's kind.
		//exhaustive:ignore
		switch value.Kind {
		case token.INT:
			return untypedConstant(source, untypedInt)
		case token.CHAR:
			return untypedConstant(source, untypedRune)
		case token.STRING:
			return untypedConstant(source, untypedString)
		case token.FLOAT:
			return untypedConstant(source, untypedFloat)
		case token.IMAG:
			return untypedConstant(source, untypedComplex)
		}
		return sourceExpr{}, constantUnspecified, false
	case *ast.UnaryExpr:
		// Go specification, "Constant expressions": "Any other operation on
		// untyped constants results in an untyped constant of the same kind",
		// so +x, -x, ^x and !x keep the operand's type and kind. &x is no
		// constant, and valueType owns it; a receive is not typed here.
		//exhaustive:ignore
		switch value.Op {
		case token.AND:
			datatype, found := e.valueType(source, depth+1)
			return datatype, constantTyped, found
		case token.ADD, token.SUB, token.XOR, token.NOT:
			return e.constantType(sourceExpr{value: value.X, file: source.file, pkg: source.pkg}, depth+1)
		}
		return sourceExpr{}, constantUnspecified, false
	case *ast.BinaryExpr:
		return e.binaryType(source, value, depth+1)
	case *ast.Ident:
		if kind, constant := predeclaredConstant(source, value.Name); constant {
			return untypedConstant(source, kind)
		}
		binding := value.Obj
		if binding == nil {
			binding = callablePackageBinding(source.pkg, value.Name)
		}
		if binding != nil {
			if binding.Kind == ast.Con {
				return e.declaredConstantType(source, value, binding, depth+1)
			}
		}
	}
	datatype, found := e.valueType(source, depth+1)
	return datatype, constantTyped, found
}

// binaryType answers the type of a binary operation by the Go specification's
// rules for operators and constant expressions. An operand whose type is
// unknown leaves the result unknown, except a comparison, whose result is an
// untyped boolean whatever it compares.
func (e *expression) binaryType(source sourceExpr, operation *ast.BinaryExpr, depth int) (sourceExpr, constantKind, bool) {
	// Only a comparison is typed before its operands.
	//exhaustive:ignore
	switch operation.Op {
	case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:
		// Go specification, "Comparison operators": "Comparison operators
		// compare two operands and yield an untyped boolean value."
		return untypedConstant(source, untypedBool)
	}
	left, leftKind, found := e.constantType(sourceExpr{value: operation.X, file: source.file, pkg: source.pkg}, depth)
	if !found {
		return sourceExpr{}, constantUnspecified, false
	}
	// Only a shift takes its type from the left operand alone.
	//exhaustive:ignore
	switch operation.Op {
	case token.SHL, token.SHR:
		// Go specification, "Constant expressions": "If the left operand of a
		// constant shift expression is an untyped constant, the result is an
		// integer constant; otherwise it is a constant of the same type as the
		// left operand, which must be of integer type." An untyped rune stays
		// a rune, as go/types keeps it.
		if leftKind == untypedFloat {
			return untypedConstant(source, untypedInt)
		}
		if leftKind == untypedComplex {
			return untypedConstant(source, untypedInt)
		}
		return left, leftKind, true
	}
	right, rightKind, found := e.constantType(sourceExpr{value: operation.Y, file: source.file, pkg: source.pkg}, depth)
	if !found {
		return sourceExpr{}, constantUnspecified, false
	}
	// Go specification, "Operators": "if one operand is an untyped constant
	// and the other operand is not, the constant is implicitly converted to
	// the type of the other operand."
	if leftKind == constantTyped {
		return left, constantTyped, true
	}
	if rightKind == constantTyped {
		return right, constantTyped, true
	}
	if leftKind == rightKind {
		return untypedConstant(source, leftKind)
	}
	// Go specification, "Constant expressions": "If the untyped operands of a
	// binary operation (other than a shift) are of different kinds, the result
	// is of the operand's kind that appears later in this list: integer, rune,
	// floating-point, complex."
	if !leftKind.numeric() {
		return sourceExpr{}, constantUnspecified, false
	}
	if !rightKind.numeric() {
		return sourceExpr{}, constantUnspecified, false
	}
	return untypedConstant(source, max(leftKind, rightKind))
}

// declaredConstantType answers the type of the constant name, which binding
// declares: the declared type of its ConstSpec where it has one, otherwise the
// type of its expression. A ConstSpec with no expression list repeats the
// first preceding non-empty list of its group, and that list's type
// (Go specification, "Constant declarations").
func (e *expression) declaredConstantType(source sourceExpr, name *ast.Ident, binding *ast.Object, depth int) (sourceExpr, constantKind, bool) {
	declaration, ok := binding.Decl.(*ast.ValueSpec)
	if !ok {
		return sourceExpr{}, constantUnspecified, false
	}
	// A package constant may live in another retained file; its expression
	// names the imports of that file, not the caller's.
	for _, file := range source.pkg.files {
		if file.File.Scope.Lookup(name.Name) == binding {
			source.file = file.File
			break
		}
	}
	index := slices.IndexFunc(declaration.Names, func(declared *ast.Ident) bool { return declared.Obj == binding })
	if index < 0 {
		return sourceExpr{}, constantUnspecified, false
	}
	spec, found := constantExpressionList(source.file, declaration)
	if !found {
		return sourceExpr{}, constantUnspecified, false
	}
	if spec.Type != nil {
		return sourceExpr{value: spec.Type, file: source.file, pkg: source.pkg}, constantTyped, true
	}
	if len(spec.Values) != len(declaration.Names) {
		return sourceExpr{}, constantUnspecified, false
	}
	return e.constantType(sourceExpr{value: spec.Values[index], file: source.file, pkg: source.pkg}, depth)
}

// constantExpressionList answers the ConstSpec whose expression list and type
// apply to spec: spec itself when it has an expression list, otherwise the
// first preceding spec of its group that has one (Go specification,
// "Constant declarations": "Such an empty list is equivalent to the textual
// substitution of the first preceding non-empty expression list and its
// type if any."). The group is searched for in file, at package level first.
func constantExpressionList(file *ast.File, spec *ast.ValueSpec) (*ast.ValueSpec, bool) {
	if len(spec.Values) != 0 {
		return spec, true
	}
	group, found := constantGroup(file, spec)
	if !found {
		return nil, false
	}
	position := slices.Index(group.Specs, ast.Spec(spec))
	for index := position - 1; index >= 0; index-- {
		preceding, ok := group.Specs[index].(*ast.ValueSpec)
		if !ok {
			return nil, false
		}
		if len(preceding.Values) != 0 {
			return preceding, true
		}
	}
	return nil, false
}

// constantGroup answers the const declaration in file that holds spec. A
// package constant's group is a top-level declaration; a local constant's
// group sits in a function body, which is searched only when the top level
// does not hold it.
func constantGroup(file *ast.File, spec *ast.ValueSpec) (*ast.GenDecl, bool) {
	holds := func(node ast.Node) (*ast.GenDecl, bool) {
		declaration, ok := node.(*ast.GenDecl)
		if !ok {
			return nil, false
		}
		if declaration.Tok != token.CONST {
			return nil, false
		}
		return declaration, slices.Contains(declaration.Specs, ast.Spec(spec))
	}
	for _, declaration := range file.Decls {
		if group, found := holds(declaration); found {
			return group, true
		}
	}
	var group *ast.GenDecl
	ast.Inspect(file, func(node ast.Node) bool {
		if group != nil {
			return false
		}
		if candidate, found := holds(node); found {
			group = candidate
			return false
		}
		return true
	})
	return group, group != nil
}
