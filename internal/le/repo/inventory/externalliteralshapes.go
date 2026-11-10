// Design: docs/architecture/plugin/feature-gates.md -- positional literals preserve field capabilities.
package repoinventory

import (
	"go/ast"
	"go/types"
)

type externalLiteralKind uint8

const (
	externalLiteralUnresolved externalLiteralKind = iota
	externalLiteralCollection
	externalLiteralStruct
	// externalLiteralOpaque is a standard-library type: its declaration is not
	// an analyzer input, so neither its fields nor its key kind are known.
	externalLiteralOpaque
)

// externalElidedLiteralUnresolved is the refusal for an element literal that
// elides its type where no enclosing literal states one the analyzer can read.
const externalElidedLiteralUnresolved = "nonempty elided literal layout is unresolved"

type externalLiteralLayout struct {
	kind   externalLiteralKind
	fields []string
	types  map[string]sourceExpr
	// key and element are a collection's key and element types, in the scope
	// that states them, for an element literal that elides its own type. A nil
	// value is a type that names a type parameter the walk cannot substitute.
	key, element sourceExpr
}

// externalLiteralType is the type a composite literal constructs: the one
// written on it, or the one its enclosing literal states when it elides it.
type externalLiteralType struct {
	datatype sourceExpr
	// opaque is set when a standard-library enclosing type states the type:
	// that declaration is not an analyzer input, so no layout is retained.
	opaque bool
	// pointer is set when the stated type is *T, so the literal is &T{...}.
	pointer bool
}

// elidedLiteralType answers the type of an element literal, or with mapKey a
// map key literal, that elides its type inside an enclosing literal of the
// given layout. Go spec, Composite literals: "Within a composite literal of
// array, slice, or map type T, elements or map keys that are themselves
// composite literals may elide the respective literal type if it is identical
// to the element or key type of T. Similarly, elements or keys that are
// addresses of composite literals may elide the &T when the element or key
// type is *T."
func elidedLiteralType(enclosing externalLiteralLayout, mapKey bool) (externalLiteralType, bool) {
	switch enclosing.kind {
	case externalLiteralOpaque:
		return externalLiteralType{opaque: true}, true
	case externalLiteralCollection:
		stated := enclosing.element
		if mapKey {
			stated = enclosing.key
		}
		if stated.value == nil {
			return externalLiteralType{}, false
		}
		if pointer, address := ast.Unparen(stated.value).(*ast.StarExpr); address {
			stated.value = pointer.X
			return externalLiteralType{datatype: stated, pointer: true}, true
		}
		return externalLiteralType{datatype: stated}, true
	case externalLiteralUnresolved, externalLiteralStruct:
		// A struct literal's values never elide their type in valid Go.
		return externalLiteralType{}, false
	default:
		panic("BUG: literal layout kind is not declared")
	}
}

// literalTypeLayout is literalLayout for a written or stated literal type.
func (e *externalEffects) literalTypeLayout(stated externalLiteralType, depth int, positional bool) (externalLiteralLayout, string) {
	if stated.opaque {
		return externalLiteralLayout{kind: externalLiteralOpaque}, ""
	}
	return e.literalLayout(stated.datatype, depth, positional)
}

// member reports whether key names a field rather than executing as a value.
// A standard-library literal's key is a field name only when it is an
// identifier that binds nothing in scope (New in sync.Pool{New: f}); any key
// that could be a value is evaluated as one, which only widens the union.
func (layout externalLiteralLayout) member(pkg *sourcePackage, key ast.Expr) bool {
	switch layout.kind {
	case externalLiteralStruct:
		return true
	case externalLiteralUnresolved, externalLiteralCollection:
		return false
	case externalLiteralOpaque:
		name, identifier := key.(*ast.Ident)
		if !identifier {
			return false
		}
		if name.Obj != nil {
			return false
		}
		if callablePackageBinding(pkg, name.Name) != nil {
			return false
		}
		return types.Universe.Lookup(name.Name) == nil
	default:
		panic("BUG: literal layout kind is not declared")
	}
}

// literalLayout follows exact retained aliases and defined types. Field slots
// retain declaration order, including each name in a grouped field declaration.
// Generic collections need no element substitution to identify their layout;
// a collection's key and element types substitute a type parameter only where
// the type is that parameter alone. Positional generic structs and embedded
// fields remain explicit uncertainty.
func (e *externalEffects) literalLayout(source sourceExpr, depth int, positional bool) (externalLiteralLayout, string) {
	walk := externalTypeWalk{effects: e}
	seen := make(map[*ast.TypeSpec]bool)
	generic := false
	// bindings maps the type parameters of the declaration being walked to
	// the arguments that instantiated it; stated is false once it cannot.
	var bindings map[string]sourceExpr
	stated := true
	for depth < 128 {
		depth++
		arguments := -1
		var instantiation []ast.Expr
		switch datatype := source.value.(type) {
		case *ast.ArrayType:
			element := sourceExpr{value: datatype.Elt, file: source.file, pkg: source.pkg}
			return externalLiteralLayout{
				kind:    externalLiteralCollection,
				element: boundLiteralType(element, bindings, stated),
			}, ""
		case *ast.MapType:
			key := sourceExpr{value: datatype.Key, file: source.file, pkg: source.pkg}
			element := sourceExpr{value: datatype.Value, file: source.file, pkg: source.pkg}
			return externalLiteralLayout{
				kind:    externalLiteralCollection,
				key:     boundLiteralType(key, bindings, stated),
				element: boundLiteralType(element, bindings, stated),
			}, ""
		case *ast.StructType:
			if generic {
				if positional {
					return externalLiteralLayout{}, "positional generic struct layout is unresolved"
				}
			}
			layout := externalLiteralLayout{kind: externalLiteralStruct, types: make(map[string]sourceExpr)}
			names := make(map[string]bool)
			for _, field := range datatype.Fields.List {
				if len(field.Names) == 0 {
					if positional {
						return externalLiteralLayout{}, "positional embedded field layout is unresolved"
					}
					name := embeddedTypeName(field.Type)
					if name == "" {
						return externalLiteralLayout{}, "embedded field ownership is unresolved"
					}
					if names[name] {
						return externalLiteralLayout{}, "struct field ownership is ambiguous"
					}
					names[name] = true
					layout.fields = append(layout.fields, name)
					layout.types[name] = sourceExpr{value: field.Type, file: source.file, pkg: source.pkg}
					continue
				}
				for _, name := range field.Names {
					if name.Name != "_" {
						if names[name.Name] {
							return externalLiteralLayout{}, "positional struct field identity is ambiguous"
						}
						names[name.Name] = true
						layout.types[name.Name] = sourceExpr{value: field.Type, file: source.file, pkg: source.pkg}
					}
					layout.fields = append(layout.fields, name.Name)
				}
			}
			return layout, ""
		case *ast.ParenExpr:
			source.value = datatype.X
			continue
		case *ast.StarExpr:
			if positional {
				return externalLiteralLayout{}, "positional pointer layout is unresolved"
			}
			source.value = datatype.X
			continue
		case *ast.IndexExpr:
			generic = true
			arguments = 1
			instantiation = []ast.Expr{datatype.Index}
			source.value = datatype.X
		case *ast.IndexListExpr:
			generic = true
			arguments = len(datatype.Indices)
			instantiation = datatype.Indices
			source.value = datatype.X
		}
		if selector, selected := source.value.(*ast.SelectorExpr); selected {
			if _, external := e.source.externalImport(source, selector); external {
				// Every element still joins the literal's union; only member
				// projection needs a retained layout.
				return externalLiteralLayout{kind: externalLiteralOpaque}, ""
			}
		}
		definition, owner, reason := walk.definition(source)
		if reason != "" {
			return externalLiteralLayout{}, "positional literal layout is unresolved: " + reason
		}
		if arguments >= 0 {
			if arguments != definition.TypeParams.NumFields() {
				return externalLiteralLayout{}, "generic root field argument ownership is unresolved"
			}
		}
		if seen[definition] {
			return externalLiteralLayout{}, "positional literal layout contains a type cycle"
		}
		seen[definition] = true
		if definition.TypeParams.NumFields() != 0 {
			generic = true
		}
		bindings, stated = literalBindings(definition, instantiation, source, bindings, stated)
		source = owner
	}
	return externalLiteralLayout{}, "positional literal layout exceeds 128 declaration steps"
}

// literalBindings names what each type parameter of definition stands for in
// the scope its declaration opens. Each argument is read in scope, the scope
// of the instantiation, under the bindings of that scope. A declaration
// without type parameters opens a scope that names none; a generic one
// reached uninstantiated states none of its own.
func literalBindings(definition *ast.TypeSpec, instantiation []ast.Expr, scope sourceExpr, outer map[string]sourceExpr, stated bool) (map[string]sourceExpr, bool) {
	if definition.TypeParams.NumFields() == 0 {
		return nil, true
	}
	if len(instantiation) != definition.TypeParams.NumFields() {
		return nil, false
	}
	bindings := make(map[string]sourceExpr, len(instantiation))
	index := 0
	for _, field := range definition.TypeParams.List {
		for _, name := range field.Names {
			argument := sourceExpr{value: instantiation[index], file: scope.file, pkg: scope.pkg}
			bindings[name.Name] = boundLiteralType(argument, outer, stated)
			index++
		}
	}
	return bindings, true
}

// boundLiteralType answers datatype with a type parameter that is the whole
// type, or the whole pointed-to type, replaced by its argument. A type that
// names a parameter anywhere else answers a nil value: substituting inside it
// would build a type the analyzer has no declaration for.
func boundLiteralType(datatype sourceExpr, bindings map[string]sourceExpr, stated bool) sourceExpr {
	if !stated {
		return sourceExpr{}
	}
	if len(bindings) == 0 {
		return datatype
	}
	value := ast.Unparen(datatype.value)
	if parameter, named := value.(*ast.Ident); named {
		if argument, bound := bindings[parameter.Name]; bound {
			return argument
		}
	}
	if pointer, address := value.(*ast.StarExpr); address {
		if parameter, named := ast.Unparen(pointer.X).(*ast.Ident); named {
			if argument, bound := bindings[parameter.Name]; bound {
				if argument.value == nil {
					return sourceExpr{}
				}
				return sourceExpr{value: &ast.StarExpr{X: argument.value}, file: argument.file, pkg: argument.pkg}
			}
		}
	}
	mentioned := false
	ast.Inspect(datatype.value, func(node ast.Node) bool {
		if identifier, named := node.(*ast.Ident); named {
			if _, bound := bindings[identifier.Name]; bound {
				mentioned = true
			}
		}
		return !mentioned
	})
	if mentioned {
		return sourceExpr{}
	}
	return datatype
}

// literalValue evaluates a literal reached as an expression of its own. A
// literal that elides its type is reached through literalElement instead,
// which passes the type its enclosing literal states, so arriving here with a
// nonempty elided literal means no enclosing literal was walked.
func (e *externalEffects) literalValue(source sourceExpr, literal *ast.CompositeLit, possible bool, depth int) externalValue {
	written := externalLiteralType{datatype: sourceExpr{value: literal.Type, file: source.file, pkg: source.pkg}}
	if literal.Type == nil {
		if len(literal.Elts) != 0 {
			result := e.typeMethods(written.datatype, depth+1)
			e.problem(source, externalElidedLiteralUnresolved)
			result.join(externalValue{unknown: externalElidedLiteralUnresolved})
			return result
		}
	}
	return e.typedLiteralValue(source, literal, written, possible, depth)
}

// literalElement evaluates one element, key or value of a literal whose layout
// is known. An element literal that elides its type takes the type the
// enclosing layout states; one whose type it cannot state takes literalValue,
// which keeps an empty one a zero construction and refuses a nonempty one.
func (e *externalEffects) literalElement(source sourceExpr, element ast.Expr, enclosing externalLiteralLayout, mapKey, possible bool, depth int) externalValue {
	child := sourceExpr{value: element, file: source.file, pkg: source.pkg}
	nested, composite := element.(*ast.CompositeLit)
	if !composite {
		return e.value(child, possible, depth+1)
	}
	if nested.Type != nil {
		return e.value(child, possible, depth+1)
	}
	stated, known := elidedLiteralType(enclosing, mapKey)
	if !known {
		return e.value(child, possible, depth+1)
	}
	// These guards are the ones value applies before it dispatches a literal.
	if e.amb.unknown != "" {
		return externalValue{unknown: e.amb.unknown}
	}
	if depth+1 == 128 {
		return externalValue{unknown: "expression capability depth exceeds 128"}
	}
	result := e.typedLiteralValue(child, nested, stated, possible, depth+1)
	if stated.pointer {
		// The elided &T{...} is an address of the literal, as value records it.
		result.references = true
	}
	result.locate(child)
	return result
}

// typedLiteralValue evaluates a literal under the type it constructs, written
// or stated, and passes each element its layout's stated type in turn.
func (e *externalEffects) typedLiteralValue(source sourceExpr, literal *ast.CompositeLit, stated externalLiteralType, possible bool, depth int) externalValue {
	result := e.typeMethods(stated.datatype, depth+1)
	if len(literal.Elts) == 0 {
		// An empty literal, written or elided as in []Shape[T]{{}}, constructs
		// zero without transferring a positional expression into a field.
		return result
	}
	positional := 0
	for _, element := range literal.Elts {
		if _, keyed := element.(*ast.KeyValueExpr); !keyed {
			positional++
		}
	}
	layout, reason := e.literalTypeLayout(stated, depth+1, positional != 0)
	if reason != "" {
		// Recording the issue here prevents a later known field projection
		// from erasing uncertainty stored only on its enclosing value.
		e.problem(source, reason)
		result.join(externalValue{unknown: reason})
		return result
	}
	if positional != 0 {
		if layout.kind == externalLiteralStruct {
			if positional != len(literal.Elts) {
				e.problem(source, "mixed keyed and positional struct layout is unresolved")
				return result
			}
			if positional != len(layout.fields) {
				e.problem(source, "positional struct field count is unresolved")
				return result
			}
		}
	}
	for index, element := range literal.Elts {
		if keyed, ok := element.(*ast.KeyValueExpr); ok {
			name := ""
			if layout.kind == externalLiteralStruct {
				member, identifier := keyed.Key.(*ast.Ident)
				if !identifier {
					e.problem(source, "struct field key ownership is unresolved")
					return result
				}
				if _, declared := layout.types[member.Name]; !declared {
					e.problem(source, "struct field key has no declared ownership")
					return result
				}
				name = member.Name
			} else if !layout.member(source.pkg, keyed.Key) {
				key := e.literalElement(source, keyed.Key, layout, true, possible, depth)
				if e.amb.unknown != "" {
					return externalValue{unknown: e.amb.unknown}
				}
				result.join(externalFlat(key, 0))
			}
			field := e.literalElement(source, keyed.Value, layout, false, possible, depth)
			if e.amb.unknown != "" {
				return externalValue{unknown: e.amb.unknown}
			}
			result.join(externalFlat(field, 0))
			if layout.kind == externalLiteralStruct {
				// join makes the field table only for a value that has fields.
				if result.fields == nil {
					result.fields = make(map[string]externalValue)
				}
				current := result.fields[name]
				current.join(field)
				result.fields[name] = current
			}
			continue
		}
		field := e.literalElement(source, element, layout, false, possible, depth)
		if e.amb.unknown != "" {
			return externalValue{unknown: e.amb.unknown}
		}
		result.join(externalFlat(field, 0))
		if layout.kind == externalLiteralStruct {
			name := layout.fields[index]
			if name == "_" {
				continue
			}
			if result.fields == nil {
				result.fields = make(map[string]externalValue)
			}
			current := result.fields[name]
			current.join(field)
			result.fields[name] = current
		}
	}
	return result
}
