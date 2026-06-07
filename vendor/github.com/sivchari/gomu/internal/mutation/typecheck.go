package mutation

import (
	"go/ast"
	"go/token"
	"go/types"
)

// TypeChecker validates mutations against type information.
type TypeChecker struct {
	typeInfo *types.Info
}

// NewTypeChecker creates a new type checker.
func NewTypeChecker(typeInfo *types.Info) *TypeChecker {
	return &TypeChecker{typeInfo: typeInfo}
}

// IsValidMutation checks if a mutation is valid for the given node.
//
// When adding a new mutation type, you MUST add a case here explicitly.
// This ensures that type-based validation is considered for every mutation type.
func (tc *TypeChecker) IsValidMutation(node ast.Node, mutant Mutant) bool {
	if tc.typeInfo == nil {
		// No type info available, assume valid
		return true
	}

	switch mutant.Type {
	// Mutation types that require type-based validation
	case arithmeticBinaryType:
		return tc.isValidArithmeticBinaryMutation(node, mutant)
	case arithmeticAssignType:
		return tc.isValidArithmeticAssignMutation(node, mutant)
	case conditionalBinaryType:
		return tc.isValidConditionalMutation(node, mutant)

	// Mutation types that don't require type-based validation
	// - arithmeticIncDecType: ++/-- only applies to numeric types (compiler enforces)
	// - bitwiseBinaryType: bitwise ops only apply to integers (compiler enforces)
	// - bitwiseAssignType: same as above
	// - logicalBinaryType: &&/|| only applies to booleans (compiler enforces)
	case arithmeticIncDecType,
		bitwiseBinaryType,
		bitwiseAssignType,
		logicalBinaryType,
		logicalNotRemovalType,
		returnBoolLiteralType,
		returnZeroValueType,
		branchConditionType,
		errorNilifyType:
		return true

	default:
		// Unknown mutation type - this should not happen.
		// If you see this, add the new mutation type to one of the cases above.
		return true
	}
}

// isValidArithmeticBinaryMutation checks if an arithmetic binary mutation is valid.
func (tc *TypeChecker) isValidArithmeticBinaryMutation(node ast.Node, mutant Mutant) bool {
	expr, ok := node.(*ast.BinaryExpr)
	if !ok {
		return false
	}

	// Get the type of the left operand
	leftType := tc.getExprType(expr.X)
	if leftType == nil {
		return true // Can't determine type, assume valid
	}

	// Check if the mutation is valid for this type
	return tc.isArithmeticOpValidForType(leftType, mutant.Mutated)
}

// isValidArithmeticAssignMutation checks if an arithmetic assignment mutation is valid.
func (tc *TypeChecker) isValidArithmeticAssignMutation(node ast.Node, mutant Mutant) bool {
	stmt, ok := node.(*ast.AssignStmt)
	if !ok || len(stmt.Lhs) == 0 {
		return false
	}

	// Get the type of the left-hand side
	lhsType := tc.getExprType(stmt.Lhs[0])
	if lhsType == nil {
		return true // Can't determine type, assume valid
	}

	// Convert assign operator to binary operator for validation
	binaryOp := tc.assignToBinaryOp(mutant.Mutated)

	return tc.isArithmeticOpValidForType(lhsType, binaryOp)
}

// isValidConditionalMutation checks if a conditional mutation is valid.
func (tc *TypeChecker) isValidConditionalMutation(node ast.Node, mutant Mutant) bool {
	expr, ok := node.(*ast.BinaryExpr)
	if !ok {
		return false
	}

	// Check if this is a nil comparison
	isNilComparison := tc.isNilIdent(expr.X) || tc.isNilIdent(expr.Y)

	// Get the type of the left operand
	leftType := tc.getExprType(expr.X)
	if leftType == nil {
		// Can't determine type
		// For nil comparisons, be conservative: only allow == and !=
		// because ordered comparisons (<, <=, >, >=) are never valid for nil
		if isNilComparison {
			return mutant.Mutated == "==" || mutant.Mutated == "!="
		}

		return true // For non-nil comparisons, assume valid
	}

	// Check if the new operator is valid for this type
	return tc.isComparisonOpValidForType(leftType, mutant.Mutated)
}

// isNilIdent checks if an expression is the nil identifier.
func (tc *TypeChecker) isNilIdent(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)

	return ok && ident.Name == nilIdentName
}

// getExprType returns the type of an expression.
func (tc *TypeChecker) getExprType(expr ast.Expr) types.Type {
	if tc.typeInfo == nil {
		return nil
	}

	// First, try the Types map
	if t := tc.getTypeFromTypesMap(expr); t != nil {
		return t
	}

	// For Ident, try the Uses map
	if t := tc.getTypeFromUsesMapIdent(expr); t != nil {
		return t
	}

	// For SelectorExpr (e.g., fileInfo.TypeInfo), check the selector
	return tc.getTypeFromUsesMapSelector(expr)
}

// getTypeFromTypesMap tries to get type from Types map.
func (tc *TypeChecker) getTypeFromTypesMap(expr ast.Expr) types.Type {
	if tc.typeInfo.Types == nil {
		return nil
	}

	tv, ok := tc.typeInfo.Types[expr]
	if !ok {
		return nil
	}

	if tv.Type != nil && !isInvalidType(tv.Type) {
		return tv.Type
	}

	return nil
}

// getTypeFromUsesMapIdent tries to get type from Uses map for Ident expressions.
func (tc *TypeChecker) getTypeFromUsesMapIdent(expr ast.Expr) types.Type {
	if tc.typeInfo.Uses == nil {
		return nil
	}

	ident, ok := expr.(*ast.Ident)
	if !ok {
		return nil
	}

	obj := tc.typeInfo.Uses[ident]
	if obj == nil {
		return nil
	}

	t := obj.Type()
	if t != nil && !isInvalidType(t) {
		return t
	}

	return nil
}

// getTypeFromUsesMapSelector tries to get type from Uses map for SelectorExpr.
func (tc *TypeChecker) getTypeFromUsesMapSelector(expr ast.Expr) types.Type {
	if tc.typeInfo.Uses == nil {
		return nil
	}

	sel, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return nil
	}

	obj := tc.typeInfo.Uses[sel.Sel]
	if obj == nil {
		return nil
	}

	t := obj.Type()
	if t != nil && !isInvalidType(t) {
		return t
	}

	return nil
}

// isInvalidType checks if a type is invalid (due to type checking errors).
func isInvalidType(t types.Type) bool {
	if t == nil {
		return true
	}

	// Check if it's directly the Invalid basic type
	if basic, ok := t.(*types.Basic); ok {
		return basic.Kind() == types.Invalid
	}

	// Check underlying type for named types
	if basic, ok := t.Underlying().(*types.Basic); ok {
		return basic.Kind() == types.Invalid
	}

	return false
}

// isArithmeticOpValidForType checks if an arithmetic operator is valid for a type.
func (tc *TypeChecker) isArithmeticOpValidForType(t types.Type, op string) bool {
	underlying := t.Underlying()

	switch underlying := underlying.(type) {
	case *types.Basic:
		info := underlying.Info()

		// String type: only + is valid
		if info&types.IsString != 0 {
			return op == "+"
		}

		// Numeric types: all arithmetic operators are valid
		if info&types.IsNumeric != 0 {
			return true
		}

		return false

	default:
		// For non-basic types, arithmetic is generally not valid
		return false
	}
}

// isComparisonOpValidForType checks if a comparison operator is valid for a type.
func (tc *TypeChecker) isComparisonOpValidForType(t types.Type, op string) bool {
	underlying := t.Underlying()

	switch underlying := underlying.(type) {
	case *types.Basic:
		info := underlying.Info()

		// Ordered operators (<, <=, >, >=) require ordered types
		if op == "<" || op == "<=" || op == ">" || op == ">=" {
			return info&types.IsOrdered != 0
		}

		// Equality operators (==, !=) work on comparable types
		// Basic types (numeric, string, boolean) are always comparable
		if op == "==" || op == "!=" {
			return info&types.IsString != 0 || info&types.IsNumeric != 0 || info&types.IsBoolean != 0
		}

		return true

	case *types.Pointer, *types.Chan, *types.Interface:
		// These types only support == and !=
		return op == "==" || op == "!="

	case *types.Slice, *types.Map, *types.Signature:
		// These types can only be compared to nil with == and !=
		return op == "==" || op == "!="

	case *types.Struct, *types.Array:
		// Structs and arrays support == and != if all fields are comparable
		return op == "==" || op == "!="

	default:
		return true
	}
}

// assignToBinaryOp converts an assignment operator string to binary operator string.
func (tc *TypeChecker) assignToBinaryOp(op string) string {
	switch op {
	case "+=":
		return "+"
	case "-=":
		return "-"
	case "*=":
		return "*"
	case "/=":
		return "/"
	case "%=":
		return "%"
	default:
		return op
	}
}

// FilterMutants filters out invalid mutations based on type information.
func FilterMutants(mutants []Mutant, node ast.Node, typeInfo *types.Info) []Mutant {
	if typeInfo == nil {
		return mutants
	}

	tc := NewTypeChecker(typeInfo)
	validMutants := make([]Mutant, 0, len(mutants))

	for _, mutant := range mutants {
		if tc.IsValidMutation(node, mutant) {
			validMutants = append(validMutants, mutant)
		}
	}

	return validMutants
}

// GetNodeTypeInfo returns the type information for a specific node position.
func GetNodeTypeInfo(_ *token.FileSet, typeInfo *types.Info, _, _ int) types.Type {
	if typeInfo == nil || typeInfo.Types == nil {
		return nil
	}

	// This is a simplified approach - in practice, we'd need to walk the AST
	// to find the exact node at the given position
	return nil
}
