// Design: docs/architecture/plugin/feature-gates.md -- ranged element summaries.
package repoinventory

import "testing"

// parcelTableDeclarations mirrors the plugin registry: a package map of struct
// pointers, filled by a Register-like function from a copy of its argument,
// ranged by a helper that reads one field of every element and hands the
// collected callbacks to external code (sort.Slice), as InProcessDecoders
// hands its map to callers.
const parcelTableDeclarations = `type parcelEntry struct { Name string; Hook func() }
var parcelTable = make(map[string]*parcelEntry)
func parcelRegister(entry parcelEntry) { copied := entry; parcelTable[entry.Name] = &copied }
func parcelHooks() map[string]func() {
	hooks := make(map[string]func())
	for _, entry := range parcelTable { if entry.Hook != nil { hooks[entry.Name] = entry.Hook } }
	return hooks
}
`

const parcelHooksExposed = `; order.Slice([]any{parcelHooks()}, func(i, j int) bool { return false })`

// TestCommittedExternalRangedElementFieldIsStoredJoin proves a field read from
// a range element is the join of that field across every stored element. The
// inert callback publishes the full report; the same shape whose registered
// element carries an installing callback refuses as a registration escaping to
// external code, so the summary carries the real stored callback, not a
// type-derived empty field.
//
// MUTATION: (externalrangeelements.go, `result.join(stored)`, `result.join(externalFlat(stored, 0))`): the element fields are dropped, so the inert case refuses as an unowned field value.
// MUTATION: (externalrangeelements.go, `			return externalElementAccess(use, parent)`, `			return nil, ""`): the element store is no source, the field reads the type's empty callback, and the installing case publishes.
// MUTATION: (methods.go, `if predeclaredIdentifier(callee, "make") {`, `if predeclaredIdentifier(callee, "make") && false {`): the table declared from make has no type, so its element type is unresolved and the inert case refuses.
// MUTATION: (externalranges.go, `	// values its writes store are judged by the capability walk, not here.`, the same line followed by an `immutable` refusal): the written table has no range kind, and the inert case refuses.
func TestCommittedExternalRangedElementFieldIsStoredJoin(t *testing.T) {
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			for _, hook := range []struct {
				name, body string
				installs   bool
			}{{"inert", "func() {}", false}, {"installer", "func() { install(0) }", true}} {
				t.Run(hook.name, func(t *testing.T) {
					call := `parcelRegister(parcelEntry{Name: "a", Hook: ` + hook.body + `})` + parcelHooksExposed
					fixture.goFile(t, callableOwnerCaller, externalEffectsSource(parcelTableDeclarations, call))
					commitInventoryFixture(t, fixture.root)
					if hook.installs {
						externalEffectsRefusal(t, fixture, "registration")
						return
					}
					externalEffectsPositive(t, fixture)
				})
			}
		})
	}
}

// TestCommittedExternalRangedElementPopulationMustBeKnown keeps the field read
// a refusal whenever the element population is not fully known. Each case
// fills the ranged collection through a path the proof cannot follow; the
// installing cases would publish if that path were silently dropped, because
// every element the proof can see carries an inert callback.
//
// MUTATION: (externalrangeelements.go, `	return nil, "ranged collection is used in a form whose effect on its elements is not modeled"`, `	return nil, ""`): the alias store is missed and the alias case publishes.
// MUTATION: (externalrangeelements.go, `	return nil, "ranged collection is passed to a call that can store into it"`, `	return nil, ""`): the retained filler's store is missed and the external callee's possible stores are ignored, so both cases publish.
// MUTATION: (externalrangeelements.go, `		return nil, "ranged collection is filled from a call result"`, `		return nil, ""`): the loaded table reads as empty and the call-result case publishes.
func TestCommittedExternalRangedElementPopulationMustBeKnown(t *testing.T) {
	const list = `type parcelEntry struct { Name string; Hook func() }
var parcelList []*parcelEntry
func parcelRegister(entry parcelEntry) { copied := entry; parcelList = append(parcelList, &copied) }
func parcelHooks() map[string]func() {
	hooks := make(map[string]func())
	for _, entry := range parcelList { if entry.Hook != nil { hooks[entry.Name] = entry.Hook } }
	return hooks
}
`
	const loaded = `type parcelEntry struct { Name string; Hook func() }
func parcelLoad() map[string]*parcelEntry { return map[string]*parcelEntry{"b": {Name: "b", Hook: func() { install(0) }}} }
var parcelTable = parcelLoad()
func parcelHooks() map[string]func() {
	hooks := make(map[string]func())
	for _, entry := range parcelTable { if entry.Hook != nil { hooks[entry.Name] = entry.Hook } }
	return hooks
}
`
	const register = `parcelRegister(parcelEntry{Name: "a", Hook: func() {}}); `
	for _, scenario := range []struct{ name, declarations, call string }{
		{"alias", list, register + `alias := parcelList; alias[0] = &parcelEntry{Name: "b", Hook: func() { install(0) }}` + parcelHooksExposed},
		{"retained filler", list + `func parcelFill(entries []*parcelEntry) { entries[0] = &parcelEntry{Name: "b", Hook: func() { install(0) }} }
`, register + `parcelFill(parcelList)` + parcelHooksExposed},
		{"external callee", list, register + `order.Slice(parcelList, func(i, j int) bool { return false })` + parcelHooksExposed},
		{"call result", loaded, `_ = 0` + parcelHooksExposed},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(scenario.name+"/"+kind, func(t *testing.T) {
				fixture := externalEffectsFixture(t, kind)
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(scenario.declarations, scenario.call))
				commitInventoryFixture(t, fixture.root)
				externalEffectsRefusal(t, fixture, "ranged element")
			})
		}
	}
}

// TestCommittedExternalRangedReceiverIsGrounded proves formal grounding of a
// method receiver bound from a range variable. The receiver's source is the
// collection's element sources: a package slice of literals grounds it, and
// the report publishes; the same method ranged over a parameter's elements,
// whose population comes from callers, refuses.
//
// MUTATION: (externalformalproof.go, `		proof.join(proofs.rangedProof(object, variable, depth+1))`, `		_ = variable`): the range variable has no source, so the grounded case refuses.
func TestCommittedExternalRangedReceiverIsGrounded(t *testing.T) {
	const label = `type parcelLabel struct { text string; Hook func() }
func (l parcelLabel) Text() string { return l.text }
var parcelLabels = []parcelLabel{{text: "a"}, {text: "b"}}
`
	const grounded = label + `func parcelTexts() []string {
	texts := make([]string, 0, len(parcelLabels))
	for _, entry := range parcelLabels { texts = append(texts, entry.Text()) }
	return texts
}
`
	const parameter = label + `func parcelTexts(labels []parcelLabel) []string {
	texts := make([]string, 0, len(labels))
	for _, entry := range labels { texts = append(texts, entry.Text()) }
	return texts
}
`
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource(grounded, `order.Strings(parcelTexts())`))
			commitInventoryFixture(t, fixture.root)
			externalEffectsPositive(t, fixture)

			fixture.goFile(t, callableOwnerCaller, externalEffectsSource(parameter, `order.Strings(parcelTexts(parcelLabels))`))
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "ranged element")
		})
	}
}

// TestCommittedExternalLengthCarriesNoCapability proves len and cap answer an
// int that holds none of their operand's capabilities. A slice holding an
// installing callback is handed to external code whole, which refuses as a
// registration; its length and capacity handed the same way publish, as
// make([]string, len(familySet)) must for the family helpers.
//
// MUTATION: (externalcalls.go, `return externalValue{reads: inputs.reads, unknown: inputs.unknown, unknownAt: inputs.unknownAt}`, `return inputs`): the length carries the installing callback, so the length case refuses as a registration.
func TestCommittedExternalLengthCarriesNoCapability(t *testing.T) {
	const declarations = "var parcelHooks = []func(){func() { install(0) }}\n"
	const less = `, func(i, j int) bool { return false })`
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			fixture.goFile(t, callableOwnerCaller, externalEffectsSource(declarations, `order.Slice([]any{parcelHooks}`+less))
			commitInventoryFixture(t, fixture.root)
			externalEffectsRefusal(t, fixture, "registration")

			for _, builtin := range []string{"len", "cap"} {
				t.Run(builtin, func(t *testing.T) {
					fixture.goFile(t, callableOwnerCaller, externalEffectsSource(declarations, `order.Slice([]any{`+builtin+`(parcelHooks)}`+less))
					commitInventoryFixture(t, fixture.root)
					externalEffectsPositive(t, fixture)
				})
			}
		})
	}
}
