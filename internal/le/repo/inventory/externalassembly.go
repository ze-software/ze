// Design: docs/architecture/plugin/feature-gates.md -- composed assembly and external certificates.
package repoinventory

import (
	"strconv"
	"strings"
)

// assemblyResult first requires the existing complete trampoline proof, then
// applies the external capability model to its exact imported destination.
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
	index := e.assembly
	if err := e.source.snapshot.certifyAssembly(function, index); err != nil {
		e.problem(source, "assembly ownership is unresolved: "+err.Error())
		return externalValue{unknown: "assembly ownership is unresolved"}
	}
	prefix := index.prefixes[function.pkg]
	// certifyAssembly established one definition and its immutable body bounds.
	definition := index.definitions[prefix+"."+function.decl.Name.Name][0]
	body := e.source.snapshot.blobs[definition.path][definition.bodyStart:definition.bodyEnd]
	destination, err := trampolineDestination(body, definition.path, prefix, definition.macros)
	if err != nil {
		e.problem(source, "assembly destination ownership is unresolved: "+err.Error())
		return externalValue{unknown: "assembly destination ownership is unresolved"}
	}
	separator := strings.LastIndexByte(destination, '.')
	for _, imported := range function.file.Imports {
		identity, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			e.problem(source, "assembly import identity is unresolved")
			return externalValue{unknown: "assembly import identity is unresolved"}
		}
		if linkerPathPrefix(identity) != destination[:separator] {
			continue
		}
		if err := e.source.witness(identity); err != nil {
			e.problem(source, err.Error())
			return externalValue{unknown: "assembly external witness is unresolved"}
		}
		var result externalValue
		result.join(inputs)
		result.join(e.amb)
		result.join(externalOrigin(identity))
		e.expose(result)
		return result
	}
	e.problem(source, "assembly destination import identity is unresolved")
	return externalValue{unknown: "assembly destination import identity is unresolved"}
}
