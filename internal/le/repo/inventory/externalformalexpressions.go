// Design: docs/architecture/plugin/feature-gates.md -- source facts for retained arguments.
package repoinventory

import (
	"go/ast"
	"go/token"
	"go/types"
)

// expression proves provenance from syntax and exact declaration identities.
// It never reads the current contents of a growing capability/return summary.
func (proofs *externalFormalProofs) expression(source sourceExpr, depth int) *externalFormalProof {
	if depth >= 128 {
		return &externalFormalProof{unknown: "argument expression depth exceeds 128"}
	}
	if source.value == nil {
		return &externalFormalProof{grounded: true}
	}
	e := proofs.effects
	child := func(expression ast.Expr) *externalFormalProof {
		value := source
		value.value = expression
		return proofs.expression(value, depth+1)
	}
	switch expression := source.value.(type) {
	case *ast.BasicLit, *ast.FuncLit:
		return &externalFormalProof{grounded: true}
	case *ast.Ident:
		if predeclaredIdentifier(source, expression.Name) {
			switch expression.Name {
			case "nil", "true", "false":
				return &externalFormalProof{grounded: true}
			}
		}
		object := e.object(source)
		if object == nil {
			if types.Universe.Lookup(expression.Name) != nil {
				return &externalFormalProof{grounded: true}
			}
			return &externalFormalProof{unknown: "argument identifier ownership is unresolved"}
		}
		if object.Kind == ast.Fun {
			if _, found := source.pkg.functions[expression.Name]; found {
				return &externalFormalProof{grounded: true}
			}
			return &externalFormalProof{unknown: "argument function ownership is unresolved"}
		}
		return externalFormalDependency(proofs.object(object, depth+1))
	case *ast.ParenExpr:
		return child(expression.X)
	case *ast.SelectorExpr:
		if _, external := e.source.externalImport(source, expression); external {
			// Opaque values carry the final ambient capabilities. Their concrete
			// data stays unknown under the already admitted external contract.
			return &externalFormalProof{grounded: true}
		}
		if directory, imported := e.source.snapshot.evaluator().callableImport(source, expression); imported {
			pkg, retained := e.source.snapshot.packages[directory]
			if !retained {
				return &externalFormalProof{unknown: "argument package source is unavailable"}
			}
			if _, function := pkg.functions[expression.Sel.Name]; function {
				return &externalFormalProof{grounded: true}
			}
			object := callablePackageBinding(pkg, expression.Sel.Name)
			if object == nil {
				return &externalFormalProof{unknown: "argument imported declaration is unresolved"}
			}
			return externalFormalDependency(proofs.object(object, depth+1))
		}
		// A retained field or method can only preserve its receiver's owners.
		// The capability evaluator independently refuses an unknown selector.
		return child(expression.X)
	case *ast.CallExpr:
		return proofs.call(source, expression, depth+1)
	case *ast.CompositeLit:
		proof := &externalFormalProof{grounded: true}
		for _, element := range expression.Elts {
			if keyed, ok := element.(*ast.KeyValueExpr); ok {
				if !e.structLiteral(sourceExpr{value: expression.Type, file: source.file, pkg: source.pkg}) {
					proof.join(child(keyed.Key))
				}
				proof.join(child(keyed.Value))
				continue
			}
			proof.join(child(element))
		}
		return proof
	case *ast.UnaryExpr:
		if expression.Op == token.ARROW {
			return &externalFormalProof{unknown: "channel argument provenance is unsupported"}
		}
		return child(expression.X)
	case *ast.BinaryExpr:
		proof := child(expression.X)
		proof.join(child(expression.Y))
		return proof
	case *ast.IndexExpr:
		proof := child(expression.X)
		proof.join(child(expression.Index))
		return proof
	case *ast.SliceExpr:
		proof := child(expression.X)
		proof.join(child(expression.Low))
		proof.join(child(expression.High))
		proof.join(child(expression.Max))
		return proof
	case *ast.StarExpr:
		return child(expression.X)
	case *ast.TypeAssertExpr:
		return child(expression.X)
	case *ast.ArrayType, *ast.MapType, *ast.FuncType, *ast.InterfaceType, *ast.StructType:
		return &externalFormalProof{grounded: true}
	}
	return &externalFormalProof{unknown: "unsupported retained argument provenance"}
}

func (proofs *externalFormalProofs) call(source sourceExpr, call *ast.CallExpr, depth int) *externalFormalProof {
	if depth >= 128 {
		return &externalFormalProof{unknown: "argument call depth exceeds 128"}
	}
	e := proofs.effects
	proof := &externalFormalProof{}
	for _, argument := range call.Args {
		value := source
		value.value = argument
		proof.join(proofs.expression(value, depth+1))
	}
	callee := source
	callee.value = call.Fun
	for {
		parenthesized, ok := callee.value.(*ast.ParenExpr)
		if !ok {
			break
		}
		callee.value = parenthesized.X
	}
	if e.builtin(callee) {
		proof.grounded = true
		return proof
	}
	if e.conversion(callee) {
		proof.grounded = true
		return proof
	}
	if selector, selected := callee.value.(*ast.SelectorExpr); selected {
		if _, external := e.source.externalImport(callee, selector); external {
			proof.grounded = true
			return proof
		}
	}
	node, defining, receiver := e.formalTarget(callee)
	if node == nil {
		proof.unknown = "argument call return provenance is unresolved"
		return proof
	}
	if receiver != nil {
		value := source
		value.value = receiver
		proof.join(proofs.expression(value, depth+1))
	}
	proof.join(externalFormalDependency(proofs.returned(node, defining, depth+1)))
	return proof
}

func externalFormalDependency(proof *externalFormalProof) *externalFormalProof {
	return &externalFormalProof{depends: map[*externalFormalProof]bool{proof: true}}
}
