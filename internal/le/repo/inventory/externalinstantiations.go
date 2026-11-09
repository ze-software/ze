// Design: docs/architecture/plugin/feature-gates.md -- a receiver type parameter reflects as the types the program instantiates it with.
// Related: externaleffecttypeidentity.go -- the lexical scopes that bind a method's receiver type parameters.
// Related: externalreflectiontypes.go -- the reflection walk that asks for an unbound receiver parameter.
package repoinventory

import (
	"go/ast"
	"slices"
)

// externalReceiverParameter names one type parameter of a generic type, read
// where no instantiation binds it: declared by the type itself (definition),
// or by a method in its receiver (method). It holds the declaring package and
// file, and its position in the type's parameter list.
type externalReceiverParameter struct {
	definition *ast.TypeSpec
	method     *ast.FuncDecl
	file       *ast.File
	pkg        *sourcePackage
	position   int
}

// declaration answers the generic type the parameter belongs to.
func (parameter externalReceiverParameter) declaration(walk *externalTypeWalk) (*ast.TypeSpec, string) {
	if parameter.definition != nil {
		return parameter.definition, ""
	}
	base, indices := externalReceiverBase(parameter.method)
	if len(indices) <= parameter.position {
		return nil, "receiver type parameter base is unresolved"
	}
	definition, _, reason := walk.definition(sourceExpr{value: base, file: parameter.file, pkg: parameter.pkg})
	return definition, reason
}

// externalInstantiationSite is one written instantiation D[A...] of a
// retained generic type: the index expression and where it is written.
type externalInstantiationSite struct {
	base      ast.Expr
	arguments []ast.Expr
	file      *ast.File
	pkg       *sourcePackage
}

// instantiationSites indexes every written instantiation in the retained
// source by the spelled name of its base type. It is built once, the first
// time a receiver parameter is reflected without an argument, and holds
// syntax only: which declaration a site names is resolved per query.
func (e *externalEffects) instantiationSites() map[string][]externalInstantiationSite {
	if e.instantiations != nil {
		return e.instantiations
	}
	e.instantiations = make(map[string][]externalInstantiationSite)
	directories := make([]string, 0, len(e.source.snapshot.packages))
	for directory := range e.source.snapshot.packages {
		directories = append(directories, directory)
	}
	slices.Sort(directories)
	for _, directory := range directories {
		pkg := e.source.snapshot.packages[directory]
		for _, file := range pkg.files {
			ast.Inspect(file.File, func(node ast.Node) bool {
				var base ast.Expr
				var arguments []ast.Expr
				switch index := node.(type) {
				case *ast.IndexExpr:
					base, arguments = index.X, []ast.Expr{index.Index}
				case *ast.IndexListExpr:
					base, arguments = index.X, index.Indices
				default:
					return true
				}
				var name string
				switch spelled := base.(type) {
				case *ast.Ident:
					name = spelled.Name
				case *ast.SelectorExpr:
					name = spelled.Sel.Name
				default:
					return true
				}
				e.instantiations[name] = append(e.instantiations[name], externalInstantiationSite{base: base, arguments: arguments, file: file.File, pkg: pkg})
				return true
			})
		}
	}
	return e.instantiations
}

// receiverInstantiations answers the type arguments the program writes for
// one type parameter of a generic type: the argument at its position in every
// instantiation of that type. A value of type T in the
// method has one of those types as its dynamic type, so their descriptors
// bound what reflecting T reveals. A site whose argument is the type's own
// parameter at the same position (a receiver, or the type naming itself
// inside its methods or declaration) instantiates nothing new and is skipped.
// The index also holds expressions that index a value rather than a type;
// they never resolve to the declaration and drop out.
func (e *externalEffects) receiverInstantiations(parameter externalReceiverParameter) ([]externalTypeArgument, string) {
	walk := externalTypeWalk{effects: e}
	definition, reason := parameter.declaration(&walk)
	if reason != "" {
		return nil, reason
	}
	var result []externalTypeArgument
	for _, site := range e.instantiationSites()[definition.Name.Name] {
		if len(site.arguments) <= parameter.position {
			continue
		}
		named, _, reason := walk.definition(sourceExpr{value: site.base, file: site.file, pkg: site.pkg})
		if reason != "" || named != definition {
			continue
		}
		argument := sourceExpr{value: site.arguments[parameter.position], file: site.file, pkg: site.pkg}
		scope := e.typeIdentities.lexicalScope(argument)
		if name, spelled := argument.value.(*ast.Ident); spelled {
			if binding, bound := scope.binding(name); bound && binding.receiver != nil {
				if externalSameReceiverParameter(*binding.receiver, parameter, definition, &walk) {
					continue
				}
			}
		}
		result = append(result, externalTypeArgument{source: argument, scope: scope})
	}
	return result, ""
}

// externalSameReceiverParameter reports whether two parameters are the same
// position of one generic declaration: Deque[T] written inside a method of
// Deque or inside Deque's own declaration, whose T is that declaration's.
func externalSameReceiverParameter(written, asked externalReceiverParameter, definition *ast.TypeSpec, walk *externalTypeWalk) bool {
	if written.position != asked.position {
		return false
	}
	named, reason := written.declaration(walk)
	return reason == "" && named == definition
}
