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
		owner, ok := node.X.(*ast.Ident)
		if !ok {
			return declarationReference{}, false, false
		}
		var found bool
		pkg, found = e.snapshot.packages[source.pkg.imports[source.file][owner.Name]]
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
	for _, pkg := range e.snapshot.packages {
		for _, file := range pkg.files {
			if reference.local != nil && file.File != reference.file {
				continue
			}
			if pkg != reference.pkg && !importsPackage(pkg.imports[file.File], reference.pkg.dir) {
				continue
			}
			var problem error
			ast.Inspect(file.File, func(node ast.Node) bool {
				var targets []ast.Expr
				switch statement := node.(type) {
				case *ast.AssignStmt:
					// Only the actual defining occurrence is excluded below;
					// := can reassign a variable bound by an earlier declaration.
					targets = statement.Lhs
				case *ast.IncDecStmt:
					targets = []ast.Expr{statement.X}
				case *ast.UnaryExpr:
					if check == callableBindingWrites && statement.Op == token.AND && refersTo(statement.X, reference, pkg, file.File, 0) {
						problem = e.problem(sourceExpr{value: statement, file: file.File, pkg: pkg}, "callable declaration input "+reference.name+" has an escaping address")
						return false
					}
				}
				for _, target := range targets {
					if assignment, ok := node.(*ast.AssignStmt); ok && assignment.Tok == token.DEFINE {
						if declared, ok := target.(*ast.Ident); ok && declared.Obj != nil && declared.Obj.Decl == assignment {
							continue
						}
					}
					if check == registrationFieldWrites {
						if field, ok := target.(*ast.SelectorExpr); ok && !metadataField(field.Sel.Name) {
							continue
						}
					}
					if refersTo(target, reference, pkg, file.File, 0) {
						problem = e.problem(sourceExpr{value: target, file: file.File, pkg: pkg}, "declaration input "+reference.name+" is mutated after initialization")
						return false
					}
				}
				return problem == nil
			})
			if problem != nil {
				return problem
			}
		}
	}
	return nil
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
			if pkg.imports[file][owner.Name] == reference.pkg.dir && node.Sel.Name == reference.name {
				return true
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
