// Design: docs/architecture/plugin/feature-gates.md -- retained lexical declaration ownership.
// Related: lexical_scope.go -- source-ordered scope construction.
package repoinventory

import (
	"go/ast"
	"go/types"
	"path"
	"strings"
)

// lexicalKind describes declarations, not values inferred from their spelling.
// Its zero value is not a declaration kind.
type lexicalKind uint8

const (
	lexicalUnspecified lexicalKind = iota
	lexicalVariable
	lexicalConstant
	lexicalType
	lexicalFunction
	lexicalImport
	lexicalUniverse
)

// lexicalDeclaration belongs to exactly one syntax index. Type-switch cases can
// share a defining identifier while retaining different declaration identities.
// Only a universe declaration has no source file, package, node or scope.
// The zero value is invalid; a successful binding never returns nil.
type lexicalDeclaration struct {
	node     ast.Node
	ident    *ast.Ident
	file     *ast.File
	pkg      *sourcePackage
	kind     lexicalKind
	scope    *lexicalScope
	universe types.Object
}

// lexicalScope stores one entry per declared name. A present nil entry is an
// ambiguous declaration and blocks lookup in enclosing scopes.
// Its zero value is uninitialized; newLexicalScope supplies the name table.
type lexicalScope struct {
	parent *lexicalScope
	names  map[string]*lexicalDeclaration
}

type lexicalOccurrence struct {
	file        *ast.File
	declaration *lexicalDeclaration
	// lexical marks source uses and definitions, including uncertain key roles.
	// False marks proven selector members, fields, labels and struct keys.
	lexical bool
}

type lexicalIndex struct {
	identifiers map[*ast.Ident]lexicalOccurrence
	files       map[*ast.File]SourceFile
	packages    map[*sourcePackage]*lexicalScope
	universe    map[string]*lexicalDeclaration
}

// binding refuses foreign nodes even when their names and positions match.
// False includes unresolved, ambiguous and non-lexical source occurrences.
func (s *Snapshot) binding(file *ast.File, identifier *ast.Ident) (*lexicalDeclaration, bool) {
	return s.lexical.binding(file, identifier)
}

func (index *lexicalIndex) binding(file *ast.File, identifier *ast.Ident) (*lexicalDeclaration, bool) {
	if index == nil {
		return nil, false
	}
	occurrence, retained := index.identifiers[identifier]
	if !retained {
		return nil, false
	}
	if occurrence.file != file {
		return nil, false
	}
	return occurrence.declaration, occurrence.declaration != nil
}

// DeclarationReference distinguishes an exact retained declaration from a
// different binding and unresolved ownership. Callers MUST keep unresolved
// references in conservative reference proofs.
func (s *Snapshot) DeclarationReference(file SourceFile, identifier *ast.Ident, node ast.Node) (same, known bool) {
	if s.lexical == nil {
		return false, false
	}
	selected, found := s.lexical.files[file.File]
	if !found {
		return false, false
	}
	if selected.Path != file.Path {
		return false, false
	}
	occurrence, retained := s.lexical.identifiers[identifier]
	if !retained {
		return false, false
	}
	if occurrence.file != file.File {
		return false, false
	}
	if !occurrence.lexical {
		return false, true
	}
	binding, known := s.binding(file.File, identifier)
	if !known {
		return false, false
	}
	return binding.node == node, true
}

func callablePackageBinding(pkg *sourcePackage, name string) *lexicalDeclaration {
	if pkg.snapshot == nil {
		return nil
	}
	if pkg.snapshot.lexical == nil {
		return nil
	}
	return pkg.snapshot.lexical.packages[pkg].names[name]
}

func sourceBinding(source sourceExpr, name *ast.Ident) *lexicalDeclaration {
	if source.pkg == nil {
		return nil
	}
	if source.pkg.snapshot == nil {
		return nil
	}
	binding, _ := source.pkg.snapshot.binding(source.file, name)
	return binding
}

func sameBinding(a, b *ast.Ident, source sourceExpr) bool {
	left := sourceBinding(source, a)
	if left == nil {
		return false
	}
	return left == sourceBinding(source, b)
}

// indexLexical publishes the complete binding graph before classifying keys:
// the existing type readers can then follow cross-file aliases without seeing
// an index under construction. Retained syntax is never changed.
func (s *Snapshot) indexLexical() {
	inputs := make([]lexicalPackageInput, 0, len(s.packages))
	for _, pkg := range s.packages {
		pkg.snapshot = s
		input := lexicalPackageInput{owner: pkg}
		for _, file := range pkg.files {
			input.files = append(input.files, file.File)
		}
		inputs = append(inputs, input)
	}
	s.lexical = indexLexicalScopes(inputs, func(imported string) (string, bool) {
		target, internal := strings.CutPrefix(imported, modulePrefix)
		if !internal {
			target = "vendor/" + imported
		}
		pkg, retained := s.packages[target]
		if !retained {
			return path.Base(imported), true
		}
		return pkg.name, true
	})
	for _, pkg := range s.packages {
		for _, file := range pkg.files {
			s.lexical.files[file.File] = file
		}
	}
	for _, pkg := range s.packages {
		for _, file := range pkg.files {
			reader := s.evaluator()
			ast.Inspect(file.File, func(node ast.Node) bool {
				if literal, composite := node.(*ast.CompositeLit); composite {
					reader.lexicalComposite(sourceExpr{value: literal, file: file.File, pkg: pkg}, literal, sourceExpr{})
					return false
				}
				return true
			})
		}
	}
}

// lexicalPackageInput supplies caller-selected syntax without assigning it a
// committed revision, a build context or an import graph.
type lexicalPackageInput struct {
	owner                *sourcePackage
	files                []*ast.File
	unresolvedDotImports bool
}

// indexLexicalScopes shares the declaration and source-order passes. Composite
// key classification remains with the caller, after every binding is indexed.
func indexLexicalScopes(inputs []lexicalPackageInput, importName func(string) (string, bool)) *lexicalIndex {
	index := &lexicalIndex{
		identifiers: make(map[*ast.Ident]lexicalOccurrence),
		files:       make(map[*ast.File]SourceFile), packages: make(map[*sourcePackage]*lexicalScope),
		universe: make(map[string]*lexicalDeclaration),
	}
	for _, input := range inputs {
		pkg := input.owner
		scope := newLexicalScope(nil)
		index.packages[pkg] = scope
		for _, file := range input.files {
			ast.Inspect(file, func(node ast.Node) bool {
				if identifier, named := node.(*ast.Ident); named {
					index.identifiers[identifier] = lexicalOccurrence{file: file}
				}
				return true
			})
			walker := lexicalWalker{index: index, pkg: pkg, file: file, scope: scope, importName: importName}
			walker.predeclare()
		}
	}
	for _, input := range inputs {
		pkg := input.owner
		for _, file := range input.files {
			walker := lexicalWalker{index: index, pkg: pkg, file: file, scope: newLexicalScope(index.packages[pkg]), importName: importName, unresolvedDotImports: input.unresolvedDotImports}
			walker.fileDeclarations()
		}
	}
	return index
}

// lexicalComposite uses the retained type reader, including alias targets, to
// distinguish a map's expression keys from a struct's field names. Its recursive
// walk is bounded by the retained AST, and type resolution has a 64-edge limit.
func (e *expression) lexicalComposite(source sourceExpr, literal *ast.CompositeLit, expected sourceExpr) {
	shape := expected
	if literal.Type != nil {
		shape = sourceExpr{value: literal.Type, file: source.file, pkg: source.pkg}
	}
	for range 64 {
		if pointer, indirect := shape.value.(*ast.StarExpr); indirect {
			shape.value = pointer.X
			continue
		}
		if _, named := shape.value.(*ast.Ident); !named {
			if _, selected := shape.value.(*ast.SelectorExpr); !selected {
				break
			}
		}
		if definition, owner, concrete := e.concreteType(shape); concrete {
			file, declared := declaringFile(owner, definition)
			if !declared {
				break
			}
			shape = sourceExpr{value: definition.Type, file: file, pkg: owner}
			continue
		}
		binding := e.typeBinding(shape)
		if binding == nil {
			break
		}
		definition, defined := binding.node.(*ast.TypeSpec)
		if !defined {
			break
		}
		if definition.TypeParams.NumFields() != 0 {
			break
		}
		shape = sourceExpr{value: definition.Type, file: binding.file, pkg: binding.pkg}
	}
	for position, element := range literal.Elts {
		value := element
		elementType := sourceExpr{}
		if keyed, pair := element.(*ast.KeyValueExpr); pair {
			value = keyed.Value
			if identifier, named := keyed.Key.(*ast.Ident); named {
				occurrence := e.snapshot.lexical.identifiers[identifier]
				switch shape.value.(type) {
				case *ast.StructType:
					occurrence.declaration = nil
					occurrence.lexical = false
				case *ast.MapType, *ast.ArrayType:
					// These keys are expressions and keep their original binding.
				default:
					// Unknown key roles stay in conservative use and write proofs.
					occurrence.declaration = nil
					occurrence.lexical = true
				}
				e.snapshot.lexical.identifiers[identifier] = occurrence
			}
			keyType := sourceExpr{}
			if mapping, mapped := shape.value.(*ast.MapType); mapped {
				keyType = sourceExpr{value: mapping.Key, file: shape.file, pkg: shape.pkg}
			}
			e.lexicalNested(source, keyed.Key, keyType)
		}
		switch datatype := shape.value.(type) {
		case *ast.ArrayType:
			elementType = sourceExpr{value: datatype.Elt, file: shape.file, pkg: shape.pkg}
		case *ast.MapType:
			elementType = sourceExpr{value: datatype.Value, file: shape.file, pkg: shape.pkg}
		case *ast.StructType:
			if keyed, pair := element.(*ast.KeyValueExpr); pair {
				if key, named := keyed.Key.(*ast.Ident); named {
					for _, field := range datatype.Fields.List {
						for _, name := range field.Names {
							if name.Name == key.Name {
								elementType = sourceExpr{value: field.Type, file: shape.file, pkg: shape.pkg}
							}
						}
					}
				}
			} else if position < len(datatype.Fields.List) {
				elementType = sourceExpr{value: datatype.Fields.List[position].Type, file: shape.file, pkg: shape.pkg}
			}
		}
		e.lexicalNested(source, value, elementType)
	}
}

func (e *expression) lexicalNested(source sourceExpr, value ast.Expr, expected sourceExpr) {
	ast.Inspect(value, func(node ast.Node) bool {
		if nested, composite := node.(*ast.CompositeLit); composite {
			e.lexicalComposite(source, nested, expected)
			return false
		}
		return true
	})
}
