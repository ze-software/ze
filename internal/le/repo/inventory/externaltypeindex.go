// Design: docs/architecture/plugin/feature-gates.md -- private structural type ownership.
package repoinventory

import (
	"go/ast"
	"go/token"
)

// externalTypeOwner preserves both the selected package and declaration
// identity. Equal spellings or source positions cannot substitute for either.
type externalTypeOwner struct {
	pkg        *sourcePackage
	definition *ast.TypeSpec
}

// externalTypeIdentityIndex contains structural source facts only. It is built
// completely before publication to one reader and never stores capabilities,
// substitutions, recursion state, ambient values or depth-dependent answers.
type externalTypeIdentityIndex struct {
	owners          map[externalTypeOwner]*ast.File
	methods         map[*sourcePackage][]sourceFunction
	parameters      map[*ast.File][]externalLexicalParameters
	bindings        map[*sourcePackage]map[string]externalPackageBinding
	receiverSources map[sourceExpr]bool
}

// A lexical entry preserves every ancestor's original containment predicates,
// not just the parameter declaration's own span. Entries keep AST preorder.
// An entry holds a generic function's type parameter list, a generic type
// declaration, or a method whose receiver declares type parameters
// (externalReceiverTypeParameters).
type externalLexicalParameters struct {
	span       externalLexicalSpan
	parameters *ast.FieldList
	method     *ast.FuncDecl
	definition *ast.TypeSpec
}

type externalLexicalSpan struct {
	start token.Pos
	end   token.Pos
}

func newExternalTypeIdentityIndex(snapshot *Snapshot) *externalTypeIdentityIndex {
	index := &externalTypeIdentityIndex{
		owners:          make(map[externalTypeOwner]*ast.File),
		methods:         make(map[*sourcePackage][]sourceFunction),
		parameters:      make(map[*ast.File][]externalLexicalParameters),
		bindings:        make(map[*sourcePackage]map[string]externalPackageBinding),
		receiverSources: make(map[sourceExpr]bool),
	}
	for _, pkg := range snapshot.packages {
		index.indexPackageBindings(pkg)
		for _, file := range pkg.files {
			var spans []externalLexicalSpan
			var parameters []externalLexicalParameters
			ast.Inspect(file.File, func(node ast.Node) bool {
				if node == nil {
					spans = spans[:len(spans)-1]
					return false
				}
				span := externalLexicalSpan{start: node.Pos(), end: node.End()}
				if len(spans) != 0 {
					parent := spans[len(spans)-1]
					span.start = max(span.start, parent.start)
					span.end = min(span.end, parent.end)
				}
				spans = append(spans, span)
				if definition, declared := node.(*ast.TypeSpec); declared {
					index.owners[externalTypeOwner{pkg: pkg, definition: definition}] = file.File
				}
				var fields *ast.FieldList
				switch declaration := node.(type) {
				case *ast.TypeSpec:
					if declaration.TypeParams.NumFields() != 0 {
						parameters = append(parameters, externalLexicalParameters{span: span, definition: declaration})
					}
				case *ast.FuncDecl:
					fields = declaration.Type.TypeParams
					if len(externalReceiverTypeParameters(declaration)) != 0 {
						parameters = append(parameters, externalLexicalParameters{span: span, method: declaration})
					}
				}
				if fields.NumFields() != 0 {
					parameters = append(parameters, externalLexicalParameters{span: span, parameters: fields})
				}
				return true
			})
			// Presence certifies a completed file scan even when it has no
			// parameter declarations. Absence requires the original traversal.
			index.parameters[file.File] = parameters
			// Preserve the existing file/Decls sequence, including unsupported
			// receiver shapes and bodyless methods. Each query still refuses them.
			for _, declaration := range file.File.Decls {
				function, declared := declaration.(*ast.FuncDecl)
				if !declared {
					continue
				}
				if function.Recv == nil {
					continue
				}
				index.methods[pkg] = append(index.methods[pkg], sourceFunction{decl: function, file: file.File, pkg: pkg})
				if len(function.Recv.List) == 1 {
					source := sourceExpr{value: function.Recv.List[0].Type, file: file.File, pkg: pkg}
					index.receiverSources[source] = true
				}
			}
		}
	}
	return index
}

func (index *externalTypeIdentityIndex) lexicalScope(source sourceExpr) *externalTypeScope {
	if source.file == nil {
		return nil
	}
	if source.value == nil {
		return nil
	}
	if !source.value.Pos().IsValid() {
		return nil
	}
	parameters, selected := index.parameters[source.file]
	if !selected {
		return externalLexicalTypeScope(source)
	}
	var result *externalTypeScope
	for _, declaration := range parameters {
		if declaration.span.start > source.value.Pos() {
			continue
		}
		if declaration.span.end < source.value.End() {
			continue
		}
		// Preserve fresh scope identity: argument.scope participates in
		// recursion-frame equality and cannot be interned across queries.
		if declaration.method != nil {
			result = externalBindReceiverTypeParameters(result, declaration.method, source.file, source.pkg, nil)
			continue
		}
		if declaration.definition != nil {
			result = externalBindDeclarationTypeParameters(result, declaration.definition, source.file, source.pkg)
			continue
		}
		result = externalBindTypeParameters(result, declaration.parameters, nil)
	}
	return result
}
