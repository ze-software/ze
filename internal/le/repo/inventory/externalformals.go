// Design: docs/architecture/plugin/feature-gates.md -- complete retained caller equations.
package repoinventory

import "go/ast"

// externalFormal identifies a parameter by its parser object and declaring
// callable. Struct fields and result names never acquire parameter authority.
type externalFormal struct {
	owner  ast.Node
	source sourceExpr
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
							e.formals[name.Obj] = externalFormal{owner: node, source: source}
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
				call, called := node.(*ast.CallExpr)
				if !called {
					return true
				}
				callee := scope
				callee.value = call.Fun
				target, _, receiver := e.formalTarget(callee)
				if target == nil {
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
	if call.Ellipsis.IsValid() {
		e.formalInvalid[node] = "variadic caller provenance is unsupported"
		return
	}
	slots := 0
	for _, field := range signature.Params.List {
		if _, variadic := field.Type.(*ast.Ellipsis); variadic {
			e.formalInvalid[node] = "variadic parameter provenance is unsupported"
			return
		}
		slots += max(1, len(field.Names))
	}
	if slots != len(call.Args) {
		e.formalInvalid[node] = "argument slot provenance is unresolved"
		return
	}
	index := 0
	for _, field := range signature.Params.List {
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
		target, _, _ := e.formalTarget(source)
		if target != nil {
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
