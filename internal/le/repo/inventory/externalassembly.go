// Design: docs/architecture/plugin/feature-gates.md -- composed assembly and external certificates.
package repoinventory

import "go/ast"

// assemblyResult first requires a bodyless proof (a complete trampoline, a
// standard-library pull linkname, or a complete leaf body), then applies the
// external capability model to the call: its inputs and the ambient set are
// exposed, a transfer adds its exact standard-library destination, and a
// leaf adds the same-package variables it reads.
// The ordinary collector still certifies this retained call independently.
func (e *externalEffects) assemblyResult(function sourceFunction, inputs externalValue) externalValue {
	source := sourceExpr{value: function.decl.Name, file: function.file, pkg: function.pkg}
	if e.assembly == nil {
		if e.assemblyErr == nil {
			e.assembly, e.assemblyErr = e.source.snapshot.assemblyEvidence()
		}
	}
	if e.assemblyErr != nil {
		e.problem(source, "assembly ownership is unresolved: "+e.assemblyErr.Error())
		return externalValue{unknown: "assembly ownership is unresolved"}
	}
	proof, err := e.source.snapshot.certifyAssembly(function, e.assembly)
	if err != nil {
		e.problem(source, "assembly ownership is unresolved: "+err.Error())
		return externalValue{unknown: "assembly ownership is unresolved"}
	}
	var result externalValue
	result.join(inputs)
	result.join(e.amb)
	switch proof := proof.(type) {
	case bodylessTransfer:
		// The destination is standard-library code, so the call is modeled as
		// a direct call into that import: its witness and its origin as well.
		if err := e.source.witness(proof.identity); err != nil {
			e.problem(source, err.Error())
			return externalValue{unknown: "assembly external witness is unresolved"}
		}
		result.join(externalOrigin(proof.identity))
	case bodylessLeaf:
		// A leaf runs no code outside its own body, so it has no import route
		// to witness and no external origin. Its arguments and the ambient set
		// are still exposed exactly as for an external call: the trust contract
		// lets it act on the memory its arguments point to. Each same-package
		// variable it reads is an input in the same way, because the leaf can
		// copy the value into that memory or its results (leafVariables).
		for _, name := range proof.variables {
			object := callablePackageBinding(function.pkg, name)
			if object == nil {
				e.problem(source, "assembly leaf reads variable "+name+", which no retained Go file declares")
				return externalValue{unknown: "assembly leaf variable is unresolved"}
			}
			if object.Kind != ast.Var {
				e.problem(source, "assembly leaf reads "+name+", which is not a Go variable")
				return externalValue{unknown: "assembly leaf variable is unresolved"}
			}
			value := e.readObject(object)
			if value.unknown != "" {
				e.problem(source, "assembly leaf variable "+name+" is unresolved: "+value.unknown)
				return externalValue{unknown: "assembly leaf variable is unresolved"}
			}
			result.join(value)
		}
	case nil:
		panic("BUG: certifyAssembly returned neither a proof nor an error")
	}
	e.expose(result)
	return result
}
