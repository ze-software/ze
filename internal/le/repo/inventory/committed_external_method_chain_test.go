// Design: docs/architecture/plugin/feature-gates.md -- a method resolves on any receiver whose static type is derived.
package repoinventory

import (
	"reflect"
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
// The interface case dispatches in the external-effects walk to the methods
// its value carries (interfaceMethod), so its refusal is the registration
// walk's: an interface receiver has no proved execution owner there.
func TestCommittedExternalMethodChainUnresolvedReceiversRefuse(t *testing.T) {
	for _, shape := range []struct{ name, declarations, reason string }{
		{"multiple results", methodChainPresence + methodChainPresent +
			"func (p pointer) Info() (presence, error) { _ = p; return presence{size: 0}, nil }\n", "root field ownership"},
		{"interface result", methodChainPresence + methodChainPresent +
			"type presenter interface { Present(index int) bool }\n" +
			"func (p pointer) Info() presenter { _ = p; return presence{size: 0} }\n", "callable receiver has unresolved execution ownership"},
		{"pointer receiver", methodChainPresence +
			"func (q *presence) Present(index int) bool { return index == q.size }\n" +
			"func (p pointer) Info() presence { _ = p; return presence{size: 0} }\n", "root field ownership"},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(shape.name+"/"+kind, func(t *testing.T) {
				fixture := externalEffectsFixture(t, kind)
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(methodChainPointer+shape.declarations, methodChainCall))
				commitInventoryFixture(t, fixture.root)
				if shape.reason == "callable receiver has unresolved execution ownership" {
					// The registration walk's refusal carries no "external",
					// so externalEffectsRefusal's checks are written out here.
					inv, reportErr := Collect(fixture.root)
					plugins, pluginErr := Plugins(fixture.root)
					if reportErr == nil {
						t.Fatalf("interface receiver chain published a report: %+v", inv.Plugins)
					}
					if pluginErr == nil {
						t.Fatalf("interface receiver chain published plugins: %+v", plugins)
					}
					if !reflect.DeepEqual(inv, Inventory{}) {
						t.Fatalf("interface receiver refusal leaked a partial report: %+v", inv)
					}
					if plugins != nil {
						t.Fatalf("interface receiver refusal leaked partial plugins: %+v", plugins)
					}
					for _, err := range []error{reportErr, pluginErr} {
						if !strings.Contains(err.Error(), shape.reason) {
							t.Fatalf("interface receiver refusal lacks %q: %v", shape.reason, err)
						}
						callableOwnerCause(t, err)
					}
					return
				}
				externalEffectsRefusal(t, fixture, shape.reason)
			})
		}
	}
}

// methodChainParcel holds a callback field. Apply returns the receiver, so the
// call result carries the field value its receiver was built with.
const methodChainParcel = "type parcel struct { hook func() }\n" +
	"func (p parcel) Apply(offset int) parcel { _ = offset; return p }\n"

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
