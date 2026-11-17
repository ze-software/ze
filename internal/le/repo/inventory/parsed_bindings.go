// Design: docs/architecture/plugin/feature-gates.md -- syntax ownership is independent of committed population.
// Related: lexical.go, lexical_scope.go -- shared lexical declaration indexing.
package repoinventory

import "go/ast"

// ParsedBindingKind identifies a declaration; zero is unspecified.
type ParsedBindingKind = lexicalKind

const (
	ParsedBindingUnspecified = lexicalUnspecified
	ParsedBindingVariable    = lexicalVariable
	ParsedBindingConstant    = lexicalConstant
	ParsedBindingType        = lexicalType
	ParsedBindingFunction    = lexicalFunction
	ParsedBindingImport      = lexicalImport
	ParsedBindingUniverse    = lexicalUniverse
)

// ParsedBinding retains an exact declaration identity. Callers MUST use its
// pointer as identity: type-switch cases can share a defining identifier.
// The zero value is invalid; Binding returns only constructed bindings.
type ParsedBinding struct {
	declaration  *lexicalDeclaration
	packageScope bool
}

func (binding *ParsedBinding) Ident() *ast.Ident       { return binding.declaration.ident }
func (binding *ParsedBinding) Node() ast.Node          { return binding.declaration.node }
func (binding *ParsedBinding) File() *ast.File         { return binding.declaration.file }
func (binding *ParsedBinding) Kind() ParsedBindingKind { return binding.declaration.kind }
func (binding *ParsedBinding) Package() bool           { return binding.packageScope }

// ParsedBindings indexes caller-selected syntax of one package. Callers MUST
// supply one valid Go package, keep its ASTs immutable and respect the prohibition on declaring a
// name in both a file block and its package block. This view does not validate
// that language precondition. It parses and loads nothing, assigns no
// committed provenance, and is safe for concurrent read-only queries.
// Its zero and nil values answer unknown for every query.
type ParsedBindings struct {
	index    *lexicalIndex
	bindings map[*lexicalDeclaration]*ParsedBinding
}

// ParsedCompositeKeys states the caller's proven role of identifier keys.
// Its zero value deliberately withholds ownership for an unresolved shape.
type ParsedCompositeKeys uint8

const (
	ParsedKeysUnknown ParsedCompositeKeys = iota
	ParsedKeysExpressions
	ParsedKeysFields
)

// ParsedBindingOptions supplies facts the lexical passes cannot derive.
// ImportName MUST return the actual declared package name, never a path guess.
// CompositeKeys MAY inspect already indexed type identifiers through Binding;
// it MUST NOT retain or modify the view. Nil callbacks leave their facts unknown.
type ParsedBindingOptions struct {
	ImportName    func(string) (string, bool)
	CompositeKeys func(*ParsedBindings, *ast.File, *ast.CompositeLit) ParsedCompositeKeys
}

// NewParsedBindings shares the inventory's source-ordered scope construction.
// Unknown implicit imports withhold universe bindings; package declarations,
// locals and explicit import aliases remain resolved. Imported types are never
// loaded and uncertain composite keys remain unresolved.
func NewParsedBindings(files []*ast.File, options ParsedBindingOptions) *ParsedBindings {
	owner := &sourcePackage{}
	index := indexLexicalScopes([]lexicalPackageInput{{owner: owner, files: files, unresolvedDotImports: true}}, options.ImportName)
	view := &ParsedBindings{index: index, bindings: make(map[*lexicalDeclaration]*ParsedBinding)}
	for _, occurrence := range index.identifiers {
		declaration := occurrence.declaration
		if declaration == nil {
			continue
		}
		if _, present := view.bindings[declaration]; present {
			continue
		}
		view.bindings[declaration] = &ParsedBinding{declaration: declaration, packageScope: declaration.scope == index.packages[owner]}
	}
	for _, file := range files {
		ast.Inspect(file, func(node ast.Node) bool {
			literal, composite := node.(*ast.CompositeLit)
			if composite {
				keys := ParsedKeysUnknown
				if options.CompositeKeys != nil {
					keys = options.CompositeKeys(view, file, literal)
				}
				view.classifyKeyShape(literal, keys)
			}
			return true
		})
	}
	return view
}

// Binding refuses foreign syntax even when its spelling and position match.
// known with a nil binding denotes a proven nonbinding (such as a blank or
// struct label). Unknown and ambiguous ownership MUST NOT establish a declaration.
func (view *ParsedBindings) Binding(file *ast.File, identifier *ast.Ident) (*ParsedBinding, bool) {
	if view == nil {
		return nil, false
	}
	if view.index == nil {
		return nil, false
	}
	occurrence, retained := view.index.identifiers[identifier]
	if !retained {
		return nil, false
	}
	if occurrence.file != file {
		return nil, false
	}
	if !occurrence.lexical {
		return nil, true
	}
	if occurrence.declaration == nil {
		return nil, false
	}
	return view.bindings[occurrence.declaration], true
}

func (view *ParsedBindings) classifyKeyShape(literal *ast.CompositeLit, keys ParsedCompositeKeys) {
	for _, element := range literal.Elts {
		pair, keyed := element.(*ast.KeyValueExpr)
		if !keyed {
			continue
		}
		identifier, named := pair.Key.(*ast.Ident)
		if !named {
			continue
		}
		occurrence := view.index.identifiers[identifier]
		switch keys {
		case ParsedKeysFields:
			occurrence.declaration = nil
			occurrence.lexical = false
		case ParsedKeysExpressions:
			// Map and array keys retain their indexed expression bindings.
		case ParsedKeysUnknown:
			occurrence.declaration = nil
			occurrence.lexical = true
		default:
			// An unrecognized caller classification supplies no ownership proof.
			occurrence.declaration = nil
			occurrence.lexical = true
		}
		view.index.identifiers[identifier] = occurrence
	}
}
