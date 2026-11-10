// Design: docs/architecture/plugin/feature-gates.md -- a method promoted through embedded fields resolves on the embedded field.
// Related: methods.go -- methodReceiver, the depth-zero method resolution that asks for promotion.
package repoinventory

import "go/ast"

// methodPromotionDepth bounds the embedding levels promotedMethod searches. A
// method promoted from deeper than four embedded fields stays unresolved.
const methodPromotionDepth = 4

// embeddedHolder is one struct a promoted method may be declared in: its
// definition, the selector path from x that reaches it, and whether a field
// selector on that path is addressable.
type embeddedHolder struct {
	definition *ast.TypeSpec
	owner      *sourcePackage
	path       sourceExpr
	// addressable is whether path.F is addressable for a field F of the
	// holder. Go specification, "Address operators": "The operand must be
	// addressable, that is, either a variable, pointer indirection, or slice
	// indexing operation; or a field selector of an addressable struct
	// operand". The path dereferences a pointer when x or an embedded field
	// on it is one, so it is addressable from there on.
	addressable bool
}

// promotedMethod resolves selector, which x's type T declares no member for,
// to a method promoted through T's embedded fields. Go specification,
// "Selectors": "For a value x of type T or *T where T is not a pointer or
// interface type, x.f denotes the field or method at the shallowest depth in
// T where there is such an f. If there is not exactly one f with shallowest
// depth, the selector expression is illegal." and "Struct types": "Given a
// struct type S and a type name T, promoted methods are included in the
// method set of the struct as follows: If S contains an embedded field T, the
// method sets of S and *S both include promoted methods with receiver T. The
// method set of *S also includes promoted methods with receiver *T. If S
// contains an embedded field *T, the method sets of S and *S both include
// promoted methods with receiver T or *T."
//
// The search runs one embedding level at a time, so the first level holding
// a member named selector.Sel decides. It refuses when that level holds more
// than one such member, when the member is a field, and when any embedded
// field of a level searched is not a retained, non-generic, non-alias
// concrete type (an embedded interface among them): its member set is
// unknown, so the shallowest depth is unknown. addressable is whether a field
// selector on x is addressable.
//
// The receiver is the embedded field, x.E: its value when the field holds
// the method's receiver type, its address when the method takes *E on an
// embedded E whose path is addressable. A value method on an embedded *E
// stays unresolved, as it does on a direct pointer receiver.
func (e *expression) promotedMethod(receiver sourceExpr, selector *ast.SelectorExpr, definition *ast.TypeSpec, owner *sourcePackage, addressable bool) (methodCall, bool) {
	holders := []embeddedHolder{{definition: definition, owner: owner, path: receiver, addressable: addressable}}
	for range methodPromotionDepth {
		var deeper []embeddedHolder
		var promoted methodCall
		members := 0
		resolved := false
		for _, holder := range holders {
			structure, isStruct := holder.definition.Type.(*ast.StructType)
			if !isStruct {
				continue
			}
			file, declared := declaringFile(holder.owner, holder.definition)
			if !declared {
				return methodCall{}, false
			}
			for _, field := range structure.Fields.List {
				if len(field.Names) != 0 {
					continue
				}
				embedded := sourceExpr{value: field.Type, file: file, pkg: holder.owner}
				pointer, holdsPointer := field.Type.(*ast.StarExpr)
				if holdsPointer {
					embedded.value = pointer.X
				}
				embeddedDefinition, embeddedOwner, retained := e.concreteType(embedded)
				if !retained {
					return methodCall{}, false
				}
				name := embeddedTypeName(field.Type)
				if name == "" {
					return methodCall{}, false
				}
				path := sourceExpr{
					value: &ast.SelectorExpr{X: holder.path.value, Sel: &ast.Ident{NamePos: selector.Sel.NamePos, Name: name}},
					file:  receiver.file,
					pkg:   receiver.pkg,
				}
				methods, known := declaredMethods(embeddedDefinition, embeddedOwner, selector.Sel.Name)
				if !known {
					return methodCall{}, false
				}
				if declaresField(embeddedDefinition, selector.Sel.Name) {
					members++
				}
				members += len(methods)
				if len(methods) == 1 {
					promoted, resolved = promotedReceiver(methods[0], path, holdsPointer, holder.addressable)
				}
				deeper = append(deeper, embeddedHolder{definition: embeddedDefinition, owner: embeddedOwner, path: path, addressable: holdsPointer || holder.addressable})
			}
		}
		if members > 1 {
			return methodCall{}, false
		}
		if members == 1 {
			return promoted, resolved
		}
		if len(deeper) == 0 {
			return methodCall{}, false
		}
		holders = deeper
	}
	return methodCall{}, false
}

// promotedReceiver binds the promoted method function to the embedded field
// path. holdsPointer is whether the field is *E rather than E; addressable is
// whether path is addressable. A bodyless method refuses, as methodReceiver
// refuses one.
func promotedReceiver(function sourceFunction, path sourceExpr, holdsPointer, addressable bool) (methodCall, bool) {
	if function.decl.Body == nil {
		return methodCall{}, false
	}
	_, pointerMethod := function.decl.Recv.List[0].Type.(*ast.StarExpr)
	if pointerMethod == holdsPointer {
		return methodCall{function: function, receiver: path}, true
	}
	// A value method on an embedded *E stays unresolved.
	if holdsPointer {
		return methodCall{}, false
	}
	// Go calls a pointer method on an embedded E through &x.E, which exists
	// only when the path is addressable.
	if !addressable {
		return methodCall{}, false
	}
	return methodCall{function: function, receiver: path, addressed: true}, true
}

// declaredMethods answers the methods owner declares on definition under
// name, in declaration order: a method set holds at most one, so more than
// one is source no compiler accepts. A same-named method whose receiver list
// is not exactly one receiver refuses, because its receiver type is unknown.
func declaredMethods(definition *ast.TypeSpec, owner *sourcePackage, name string) ([]sourceFunction, bool) {
	var methods []sourceFunction
	for _, file := range owner.files {
		for _, declaration := range file.File.Decls {
			function, ok := declaration.(*ast.FuncDecl)
			if !ok {
				continue
			}
			if function.Name.Name != name {
				continue
			}
			if function.Recv == nil {
				continue
			}
			if len(function.Recv.List) != 1 {
				return nil, false
			}
			base := function.Recv.List[0].Type
			if pointer, indirect := base.(*ast.StarExpr); indirect {
				base = pointer.X
			}
			// A generic receiver T[P] names a generic type, which concreteType
			// never admits as definition.
			typeName, ok := base.(*ast.Ident)
			if !ok {
				continue
			}
			binding := typeName.Obj
			if binding == nil {
				binding = callablePackageBinding(owner, typeName.Name)
			}
			if binding == nil {
				continue
			}
			if binding.Decl != definition {
				continue
			}
			methods = append(methods, sourceFunction{decl: function, file: file.File, pkg: owner})
		}
	}
	return methods, true
}

// declaresField reports whether definition is a struct with a field named
// name, an embedded field's implicit name included. Go specification,
// "Struct types": "The unqualified type name acts as the field name."
func declaresField(definition *ast.TypeSpec, name string) bool {
	structure, isStruct := definition.Type.(*ast.StructType)
	if !isStruct {
		return false
	}
	for _, field := range structure.Fields.List {
		if len(field.Names) == 0 {
			if embeddedTypeName(field.Type) == name {
				return true
			}
			continue
		}
		for _, declared := range field.Names {
			if declared.Name == name {
				return true
			}
		}
	}
	return false
}
