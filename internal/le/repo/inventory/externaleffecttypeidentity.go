// Design: docs/architecture/plugin/feature-gates.md -- exact retained type declaration scopes.
package repoinventory

import "go/ast"

func (w *externalTypeWalk) definition(source sourceExpr) (*ast.TypeSpec, sourceExpr, string) {
	var binding *ast.Object
	switch datatype := source.value.(type) {
	case *ast.Ident:
		var reason string
		binding = datatype.Obj
		if binding == nil {
			binding, reason = w.effects.typeIdentities.packageBinding(source.pkg, datatype.Name)
		}
		if reason != "" {
			return nil, sourceExpr{}, reason
		}
	case *ast.SelectorExpr:
		if identity, external := w.effects.source.externalImport(source, datatype); external {
			// Standard-library source is not an analyzer input, so there is no
			// declaration to return. The capability, reflection, interface-root
			// and literal walks model the type as external before asking here;
			// a question only its declaration answers (a range kind, a field
			// type) refuses with this reason.
			return nil, sourceExpr{}, "standard-library type " + identity + "." + datatype.Sel.Name + " has no retained declaration"
		}
		directory, imported := w.effects.source.snapshot.evaluator().callableImport(source, datatype)
		if !imported {
			return nil, sourceExpr{}, "retained type import binding is unresolved"
		}
		pkg, retained := w.effects.source.snapshot.packages[directory]
		if !retained {
			return nil, sourceExpr{}, "retained type source identity is unresolved"
		}
		source.pkg = pkg
		var reason string
		binding, reason = w.effects.typeIdentities.packageBinding(pkg, datatype.Sel.Name)
		if reason != "" {
			return nil, sourceExpr{}, reason
		}
	default:
		return nil, sourceExpr{}, "retained type declaration shape is unresolved"
	}
	if binding == nil {
		return nil, sourceExpr{}, "retained type declaration ownership is unresolved"
	}
	if binding.Kind != ast.Typ {
		return nil, sourceExpr{}, "retained declaration is not a type"
	}
	definition, declared := binding.Decl.(*ast.TypeSpec)
	if !declared {
		return nil, sourceExpr{}, "retained type parameter scope is unresolved"
	}
	owner := w.effects.typeIdentities.owners[externalTypeOwner{pkg: source.pkg, definition: definition}]
	if owner == nil {
		return nil, sourceExpr{}, "retained type defining file is unavailable"
	}
	return definition, sourceExpr{value: definition.Type, file: owner, pkg: source.pkg}, ""
}

func externalPackageTypeBinding(pkg *sourcePackage, name string) (*ast.Object, string) {
	if pkg == nil {
		return nil, "retained type package is unavailable"
	}
	var result *ast.Object
	for _, file := range pkg.files {
		if file.File.Scope == nil {
			continue
		}
		binding := file.File.Scope.Lookup(name)
		if binding == nil {
			continue
		}
		if result != nil {
			if result != binding {
				return nil, "retained type package binding is ambiguous: " + name
			}
		}
		result = binding
	}
	return result, ""
}

func externalLexicalTypeScope(source sourceExpr) *externalTypeScope {
	if source.file == nil {
		return nil
	}
	if source.value == nil {
		return nil
	}
	if !source.value.Pos().IsValid() {
		return nil
	}
	var result *externalTypeScope
	ast.Inspect(source.file, func(node ast.Node) bool {
		if node == nil {
			return false
		}
		if node.Pos() > source.value.Pos() {
			return false
		}
		if node.End() < source.value.End() {
			return false
		}
		var parameters *ast.FieldList
		switch declaration := node.(type) {
		case *ast.TypeSpec:
			parameters = declaration.TypeParams
		case *ast.FuncDecl:
			parameters = declaration.Type.TypeParams
		}
		if parameters.NumFields() != 0 {
			result = externalBindTypeParameters(result, parameters, nil)
		}
		return true
	})
	return result
}

func externalBindTypeParameters(parent *externalTypeScope, parameters *ast.FieldList, arguments []externalTypeArgument) *externalTypeScope {
	scope := &externalTypeScope{parent: parent, parameters: make(map[string]externalTypeBinding)}
	index := 0
	for _, field := range parameters.List {
		for _, name := range field.Names {
			var argument externalTypeArgument
			if index < len(arguments) {
				argument = arguments[index]
			}
			scope.parameters[name.Name] = externalTypeBinding{object: name.Obj, argument: argument}
			index++
		}
	}
	return scope
}

// methods joins pointer and value receiver capabilities. Alias normalization is
// private to this conservative walk and does not widen expression.method.
func (w *externalTypeWalk) methods(definition *ast.TypeSpec, owner sourceExpr) externalValue {
	var result externalValue
	cursor := w.effects.methodCursor(owner.pkg, definition)
	for row, present := cursor.next(); present; row, present = cursor.next() {
		function := row.function
		// A raw row MUST pass this shape guard before resolution.
		if len(function.decl.Recv.List) != 1 {
			result.join(externalValue{unknown: "retained receiver shape is unresolved"})
			continue
		}
		receiver := row.resolution(w, owner.pkg)
		if receiver.reason != "" {
			result.join(externalValue{unknown: receiver.reason})
			continue
		}
		if receiver.definition != definition {
			continue
		}
		if receiver.generic {
			result.join(externalValue{unknown: "generic receiver method capability execution is unsupported"})
			continue
		}
		if function.decl.Body == nil {
			result.join(externalValue{unknown: "bodyless retained method capability execution is unsupported"})
			continue
		}
		result.join(externalCallable(function.decl, sourceExpr{value: function.decl.Name, file: function.file, pkg: owner.pkg}))
	}
	return result
}

func (w *externalTypeWalk) receiverSource(source sourceExpr, depth int) (*ast.TypeSpec, bool, string) {
	generic := false
	seen := make(map[*ast.TypeSpec]bool)
	for depth < 128 {
		switch datatype := source.value.(type) {
		case *ast.StarExpr:
			source.value = datatype.X
			depth++
			continue
		case *ast.ParenExpr:
			source.value = datatype.X
			depth++
			continue
		case *ast.IndexExpr:
			generic = true
			source.value = datatype.X
		case *ast.IndexListExpr:
			generic = true
			source.value = datatype.X
		}
		definition, owner, reason := w.definition(source)
		if reason != "" {
			return nil, false, reason
		}
		if seen[definition] {
			return nil, false, "cyclic retained receiver alias capability"
		}
		seen[definition] = true
		if definition.TypeParams.NumFields() != 0 {
			generic = true
		}
		if !definition.Assign.IsValid() {
			return definition, generic, ""
		}
		source = owner
		depth++
	}
	return nil, false, "receiver capability depth exceeds 128"
}
