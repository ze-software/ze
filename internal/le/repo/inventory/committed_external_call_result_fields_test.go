// Design: docs/architecture/plugin/feature-gates.md -- a call result's fields are the fields its callee returns.
package repoinventory

import (
	"strings"
	"testing"
)

// callResultSlice hands one value to sort.Slice, an external callee, so an
// unknown value read from a call result would reach the ambient pool.
const callResultSlice = "order.Slice([]any{%s}, func(i, j int) bool { return false })"

func callResultSliced(value string) string {
	return strings.Replace(callResultSlice, "%s", value, 1)
}

// callResultTypes mirrors grpc's serviceconfig.ParseResult and the config it
// points to.
const callResultTypes = "type parcelConfig struct { name string }\n" +
	"type parcelResult struct { Config any; Err error; Hook func() }\n"

// callResultRead is grpc's init in clientconn.go: the result's Err is read
// inside a branch and handed to an external callee, then Config is read.
var callResultRead = "cfg := parseParcel(\"\"); if cfg.Err != nil { " + callResultSliced("cfg.Err") + " }; " + callResultSliced("cfg.Config")

// TestCommittedExternalCallResultFieldsPublish requires a field read from a
// retained call result to project the field values the callee returns, as
// grpc's init reads parseServiceConfig's Err and Config. Method: parseParcel
// returns &parcelResult{Err: ...} on one path and &parcelResult{Config: &c}
// on the other, once with an error from os.Stat (grpc's shape, whose result
// then carries an external origin) and once with a nil error, and the init
// hands cfg.Err and cfg.Config to sort.Slice. Every value is inert, so the
// report publishes. The walk reads cfg.Err in the round that first schedules
// parseParcel, before its returns are summarized.
//
// MUTATION: (internal/le/repo/inventory/externalrootfields.go,
// `return externalValue{reads: receiver.reads}`,
// `return externalValue{unknown: "declared root field value ownership is unresolved"}`):
// the first round's missing field is an unknown value, sort.Slice exposes it
// to the ambient pool, the pool's unknown is permanent, and both shapes
// refuse as registration capability is unknown, the actual-root refusal at
// grpc clientconn.go:779.
// MUTATION: (internal/le/repo/inventory/externalcalls.go,
// `return externalFlat(inputs, 0)`, `return inputs`): os.Stat's result keeps
// the ambient pool's root fields, the pool absorbs cfg's Err field holding
// that result, and each round nests the pool one level deeper inside Err:
// the external-error shape refuses as the effect closure exceeding 128
// rounds, while the nil-error shape still publishes.
func TestCommittedExternalCallResultFieldsPublish(t *testing.T) {
	for _, shape := range []struct{ name, failure string }{
		{"external error", "_, fault := remote.Stat(text); if fault != nil { return &parcelResult{Err: fault} }"},
		{"nil error", "if len(text) == 0 { return &parcelResult{Err: nil} }"},
	} {
		declarations := callResultTypes +
			"func parseParcel(text string) *parcelResult { " + shape.failure + "; c := parcelConfig{name: text}; return &parcelResult{Config: &c} }\n"
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(shape.name+"/"+kind, func(t *testing.T) {
				fixture := externalEffectsFixture(t, kind)
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(declarations, callResultRead))
				commitInventoryFixture(t, fixture.root)
				externalEffectsPositive(t, fixture)
			})
		}
	}
}

// TestCommittedExternalCallResultFieldCarriesRegistration requires the field
// value a callee returns to reach the reader whole, when another return path
// omits that field. Method: parseParcel returns &parcelResult{Err: nil},
// whose omitted Hook is zero, on one path and &parcelResult{Hook: func() {
// BODY }} on the other; the init hands cfg.Hook to sort.Slice. An inert Hook
// publishes and an installing one refuses as registration reaching external
// code, so the projected field carries the callback.
//
// MUTATION: (internal/le/repo/inventory/externalliteralshapes.go,
// `if _, listed := result.fields[name]; listed {`, `if false {`): the zero of
// an omitted member overwrites the Hook the literal lists, the callback
// never reaches sort.Slice, and the installing case publishes.
// MUTATION: (internal/le/repo/inventory/externalrootfields.go,
// `return externalValue{reads: receiver.reads}`,
// `return externalValue{unknown: "declared root field value ownership is unresolved"}`):
// the inert case refuses on the first round's missing field.
func TestCommittedExternalCallResultFieldCarriesRegistration(t *testing.T) {
	for _, body := range []string{"", "install(0)"} {
		declarations := callResultTypes +
			"func parseParcel(text string) *parcelResult { if len(text) == 0 { return &parcelResult{Err: nil} }; return &parcelResult{Hook: func() { " + body + " }} }\n"
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(kind+"/"+body, func(t *testing.T) {
				fixture := externalEffectsFixture(t, kind)
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(declarations, "cfg := parseParcel(\"\"); "+callResultSliced("cfg.Hook")))
				commitInventoryFixture(t, fixture.root)
				if body == "" {
					externalEffectsPositive(t, fixture)
					return
				}
				externalEffectsRefusal(t, fixture, "registration")
			})
		}
	}
}

// TestCommittedExternalOmittedFieldIsZero requires a member a retained struct
// literal omits to read as the zero value Go gives it. Method: parseParcel's
// only return lists Config alone, or nothing, and the init hands cfg.Err to
// sort.Slice. No return path names Err, so only the omitted member's zero
// lets the report publish.
//
// The literal starts from its type's capability, whose member table lists
// every declared field, so Err is present and holds nothing.
//
// MUTATION: (internal/le/repo/inventory/externaleffecttypes.go,
// `				result.fields[name.Name] = current`, `				_ = current`):
// the type's member table lists no named field, the last round still finds no
// Err, and both shapes refuse as declared root field value ownership is
// unresolved.
func TestCommittedExternalOmittedFieldIsZero(t *testing.T) {
	for _, shape := range []struct{ name, result string }{
		{"keyed", "c := parcelConfig{name: text}; return &parcelResult{Config: &c}"},
		{"empty", "_ = text; return &parcelResult{}"},
	} {
		declarations := callResultTypes + "func parseParcel(text string) *parcelResult { " + shape.result + " }\n"
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(shape.name+"/"+kind, func(t *testing.T) {
				fixture := externalEffectsFixture(t, kind)
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(declarations, "cfg := parseParcel(\"\"); "+callResultSliced("cfg.Err")))
				commitInventoryFixture(t, fixture.root)
				externalEffectsPositive(t, fixture)
			})
		}
	}
}

// TestCommittedExternalCallResultUnknownFieldRefuses keeps refusing a field
// whose value the callee's returns never certify, now that a missing field
// is a refusal of one round. Method: parseParcel declares *parcelResult but
// returns a parcelOther literal, which has no Err member, so the last round
// still finds no Err; and parseParcel declares an interface result, whose
// dynamic types no retained struct layout states. The analyzer reads syntax,
// so the first shape need not type-check.
//
// MUTATION: (internal/le/repo/inventory/externalrootfields.go,
// `e.problem(source, "declared root field value ownership is unresolved")`,
// `_ = source`): the last round's missing Err refuses nothing, the empty
// projection publishes, and the undeclared shape fails. The interface
// shape's refusal is the unchanged static root-field rule: no break of
// projectField's value branch reaches it.
func TestCommittedExternalCallResultUnknownFieldRefuses(t *testing.T) {
	for _, shape := range []struct{ name, declarations, reason string }{
		{"undeclared member", "type parcelOther struct { Config any }\n" +
			"func parseParcel(text string) *parcelResult { _ = text; return &parcelOther{Config: nil} }\n",
			"declared root field value ownership is unresolved"},
		{"interface result", "func parseParcel(text string) any { _ = text; return &parcelResult{Err: nil} }\n",
			"root field ownership"},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(shape.name+"/"+kind, func(t *testing.T) {
				fixture := externalEffectsFixture(t, kind)
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(callResultTypes+shape.declarations, "cfg := parseParcel(\"\"); "+callResultSliced("cfg.Err")))
				commitInventoryFixture(t, fixture.root)
				externalEffectsRefusal(t, fixture, shape.reason)
			})
		}
	}
}

// TestCommittedExternalCallResultFieldProjectsReturnedValue is the proposed
// replacement of TestCommittedExternalCallResultFieldIsProjection: the field
// of x.Apply(1), whose Apply returns its receiver p, is the hook the
// receiver was built with. Method: the same call; an inert hook publishes
// and an installing one refuses as registration reaching external code, so
// the projection carries the receiver's actual hook.
//
// MUTATION: (internal/le/repo/inventory/externalrootfields.go,
// `return externalValue{reads: receiver.reads}`,
// `return externalValue{unknown: "declared root field value ownership is unresolved"}`):
// the first round reads hook before Apply's returns exist, the unknown
// reaches the ambient pool through sort.Slice, and the inert case refuses.
func TestCommittedExternalCallResultFieldProjectsReturnedValue(t *testing.T) {
	const call = "x := parcel{hook: func() { BODY }}; order.Slice([]any{x.Apply(1).hook}, func(i, j int) bool { return false })"
	for _, body := range []string{"", "install(0)"} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(kind+"/"+body, func(t *testing.T) {
				fixture := externalEffectsFixture(t, kind)
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(methodChainParcel, strings.Replace(call, "BODY", body, 1)))
				commitInventoryFixture(t, fixture.root)
				if body == "" {
					externalEffectsPositive(t, fixture)
					return
				}
				externalEffectsRefusal(t, fixture, "registration")
			})
		}
	}
}
