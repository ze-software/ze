// Design: docs/architecture/plugin/feature-gates.md -- executable owners cannot disappear behind aliases.
// Related: registrations.go -- the same normalized callee reaches execution dispatch.
package repoinventory

import (
	"go/ast"
	"go/types"
	"path"
	"strconv"
	"strings"
)

// callableSource normalizes only parentheses and immutable declaration aliases.
// Its source MUST be consumed by execution dispatch. declared distinguishes a
// retained function/closure from a language conversion, builtin or external call;
// dispatch MUST NOT reclassify a conversion using a same-named package function.
func (r *registrations) callableSource(call *ast.CallExpr, scope initializationScope) (source sourceExpr, declared bool, err error) {
	origin := sourceExpr{value: call, file: scope.file, pkg: scope.pkg}
	source = sourceExpr{value: call.Fun, file: scope.file, pkg: scope.pkg}
	aliased := false
	for range 64 {
		switch value := source.value.(type) {
		case *ast.ParenExpr:
			source.value = value.X
			continue
		case *ast.FuncLit:
			if aliased {
				return sourceExpr{}, false, r.eval.problem(origin, "callable closure alias has unresolved execution ownership")
			}
			return source, true, nil
		case *ast.Ident:
			binding := sourceBinding(source, value)
			if binding == nil {
				return sourceExpr{}, false, r.eval.problem(origin, "callable identifier "+value.Name+" has unresolved execution ownership")
			}
			if binding.kind == lexicalUniverse {
				switch binding.universe.(type) {
				case *types.Builtin, *types.TypeName:
					return source, false, nil
				}
				return sourceExpr{}, false, r.eval.problem(origin, "callable identifier "+value.Name+" has unresolved execution ownership")
			}
			switch binding.kind {
			case lexicalFunction:
				if _, found := source.pkg.functions[value.Name]; !found {
					return sourceExpr{}, false, r.eval.problem(origin, "callable function "+value.Name+" has unresolved execution ownership")
				}
				if aliased && registrationAPI(source.pkg.dir, value.Name) {
					return sourceExpr{}, false, r.eval.problem(origin, "callable registration alias has unresolved execution ownership")
				}
				return source, true, nil
			case lexicalType:
				return source, false, nil
			case lexicalVariable:
				// A local closure called only through its name executes
				// its own literal, as zstd's fatalErr(binary.Read(...))
				// does; any other use of the name keeps the alias refusal.
				if !aliased {
					if literal, bound := closureBinding(value, source); bound {
						if closureCalledOnly(source.file, binding, r.eval.snapshot) {
							return sourceExpr{value: literal, file: source.file, pkg: source.pkg}, true, nil
						}
					}
				}
				if !callableAliasBinding(binding) {
					return sourceExpr{}, false, r.eval.problem(origin, "callable parameter or variable "+value.Name+" has unresolved execution ownership")
				}
			case lexicalUnspecified, lexicalImport, lexicalConstant, lexicalUniverse:
				return sourceExpr{}, false, r.eval.problem(origin, "callable declaration "+value.Name+" has unresolved execution ownership")
			}
		case *ast.SelectorExpr:
			directory, imported := r.eval.callableImport(source, value)
			if !imported {
				if _, found := r.eval.method(source); found {
					return source, true, nil
				}
				return sourceExpr{}, false, r.eval.problem(origin, "callable receiver has unresolved execution ownership")
			}
			if registrationAPI(directory, value.Sel.Name) {
				if aliased {
					return sourceExpr{}, false, r.eval.problem(origin, "callable registration alias has unresolved execution ownership")
				}
				return source, true, nil
			}
			if directory == "" {
				return source, false, nil
			}
			pkg, found := r.eval.snapshot.packages[directory]
			if !found {
				return sourceExpr{}, false, r.eval.problem(origin, "callable imported package has unresolved execution ownership")
			}
			binding := callablePackageBinding(pkg, value.Sel.Name)
			if binding == nil {
				return sourceExpr{}, false, r.eval.problem(origin, "callable imported declaration has unresolved execution ownership")
			}
			if binding.kind == lexicalFunction {
				return source, true, nil
			}
			if binding.kind == lexicalType {
				return source, false, nil
			}
			if _, found := pkg.values[value.Sel.Name]; !found {
				return sourceExpr{}, false, r.eval.problem(origin, "callable imported declaration has unresolved execution ownership")
			}
		case *ast.ArrayType, *ast.MapType, *ast.ChanType, *ast.FuncType, *ast.InterfaceType, *ast.StructType:
			// A syntactic type conversion executes its arguments, not a body.
			return source, false, nil
		default:
			return sourceExpr{}, false, r.eval.problem(origin, "callable expression has unresolved execution ownership")
		}
		if err := r.eval.callableImmutable(source); err != nil {
			return sourceExpr{}, false, r.eval.problem(origin, "callable declaration has unresolved execution ownership: "+err.Error())
		}
		resolved, err := r.eval.target(source)
		if err != nil {
			return sourceExpr{}, false, r.eval.problem(origin, "callable declaration has unresolved execution ownership: "+err.Error())
		}
		source = resolved
		aliased = true
	}
	return sourceExpr{}, false, r.eval.problem(origin, "callable declaration exceeds 64 steps with unresolved execution ownership")
}

func registrationAPI(directory, name string) bool {
	return directory == registrationPackage && name == registrationFunction ||
		directory == moduleRegistrationPackage && name == moduleRegistrationFunction
}

// predeclaredIdentifier requires retained universe ownership or an explicitly
// inferred universe type; unresolved syntax is never a predeclared binding.
func predeclaredIdentifier(source sourceExpr, name string) bool {
	identifier, found := source.value.(*ast.Ident)
	if !found {
		return false
	}
	if identifier.Name != name {
		return false
	}
	if source.universe != nil {
		return source.universe.Name() == name
	}
	binding := sourceBinding(source, identifier)
	if binding == nil {
		return false
	}
	return binding.kind == lexicalUniverse
}

// callableAliasBinding admits only a declaration whose corresponding initializer
// the existing expression.target can identify. Parameters, ranges, zero values
// and multi-result assignments are not a fallback to a same-named package value.
func callableAliasBinding(binding *lexicalDeclaration) bool {
	switch declaration := binding.node.(type) {
	case *ast.AssignStmt:
		return len(declaration.Lhs) == len(declaration.Rhs) && len(declaration.Rhs) != 0
	case *ast.ValueSpec:
		return len(declaration.Names) == len(declaration.Values) && len(declaration.Values) != 0
	}
	return false
}

// ImportedDirectory answers the committed directory the package operand of
// selector names in file, through the lexical import binding callableImport
// proves for the registration walk, so a reader outside this package resolves
// an aliased import the way the walk does. The directory is empty for a
// standard-library import. It answers false when the operand is not exactly one
// import of file: a local value, a binding that shadows the import, or a file
// this snapshot did not select.
func (s *Snapshot) ImportedDirectory(file SourceFile, selector *ast.SelectorExpr) (string, bool) {
	if s.lexical == nil {
		return "", false
	}
	selectedFile, retained := s.lexical.files[file.File]
	if !retained {
		return "", false
	}
	if selectedFile.Path != file.Path {
		return "", false
	}
	pkg, selected := s.packages[path.Dir(file.Path)]
	if !selected {
		return "", false
	}
	return s.evaluator().callableImport(sourceExpr{file: file.File, pkg: pkg}, selector)
}

// callableImport MUST prove the selector's lexical import binding before an
// empty directory can mean the existing external-standard-library boundary.
func (e *expression) callableImport(source sourceExpr, selector *ast.SelectorExpr) (string, bool) {
	owner, ok := selector.X.(*ast.Ident)
	if !ok {
		return "", false
	}
	binding := sourceBinding(source, owner)
	if binding == nil {
		return "", false
	}
	if binding.kind != lexicalImport {
		return "", false
	}
	directory, found := source.pkg.imports[source.file][owner.Name]
	if !found {
		return "", false
	}
	matches := 0
	for _, imported := range source.file.Imports {
		if binding.node != imported {
			continue
		}
		name, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			return "", false
		}
		target := ""
		first, _, _ := strings.Cut(name, "/")
		if internal, found := strings.CutPrefix(name, modulePrefix); found {
			target = internal
		} else if strings.Contains(first, ".") {
			target = "vendor/" + name
		}
		if target != directory {
			continue
		}
		alias := path.Base(name)
		if directory != "" {
			pkg, retained := e.snapshot.packages[directory]
			if !retained {
				return "", false
			}
			alias = pkg.name
		}
		if imported.Name != nil {
			alias = imported.Name.Name
		}
		if alias == owner.Name {
			matches++
		}
	}
	return directory, matches == 1
}
