// Design: docs/architecture/plugin/feature-gates.md -- positional literals preserve field capabilities.
package repoinventory

import "go/ast"

type externalLiteralKind uint8

const (
	externalLiteralUnresolved externalLiteralKind = iota
	externalLiteralCollection
	externalLiteralStruct
)

type externalLiteralLayout struct {
	kind   externalLiteralKind
	fields []string
	types  map[string]sourceExpr
}

// structLiteral is also used by formal provenance to distinguish member names
// from executable collection-key expressions under the same root proof.
func (e *externalEffects) structLiteral(source sourceExpr) bool {
	layout, reason := e.literalLayout(source, 0, false)
	if reason != "" {
		return false
	}
	return layout.kind == externalLiteralStruct
}

// literalLayout follows exact retained aliases and defined types. Field slots
// retain declaration order, including each name in a grouped field declaration.
// Generic collections need no element substitution to identify their layout;
// positional generic structs and embedded fields remain explicit uncertainty.
func (e *externalEffects) literalLayout(source sourceExpr, depth int, positional bool) (externalLiteralLayout, string) {
	walk := externalTypeWalk{effects: e}
	seen := make(map[*ast.TypeSpec]bool)
	generic := false
	for depth < 128 {
		depth++
		arguments := -1
		switch datatype := source.value.(type) {
		case *ast.ArrayType, *ast.MapType:
			return externalLiteralLayout{kind: externalLiteralCollection}, ""
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
					name := externalEmbeddedTypeName(field.Type)
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
			source.value = datatype.X
		case *ast.IndexListExpr:
			generic = true
			arguments = len(datatype.Indices)
			source.value = datatype.X
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
		source = owner
	}
	return externalLiteralLayout{}, "positional literal layout exceeds 128 declaration steps"
}

func (e *externalEffects) literalValue(source sourceExpr, literal *ast.CompositeLit, possible bool, depth int) externalValue {
	datatype := sourceExpr{value: literal.Type, file: source.file, pkg: source.pkg}
	result := e.typeMethods(datatype, depth+1)
	if len(literal.Elts) == 0 {
		// An elided empty element, such as []Shape[T]{{}}, constructs zero
		// without transferring a positional expression into an unknown field.
		return result
	}
	if literal.Type == nil {
		e.problem(source, "nonempty elided literal layout is unresolved")
		result.join(externalValue{unknown: "nonempty elided literal layout is unresolved"})
		return result
	}
	positional := 0
	for _, element := range literal.Elts {
		if _, keyed := element.(*ast.KeyValueExpr); !keyed {
			positional++
		}
	}
	layout, reason := e.literalLayout(datatype, depth+1, positional != 0)
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
			} else {
				key := e.value(sourceExpr{value: keyed.Key, file: source.file, pkg: source.pkg}, possible, depth+1)
				if e.amb.unknown != "" {
					return externalValue{unknown: e.amb.unknown}
				}
				result.join(externalFlat(key, 0))
			}
			field := e.value(sourceExpr{value: keyed.Value, file: source.file, pkg: source.pkg}, possible, depth+1)
			if e.amb.unknown != "" {
				return externalValue{unknown: e.amb.unknown}
			}
			result.join(externalFlat(field, 0))
			if layout.kind == externalLiteralStruct {
				current := result.fields[name]
				current.join(field)
				result.fields[name] = current
			}
			continue
		}
		field := e.value(sourceExpr{value: element, file: source.file, pkg: source.pkg}, possible, depth+1)
		if e.amb.unknown != "" {
			return externalValue{unknown: e.amb.unknown}
		}
		result.join(externalFlat(field, 0))
		if layout.kind == externalLiteralStruct {
			name := layout.fields[index]
			if name == "_" {
				continue
			}
			current := result.fields[name]
			current.join(field)
			result.fields[name] = current
		}
	}
	return result
}
