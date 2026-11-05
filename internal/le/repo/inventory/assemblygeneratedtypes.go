// Design: docs/architecture/plugin/feature-gates.md -- generated struct names without layouts.
// Related: assemblygenerated.go -- package-level declaration and emission order.
package repoinventory

import (
	"fmt"
	"go/ast"
	"go/constant"
)

type generatedShape struct {
	scalar     constant.Kind
	structure  *ast.StructType
	parameters int
}

func (reader *generatedAssembly) declaredShape(item *generatedType) (generatedShape, error) {
	if shape, found := reader.shapes[item]; found {
		return shape, nil
	}
	if reader.shapeActive[item] {
		return generatedShape{}, fmt.Errorf("cyclic generated type %s", item.name)
	}
	reader.shapeActive[item] = true
	shape, err := reader.typeShape(item.spec.Type, item.pkg, item.file, item.spec.TypeParams)
	delete(reader.shapeActive, item)
	if err != nil {
		return generatedShape{}, err
	}
	shape.parameters = item.spec.TypeParams.NumFields()
	reader.shapes[item] = shape
	return shape, nil
}

func (reader *generatedAssembly) typeShape(expression ast.Expr, pkg *sourcePackage, file *ast.File, parameters *ast.FieldList) (generatedShape, error) {
	switch node := expression.(type) {
	case *ast.ParenExpr:
		return reader.typeShape(node.X, pkg, file, parameters)
	case *ast.StructType:
		return generatedShape{structure: node}, nil
	case *ast.InterfaceType, *ast.ArrayType, *ast.MapType, *ast.ChanType, *ast.FuncType, *ast.StarExpr:
		return generatedShape{}, nil
	case *ast.Ident:
		if generatedParameterNamed(parameters, node.Name) {
			return generatedShape{}, fmt.Errorf("parameter-dependent generated type %s", node.Name)
		}
		declarations, err := reader.packageDeclarations(pkg)
		if err != nil {
			return generatedShape{}, err
		}
		if item, found := declarations.types[node.Name]; found {
			return reader.declaredShape(item)
		}
		kind := generatedScalarKind(node.Name)
		if kind != constant.Unknown {
			return generatedShape{scalar: kind}, nil
		}
		return generatedShape{}, fmt.Errorf("unresolved generated type %s", node.Name)
	case *ast.SelectorExpr:
		if qualifier, ok := node.X.(*ast.Ident); ok {
			if generatedParameterNamed(parameters, qualifier.Name) {
				return generatedShape{}, fmt.Errorf("parameter-dependent generated qualifier %s", qualifier.Name)
			}
		}
		defining, err := reader.importedPackage(pkg, file, node.X)
		if err != nil {
			return generatedShape{}, err
		}
		declarations, err := reader.packageDeclarations(defining)
		if err != nil {
			return generatedShape{}, err
		}
		item, found := declarations.types[node.Sel.Name]
		if !found {
			return generatedShape{}, fmt.Errorf("unresolved imported generated type %s", node.Sel.Name)
		}
		return reader.declaredShape(item)
	case *ast.IndexExpr:
		return reader.instantiatedShape(node.X, 1, pkg, file, parameters)
	case *ast.IndexListExpr:
		return reader.instantiatedShape(node.X, len(node.Indices), pkg, file, parameters)
	}
	return generatedShape{}, fmt.Errorf("unsupported generated type form %T", expression)
}

// Parameter names take precedence over package declarations AND predeclared
// types. The caller supplies the defining declaration's lexical scope, never a
// caller's argument scope. Field types inside a direct struct need no lookup.
func generatedParameterNamed(parameters *ast.FieldList, name string) bool {
	if parameters == nil {
		return false
	}
	for _, field := range parameters.List {
		for _, parameter := range field.Names {
			if parameter.Name == name {
				return true
			}
		}
	}
	return false
}

// Field identities do not depend on layout or the argument values of a direct
// generic struct. Parameter-dependent underlying shapes remain unresolved.
func (reader *generatedAssembly) instantiatedShape(base ast.Expr, arguments int, pkg *sourcePackage, file *ast.File, parameters *ast.FieldList) (generatedShape, error) {
	shape, err := reader.typeShape(base, pkg, file, parameters)
	if err != nil {
		return generatedShape{}, err
	}
	if shape.parameters == 0 {
		return generatedShape{}, fmt.Errorf("generated instantiation has no retained type parameters")
	}
	if shape.parameters != arguments {
		return generatedShape{}, fmt.Errorf("generated instantiation has unsupported argument count")
	}
	shape.parameters = 0
	return shape, nil
}

func generatedEmbeddedName(expression ast.Expr) (string, error) {
	// Every step removes an AST wrapper; source depth bounds traversal.
	for {
		switch node := expression.(type) {
		case *ast.Ident:
			return node.Name, nil
		case *ast.SelectorExpr:
			return node.Sel.Name, nil
		case *ast.StarExpr:
			expression = node.X
		case *ast.IndexExpr:
			expression = node.X
		case *ast.IndexListExpr:
			expression = node.X
		default:
			return "", fmt.Errorf("unsupported generated embedded field %T", expression)
		}
	}
}

func generatedScalarKind(name string) constant.Kind {
	switch name {
	case "bool":
		return constant.Bool
	case "string":
		return constant.String
	case "int", "int8", "int16", "int32", "int64", "uint", "uint8", "uint16", "uint32", "uint64", "uintptr", "byte", "rune":
		return constant.Int
	case "float32", "float64":
		return constant.Float
	case "complex64", "complex128":
		return constant.Complex
	}
	return constant.Unknown
}
