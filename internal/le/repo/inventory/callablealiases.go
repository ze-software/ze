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
			binding := value.Obj
			if binding == nil {
				binding = callablePackageBinding(source.pkg, value.Name)
			}
			if binding == nil {
				switch types.Universe.Lookup(value.Name).(type) {
				case *types.Builtin, *types.TypeName:
					return source, false, nil
				}
				return sourceExpr{}, false, r.eval.problem(origin, "callable identifier "+value.Name+" has unresolved execution ownership")
			}
			switch binding.Kind {
			case ast.Fun:
				if _, found := source.pkg.functions[value.Name]; !found {
					return sourceExpr{}, false, r.eval.problem(origin, "callable function "+value.Name+" has unresolved execution ownership")
				}
				if aliased && registrationAPI(source.pkg.dir, value.Name) {
					return sourceExpr{}, false, r.eval.problem(origin, "callable registration alias has unresolved execution ownership")
				}
				return source, true, nil
			case ast.Typ:
				return source, false, nil
			case ast.Var:
				if !callableAliasBinding(binding) {
					return sourceExpr{}, false, r.eval.problem(origin, "callable parameter or variable "+value.Name+" has unresolved execution ownership")
				}
			default:
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
			if _, found := pkg.functions[value.Sel.Name]; found {
				return source, true, nil
			}
			if binding := callablePackageBinding(pkg, value.Sel.Name); binding != nil && binding.Kind == ast.Typ {
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
	return directory == registrationPackage && name == "Register" ||
		directory == "internal/component/config/yang" && name == "RegisterModule"
}

// callablePackageBinding also finds uninitialized package variables and type
// names, so either can shadow the language universe without becoming a builtin.
func callablePackageBinding(pkg *sourcePackage, name string) *ast.Object {
	for _, source := range pkg.files {
		if source.File.Scope == nil {
			continue
		}
		if binding := source.File.Scope.Lookup(name); binding != nil {
			return binding
		}
	}
	return nil
}

// predeclaredIdentifier excludes both lexical and cross-file package shadows.
// A parser object alone cannot identify the language universe across files.
func predeclaredIdentifier(source sourceExpr, name string) bool {
	identifier, found := source.value.(*ast.Ident)
	if !found {
		return false
	}
	if identifier.Name != name {
		return false
	}
	if identifier.Obj != nil {
		return false
	}
	return callablePackageBinding(source.pkg, name) == nil
}

// callableAliasBinding admits only a declaration whose corresponding initializer
// the existing expression.target can identify. Parameters, ranges, zero values
// and multi-result assignments are not a fallback to a same-named package value.
func callableAliasBinding(binding *ast.Object) bool {
	switch declaration := binding.Decl.(type) {
	case *ast.AssignStmt:
		return len(declaration.Lhs) == len(declaration.Rhs) && len(declaration.Rhs) != 0
	case *ast.ValueSpec:
		return len(declaration.Names) == len(declaration.Values) && len(declaration.Values) != 0
	}
	return false
}

// callableImport MUST prove the selector's lexical import binding before an
// empty directory can mean the existing external-standard-library boundary.
func (e *expression) callableImport(source sourceExpr, selector *ast.SelectorExpr) (string, bool) {
	owner, ok := selector.X.(*ast.Ident)
	if !ok {
		return "", false
	}
	if owner.Obj != nil && owner.Obj.Kind != ast.Pkg {
		return "", false
	}
	directory, found := source.pkg.imports[source.file][owner.Name]
	if !found {
		return "", false
	}
	matches := 0
	for _, imported := range source.file.Imports {
		if owner.Obj != nil && owner.Obj.Decl != imported {
			continue
		}
		name, err := strconv.Unquote(imported.Path.Value)
		if err != nil {
			return "", false
		}
		target := ""
		first, _, _ := strings.Cut(name, "/")
		if strings.HasPrefix(name, modulePrefix) {
			target = strings.TrimPrefix(name, modulePrefix)
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
