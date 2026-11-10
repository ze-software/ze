// Design: docs/architecture/plugin/feature-gates.md -- complete retained caller equations.
package repoinventory

import (
	"go/ast"
)

// externalFormal identifies a parameter by its parser object and declaring
// callable. Struct fields and result names never acquire parameter authority.
type externalFormal struct {
	owner  ast.Node
	source sourceExpr
	// receiver marks a method receiver, which a method value binds.
	receiver bool
}

// indexFormalSources records dormant callers without executing their bodies.
// Their argument equations retain the caller's execution owner, just like other
// retained assignments. An indirect use leaves an explicit census obligation.
func (e *externalEffects) indexFormalSources(directories []string) {
	methods := make(map[string][]ast.Node)
	for _, directory := range directories {
		pkg := e.source.snapshot.packages[directory]
		for _, file := range pkg.files {
			scope := sourceExpr{file: file.File, pkg: pkg}
			ast.Inspect(file.File, func(node ast.Node) bool {
				if node == nil {
					return false
				}
				signature, receiver := externalFormalSignature(node)
				if signature == nil {
					return true
				}
				if receiver != nil {
					function := node.(*ast.FuncDecl)
					methods[function.Name.Name] = append(methods[function.Name.Name], node)
				}
				for _, fields := range []*ast.FieldList{signature.Params, receiver} {
					if fields == nil {
						continue
					}
					for _, field := range fields.List {
						for _, name := range field.Names {
							source := scope
							source.value = name
							e.formals[name.Obj] = externalFormal{owner: node, source: source, receiver: fields == receiver}
						}
					}
				}
				return true
			})
		}
	}
	for _, directory := range directories {
		pkg := e.source.snapshot.packages[directory]
		for _, file := range pkg.files {
			scope := sourceExpr{file: file.File, pkg: pkg}
			direct := make(map[ast.Expr]bool)
			var owner ast.Node
			var owners []ast.Node
			ast.Inspect(file.File, func(node ast.Node) bool {
				if node == nil {
					owner = owners[len(owners)-1]
					owners = owners[:len(owners)-1]
					return false
				}
				owners = append(owners, owner)
				switch node.(type) {
				case *ast.FuncDecl, *ast.FuncLit:
					owner = node
				}
				// The declaration of a local closure is not a use of it
				// as a value: its literal and its declaring name are the
				// binding the direct calls through the name resolve to.
				if statement, assigned := node.(*ast.AssignStmt); assigned {
					for _, left := range statement.Lhs {
						name, named := left.(*ast.Ident)
						if !named {
							continue
						}
						literal, bound := closureBinding(name)
						if !bound {
							continue
						}
						if name.Obj.Decl == statement {
							direct[name] = true
							direct[literal] = true
						}
					}
				}
				call, called := node.(*ast.CallExpr)
				if !called {
					return true
				}
				callee := scope
				callee.value = call.Fun
				target, _, receiver := e.formalTarget(callee)
				if target == nil {
					// A call through an interface receiver runs a method of
					// that name the receiver's value carries (interfaceMethod),
					// so each is a direct callee of this call with this
					// receiver and these arguments. An extra caller only adds
					// provenance a proof must ground.
					selector, selected := call.Fun.(*ast.SelectorExpr)
					if !selected || !e.interfaceReceiver(sourceExpr{value: selector.X, file: scope.file, pkg: scope.pkg}, 0) {
						return true
					}
					for _, method := range methods[selector.Sel.Name] {
						e.indexFormalCall(method, selector.X, call, scope, owner)
					}
					direct[call.Fun] = true
					return true
				}
				e.indexFormalCall(target, receiver, call, scope, owner)
				for value := call.Fun; ; {
					direct[value] = true
					parenthesized, ok := value.(*ast.ParenExpr)
					if !ok {
						break
					}
					value = parenthesized.X
				}
				return true
			})
			e.indexFormalReferences(scope, direct, methods)
		}
	}
}

func externalFormalSignature(node ast.Node) (*ast.FuncType, *ast.FieldList) {
	switch function := node.(type) {
	case *ast.FuncDecl:
		return function.Type, function.Recv
	case *ast.FuncLit:
		return function.Type, nil
	}
	return nil, nil
}

func (e *externalEffects) formalTarget(source sourceExpr) (ast.Node, sourceExpr, ast.Expr) {
	for {
		parenthesized, ok := source.value.(*ast.ParenExpr)
		if !ok {
			break
		}
		source.value = parenthesized.X
	}
	if literal, ok := source.value.(*ast.FuncLit); ok {
		return literal, source, nil
	}
	// A local closure name := func(...) {...} is called directly through
	// its name, as zstd's fatalErr(binary.Read(...)) is. Any other use of
	// the name is a reference indexFormalReferences counts as indirect.
	if name, ok := source.value.(*ast.Ident); ok {
		if literal, bound := closureBinding(name); bound {
			return literal, sourceExpr{value: literal, file: source.file, pkg: source.pkg}, nil
		}
	}
	function, found := e.resolver.function(source)
	if !found {
		return nil, sourceExpr{}, nil
	}
	var receiver ast.Expr
	if function.decl.Recv != nil {
		selector, ok := source.value.(*ast.SelectorExpr)
		if !ok {
			return nil, sourceExpr{}, nil
		}
		receiver = selector.X
	}
	return function.decl, sourceExpr{value: function.decl.Name, file: function.file, pkg: function.pkg}, receiver
}

func (e *externalEffects) indexFormalCall(node ast.Node, receiver ast.Expr, call *ast.CallExpr, caller sourceExpr, owner ast.Node) {
	signature, receivers := externalFormalSignature(node)
	if signature == nil {
		e.formalInvalid[node] = "callable signature is unavailable"
		return
	}
	if signature.TypeParams.NumFields() != 0 {
		e.formalInvalid[node] = "generic caller provenance is unsupported"
		return
	}
	spread := call.Ellipsis.IsValid()
	slots := 0
	var variadic *ast.Field
	for _, field := range signature.Params.List {
		if _, final := field.Type.(*ast.Ellipsis); final {
			variadic = field
			continue
		}
		slots += max(1, len(field.Names))
	}
	if reason := externalVariadicSlots(slots, len(call.Args), variadic != nil, spread); reason != "" {
		e.formalInvalid[node] = reason
		return
	}
	index := 0
	for _, field := range signature.Params.List {
		if field == variadic {
			// The final ...T parameter takes every remaining argument.
			e.indexVariadicFormal(field, call.Args[index:], spread, caller, owner)
			break
		}
		for _, name := range field.Names {
			source := caller
			source.value = call.Args[index]
			e.definitions[name.Obj] = append(e.definitions[name.Obj], externalDefinition{source: source, owner: owner})
			index++
		}
		if len(field.Names) == 0 {
			index++
		}
	}
	if receivers == nil {
		return
	}
	if receiver == nil {
		e.formalInvalid[node] = "method receiver provenance is unresolved"
		return
	}
	for _, field := range receivers.List {
		for _, name := range field.Names {
			source := caller
			source.value = receiver
			e.definitions[name.Obj] = append(e.definitions[name.Obj], externalDefinition{source: source, owner: owner})
		}
	}
}

// externalVariadicSlots refuses a call whose arguments do not fill the
// callee's parameters: slots counts the parameters before a final ...T one.
// Without one, the arguments fill the slots exactly and a spread has no
// parameter to pass. With one, a spread passes exactly one slice after the
// slots, and otherwise any number of arguments may follow them.
func externalVariadicSlots(slots, arguments int, variadic, spread bool) string {
	if !variadic {
		if spread {
			return "variadic caller provenance is unsupported"
		}
		if slots != arguments {
			return "argument slot provenance is unresolved"
		}
		return ""
	}
	if spread {
		if arguments != slots+1 {
			return "variadic caller provenance is unsupported"
		}
		return ""
	}
	if arguments < slots {
		return "argument slot provenance is unresolved"
	}
	return ""
}

// indexVariadicFormal records what a direct call passes to the callee's final
// ...T parameter. Go specification, "Passing arguments to ... parameters":
// "If the final argument is assignable to a slice type []T and is followed by
// ..., it is passed unchanged as the value for a ...T parameter." So a spread
// argument is the parameter's definition, exactly as a []T argument is for a
// []T parameter. "Otherwise, the value passed is a new slice of type []T with
// a new underlying array whose successive elements are the actual arguments,
// which all must be assignable to T." That slice is what a literal []T{a, b}
// builds, so its provenance is its elements', recorded as proof sources
// (formalElements) rather than as a definition of the slice: a definition is
// also read as the parameter's value, and one element is not the slice.
func (e *externalEffects) indexVariadicFormal(field *ast.Field, arguments []ast.Expr, spread bool, caller sourceExpr, owner ast.Node) {
	elements := make([]sourceExpr, 0, len(arguments))
	for _, argument := range arguments {
		source := caller
		source.value = argument
		elements = append(elements, source)
	}
	for _, name := range field.Names {
		if spread {
			e.definitions[name.Obj] = append(e.definitions[name.Obj], externalDefinition{source: elements[0], owner: owner})
			continue
		}
		e.formalElements[name.Obj] = append(e.formalElements[name.Obj], elements)
	}
}

// indexFormalReferences counts uses as values separately from direct calls.
// An unresolved selector cannot silently disappear from a method's caller census.
func (e *externalEffects) indexFormalReferences(scope sourceExpr, direct map[ast.Expr]bool, methods map[string][]ast.Node) {
	var parents []ast.Node
	ast.Inspect(scope.file, func(node ast.Node) bool {
		if node == nil {
			parents = parents[:len(parents)-1]
			return false
		}
		var parent ast.Node
		if len(parents) != 0 {
			parent = parents[len(parents)-1]
		}
		parents = append(parents, node)
		expression, value := node.(ast.Expr)
		if !value {
			return true
		}
		if direct[expression] {
			return true
		}
		if !externalFormalReference(expression, parent) {
			return true
		}
		source := scope
		source.value = expression
		target, _, receiver := e.formalTarget(source)
		if target != nil {
			if receiver != nil {
				// A method value x.M binds M's receiver to x where it is
				// taken, as a call x.M() does; only its parameters come
				// from the unseen callers of the value.
				e.indexFormalMethodValue(target, receiver, source, externalFormalOwner(parents))
				return true
			}
			e.formalIndirect[target] = source
			return true
		}
		selector, selected := expression.(*ast.SelectorExpr)
		if !selected {
			return true
		}
		if _, imported := e.resolver.callableImport(source, selector); imported {
			return true
		}
		for _, method := range methods[selector.Sel.Name] {
			e.formalIndirect[method] = source
		}
		return true
	})
}

// indexFormalMethodValue records the receiver a method value binds, with the
// same equation a direct call writes, and marks its parameters unbound.
func (e *externalEffects) indexFormalMethodValue(node ast.Node, receiver ast.Expr, value sourceExpr, owner ast.Node) {
	e.formalUnbound[node] = value
	_, receivers := externalFormalSignature(node)
	if receivers == nil {
		e.formalInvalid[node] = "method receiver provenance is unresolved"
		return
	}
	for _, field := range receivers.List {
		for _, name := range field.Names {
			source := value
			source.value = receiver
			e.definitions[name.Obj] = append(e.definitions[name.Obj], externalDefinition{source: source, owner: owner})
		}
	}
}

// externalFormalOwner answers the innermost function among parents, the
// execution owner of a reference inside it; nil at package level.
func externalFormalOwner(parents []ast.Node) ast.Node {
	for index := len(parents) - 1; index >= 0; index-- {
		switch parents[index].(type) {
		case *ast.FuncDecl, *ast.FuncLit:
			return parents[index]
		}
	}
	return nil
}

func externalFormalReference(expression ast.Expr, parent ast.Node) bool {
	switch value := expression.(type) {
	case *ast.FuncLit, *ast.SelectorExpr:
		return true
	case *ast.Ident:
		switch enclosing := parent.(type) {
		case *ast.FuncDecl:
			return enclosing.Name != value
		case *ast.SelectorExpr:
			return enclosing.Sel != value
		case *ast.KeyValueExpr:
			return enclosing.Key != value
		case *ast.Field, *ast.TypeSpec:
			return false
		}
		return true
	}
	return false
}
