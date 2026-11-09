// Design: docs/architecture/plugin/feature-gates.md -- a field of a variable declared from a multi-valued call has its result's type.
package repoinventory

import (
	"strings"
	"testing"
)

// tupleFieldTypes mirrors protobuf's lazy.go: skipField returns an
// unmarshalOutput and a status, and the caller reads o.initialized.
const tupleFieldTypes = "type output struct { initialized bool; hook func() }\n" +
	"type info struct { size int }\n" +
	"func (mi *info) skip() (out output, status int) { _ = mi; return output{initialized: true, hook: func() { BODY }}, 0 }\n"

// TestCommittedExternalTupleOperandFieldsProject requires a field read of a
// variable declared from a multi-valued call to project the field values the
// callee returns, as protobuf's o, valid := mi.skipField(...) reads
// o.initialized. Method: the init declares o and status from skip and hands
// o.hook to sort.Slice; an inert hook publishes, an installing hook refuses
// as registration, so the read projected the returned field.
//
// MUTATION: (internal/le/repo/inventory/externalrootfields.go,
// `	if externalTupleOperand(object) {`, `	if externalTupleOperand(object) && false {`):
// o's root falls back to the whole call, which has no single type, and the
// inert case refuses as returned root field ownership is unresolved, the
// actual-root refusal at protobuf lazy.go:282.
func TestCommittedExternalTupleOperandFieldsProject(t *testing.T) {
	for _, shape := range []struct{ name, body, reason string }{
		{"inert", "", ""},
		{"installing", "install(0)", "registration"},
	} {
		declarations := strings.Replace(tupleFieldTypes, "BODY", shape.body, 1)
		call := "mi := &info{}; o, status := mi.skip(); _ = status; if o.initialized { " + callResultSliced("o.hook") + " }"
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(shape.name+"/"+kind, func(t *testing.T) {
				fixture := externalEffectsFixture(t, kind)
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(declarations, call))
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

// TestCommittedExternalNamedResultsReturnTheirValues requires a function with
// named results to return what its result variables hold, by a naked return
// or a deferred assignment as well as by an explicit return. Method: produce
// declares (out func(), status int) and fills out three ways; the init hands
// the result to sort.Slice. An inert callback publishes on every route, and
// an installing one refuses as registration on every route, so each route's
// value reaches the caller.
//
// MUTATION: (internal/le/repo/inventory/externaleffects.go,
// `			e.namedResults(node)`, `			_ = node`): the naked and deferred
// routes return nothing, and their installing shapes publish.
func TestCommittedExternalNamedResultsReturnTheirValues(t *testing.T) {
	for _, route := range []struct{ name, body string }{
		{"explicit", "return func() { BODY }, 0"},
		{"naked", "out = func() { BODY }; return"},
		{"deferred", "defer func() { out = func() { BODY } }(); return nil, 0"},
	} {
		for _, shape := range []struct{ name, body, reason string }{
			{"inert", "", ""},
			{"installing", "install(0)", "registration"},
		} {
			declarations := "func produce() (out func(), status int) { " + strings.Replace(route.body, "BODY", shape.body, 1) + " }\n"
			call := "hook, _ := produce(); " + callResultSliced("hook")
			for _, kind := range []string{"plugin", "yang"} {
				t.Run(route.name+"/"+shape.name+"/"+kind, func(t *testing.T) {
					fixture := externalEffectsFixture(t, kind)
					fixture.goFile(t, callableOwnerCaller, externalEffectsSource(declarations, call))
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
}
