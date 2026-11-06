// Design: docs/architecture/plugin/feature-gates.md -- declaration inputs retain immutable ownership.
package repoinventory

import (
	"fmt"
	"go/ast"
	"go/token"
)

type declarationReference struct {
	pkg   *sourcePackage
	file  *ast.File
	name  string
	local *ast.Object
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
	var name string
	pkg := source.pkg
	var local *ast.Object
	switch node := source.value.(type) {
	case *ast.Ident:
		name = node.Name
		local = node.Obj
	case *ast.SelectorExpr:
		directory, imported := e.callableImport(source, node)
		if !imported {
			return e.reference(sourceExpr{value: node.X, file: source.file, pkg: source.pkg})
		}
		var found bool
		pkg, found = e.snapshot.packages[directory]
		if !found {
			return declarationReference{}, false, false
		}
		name = node.Sel.Name
	default:
		return declarationReference{}, false, false
	}
	for _, file := range pkg.files {
		for _, declaration := range file.File.Decls {
			group, ok := declaration.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, spec := range group.Specs {
				value, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, declared := range value.Names {
					if declared.Name == name && (local == nil || declared.Obj == local) {
						return declarationReference{pkg: pkg, file: file.File, name: name}, true, group.Tok == token.CONST
					}
				}
			}
		}
	}
	if local != nil {
		return declarationReference{pkg: pkg, file: source.file, name: name, local: local}, true, local.Kind == ast.Con
	}
	return declarationReference{}, false, false
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
		if reference.local != nil {
			if node.Obj == reference.local {
				return true
			}
		} else if pkg == reference.pkg && node.Name == reference.name {
			if node.Obj == nil {
				return true
			}
			if _, global := pkg.values[node.Name]; global {
				if spec, ok := node.Obj.Decl.(*ast.ValueSpec); ok {
					for _, declaration := range file.Decls {
						group, ok := declaration.(*ast.GenDecl)
						if !ok {
							continue
						}
						for _, candidate := range group.Specs {
							if candidate == spec {
								return true
							}
						}
					}
				}
			}
		}
		if node.Obj != nil {
			switch declaration := node.Obj.Decl.(type) {
			case *ast.AssignStmt:
				for index, left := range declaration.Lhs {
					name, ok := left.(*ast.Ident)
					if !ok {
						continue
					}
					if name.Obj != node.Obj {
						continue
					}
					if index < len(declaration.Rhs) {
						return refersTo(declaration.Rhs[index], reference, pkg, file, depth+1)
					}
				}
			case *ast.ValueSpec:
				for index, name := range declaration.Names {
					if name.Obj != node.Obj {
						continue
					}
					if len(declaration.Values) == 0 {
						return false
					}
					if len(declaration.Names) != len(declaration.Values) {
						return true
					}
					return refersTo(declaration.Values[index], reference, pkg, file, depth+1)
				}
			}
		}
	case *ast.SelectorExpr:
		if owner, ok := node.X.(*ast.Ident); ok && reference.local == nil {
			lexicalImport := owner.Obj == nil
			if owner.Obj != nil {
				lexicalImport = owner.Obj.Kind == ast.Pkg
			}
			if lexicalImport {
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
