// Design: docs/architecture/plugin/feature-gates.md -- a channel is a reference container like a slice.
package repoinventory

import (
	"strings"
	"testing"
)

const externalChannelEscaped = "registration may execute through an escaped capability"

const externalChannelFunctions = "func quiet() {}\nfunc armed() { install(0) }\n"

// externalChannelArmed writes source with FN naming quiet and requires the
// report to publish, then with FN naming armed and requires the refusal the
// escaped registering function produces. Only the function changes, so the
// refusal is caused by what the channel carried, not by the channel shape.
func externalChannelArmed(t *testing.T, kind, declarations, call string) {
	t.Helper()
	fixture := externalEffectsFixture(t, kind)
	quiet := strings.ReplaceAll(call, "FN", "quiet")
	fixture.goFile(t, callableOwnerCaller, externalEffectsSource(externalChannelFunctions+declarations, quiet))
	commitInventoryFixture(t, fixture.root)
	externalEffectsPositive(t, fixture)
	armed := strings.ReplaceAll(call, "FN", "armed")
	fixture.goFile(t, callableOwnerCaller, externalEffectsSource(externalChannelFunctions+declarations, armed))
	commitInventoryFixture(t, fixture.root)
	externalEffectsRefusal(t, fixture, externalChannelEscaped)
}

// TestCommittedExternalChannelPoolPublishes requires the shape that refused in
// klauspost/compress zstd decoder.go, a struct field `chan *T` used as a pool,
// to publish. Method: the field is allocated with make, filled by a send,
// drained by a receive and refilled, then the holder is exposed to an opaque
// call; the report and plugins must match the control's.
//
// MUTATION: (internal/le/repo/inventory/externaleffecttypes.go,
// "result := externalFlat(child(datatype.Value, true), 0)",
// "result := externalValue{unknown: \"channel element capability transfer is unsupported\"}")
// refuses with today's unknown at the field.
// MUTATION: (internal/le/repo/inventory/externaleffects.go,
// "\tif len(location.locations) == 0 {\n\t\treason := \"channel send",
// "\tif len(location.locations) != -1 {\n\t\treason := \"channel send")
// refuses every send as an unresolved location.
// MUTATION: (internal/le/repo/inventory/externalbindings.go,
// "if store.element {", "if !store.element {") binds the exposed holder's
// dormant send of a received value unflattened: its field obligation becomes
// the holder's, and d.decoders refuses.
func TestCommittedExternalChannelPoolPublishes(t *testing.T) {
	const declarations = "type blockDec struct{ size int }\ntype Decoder struct{ decoders chan *blockDec }\n"
	const call = "var d Decoder; d.decoders = make(chan *blockDec, 2); d.decoders <- &blockDec{size: 1}; block := <-d.decoders; d.decoders <- block; order.Slice(&d, func(i, j int) bool { return false })"
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource(declarations, call))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)
		})
	}
}

// TestCommittedExternalChannelDirectionsPublish requires a send-only and a
// receive-only channel to be the same container as a bidirectional one.
// Method: package channels of each direction are sent to, received from and
// exposed whole through a struct whose fields carry both directions.
//
// MUTATION: (internal/le/repo/inventory/externaleffecttypes.go,
// "result := externalFlat(child(datatype.Value, true), 0)",
// "result := externalValue{unknown: \"channel element capability transfer is unsupported\"}")
// refuses at the directional types.
func TestCommittedExternalChannelDirectionsPublish(t *testing.T) {
	const declarations = "type blockDec struct{ size int }\ntype ends struct{ in <-chan *blockDec; out chan<- *blockDec }\nvar inbox <-chan *blockDec = make(chan *blockDec, 1)\nvar outbox chan<- *blockDec = make(chan *blockDec, 1)\n"
	const call = "outbox <- &blockDec{size: 1}; block := <-inbox; outbox <- block; order.Slice(&ends{in: inbox, out: outbox}, func(i, j int) bool { return false })"
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource(declarations, call))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)
		})
	}
}

// TestCommittedExternalChannelCarriesRegistration requires what a send puts in
// a channel to be what a receive takes out. Method: a function, or a struct
// holding one, is sent; the receiver either calls it inside a callback handed
// to an opaque call or exposes the received value whole. A quiet function
// publishes and a registering one refuses as escaped.
//
// MUTATION: (internal/le/repo/inventory/externaleffects.go,
// "sent := externalFlat(e.value(sourceExpr{value: statement.Value, file: scope.file, pkg: scope.pkg}, possible, 0), 0)",
// "sent := externalFlat(externalValue{}, 0)") drops the sent value: the armed
// function publishes.
// MUTATION: (internal/le/repo/inventory/externalexpressions.go,
// "result := child(value.X)\n\t\t\tresult.fields = nil",
// "result := externalValue{}\n\t\t\tresult.fields = nil") makes a receive
// yield nothing: the quiet call refuses as unresolved.
func TestCommittedExternalChannelCarriesRegistration(t *testing.T) {
	for _, shape := range []struct{ name, declarations, call string }{
		{
			name: "function",
			call: "queue := make(chan func(), 1); queue <- FN; _ = clock.AfterFunc(0, func() { callback := <-queue; callback() })",
		},
		{
			name:         "struct-holder",
			declarations: "type hook struct{ fn func() }\n",
			call:         "queue := make(chan hook, 1); queue <- hook{fn: FN}; held := <-queue; order.Slice(held, func(i, j int) bool { return false })",
		},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(shape.name+"/"+kind, func(t *testing.T) {
				externalChannelArmed(t, kind, shape.declarations, shape.call)
			})
		}
	}
}

// TestCommittedExternalChannelReceiveFormsCarryRegistration requires a select
// receive and a range over a channel to yield the channel's contents. Method:
// as TestCommittedExternalChannelCarriesRegistration, with the callback taking
// the function from a select case, or from a range whose element type is any
// (a func element refuses every range callback, as a slice range does).
//
// MUTATION: (internal/le/repo/inventory/externalexpressions.go,
// "result := child(value.X)\n\t\t\tresult.fields = nil",
// "result := externalValue{}\n\t\t\tresult.fields = nil") makes the select
// receive yield nothing: its quiet call refuses as unresolved.
// MUTATION: (internal/le/repo/inventory/methods.go,
// "return 0, kind.Value, true", "return 1, kind.Value, true") loses the
// ranged element's type: the quiet range refuses.
// MUTATION: (internal/le/repo/inventory/externalranges.go,
// "\tcase externalRangeChannel:\n",
// "\tcase externalRangeChannel:\n\t\te.problem(source, \"channel range capability transfer is unsupported\")\n")
// refuses the range itself.
func TestCommittedExternalChannelReceiveFormsCarryRegistration(t *testing.T) {
	for _, shape := range []struct{ name, call string }{
		{
			name: "select",
			call: "queue := make(chan func(), 1); queue <- FN; _ = clock.AfterFunc(0, func() { select { case callback := <-queue: callback() } })",
		},
		{
			name: "range",
			call: "var queue chan any = make(chan any, 1); queue <- FN; _ = clock.AfterFunc(0, func() { for held := range queue { held.(func())() } })",
		},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(shape.name+"/"+kind, func(t *testing.T) {
				externalChannelArmed(t, kind, "", shape.call)
			})
		}
	}
}

// TestCommittedExternalChannelDormantSendCarriesRegistration requires a send
// in code nothing calls to reach a channel exposed to external code, as an
// indexed store into an exposed slice does. Method: the channel alone is
// exposed, and only a never-called function sends to it.
//
// MUTATION: (internal/le/repo/inventory/externalbindings.go,
// "values: []ast.Expr{declaration.Value}, element: true})",
// "values: nil, element: true})") drops the dormant
// send: the armed function publishes.
func TestCommittedExternalChannelDormantSendCarriesRegistration(t *testing.T) {
	const declarations = "var queue = make(chan func(), 1)\nfunc later() { queue <- FN }\n"
	const call = "order.Slice(queue, func(i, j int) bool { return false })"
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			for _, step := range []struct {
				function string
				armed    bool
			}{{"quiet", false}, {"armed", true}} {
				source := externalEffectsSource(externalChannelFunctions+strings.ReplaceAll(declarations, "FN", step.function), call)
				fixture.goFile(t, callableOwnerCaller, source)
				commitInventoryFixture(t, fixture.root)
				if step.armed {
					externalEffectsRefusal(t, fixture, externalChannelEscaped)
					continue
				}
				externalEffectsPositive(t, fixture)
			}
		})
	}
}
