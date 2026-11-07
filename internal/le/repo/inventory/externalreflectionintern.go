// Design: docs/architecture/plugin/feature-gates.md -- the reflection walk resolves each type reference once and keys its tables on small integers.
// Related: externalreflectiontypes.go -- collectNamed asks here before it builds a frame.
// Related: externaltypeterms.go -- an exact frame is interned on its owner's id.
package repoinventory

import "go/ast"

// externalReflectionNamed is collectNamed's resolution of one type reference:
// a standard-library origin, or a retained definition and its owner, or the
// reason neither is known. ownerID interns the definition with its owner's
// package, which fixes the whole owner (externalTypeWalk.definition builds it
// from the definition's type, the package and the file the identity index
// holds for both).
type externalReflectionNamed struct {
	external   bool
	origin     string
	definition *ast.TypeSpec
	owner      sourceExpr
	reason     string
	ownerID    int32
}

// externalReflectionKeys interns retained definitions with their owner's
// package to dense ids. Equal ids mean equal identities and
// unequal ids unequal ones, so a table keyed on an id answers what a table
// keyed on the identity answers. It holds at most one entry per retained type
// declaration and package. Not safe for concurrent use; one analysis runs on
// one goroutine.
type externalReflectionKeys struct {
	owners map[externalTypeOwner]int32
}

func (k *externalReflectionKeys) owner(pkg *sourcePackage, definition *ast.TypeSpec) int32 {
	key := externalTypeOwner{pkg: pkg, definition: definition}
	if id, found := k.owners[key]; found {
		return id
	}
	if k.owners == nil {
		k.owners = make(map[externalTypeOwner]int32)
	}
	id := int32(len(k.owners))
	k.owners[key] = id
	return id
}

// externalReflectionNameKey is a type reference collectNamed resolves: the
// whole sourceExpr, with its expression split into the two shapes a named
// type takes, so the key holds pointers and a string and no interface.
type externalReflectionNameKey struct {
	ident    *ast.Ident
	selector *ast.SelectorExpr
	file     *ast.File
	pkg      *sourcePackage
	name     string
}

// reflectionNamed answers what collectNamed resolves source to. Both questions
// it asks, whether the reference is a standard-library import and which
// retained declaration it names, read only the captured source and the type
// identity index, which no analysis step writes, so the answer is the same on
// every visit and is kept for the analysis. The table holds at most one entry
// per type reference the reflection walk meets in retained source. Any other
// expression shape refuses in externalTypeWalk.definition, uncached.
func (e *externalEffects) reflectionNamed(source sourceExpr) externalReflectionNamed {
	key := externalReflectionNameKey{file: source.file, pkg: source.pkg, name: source.name}
	switch datatype := source.value.(type) {
	case *ast.Ident:
		key.ident = datatype
	case *ast.SelectorExpr:
		key.selector = datatype
	default:
		return e.resolveReflectionNamed(source)
	}
	if named, found := e.reflectionNames[key]; found {
		return named
	}
	named := e.resolveReflectionNamed(source)
	if e.reflectionNames == nil {
		e.reflectionNames = make(map[externalReflectionNameKey]externalReflectionNamed)
	}
	e.reflectionNames[key] = named
	return named
}

// sourceOf rebuilds the reference a name key was made from.
func (k externalReflectionNameKey) sourceOf() sourceExpr {
	source := sourceExpr{file: k.file, pkg: k.pkg, name: k.name}
	if k.ident != nil {
		source.value = k.ident
		return source
	}
	source.value = k.selector
	return source
}

func (e *externalEffects) resolveReflectionNamed(source sourceExpr) externalReflectionNamed {
	if selector, selected := source.value.(*ast.SelectorExpr); selected {
		if origin, external := e.source.externalImport(source, selector); external {
			return externalReflectionNamed{external: true, origin: origin}
		}
	}
	identity := externalTypeWalk{effects: e}
	definition, owner, reason := identity.definition(source)
	if reason != "" {
		return externalReflectionNamed{reason: reason}
	}
	return externalReflectionNamed{
		definition: definition,
		owner:      owner,
		ownerID:    e.reflectionKeys.owner(owner.pkg, definition),
	}
}
