// Design: docs/architecture/plugin/feature-gates.md -- lexical identity is independent of source population
// Related: enumeration.go -- registrar roots and const-block judgement

package archenumeration

import (
	"go/ast"
	"go/token"

	repoinventory "github.com/ze-software/ze/internal/le/repo/inventory"
)

// registrationExpression retains the file whose lexical index owns expression.
type registrationExpression struct {
	expression ast.Expr
	file       *ast.File
}

// registrationValueGraph follows initializer and alias edges within one parsed
// package. Bindings are identities, including locals; names never identify an
// edge. A missing initializer is unknown and establishes no declaration credit.
type registrationValueGraph struct {
	bindings     *repoinventory.ParsedBindings
	initializers map[*repoinventory.ParsedBinding][]registrationExpression
	aliases      map[*repoinventory.ParsedBinding][]registrationExpression
	written      map[*repoinventory.ParsedBinding]bool
}

func newRegistrationValues(files []*ast.File) *registrationValueGraph {
	values := &registrationValueGraph{
		bindings: repoinventory.NewParsedBindings(files, repoinventory.ParsedBindingOptions{
			CompositeKeys: registrationCompositeKeys,
		}),
		initializers: map[*repoinventory.ParsedBinding][]registrationExpression{},
		aliases:      map[*repoinventory.ParsedBinding][]registrationExpression{},
		written:      map[*repoinventory.ParsedBinding]bool{},
	}
	for _, file := range files {
		ast.Inspect(file, func(node ast.Node) bool {
			switch typed := node.(type) {
			case *ast.ValueSpec:
				for index, name := range typed.Names {
					if len(typed.Names) == len(typed.Values) {
						values.define(file, name, typed.Values[index])
						continue
					}
					if binding := values.target(file, name); binding != nil {
						for _, input := range typed.Values {
							values.aliases[binding] = append(values.aliases[binding],
								registrationExpression{expression: input, file: file})
						}
					}
				}
			case *ast.AssignStmt:
				values.assignment(file, typed)
			case *ast.RangeStmt:
				if typed.Tok == token.DEFINE {
					if name, named := typed.Value.(*ast.Ident); named {
						values.define(file, name, typed.X)
					}
				} else {
					values.modify(file, typed.Key)
					values.modify(file, typed.Value)
				}
			case *ast.IncDecStmt:
				values.modify(file, typed.X)
			}
			return true
		})
	}
	values.invalidateWrites()
	return values
}

func (values *registrationValueGraph) define(file *ast.File, name *ast.Ident, expression ast.Expr) {
	binding, known := values.bindings.Binding(file, name)
	if !known {
		return
	}
	if binding == nil {
		return
	}
	source := registrationExpression{expression: expression, file: file}
	values.initializers[binding] = append(values.initializers[binding], source)
	if binding.Kind() == repoinventory.ParsedBindingVariable {
		values.aliases[binding] = append(values.aliases[binding], source)
	}
}

func (values *registrationValueGraph) assignment(file *ast.File, assignment *ast.AssignStmt) {
	for index, target := range assignment.Lhs {
		binding := values.target(file, target)
		if binding == nil {
			continue
		}
		if assignment.Tok == token.DEFINE {
			if name, named := target.(*ast.Ident); named {
				if binding.Ident() == name {
					if binding.Node() == assignment {
						if len(assignment.Lhs) == len(assignment.Rhs) {
							values.define(file, name, assignment.Rhs[index])
						}
						continue
					}
				}
			}
		}
		var inputs []ast.Expr
		if len(assignment.Lhs) == len(assignment.Rhs) {
			inputs = []ast.Expr{assignment.Rhs[index]}
		} else {
			inputs = assignment.Rhs
		}
		for _, input := range inputs {
			values.aliases[binding] = append(values.aliases[binding], registrationExpression{expression: input, file: file})
		}
		values.written[binding] = true
	}
}

// target resolves the base variable a write reaches. Proven nonbindings, such
// as _, and unresolved ownership yield no variable, never a spelling fallback.
func (values *registrationValueGraph) target(file *ast.File, target ast.Expr) *repoinventory.ParsedBinding {
	for target != nil {
		switch typed := target.(type) {
		case *ast.Ident:
			binding, known := values.bindings.Binding(file, typed)
			if !known {
				return nil
			}
			if binding == nil {
				return nil
			}
			if binding.Kind() != repoinventory.ParsedBindingVariable {
				return nil
			}
			return binding
		case *ast.SelectorExpr:
			target = typed.X
		case *ast.IndexExpr:
			target = typed.X
		case *ast.StarExpr:
			target = typed.X
		case *ast.ParenExpr:
			target = typed.X
		default:
			return nil
		}
	}
	return nil
}

func (values *registrationValueGraph) modify(file *ast.File, target ast.Expr) {
	if binding := values.target(file, target); binding != nil {
		values.written[binding] = true
	}
}

// invalidateWrites visits each variable binding once. Constant edges are
// excluded, so a mutated table cannot poison an independent table that uses
// the same constants. The file walk includes function-literal initializers.
func (values *registrationValueGraph) invalidateWrites() {
	pending := make([]*repoinventory.ParsedBinding, 0, len(values.written))
	for binding := range values.written {
		pending = append(pending, binding)
	}
	seen := map[*repoinventory.ParsedBinding]bool{}
	for len(pending) > 0 {
		last := len(pending) - 1
		binding := pending[last]
		pending = pending[:last]
		if seen[binding] {
			continue
		}
		seen[binding] = true
		values.written[binding] = true
		for _, source := range values.aliases[binding] {
			ast.Inspect(source.expression, func(node ast.Node) bool {
				name, named := node.(*ast.Ident)
				if !named {
					return true
				}
				alias, known := values.bindings.Binding(source.file, name)
				if !known {
					return true
				}
				if alias == nil {
					return true
				}
				if alias.Kind() == repoinventory.ParsedBindingVariable {
					pending = append(pending, alias)
				}
				return true
			})
		}
	}
}

// constants keeps unresolved expressions opaque and follows each declaration
// identity at most once, bounding aliases and cycles by the parsed package.
func (values *registrationValueGraph) constants(roots []registrationExpression) map[*ast.Ident]bool {
	used := map[*ast.Ident]bool{}
	seen := map[*repoinventory.ParsedBinding]bool{}
	pending := append([]registrationExpression(nil), roots...)
	for len(pending) > 0 {
		last := len(pending) - 1
		source := pending[last]
		pending = pending[:last]
		switch typed := source.expression.(type) {
		case *ast.Ident:
			binding, known := values.bindings.Binding(source.file, typed)
			if !known {
				continue
			}
			if binding == nil {
				continue
			}
			if seen[binding] {
				continue
			}
			seen[binding] = true
			if values.written[binding] {
				continue
			}
			if binding.Kind() == repoinventory.ParsedBindingConstant {
				if binding.Package() {
					used[binding.Ident()] = true
				}
			}
			pending = append(pending, values.initializers[binding]...)
		case *ast.CompositeLit:
			for _, element := range typed.Elts {
				pending = append(pending, registrationExpression{expression: element, file: source.file})
			}
		case *ast.KeyValueExpr:
			pending = append(pending,
				registrationExpression{expression: typed.Key, file: source.file},
				registrationExpression{expression: typed.Value, file: source.file})
		case *ast.CallExpr:
			name, named := typed.Fun.(*ast.Ident)
			if !named {
				continue
			}
			if name.Name != "append" {
				continue
			}
			binding, known := values.bindings.Binding(source.file, name)
			if !known {
				continue
			}
			if binding == nil {
				continue
			}
			if binding.Kind() != repoinventory.ParsedBindingUniverse {
				continue
			}
			for _, argument := range typed.Args {
				pending = append(pending, registrationExpression{expression: argument, file: source.file})
			}
		case *ast.ParenExpr:
			pending = append(pending, registrationExpression{expression: typed.X, file: source.file})
		case *ast.UnaryExpr:
			if typed.Op == token.AND {
				pending = append(pending, registrationExpression{expression: typed.X, file: source.file})
			}
		}
	}
	return used
}

// registrationCompositeKeys projects only exact package-local TypeSpecs.
// Imported, ambiguous and cyclic shapes remain unknown and grant no key credit.
func registrationCompositeKeys(view *repoinventory.ParsedBindings, file *ast.File, literal *ast.CompositeLit) repoinventory.ParsedCompositeKeys {
	datatype := literal.Type
	seen := map[*repoinventory.ParsedBinding]bool{}
	for datatype != nil {
		switch typed := datatype.(type) {
		case *ast.MapType, *ast.ArrayType:
			return repoinventory.ParsedKeysExpressions
		case *ast.StructType:
			return repoinventory.ParsedKeysFields
		case *ast.ParenExpr:
			datatype = typed.X
		case *ast.Ident:
			binding, known := view.Binding(file, typed)
			if !known {
				return repoinventory.ParsedKeysUnknown
			}
			if binding == nil {
				return repoinventory.ParsedKeysUnknown
			}
			if !binding.Package() {
				return repoinventory.ParsedKeysUnknown
			}
			if binding.Kind() != repoinventory.ParsedBindingType {
				return repoinventory.ParsedKeysUnknown
			}
			if seen[binding] {
				return repoinventory.ParsedKeysUnknown
			}
			seen[binding] = true
			declaration, named := binding.Node().(*ast.TypeSpec)
			if !named {
				return repoinventory.ParsedKeysUnknown
			}
			file, datatype = binding.File(), declaration.Type
		default:
			return repoinventory.ParsedKeysUnknown
		}
	}
	return repoinventory.ParsedKeysUnknown
}
