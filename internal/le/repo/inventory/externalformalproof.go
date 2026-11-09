// Design: docs/architecture/plugin/feature-gates.md -- grounded retained argument provenance.
package repoinventory

import (
	"go/ast"
	"go/types"
)

type externalFormalShape uint8

const (
	externalFormalShapeUnknown externalFormalShape = iota
	externalFormalScalar
	externalFormalCallable
	externalFormalReferenceShape
)

// formalShape proves only the reference-carrying representation. A named
// scalar still contributes its own methods through the capability type walker.
func (e *externalEffects) formalShape(source sourceExpr, depth int) (externalFormalShape, string) {
	if depth >= 128 {
		return externalFormalShapeUnknown, "parameter type depth exceeds 128"
	}
	switch datatype := source.value.(type) {
	case *ast.ParenExpr:
		source.value = datatype.X
		return e.formalShape(source, depth+1)
	case *ast.FuncType:
		return externalFormalCallable, ""
	case *ast.StarExpr, *ast.ArrayType, *ast.MapType, *ast.ChanType, *ast.InterfaceType, *ast.StructType:
		return externalFormalReferenceShape, ""
	case *ast.IndexExpr, *ast.IndexListExpr:
		// The existing type walker validates substitutions and their methods.
		// Conservatively include ambient references for instantiated data.
		return externalFormalReferenceShape, ""
	case *ast.Ident:
		if _, parameter := e.typeIdentities.lexicalScope(source).binding(datatype); parameter {
			// A type parameter may be instantiated with any type, a
			// reference-bearing one included, so it takes the conservative
			// shape: an exposed parameter of that type holds the ambient pool.
			return externalFormalReferenceShape, ""
		}
		if datatype.Obj == nil {
			if callablePackageBinding(source.pkg, datatype.Name) == nil {
				if binding, found := types.Universe.Lookup(datatype.Name).(*types.TypeName); found {
					if _, scalar := binding.Type().Underlying().(*types.Basic); scalar {
						return externalFormalScalar, ""
					}
					return externalFormalReferenceShape, ""
				}
			}
		}
	case *ast.SelectorExpr:
		if _, external := e.source.externalImport(source, datatype); external {
			return externalFormalReferenceShape, ""
		}
	default:
		return externalFormalShapeUnknown, "unsupported parameter type representation"
	}
	walk := externalTypeWalk{effects: e}
	_, defining, reason := walk.definition(source)
	if reason != "" {
		return externalFormalShapeUnknown, reason
	}
	return e.formalShape(defining, depth+1)
}

// externalFormalProof is a source equation, not a capability summary. Grounded
// is written only for an actual known source or external parameter authority;
// a temporary empty binding or return summary cannot write it.
type externalFormalProof struct {
	grounded bool
	unknown  string
	depends  map[*externalFormalProof]bool
}

func (proof *externalFormalProof) join(other *externalFormalProof) {
	if other == nil {
		return
	}
	if other.grounded {
		proof.grounded = true
	}
	if proof.unknown == "" {
		proof.unknown = other.unknown
	}
	if proof.depends == nil {
		proof.depends = make(map[*externalFormalProof]bool)
	}
	for dependency := range other.depends {
		proof.depends[dependency] = true
	}
}

type externalFormalProofs struct {
	effects *externalEffects
	objects map[*ast.Object]*externalFormalProof
	returns map[ast.Node]*externalFormalProof
}

// validateFormals runs only after capability stabilization. Every reachable
// obligation must be known, and every forwarding component must reach a real
// source. A grounded cycle does not excuse a different ungrounded outgoing edge.
func (e *externalEffects) validateFormals() {
	proofs := externalFormalProofs{
		effects: e,
		objects: make(map[*ast.Object]*externalFormalProof),
		returns: make(map[ast.Node]*externalFormalProof),
	}
	for object := range e.formalDemand {
		proof := proofs.object(object, 0)
		if reason := externalFormalComplete(proof); reason != "" {
			formal := e.formals[object]
			e.problem(formal.source, "formal "+object.Name+" capability is unknown: "+reason)
		}
	}
}

func (proofs *externalFormalProofs) object(object *ast.Object, depth int) *externalFormalProof {
	if proof, found := proofs.objects[object]; found {
		return proof
	}
	proof := &externalFormalProof{}
	proofs.objects[object] = proof
	if depth >= 128 {
		proof.unknown = "argument provenance depth exceeds 128"
		return proof
	}
	e := proofs.effects
	if formal, parameter := e.formals[object]; parameter {
		shape, reason := e.formalShape(e.types[object], 0)
		if reason != "" {
			proof.unknown = reason
			return proof
		}
		if shape == externalFormalScalar {
			proof.grounded = true
			return proof
		}
		if _, indirect := e.formalIndirect[formal.owner]; indirect {
			proof.unknown = "indirect retained caller provenance is unresolved"
		}
		// A method value binds the receiver, never the parameters.
		if _, unbound := e.formalUnbound[formal.owner]; unbound {
			if !formal.receiver {
				proof.unknown = "indirect retained caller provenance is unresolved"
			}
		}
		if reason := e.formalInvalid[formal.owner]; reason != "" {
			proof.unknown = reason
		}
		proof.grounded = e.formalExternal[object]
	} else if declaration, declared := object.Decl.(*ast.ValueSpec); declared {
		if len(declaration.Values) == 0 {
			// A declared variable without an initializer has a real zero value;
			// this is distinct from a formal with no argument evidence.
			proof.grounded = true
		}
	} else if variable, ranged := e.rangeVariables[object]; ranged {
		// A fresh range variable's source is its collection's elements.
		proof.join(proofs.rangedProof(object, variable, depth+1))
	}
	for _, definition := range e.definitions[object] {
		proof.join(proofs.expression(definition.source, depth+1))
	}
	return proof
}

func (proofs *externalFormalProofs) returned(node ast.Node, source sourceExpr, depth int) *externalFormalProof {
	if proof, found := proofs.returns[node]; found {
		return proof
	}
	proof := &externalFormalProof{}
	proofs.returns[node] = proof
	if depth >= 128 {
		proof.unknown = "returned argument provenance depth exceeds 128"
		return proof
	}
	var body *ast.BlockStmt
	switch function := node.(type) {
	case *ast.FuncDecl:
		body = function.Body
	case *ast.FuncLit:
		body = function.Body
	}
	if body == nil {
		proof.unknown = "retained returned argument body is unavailable"
		return proof
	}
	ast.Inspect(body, func(child ast.Node) bool {
		if _, closure := child.(*ast.FuncLit); closure {
			return false
		}
		statement, returned := child.(*ast.ReturnStmt)
		if !returned {
			return true
		}
		for _, expression := range statement.Results {
			value := source
			value.value = expression
			proof.join(proofs.expression(value, depth+1))
		}
		return false
	})
	return proof
}

func externalFormalComplete(root *externalFormalProof) string {
	pending := []*externalFormalProof{root}
	seen := make(map[*externalFormalProof]bool)
	reason := ""
	for len(pending) != 0 {
		proof := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if seen[proof] {
			continue
		}
		seen[proof] = true
		if proof.unknown != "" {
			if reason == "" {
				reason = proof.unknown
			} else if proof.unknown < reason {
				reason = proof.unknown
			}
		}
		if !externalFormalGrounded(proof) {
			const ungrounded = "no grounded retained argument or external authority"
			if reason == "" {
				reason = ungrounded
			} else if ungrounded < reason {
				reason = ungrounded
			}
		}
		for dependency := range proof.depends {
			pending = append(pending, dependency)
		}
	}
	return reason
}

func externalFormalGrounded(root *externalFormalProof) bool {
	pending := []*externalFormalProof{root}
	seen := make(map[*externalFormalProof]bool)
	for len(pending) != 0 {
		proof := pending[len(pending)-1]
		pending = pending[:len(pending)-1]
		if seen[proof] {
			continue
		}
		seen[proof] = true
		if proof.grounded {
			return true
		}
		for dependency := range proof.depends {
			pending = append(pending, dependency)
		}
	}
	return false
}
