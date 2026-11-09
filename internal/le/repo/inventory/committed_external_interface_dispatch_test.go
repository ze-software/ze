// Design: docs/architecture/plugin/feature-gates.md -- an interface receiver dispatches to the methods its value carries.
package repoinventory

import (
	"strings"
	"testing"
)

// interfaceDispatchTypes mirrors protobuf's MessageInfo.Desc: a struct field
// of a retained interface type, assigned a concrete value whose method the
// caller invokes through the field, as st.mi.Desc.FullName() does.
const interfaceDispatchTypes = "type naming interface { Name() string }\n" +
	"type message struct { label string }\n" +
	"func (m *message) Name() string { BODY; return m.label }\n" +
	"type info struct { desc naming }\n"

// interfaceDispatchCall makes the interface call inside a callback sort.Slice
// runs, as protobuf's validator runs inside external code's call: the
// registration walk of the initializer itself still refuses an interface call
// (TestCommittedInterfaceReceiverCannotBorrowConcreteMethod).
const interfaceDispatchCall = "order.Slice([]int{0}, func(a, b int) bool { _ = i.desc.Name(); return a < b })"

// TestCommittedExternalInterfaceReceiverDispatches requires a method called
// on an interface-typed receiver to run the methods named so that its value
// carries, bound to that value. Method: an info holds a *message in its
// naming field and a callback calls i.desc.Name(); an inert Name publishes, a
// Name that installs refuses as registration, so the body ran. A field the
// init never assigns carries no method named Name, so the call has no
// candidate and refuses rather than reading the nil interface as inert.
//
// MUTATION: (internal/le/repo/inventory/externalexpressions.go,
// `		if e.interfaceReceiver(sourceExpr{value: value.X, file: source.file, pkg: source.pkg}, depth) {`,
// `		if e.interfaceReceiver(sourceExpr{value: value.X, file: source.file, pkg: source.pkg}, depth) && false {`):
// the method is projected as a field of an interface, and every case refuses
// on root field ownership, the actual-root refusal at protobuf validate.go:525.
// MUTATION: (internal/le/repo/inventory/externalinterfaces.go,
// `		result.callables[node] = source`, `		_ = source`): the call
// has no candidate, and the inert case refuses as callable ownership is
// unresolved.
// MUTATION: (internal/le/repo/inventory/externalformals.go,
// `					for _, method := range methods[selector.Sel.Name] {`,
// `					for _, method := range methods[selector.Sel.Name][:0] {`):
// the interface call indexes no caller of Name, its receiver formal m has no
// grounded provenance, and the inert case refuses.
// MUTATION: (internal/le/repo/inventory/externalinterfaces.go,
// `		if !declared || method.Recv == nil || method.Name.Name != selector.Sel.Name {`,
// `		if !declared || method.Recv == nil {`): every method of the value
// runs, so a sibling that installs refuses the inert Name case.
func TestCommittedExternalInterfaceReceiverDispatches(t *testing.T) {
	const sibling = "func (m *message) Arm() { install(0) }\n"
	for _, shape := range []struct{ name, body, extra, call, reason string }{
		{"inert", "", sibling, "i := &info{desc: &message{label: \"a\"}}; " + interfaceDispatchCall, ""},
		{"installing", "install(0)", "", "i := &info{desc: &message{label: \"a\"}}; " + interfaceDispatchCall, "registration"},
		{"nil interface", "install(0)", "", "i := &info{}; " + interfaceDispatchCall, "callable ownership is unresolved"},
	} {
		declarations := strings.Replace(interfaceDispatchTypes, "BODY", shape.body, 1) + shape.extra
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(shape.name+"/"+kind, func(t *testing.T) {
				fixture := externalEffectsFixture(t, kind)
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(declarations, shape.call))
				commitInventoryFixture(t, fixture.root)
				if shape.reason == "" {
					externalEffectsPositive(t, fixture)
					return
				}
				externalEffectsRefusal(t, fixture, shape.reason)
			})
		}
	}
}
