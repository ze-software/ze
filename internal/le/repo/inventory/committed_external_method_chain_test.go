// Design: docs/architecture/plugin/feature-gates.md -- a method resolves on any receiver whose static type is derived.
package repoinventory

import (
	"strings"
	"testing"
)

// methodChainPointer is the vendored protobuf pointer shape of
// message_opaque.go: a value method returns the receiver's own type, and a
// second value method returns a separate value type whose method reads a
// field, as p.Apply(mi.presenceOffset).PresenceInfo().Present(index) does.
const methodChainPointer = "type pointer struct { base *int }\n" +
	"func (p pointer) Apply(offset int) pointer { _ = offset; return p }\n"

const methodChainPresence = "type presence struct { size int }\n"

const methodChainPresent = "func (q presence) Present(index int) bool { return index == q.size }\n"

const methodChainCall = "x := pointer{base: nil}; _ = x.Apply(1).Info().Present(0)"

// TestCommittedExternalMethodChainPublishes requires a method selected on a
// call result or a literal to resolve to its declaration, so an inert chain
// publishes the full report. Method: the protobuf chain
// x.Apply(1).Info().Present(0) and a literal receiver presence{...}.Present(0)
// run in an initializer the external effects walk reads; every method is
// inert, so the only possible refusal is an unresolved receiver.
//
// MUTATION: (internal/le/repo/inventory/methods.go,
// `function, found = e.methodAt(callee, depth+1)`,
// `function, found = e.declaredFunction(callee)`): a call result no longer
// types, so x.Apply(1).Info is projected as a field and the chain refuses as
// returned root field ownership is unresolved, the defect the actual root
// met. The literal receiver stays green, because valueType types a literal
// without the fallback.
func TestCommittedExternalMethodChainPublishes(t *testing.T) {
	declarations := methodChainPointer + methodChainPresence + methodChainPresent +
		"func (p pointer) Info() presence { _ = p; return presence{size: 0} }\n"
	for _, shape := range []struct{ name, call string }{
		{"call chain", methodChainCall},
		{"literal receiver", "_ = presence{size: 0}.Present(0)"},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(shape.name+"/"+kind, func(t *testing.T) {
				fixture := externalEffectsFixture(t, kind)
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(declarations, shape.call))
				commitInventoryFixture(t, fixture.root)
				externalEffectsPositive(t, fixture)
			})
		}
	}
}

// TestCommittedMethodReceiverCallExecutesRegistration requires the
// registration walk to execute a directly called method's receiver, as Go
// evaluates it before the arguments. Method: build() installs the owner and
// returns a value whose plain Register method is empty, so the report holds
// the owner only if build().Register() executed build(). The fixture imports
// no external package, so the registration walk alone decides.
//
// MUTATION: (internal/le/repo/inventory/registrations.go,
// `if ast.Unparen(current.Fun) == callee.value {`,
// `if ast.Unparen(current.Fun) == nil {`): the receiver is never walked, so
// build() never runs and the report publishes without the owner: the plugin
// case fails as the executed callable plugin owner disappeared, the YANG case
// as the executed callable YANG owner disappeared. The parenthesised call is
// the same direct call. Through the method-value alias the receiver already
// ran, unconditionally, where the value was taken, so the alias called inside
// a loop still publishes.
//
// MUTATION: (internal/le/repo/inventory/registrations.go,
// `if ast.Unparen(current.Fun) == callee.value {`, `if true {`): the alias
// call runs build() again inside the loop, and its registration refuses as
// depending on an unresolved condition or iteration.
func TestCommittedMethodReceiverCallExecutesRegistration(t *testing.T) {
	scenario := callableOwnerCase{
		declarations: "type sealed struct { size int }\n" +
			"func build() sealed { install(0); return sealed{size: 0} }\n" +
			"func (s sealed) Register() { _ = s }\n",
	}
	for _, call := range []string{"build().Register()", "(build().Register)()", "f := build().Register; for range 2 { f() }"} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(call+"/"+kind, func(t *testing.T) {
				fixture := callableOwnerFixture(t, kind, scenario)
				fixture.goFile(t, callableOwnerCaller, callableOwnerSource(scenario, call))
				commitInventoryFixture(t, fixture.root)
				if _, _, err := callableOwnerOutcome(t, fixture, kind, false); err != nil {
					t.Fatalf("receiver call refused its registration: %v", err)
				}
			})
		}
	}
}

// TestCommittedExternalMethodChainUnresolvedReceiversRefuse keeps refusing a
// method the bounded method-owner contract does not resolve, now that the
// receiver may be a call result. Method: the chain of the publishing test,
// with Info or Present changed to one unsupported shape each; the selector
// then falls back to a field projection, which refuses under the root-field
// rules. The multiple-result shape is not valid Go: inventory reads syntax
// and must refuse it rather than type the first result.
//
// MUTATION: (internal/le/repo/inventory/methods.go,
// `if function.decl.Type.Results.NumFields() != 1 {`,
// `if function.decl.Type.Results.NumFields() < 1 {`): Info's first result
// types the receiver, Present resolves, and the multiple-result case
// publishes.
// MUTATION: (internal/le/repo/inventory/methods.go,
// `if receiverIndirect != indirect {`,
// `if receiverIndirect != indirect && false {`): the pointer method matches
// the value call result, and the pointer-receiver case publishes.
// No single-line break admits the interface case: concreteType's arm list
// could admit an interface type, but an interface declares no FuncDecl, so
// the method search would still find nothing and the case keeps refusing.
func TestCommittedExternalMethodChainUnresolvedReceiversRefuse(t *testing.T) {
	for _, shape := range []struct{ name, declarations string }{
		{"multiple results", methodChainPresence + methodChainPresent +
			"func (p pointer) Info() (presence, error) { _ = p; return presence{size: 0}, nil }\n"},
		{"interface result", methodChainPresence + methodChainPresent +
			"type presenter interface { Present(index int) bool }\n" +
			"func (p pointer) Info() presenter { _ = p; return presence{size: 0} }\n"},
		{"pointer receiver", methodChainPresence +
			"func (q *presence) Present(index int) bool { return index == q.size }\n" +
			"func (p pointer) Info() presence { _ = p; return presence{size: 0} }\n"},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(shape.name+"/"+kind, func(t *testing.T) {
				fixture := externalEffectsFixture(t, kind)
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(methodChainPointer+shape.declarations, methodChainCall))
				commitInventoryFixture(t, fixture.root)
				externalEffectsRefusal(t, fixture, "root field ownership")
			})
		}
	}
}

// methodChainParcel holds a callback field. Apply returns the receiver, so the
// call result carries the field value its receiver was built with.
const methodChainParcel = "type parcel struct { hook func() }\n" +
	"func (p parcel) Apply(offset int) parcel { _ = offset; return p }\n"

// TestCommittedExternalCallResultFieldIsProjection requires a selector on a
// call result that names no method to stay a field projection under the
// root-field rules, now that the call result has a static type. Method: the
// callback field of x.Apply(1) is handed to an external callee; the call's
// return summary does not certify which field value the result holds, so
// both an inert and a registering callback refuse on root field ownership
// rather than publish.
//
// MUTATION: (internal/le/repo/inventory/externalrootfields.go, `if !known {`,
// `if !known && false {`): a field the receiver summary does not hold projects as
// empty, and the inert case publishes.
func TestCommittedExternalCallResultFieldIsProjection(t *testing.T) {
	const call = "x := parcel{hook: func() { BODY }}; order.Slice([]any{x.Apply(1).hook}, func(i, j int) bool { return false })"
	for _, body := range []string{"", "install(0)"} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(kind+"/"+body, func(t *testing.T) {
				fixture := externalEffectsFixture(t, kind)
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(methodChainParcel, strings.Replace(call, "BODY", body, 1)))
				commitInventoryFixture(t, fixture.root)
				externalEffectsRefusal(t, fixture, "root field")
			})
		}
	}
}

// methodChainEnvelope embeds presence by value, so Present is promoted to
// the call result x.Apply(1).Info().
const methodChainEnvelope = "type envelope struct { presence }\n" +
	"func (p pointer) Info() envelope { _ = p; return envelope{presence: presence{size: 0}} }\n"

// TestCommittedExternalMethodChainPromotedPublishes requires a value method
// promoted through an embedded value of a call result to resolve, with the
// embedded field Info().presence as its receiver. Method: the chain
// x.Apply(1).Info().Present(0) of the publishing test, with Info returning an
// envelope embedding presence; every method is inert, so the only possible
// refusal is an unresolved receiver. Present then hands a callback to
// sort.Slice beside its read of q.size: an inert callback publishes and a
// registering one refuses as registration, so Present's body was walked
// with q bound.
//
// MUTATION: (internal/le/repo/inventory/methodpromotion.go,
// `if members == 1 {`, `if members == 2 {`): Present no longer resolves, is
// projected as a field of the call result, and every case refuses on root
// field ownership: the inert ones publish nothing and the registering one
// loses its registration cause.
func TestCommittedExternalMethodChainPromotedPublishes(t *testing.T) {
	const walked = "func (q presence) Present(index int) bool { order.Slice([]int{q.size}, func(i, j int) bool { BODY return false }); return index == q.size }\n"
	for _, kind := range []string{"plugin", "yang"} {
		t.Run("exact chain/"+kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource(methodChainPointer+methodChainPresence+methodChainPresent+methodChainEnvelope, methodChainCall))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)
		})
		t.Run("walked body/"+kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			inert := methodChainPointer + methodChainPresence + strings.Replace(walked, "BODY", "", 1) + methodChainEnvelope
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource(inert, methodChainCall))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)
			registering := methodChainPointer + methodChainPresence + strings.Replace(walked, "BODY", "install(0);", 1) + methodChainEnvelope
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource(registering, methodChainCall))
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "registration")
		})
	}
}
