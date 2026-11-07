// Design: docs/architecture/plugin/feature-gates.md -- the storage an address points into.
// Related: externalexpressions.go -- the unary address arm and the implicitly addressed method receiver that ask for it.
package repoinventory

import "go/ast"

// addressValue is the value of &operand, written at site: the operand's value,
// marked a reference, joined with the storage it points into, so an external
// callee or a pointer method that writes through it writes that storage's
// locations. Both an explicit &x and a pointer method called on an
// addressable x (methodReceiver) take their address here.
func (e *externalEffects) addressValue(operand, site sourceExpr, possible bool, depth int) externalValue {
	result := e.value(operand, possible, depth+1)
	result.references = true
	// A literal's address is a fresh allocation no binding names yet; the
	// value it initializes carries its capabilities.
	if _, literal := operand.value.(*ast.CompositeLit); literal {
		return result
	}
	storage, reason := e.addressStorage(operand, possible, depth+1)
	if reason != "" {
		// This reason replaces the operand's, so its site does too.
		result.unknown = "address ownership is unresolved: " + reason
		result.unknownAt = site
		return result
	}
	result.join(storage)
	return result
}

// addressStorage answers the locations that &operand points into, in the one
// location model the evaluator keeps: a retained variable is one location,
// and every field and element stored inline in it belongs to it.
//
// &x is x's location. &s.f, &a[i] and &*p point into their holder's storage:
// the holder's own location when it is a variable path (a struct or array
// value), and the locations its value refers to when it is a pointer or a
// slice. The two are joined rather than chosen by static type, because an
// anonymous pointee (p := &T{}) has no location of its own and the pointer
// variable is the location its field reads are recorded against, as new
// already makes it. A holder that yields neither refuses: a write through
// the address would land on storage no binding names, which is the same
// answer an assignment through that holder gives.
// Bounded at 128 source steps.
func (e *externalEffects) addressStorage(source sourceExpr, possible bool, depth int) (externalValue, string) {
	if depth >= 128 {
		return externalValue{}, "addressed operand exceeds 128 source steps"
	}
	if object := e.object(source); object != nil {
		return externalLocation(object), ""
	}
	var holder ast.Expr
	switch operand := source.value.(type) {
	case *ast.ParenExpr:
		return e.addressStorage(sourceExpr{value: operand.X, file: source.file, pkg: source.pkg}, possible, depth+1)
	case *ast.SelectorExpr:
		if _, imported := e.resolver.callableImport(source, operand); imported {
			return externalValue{}, "imported declaration ownership is unresolved"
		}
		holder = operand.X
	case *ast.IndexExpr:
		holder = operand.X
	case *ast.StarExpr:
		holder = operand.X
	default:
		return externalValue{}, "operand is not a variable, field or element"
	}
	held := sourceExpr{value: holder, file: source.file, pkg: source.pkg}
	storage := externalValue{references: true}
	// An unresolved inline root is not an answer by itself: a pointer or a
	// slice holder still names its storage through its value below.
	if inline, reason := e.addressStorage(held, possible, depth+1); reason == "" {
		storage.join(inline)
	}
	referred := e.value(held, possible, depth+1)
	if referred.unknown != "" {
		return externalValue{}, referred.unknown
	}
	storage.join(externalValue{locations: referred.locations})
	if len(storage.locations) == 0 {
		return externalValue{}, "addressed storage has no retained location"
	}
	return storage, ""
}
