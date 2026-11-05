// Design: docs/architecture/plugin/feature-gates.md -- semantic constant kinds, not literal spellings.
// Related: assemblygeneratedtypes.go -- retained named scalar conversions.
package repoinventory

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
)

type generatedKind struct {
	kind  constant.Kind
	typed bool
}

func (reader *generatedAssembly) constantKind(item *generatedConstant) (generatedKind, error) {
	if kind, found := reader.kinds[item]; found {
		return kind, nil
	}
	if reader.kindActive[item] {
		return generatedKind{}, fmt.Errorf("cyclic generated constant %s", item.name)
	}
	if item.value == nil {
		return generatedKind{}, fmt.Errorf("missing generated constant expression %s", item.name)
	}
	reader.kindActive[item] = true
	kind, err := reader.declaredConstantKind(item)
	delete(reader.kindActive, item)
	if err != nil {
		return generatedKind{}, err
	}
	reader.kinds[item] = kind
	return kind, nil
}

func (reader *generatedAssembly) declaredConstantKind(item *generatedConstant) (generatedKind, error) {
	value, err := reader.expressionKind(item.value, item.pkg, item.file)
	if err != nil {
		return generatedKind{}, err
	}
	if item.datatype != nil {
		shape, err := reader.typeShape(item.datatype, item.pkg, item.file, nil)
		if err != nil {
			return generatedKind{}, err
		}
		if shape.scalar == constant.Unknown {
			return generatedKind{}, fmt.Errorf("non-scalar generated constant type %s", item.name)
		}
		return generatedConversionKind(shape.scalar, value)
	}
	return value, nil
}

// expressionKind follows only kind/type rules. It never evaluates magnitudes,
// shifts, layouts or calls; declaration/AST depth bounds every recursive edge.
func (reader *generatedAssembly) expressionKind(expression ast.Expr, pkg *sourcePackage, file *ast.File) (generatedKind, error) {
	switch node := expression.(type) {
	case *ast.ParenExpr:
		return reader.expressionKind(node.X, pkg, file)
	case *ast.BasicLit:
		switch node.Kind {
		case token.INT, token.CHAR:
			return generatedKind{kind: constant.Int}, nil
		case token.FLOAT:
			return generatedKind{kind: constant.Float}, nil
		case token.IMAG:
			return generatedKind{kind: constant.Complex}, nil
		case token.STRING:
			return generatedKind{kind: constant.String}, nil
		}
	case *ast.Ident:
		declarations, err := reader.packageDeclarations(pkg)
		if err != nil {
			return generatedKind{}, err
		}
		if item, found := declarations.constants[node.Name]; found {
			return reader.constantKind(item)
		}
		switch node.Name {
		case "true", "false":
			return generatedKind{kind: constant.Bool}, nil
		case "iota":
			return generatedKind{kind: constant.Int}, nil
		}
		return generatedKind{}, fmt.Errorf("unresolved generated constant %s", node.Name)
	case *ast.SelectorExpr:
		defining, err := reader.importedPackage(pkg, file, node.X)
		if err != nil {
			return generatedKind{}, err
		}
		declarations, err := reader.packageDeclarations(defining)
		if err != nil {
			return generatedKind{}, err
		}
		item, found := declarations.constants[node.Sel.Name]
		if !found {
			return generatedKind{}, fmt.Errorf("unresolved imported generated constant %s", node.Sel.Name)
		}
		return reader.constantKind(item)
	case *ast.UnaryExpr:
		kind, err := reader.expressionKind(node.X, pkg, file)
		if err != nil {
			return generatedKind{}, err
		}
		switch node.Op {
		case token.NOT:
			if kind.kind == constant.Bool {
				return kind, nil
			}
		case token.XOR:
			if kind.kind == constant.Int {
				return kind, nil
			}
		case token.ADD, token.SUB:
			if generatedNumericRank(kind.kind) != 0 {
				return kind, nil
			}
		}
		return generatedKind{}, fmt.Errorf("unsupported generated unary kind %s", node.Op)
	case *ast.BinaryExpr:
		left, err := reader.expressionKind(node.X, pkg, file)
		if err != nil {
			return generatedKind{}, err
		}
		right, err := reader.expressionKind(node.Y, pkg, file)
		if err != nil {
			return generatedKind{}, err
		}
		switch node.Op {
		case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ, token.LAND, token.LOR:
			return generatedKind{kind: constant.Bool}, nil
		}
		return generatedBinaryKind(node.Op, left, right)
	case *ast.CallExpr:
		if len(node.Args) != 1 {
			return generatedKind{}, fmt.Errorf("unsupported generated constant call arity")
		}
		shape, err := reader.typeShape(node.Fun, pkg, file, nil)
		if err != nil {
			return generatedKind{}, err
		}
		if shape.scalar == constant.Unknown {
			return generatedKind{}, fmt.Errorf("unsupported generated constant conversion")
		}
		value, err := reader.expressionKind(node.Args[0], pkg, file)
		if err != nil {
			return generatedKind{}, err
		}
		return generatedConversionKind(shape.scalar, value)
	}
	return generatedKind{}, fmt.Errorf("unsupported generated constant form %T", expression)
}

func generatedConversionKind(target constant.Kind, value generatedKind) (generatedKind, error) {
	accepted := false
	switch target {
	case constant.Bool:
		accepted = value.kind == constant.Bool
	case constant.String:
		switch value.kind {
		case constant.String, constant.Int:
			accepted = true
		}
	case constant.Int, constant.Float, constant.Complex:
		accepted = generatedNumericRank(value.kind) != 0
	}
	if !accepted {
		return generatedKind{}, fmt.Errorf("unsupported generated scalar conversion kind")
	}
	return generatedKind{kind: target, typed: true}, nil
}

func generatedBinaryKind(operator token.Token, left, right generatedKind) (generatedKind, error) {
	if operator == token.ADD {
		if left.kind == constant.String {
			if right.kind == constant.String {
				return generatedKind{kind: constant.String, typed: left.typed || right.typed}, nil
			}
		}
	}
	leftRank, rightRank := generatedNumericRank(left.kind), generatedNumericRank(right.kind)
	if leftRank == 0 {
		return generatedKind{}, fmt.Errorf("unsupported generated left numeric kind")
	}
	if rightRank == 0 {
		return generatedKind{}, fmt.Errorf("unsupported generated right numeric kind")
	}
	switch operator {
	case token.SHL, token.SHR:
		if left.typed {
			if left.kind != constant.Int {
				return generatedKind{}, fmt.Errorf("unsupported generated typed shift kind")
			}
		}
		if left.kind == constant.Complex {
			return generatedKind{}, fmt.Errorf("unsupported generated complex shift kind")
		}
		// A valid untyped constant shift converts its left operand to an
		// integer; even 1.0 << 2 has integer constant kind after that step.
		return generatedKind{kind: constant.Int, typed: left.typed}, nil
	case token.AND, token.OR, token.XOR, token.AND_NOT, token.REM:
		if left.kind != constant.Int {
			return generatedKind{}, fmt.Errorf("unsupported generated integral operator kind")
		}
		if right.kind != constant.Int {
			return generatedKind{}, fmt.Errorf("unsupported generated integral operator kind")
		}
	case token.ADD, token.SUB, token.MUL, token.QUO:
	default:
		return generatedKind{}, fmt.Errorf("unsupported generated binary operator %s", operator)
	}
	if left.typed {
		if right.typed {
			if left.kind != right.kind {
				return generatedKind{}, fmt.Errorf("incompatible generated typed operand kinds")
			}
		}
		return left, nil
	}
	if right.typed {
		return right, nil
	}
	if rightRank > leftRank {
		return right, nil
	}
	return left, nil
}

func generatedNumericRank(kind constant.Kind) int {
	switch kind {
	case constant.Int:
		return 1
	case constant.Float:
		return 2
	case constant.Complex:
		return 3
	}
	return 0
}
