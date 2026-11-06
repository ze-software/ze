// Design: docs/architecture/plugin/feature-gates.md -- composed assembly and external certificates.
package repoinventory

// assemblyResult first requires the existing bodyless transfer proof (a
// complete trampoline or a standard-library pull linkname), then applies the
// external capability model to its exact standard-library destination.
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
	transfer, err := e.source.snapshot.certifyAssembly(function, e.assembly)
	if err != nil {
		e.problem(source, "assembly ownership is unresolved: "+err.Error())
		return externalValue{unknown: "assembly ownership is unresolved"}
	}
	// The destination is standard-library code, so the call is modeled as a
	// direct call into that import: its witness, its inputs and the ambient set.
	if err := e.source.witness(transfer.identity); err != nil {
		e.problem(source, err.Error())
		return externalValue{unknown: "assembly external witness is unresolved"}
	}
	var result externalValue
	result.join(inputs)
	result.join(e.amb)
	result.join(externalOrigin(transfer.identity))
	e.expose(result)
	return result
}
