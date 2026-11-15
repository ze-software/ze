// Design: docs/architecture/plugin/feature-gates.md -- declaration inputs retain immutable ownership.
package repoinventory

import (
	"fmt"
	"go/ast"
)

type declarationReference struct {
	pkg     *sourcePackage
	file    *ast.File
	name    string
	binding *lexicalDeclaration
}

type mutationCheck uint8

const (
	mutationCheckUnspecified mutationCheck = iota
	allDeclarationWrites
	registrationFieldWrites
	callableBindingWrites
)

// immutable refuses writes to a declaration used by a projected metadata field.
// Constants need no mutation scan: Go itself forbids assigning to them.
func (e *expression) immutable(source sourceExpr) error {
	reference, found, constant := e.reference(source)
	if !found {
		if _, named := source.value.(*ast.Ident); named {
			return e.problem(source, "declaration input has unresolved mutation ownership")
		}
		return nil
	}
	if constant {
		return nil
	}
	return e.writes(reference, allDeclarationWrites)
}

// callableImmutable uses the same bounded write proof but rejects an addressed
// callable binding: this reader does not interpret writes through helper parameters.
func (e *expression) callableImmutable(source sourceExpr) error {
	reference, found, constant := e.reference(source)
	if !found {
		return e.problem(source, "callable declaration has no immutable execution ownership")
	}
	if constant {
		return nil
	}
	return e.writes(reference, callableBindingWrites)
}

func (e *expression) reference(source sourceExpr) (declarationReference, bool, bool) {
	var binding *lexicalDeclaration
	switch node := source.value.(type) {
	case *ast.Ident:
		binding = sourceBinding(source, node)
	case *ast.SelectorExpr:
		directory, imported := e.callableImport(source, node)
		if !imported {
			return e.reference(sourceExpr{value: node.X, file: source.file, pkg: source.pkg})
		}
		pkg, found := e.snapshot.packages[directory]
		if !found {
			return declarationReference{}, false, false
		}
		binding = callablePackageBinding(pkg, node.Sel.Name)
	default:
		return declarationReference{}, false, false
	}
	if binding == nil {
		return declarationReference{}, false, false
	}
	if binding.kind != lexicalVariable {
		if binding.kind != lexicalConstant {
			return declarationReference{}, false, false
		}
	}
	return declarationReference{pkg: binding.pkg, file: binding.file, name: binding.ident.Name, binding: binding}, true, binding.kind == lexicalConstant
}

func (e *expression) writes(reference declarationReference, check mutationCheck) error {
	if check != allDeclarationWrites && check != registrationFieldWrites && check != callableBindingWrites {
		return fmt.Errorf("unsupported declaration mutation scan mode %d; select a named mutation policy before reading declarations", check)
	}
	query := declarationMutationQuery{reference: reference, check: check}
	if completed, found := e.mutationQueries[query]; found {
		return completed.failure
	}
	if e.mutations == nil {
		e.mutations = newDeclarationMutationIndex(e.snapshot)
	}
	failure := e.mutations.writes(e, reference, check)
	if e.mutationQueries == nil {
		e.mutationQueries = make(map[declarationMutationQuery]declarationMutationResult)
	}
	// Only a completed query writes an entry, including a successful nil error.
	e.mutationQueries[query] = declarationMutationResult{failure: failure}
	return failure
}

func importsPackage(aliases map[string]string, dir string) bool {
	for _, target := range aliases {
		if target == dir {
			return true
		}
	}
	return false
}

// refersTo follows a bounded local alias chain, including &reg and slice index
// writes. An overlong alias chain is conservatively treated as uncertain ownership.
func refersTo(value ast.Expr, reference declarationReference, pkg *sourcePackage, file *ast.File, depth int) bool {
	if depth == 64 {
		return true
	}
	switch node := value.(type) {
	case *ast.Ident:
		binding, known := pkg.snapshot.binding(file, node)
		if !known {
			occurrence := pkg.snapshot.lexical.identifiers[node]
			// An unresolved write or alias cannot prove this declaration safe.
			return occurrence.lexical
		}
		if binding == reference.binding {
			return true
		}
		if binding.kind == lexicalConstant {
			return false
		}
		if binding.kind == lexicalVariable {
			switch declaration := binding.node.(type) {
			case *ast.AssignStmt:
				for index, left := range declaration.Lhs {
					name, ok := left.(*ast.Ident)
					if !ok {
						continue
					}
					if name != binding.ident {
						continue
					}
					if index < len(declaration.Rhs) {
						return refersTo(declaration.Rhs[index], reference, binding.pkg, binding.file, depth+1)
					}
				}
			case *ast.ValueSpec:
				for index, name := range declaration.Names {
					if name != binding.ident {
						continue
					}
					if len(declaration.Values) == 0 {
						return false
					}
					if len(declaration.Names) != len(declaration.Values) {
						return true
					}
					return refersTo(declaration.Values[index], reference, binding.pkg, binding.file, depth+1)
				}
			}
		}
	case *ast.SelectorExpr:
		if owner, ok := node.X.(*ast.Ident); ok {
			binding, known := pkg.snapshot.binding(file, owner)
			if !known {
				if node.Sel.Name == reference.name {
					return true
				}
			} else if binding.kind == lexicalImport {
				if reference.binding.scope != pkg.snapshot.lexical.packages[reference.pkg] {
					return false
				}
				if pkg.imports[file][owner.Name] == reference.pkg.dir {
					if node.Sel.Name == reference.name {
						return true
					}
				}
			}
		}
		return refersTo(node.X, reference, pkg, file, depth+1)
	case *ast.IndexExpr:
		return refersTo(node.X, reference, pkg, file, depth+1)
	case *ast.UnaryExpr:
		return refersTo(node.X, reference, pkg, file, depth+1)
	case *ast.StarExpr:
		return refersTo(node.X, reference, pkg, file, depth+1)
	case *ast.ParenExpr:
		return refersTo(node.X, reference, pkg, file, depth+1)
	}
	return false
}
