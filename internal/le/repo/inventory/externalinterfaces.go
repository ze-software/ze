// Design: docs/architecture/plugin/feature-gates.md -- an interface receiver dispatches to the methods its value carries.
package repoinventory

import "go/ast"

// interfaceReceiver reports whether the receiver of a selector has a
// retained interface type, written inline or reached through named types and
// aliases. A receiver whose static root does not resolve, or whose type is
// a standard-library type, is not proved an interface and keeps the field
// rules.
func (e *externalEffects) interfaceReceiver(receiver sourceExpr, depth int) bool {
	datatype, reason := e.fieldReceiverType(receiver, depth+1)
	if reason != "" {
		return false
	}
	walk := externalTypeWalk{effects: e}
	for range 64 {
		switch ast.Unparen(datatype.value).(type) {
		case *ast.InterfaceType:
			return true
		case *ast.Ident, *ast.SelectorExpr:
			_, owner, reason := walk.definition(datatype)
			if reason != "" {
				return false
			}
			datatype = owner
		default:
			return false
		}
	}
	return false
}

// interfaceMethod is the method value x.M of an interface-typed receiver x.
// The approved contract has an interface receiver carry the entire capability
// union of its value, and every retained value carries its concrete type's
// methods (typeMethods), so the dynamic method is one of the methods named M
// the receiver's value holds. Each is bound to the receiver's value, as a
// concrete method call binds its receiver, and the call executes them all.
// A value holding no method named M can only be a nil interface, whose call
// panics before any body runs. The receiver's reads stay with the result.
func (e *externalEffects) interfaceMethod(selector *ast.SelectorExpr, receiver externalValue) externalValue {
	if receiver.unknown != "" {
		return externalValue{unknown: receiver.unknown, unknownAt: receiver.unknownAt}
	}
	result := externalValue{reads: receiver.reads}
	for node, source := range receiver.callables {
		method, declared := node.(*ast.FuncDecl)
		if !declared || method.Recv == nil || method.Name.Name != selector.Sel.Name {
			continue
		}
		for _, field := range method.Recv.List {
			for _, name := range field.Names {
				e.bind(name.Obj, receiver)
			}
		}
		if result.callables == nil {
			result.callables = make(map[ast.Node]sourceExpr)
		}
		result.callables[node] = source
	}
	return result
}
