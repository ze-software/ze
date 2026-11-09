// Design: docs/architecture/plugin/feature-gates.md -- external callback and result provenance.
package repoinventory

import (
	"go/ast"
	"go/types"
)

func (e *externalEffects) call(source sourceExpr, call *ast.CallExpr, possible bool, depth int) externalValue {
	callee := sourceExpr{value: call.Fun, file: source.file, pkg: source.pkg}
	owner := e.value(callee, possible, depth)
	if e.amb.unknown != "" {
		return externalValue{unknown: e.amb.unknown}
	}
	arguments := make([]externalValue, 0, len(call.Args))
	var inputs externalValue
	for _, argument := range call.Args {
		value := e.value(sourceExpr{value: argument, file: source.file, pkg: source.pkg}, possible, depth)
		if e.amb.unknown != "" {
			return externalValue{unknown: e.amb.unknown}
		}
		arguments = append(arguments, value)
		inputs.join(value)
	}
	// A proved type conversion does not execute callable ownership. Its input
	// and result-type capabilities still flow to any later use of the value.
	if e.conversion(callee) {
		inputs.join(e.typeMethods(callee, depth))
		return inputs
	}
	if owner.unknown != "" {
		e.problem(source, "callable ownership is unknown: "+owner.unknown)
		return externalValue{unknown: owner.unknown}
	}
	if len(owner.origins) != 0 {
		for identity := range owner.origins {
			if identity == "unsafe" {
				e.problem(source, "unsafe reference manufacture is outside the admitted authority")
				continue
			}
			if err := e.source.witness(identity); err != nil {
				e.problem(source, err.Error())
			}
		}
		inputs.join(owner)
		inputs.join(e.amb)
		if inputs.unknown != "" {
			e.problem(source, "registration capability is unknown: "+inputs.unknown)
		}
		e.expose(inputs)
		site := externalSite(e.source.snapshot, call)
		e.operations[site] = source
		if e.amb.unknown != "" {
			// Keep the observed site private without returning its large union
			// to a parent expression after the report must already refuse.
			return externalValue{unknown: e.amb.unknown}
		}
		// The result is flattened: it carries the ambient pool, and the pool
		// holds the root fields of every exposed retained value. Kept whole, a
		// result stored into a retained field (&T{Err: fault}) and exposed
		// again would nest the pool inside its own field one level deeper each
		// round, and the fixpoint would never close. No field name is lost:
		// the result carries an origin, so a selector on it never projects a
		// field (expressionValue) and reads the whole value instead.
		return externalFlat(inputs, 0)
	}
	if len(owner.callables) == 0 {
		if e.builtin(callee) {
			name := callee.value.(*ast.Ident).Name
			switch name {
			case "append":
				if len(arguments) == 0 {
					e.problem(source, "append has no destination ownership")
					return externalValue{unknown: "append destination ownership is unresolved"}
				}
				var appended externalValue
				for _, argument := range arguments[1:] {
					appended.join(argument)
				}
				for object := range arguments[0].locations {
					e.bind(object, appended)
					e.writes[object] = source
				}
			case "new", "make":
				if len(call.Args) == 0 {
					e.problem(source, "allocation type ownership is unresolved")
					return externalValue{unknown: "allocation type ownership is unresolved"}
				}
				inputs.join(e.typeMethods(sourceExpr{value: call.Args[0], file: source.file, pkg: source.pkg}, depth))
			case externalBuiltinLen, externalBuiltinCap:
				// Go specification, "Length and capacity": "The built-in
				// functions len and cap take arguments of various types and
				// return a result of type int." An int holds no callable,
				// location or origin, so the operand's capabilities stay with
				// the operand. The result is still derived from the operand's
				// contents, so its reads remain a metadata dependency, and an
				// unknown operand stays unknown.
				return externalValue{reads: inputs.reads, unknown: inputs.unknown, unknownAt: inputs.unknownAt}
			case "copy", "delete", "clear", "close":
				e.problem(source, "builtin mutation capability transfer is unsupported")
			}
			return inputs
		}
		e.problem(source, "callable ownership is unresolved")
		return externalValue{}
	}
	var result externalValue
	for node, defining := range owner.callables {
		var signature *ast.FuncType
		var assembly *ast.FuncDecl
		switch function := node.(type) {
		case *ast.FuncDecl:
			if externalRegistration(node, defining) {
				if possible {
					e.problem(source, "registration may execute through an escaped capability")
				}
				for object := range inputs.reads {
					e.metadata[object] = true
				}
				continue
			}
			signature = function.Type
			if function.Body == nil {
				assembly = function
			}
		case *ast.FuncLit:
			signature = function.Type
		}
		if signature == nil {
			e.problem(source, "callable signature ownership is unresolved")
			continue
		}
		if signature.TypeParams.NumFields() != 0 {
			e.problem(source, "generic callable ownership is unresolved")
			continue
		}
		if call.Ellipsis.IsValid() {
			e.problem(source, "variadic argument ownership is unresolved")
			continue
		}
		index := 0
		for _, field := range signature.Params.List {
			if _, variadic := field.Type.(*ast.Ellipsis); variadic {
				e.problem(source, "variadic parameter ownership is unresolved")
				continue
			}
			count := len(field.Names)
			if count == 0 {
				count = 1
			}
			for slot := range count {
				if index < len(arguments) {
					if slot < len(field.Names) {
						e.bind(field.Names[slot].Obj, arguments[index])
					}
				} else {
					e.problem(source, "argument slot ownership is unresolved")
				}
				index++
			}
		}
		if index != len(arguments) {
			e.problem(source, "argument arity is unresolved")
		}
		if assembly != nil {
			returned := e.assemblyResult(sourceFunction{decl: assembly, file: defining.file, pkg: defining.pkg}, inputs)
			if e.amb.unknown != "" {
				return externalValue{unknown: e.amb.unknown}
			}
			result.join(returned)
			result.reflection.join(e.returnedReflection(signature, defining, depth+1))
			continue
		}
		e.body(node, defining, possible)
		result.join(e.returns[node])
		result.reflection.join(e.returnedReflection(signature, defining, depth+1))
	}
	return result
}

// externalRegistration identifies the package-level semantic sink by its
// retained declaration. A receiver method with the same name is an ordinary
// callable whose body still needs analysis.
func externalRegistration(node ast.Node, source sourceExpr) bool {
	function, ok := node.(*ast.FuncDecl)
	if !ok {
		return false
	}
	if function.Recv != nil {
		return false
	}
	declared, found := source.pkg.functions[function.Name.Name]
	if !found {
		return false
	}
	if declared.decl != function {
		return false
	}
	return registrationAPI(source.pkg.dir, function.Name.Name)
}

func (e *externalEffects) builtin(source sourceExpr) bool {
	name, ok := source.value.(*ast.Ident)
	if !ok {
		return false
	}
	if e.object(source) != nil {
		return false
	}
	_, builtin := types.Universe.Lookup(name.Name).(*types.Builtin)
	return builtin
}

func (e *externalEffects) conversion(source sourceExpr) bool {
	switch source.value.(type) {
	case *ast.ArrayType, *ast.MapType, *ast.ChanType, *ast.FuncType, *ast.InterfaceType, *ast.StructType:
		return true
	}
	if object := e.object(source); object != nil {
		return object.Kind == ast.Typ
	}
	if name, ok := source.value.(*ast.Ident); ok {
		_, datatype := types.Universe.Lookup(name.Name).(*types.TypeName)
		return datatype
	}
	return false
}
