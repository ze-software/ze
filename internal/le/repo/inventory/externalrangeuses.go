// Design: docs/architecture/plugin/feature-gates.md -- ranged element summaries.
// Related: externalrangeelements.go -- the element population proof that reads this index.
package repoinventory

import (
	"go/ast"
	"slices"
)

// externalRangeUse is one occurrence the element population proof judges,
// with its four nearest enclosing nodes, nearest first.
type externalRangeUse struct {
	// node is the identifier, a keyed literal's value, or a positional literal.
	node ast.Expr
	// selector is set for a field occurrence x.f; its parents are the selector's.
	selector *ast.SelectorExpr
	parents  [4]ast.Node
	owner    ast.Node
	file     *ast.File
	pkg      *sourcePackage
}

func (u externalRangeUse) occurrence() ast.Expr {
	if u.selector != nil {
		return u.selector
	}
	return u.node
}

// externalRangeUses indexes a package's occurrences once. It is syntax and
// exact declaration identities only, so it never goes stale during a solve.
type externalRangeUses struct {
	objects    map[*ast.Object][]externalRangeUse
	fields     map[string][]externalRangeUse
	keys       map[string][]externalRangeUse
	positional []externalRangeUse
}

type externalFieldIdentity struct {
	datatype *ast.Object
	name     string
}

// rangeUses answers the package's occurrence index, building it on first use.
// Not safe for concurrent use, as no part of one external-effect solve is.
func (e *externalEffects) rangeUses(pkg *sourcePackage) *externalRangeUses {
	if uses, built := e.rangeIndexes[pkg]; built {
		return uses
	}
	uses := &externalRangeUses{
		objects: make(map[*ast.Object][]externalRangeUse),
		fields:  make(map[string][]externalRangeUse),
		keys:    make(map[string][]externalRangeUse),
	}
	for _, file := range pkg.files {
		var stack []ast.Node
		var owners []ast.Node
		ast.Inspect(file.File, func(node ast.Node) bool {
			if node == nil {
				stack = stack[:len(stack)-1]
				owners = owners[:len(owners)-1]
				return false
			}
			var owner ast.Node
			if len(owners) != 0 {
				owner = owners[len(owners)-1]
			}
			use := externalRangeUse{owner: owner, file: file.File, pkg: pkg}
			for index := range use.parents {
				if index < len(stack) {
					use.parents[index] = stack[len(stack)-1-index]
				}
			}
			e.indexRangeUse(uses, use, node)
			stack = append(stack, node)
			switch node.(type) {
			case *ast.FuncDecl, *ast.FuncLit:
				owner = node
			}
			owners = append(owners, owner)
			return true
		})
	}
	e.rangeIndexes[pkg] = uses
	return uses
}

func (e *externalEffects) indexRangeUse(uses *externalRangeUses, use externalRangeUse, node ast.Node) {
	switch value := node.(type) {
	case *ast.Ident:
		switch parent := use.parents[0].(type) {
		case *ast.SelectorExpr:
			if parent.Sel == value {
				return
			}
		case *ast.KeyValueExpr:
			// A collection is never comparable, so a key never names one, and
			// a struct literal's key is a member name, not an occurrence.
			if parent.Key == value {
				return
			}
		case *ast.Field:
			// A struct member or parameter name declares; it is never an
			// occurrence of a package variable it happens to spell.
			if slices.Contains(parent.Names, value) {
				return
			}
		case *ast.FuncDecl, *ast.TypeSpec, *ast.LabeledStmt, *ast.BranchStmt, *ast.ImportSpec:
			return
		}
		object := e.object(sourceExpr{value: value, file: use.file, pkg: use.pkg})
		if object == nil {
			return
		}
		use.node = value
		uses.objects[object] = append(uses.objects[object], use)
	case *ast.SelectorExpr:
		use.node = value
		use.selector = value
		uses.fields[value.Sel.Name] = append(uses.fields[value.Sel.Name], use)
	case *ast.KeyValueExpr:
		if _, literal := use.parents[0].(*ast.CompositeLit); !literal {
			return
		}
		member, identifier := value.Key.(*ast.Ident)
		if !identifier {
			return
		}
		shifted := use
		shifted.node = value.Value
		shifted.parents = [4]ast.Node{value, use.parents[0], use.parents[1], use.parents[2]}
		uses.keys[member.Name] = append(uses.keys[member.Name], shifted)
	case *ast.CompositeLit:
		if len(value.Elts) == 0 {
			return
		}
		if _, keyed := value.Elts[0].(*ast.KeyValueExpr); keyed {
			return
		}
		use.node = value
		uses.positional = append(uses.positional, use)
	}
}

// rangeFieldIdentity proves the exact struct type and direct member a field
// occurrence names, from the receiver's static type. A promoted member, an
// alias, a generic or a defined type naming another type refuses: each can
// name the same storage under another identity.
func (e *externalEffects) rangeFieldIdentity(source sourceExpr, selector *ast.SelectorExpr) (externalFieldIdentity, *sourcePackage, string) {
	datatype, known := e.resolver.valueType(sourceExpr{value: selector.X, file: source.file, pkg: source.pkg}, 0)
	if !known {
		return externalFieldIdentity{}, nil, "field receiver type is unresolved"
	}
	if pointer, indirect := ast.Unparen(datatype.value).(*ast.StarExpr); indirect {
		datatype.value = pointer.X
	}
	object, reason := e.rangeTypeObject(datatype, true)
	if reason != "" {
		return externalFieldIdentity{}, nil, reason
	}
	if object == nil {
		return externalFieldIdentity{}, nil, "field owner is not a retained named struct type"
	}
	// rangeTypeObject answers only a TypeSpec declaring a struct type.
	spec, declared := object.Decl.(*ast.TypeSpec)
	if !declared {
		panic("BUG: range type object is not a type declaration")
	}
	structure, isStruct := spec.Type.(*ast.StructType)
	if !isStruct {
		panic("BUG: range type object is not a struct type")
	}
	for _, member := range structure.Fields.List {
		for _, name := range member.Names {
			if name.Name == selector.Sel.Name {
				return externalFieldIdentity{datatype: object, name: name.Name}, datatype.pkg, ""
			}
		}
	}
	return externalFieldIdentity{}, nil, "field owner has no direct member " + selector.Sel.Name
}

// rangeTypeObject answers the declaration of a retained, non-generic named
// struct type, nil for a type that is provably not one (a predeclared, an
// imported or an unnamed type), or the reason its identity is unresolved.
// An alias always refuses: it names another type's storage. For a literal
// (owner false), a generic or a defined non-struct type constructs a type
// that is not the summarized struct, so it answers nil; for a field owner it
// refuses, because its members are not a retained struct's direct members.
func (e *externalEffects) rangeTypeObject(datatype sourceExpr, owner bool) (*ast.Object, string) {
	switch value := ast.Unparen(datatype.value).(type) {
	case *ast.Ident:
		object := e.object(sourceExpr{value: value, file: datatype.file, pkg: datatype.pkg})
		if object == nil {
			return nil, ""
		}
		if object.Kind != ast.Typ {
			return nil, "type identity is unresolved"
		}
		spec, declared := object.Decl.(*ast.TypeSpec)
		if !declared {
			return nil, "type declaration is unresolved"
		}
		if spec.Assign.IsValid() {
			return nil, "type alias ownership is unresolved"
		}
		if spec.TypeParams != nil {
			if !owner {
				return nil, ""
			}
			return nil, "generic type ownership is unresolved"
		}
		if _, structure := spec.Type.(*ast.StructType); !structure {
			if !owner {
				return nil, ""
			}
			return nil, "defined type ownership is unresolved"
		}
		return object, ""
	case *ast.SelectorExpr, *ast.IndexExpr, *ast.IndexListExpr, *ast.ArrayType, *ast.MapType, *ast.StructType, *ast.ChanType, *ast.FuncType, *ast.InterfaceType:
		return nil, ""
	}
	return nil, "type form is unresolved"
}

// externalElidedRangeLiteral refuses a literal whose elided type no
// enclosing literal states.
const externalElidedRangeLiteral = "elided literal type is unresolved"

// rangeLiteralType answers the type a keyed or positional literal constructs.
// A literal that elides its type takes the element or key type its enclosing
// literal states (Go specification, "Composite literals"); a deeper elision
// refuses.
func (e *externalEffects) rangeLiteralType(use externalRangeUse, pkg *sourcePackage, positional bool) (*ast.Object, string) {
	literal, parent, grandparent := externalRangeLiteral(use, positional)
	if literal == nil {
		return nil, "literal is unresolved"
	}
	if literal.Type != nil {
		return e.rangeTypeObject(sourceExpr{value: literal.Type, file: use.file, pkg: pkg}, false)
	}
	enclosing, mapKey := externalEnclosingLiteral(literal, parent, grandparent)
	if enclosing == nil {
		return nil, externalElidedRangeLiteral
	}
	if enclosing.Type == nil {
		return nil, "nested elided literal type is unresolved"
	}
	layout, reason := e.literalLayout(sourceExpr{value: enclosing.Type, file: use.file, pkg: pkg}, 0, false)
	if reason != "" {
		return nil, reason
	}
	if layout.kind != externalLiteralCollection {
		return nil, externalElidedRangeLiteral
	}
	stated := layout.element
	if mapKey {
		stated = layout.key
	}
	if stated.value == nil {
		return nil, externalElidedRangeLiteral
	}
	element := stated
	if pointer, indirect := ast.Unparen(element.value).(*ast.StarExpr); indirect {
		element.value = pointer.X
	}
	return e.rangeTypeObject(element, false)
}

// externalRangeLiteral answers the literal a key or positional occurrence
// belongs to, with the literal's own two nearest enclosing nodes. A positional
// occurrence is the literal itself; a key occurrence is the keyed value, whose
// parents are the pair and then the literal.
func externalRangeLiteral(use externalRangeUse, positional bool) (*ast.CompositeLit, ast.Node, ast.Node) {
	if positional {
		literal, composite := use.node.(*ast.CompositeLit)
		if !composite {
			return nil, nil, nil
		}
		return literal, use.parents[0], use.parents[1]
	}
	literal, keyed := use.parents[1].(*ast.CompositeLit)
	if !keyed {
		return nil, nil, nil
	}
	return literal, use.parents[2], use.parents[3]
}

func externalEnclosingLiteral(literal *ast.CompositeLit, parent, grandparent ast.Node) (*ast.CompositeLit, bool) {
	if enclosing, element := parent.(*ast.CompositeLit); element {
		return enclosing, false
	}
	keyed, pair := parent.(*ast.KeyValueExpr)
	if !pair {
		return nil, false
	}
	enclosing, inside := grandparent.(*ast.CompositeLit)
	if !inside {
		return nil, false
	}
	return enclosing, keyed.Key == literal
}
