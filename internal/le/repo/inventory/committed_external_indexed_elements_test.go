// Design: docs/architecture/plugin/feature-gates.md -- indexed element summaries.
package repoinventory

import (
	"strings"
	"testing"
)

// parcelStateType is the element of every indexed collection below: a scalar
// kind, a mask the loop updates in place, and the callback whose stored value
// the proof must carry.
const parcelStateType = "type parcelState struct { typ int; mask uint64; tail []byte; hook func() }\n"

// parcelStateHook stands for the callback a test stores, and is replaced by an
// inert or an installing function literal.
const parcelStateHook = "HOOK"

const parcelInertHook = "func() {}"
const parcelInstallingHook = "func() { install(0) }"

// parcelIndexedDiscriminates runs a shape with an inert and with an
// installing callback, for both inventory kinds. Each case writes the shape
// with its callback in place of parcelStateHook, then checks it: the inert
// callback publishes, the installing one refuses as the registration it
// escapes to external code. The checks sit in the closure itself, not in a
// further helper, so the test-sensitivity gate's two-level helper walk sees
// the assertions the calling tests depend on.
func parcelIndexedDiscriminates(t *testing.T, declarations string) {
	t.Helper()
	for _, kind := range []string{"plugin", "yang"} {
		t.Run(kind, func(t *testing.T) {
			fixture := externalEffectsFixture(t, kind)
			for _, hook := range []struct {
				name, body string
				installs   bool
			}{{"inert", parcelInertHook, false}, {"installer", parcelInstallingHook, true}} {
				t.Run(hook.name, func(t *testing.T) {
					program := strings.ReplaceAll(declarations, parcelStateHook, hook.body)
					fixture.goFile(t, callableOwnerCaller, externalEffectsSource(parcelStateType+program, `_ = 0`+parcelHooksExposed))
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

// TestCommittedExternalIndexedStackLoop proves the protobuf validator's work
// stack (google.golang.org/protobuf/internal/impl/validate.go): a local slice
// filled by append, a field written in place through an index, the top
// element bound to a fresh pointer whose fields the loop reads and updates,
// and a reslice assigned back. The callback read through the pointer is the
// join of every stored element's callback, so the inert stack publishes and
// the stack holding an installing callback refuses as a registration.
//
// MUTATION: (externalrangeelements.go, `			if pointer, bound := externalElementPointer(use, parent); bound {`, `			if pointer, bound := externalElementPointer(use, parent); bound && false {`): the element address bound to st escapes, and the inert stack refuses.
// MUTATION: (externalrangeelements.go, `			if externalAssignedInPlace(w.effects, use, parent) {`, `			if externalAssignedInPlace(w.effects, use, parent) && false {`): the reslice is an unmodeled use, and the inert stack refuses.
// MUTATION: (externalrangeelements.go, `		return w.collection(operand, owner, depth+1)`, `		_ = operand; return nil, "reslice"`): the reslice definition is no source, and the inert stack refuses.
// MUTATION: (externalrangeelements.go, `	result.join(held)`, `	result.join(externalFlat(held, 0))`): the indexed read lends no field, st.hook has no value, and the inert stack refuses.
// MUTATION: (externalrootfields.go, `		return e.indexedReceiverType(source, value, depth+1)`, `		return sourceExpr{}, externalIndexedObligation`): st's receiver keeps the obligation, and the inert stack refuses.
// MUTATION: (externalexpressions.go, `		return e.indexedValue(source, value, collection, key, depth)`, `		_, _ = collection, key; return externalValue{unresolvedFields: externalIndexedObligation}`): the indexed read keeps the obligation, and the inert stack refuses.
func TestCommittedExternalIndexedStackLoop(t *testing.T) {
	const stack = `func parcelHooks() map[string]func() {
	hooks := make(map[string]func())
	states := make([]parcelState, 0, 16)
	states = append(states, parcelState{typ: 1, hook: func() {}})
	states[0].typ = 2
	for len(states) > 0 {
		st := &states[len(states)-1]
		st.mask |= 1
		st.mask++
		if st.typ == 2 {
			hooks["a"] = st.hook
			states = append(states, parcelState{typ: 3, hook: HOOK})
			continue
		}
		_ = st.tail
		states = states[:len(states)-1]
	}
	return hooks
}
`
	parcelIndexedDiscriminates(t, stack)
}

// TestCommittedExternalIndexedFieldWrite proves an in-place field write
// through an index is an element source for that field: the callback it
// stores reaches the indexed read, so storing an installing callback refuses
// while storing an inert one publishes.
//
// MUTATION: (externalrangeelements.go, `					return externalFieldStore(use, selected, position, holder.Sel.Name)`, `					_ = position; return nil, ""`): the write is no source, the read carries only the literal's inert callback, and the installing case publishes.
// MUTATION: (externalrangeelements.go, `	if element.field != "" {`, `	if false {`): the written callback joins the element whole, not its hook field, and the installing case publishes.
func TestCommittedExternalIndexedFieldWrite(t *testing.T) {
	const write = `func parcelHooks() map[string]func() {
	hooks := make(map[string]func())
	states := []parcelState{{typ: 1, hook: func() {}}}
	states[0].hook = HOOK
	states[0].mask++
	hooks["a"] = states[0].hook
	return hooks
}
`
	parcelIndexedDiscriminates(t, write)
}

// TestCommittedExternalIndexedPointerWrite proves a field write through a
// fresh element pointer p := &x[i] is an element source for that field, so
// the indexed read sees the callback stored through the pointer.
//
// MUTATION: (externalrangeelements.go, `				return externalFieldStore(occurrence, holder, position, selector.Sel.Name)`, `				_ = position; return nil, ""`): the pointer write is no source, and the installing case publishes.
// MUTATION: (externalrangeelements.go, `		return externalPointerField(occurrence, parent)`, `		return nil, ""`): no pointer field occurrence is judged, and the installing case publishes.
func TestCommittedExternalIndexedPointerWrite(t *testing.T) {
	const pointer = `func parcelHooks() map[string]func() {
	hooks := make(map[string]func())
	states := []parcelState{{typ: 1, hook: func() {}}}
	st := &states[0]
	st.hook = HOOK
	hooks["a"] = states[0].hook
	return hooks
}
`
	parcelIndexedDiscriminates(t, pointer)
}

// TestCommittedExternalIndexedPointerMustStayLocal keeps the indexed read an
// obligation when the element pointer is used in a way the proof cannot
// follow. Each shape publishes without that use, and refuses with the use it
// names once the pointer is passed to a call that arms the callback, or once
// the pointer is reassigned.
//
// MUTATION: (externalrangeelements.go, `		return nil, "collection element pointer " + name + " is passed to a call"`, `		return nil, ""`): the callee's store is missed, and the escape case publishes.
// MUTATION: (externalrangeelements.go, `			return nil, "collection element pointer " + name + " is reassigned"`, `			return nil, ""`): the reassignment is admitted, and the reassigned case publishes.
func TestCommittedExternalIndexedPointerMustStayLocal(t *testing.T) {
	const control = `func parcelHooks() map[string]func() {
	hooks := make(map[string]func())
	states := []parcelState{{typ: 1, hook: func() {}}}
	st := &states[0]
	_ = st.typ
	USE
	hooks["a"] = states[0].hook
	return hooks
}
`
	const arm = "func parcelArm(s *parcelState) { s.hook = " + parcelInstallingHook + " }\n"
	for _, scenario := range []struct{ name, use, reason string }{
		{"escape", "parcelArm(st)", "collection element pointer st is passed to a call"},
		{"reassigned", "st = &parcelState{hook: " + parcelInertHook + "}", "collection element pointer st is reassigned"},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(scenario.name+"/"+kind, func(t *testing.T) {
				fixture := externalEffectsFixture(t, kind)
				// The control holds no parcelArm: a function nothing calls has
				// a parameter no caller grounds, which refuses on its own.
				local := strings.Replace(control, "USE", "_ = 0", 1)
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(parcelStateType+local, `_ = 0`+parcelHooksExposed))
				commitInventoryFixture(t, fixture.root)
				externalEffectsPositive(t, fixture)

				changed := strings.Replace(control, "USE", scenario.use, 1)
				if scenario.name == "escape" {
					changed = arm + changed
				}
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(parcelStateType+changed, `_ = 0`+parcelHooksExposed))
				commitInventoryFixture(t, fixture.root)
				externalEffectsRefusal(t, fixture, scenario.reason)
			})
		}
	}
}

// parcelNestedTypes give the element an inline struct field (inner, whose
// own inline field deep reaches past the occurrence index's retained
// context) and a pointer field (p), whose writes land on a referent.
const parcelNestedTypes = `type parcelLeaf struct { n int }
type parcelInner struct { hook func(); n int; deep parcelLeaf }
type parcelNest struct { typ int; inner parcelInner; p *parcelInner }
`

// TestCommittedExternalIndexedNestedRead proves a read below an inline
// element field, states[0].inner.hook, still carries the stored callback: the
// inert hook publishes and the installing one refuses as a registration.
//
// MUTATION: (externalrangeelements.go, `			if slices.Contains(context.Lhs, current) {`, `			if true {`): the nested read is judged a write, and the inert case refuses.
func TestCommittedExternalIndexedNestedRead(t *testing.T) {
	const read = parcelNestedTypes + `func parcelHooks() map[string]func() {
	hooks := make(map[string]func())
	states := []parcelNest{{typ: 1, inner: parcelInner{hook: HOOK}}}
	hooks["a"] = states[0].inner.hook
	return hooks
}
`
	parcelIndexedDiscriminates(t, read)
}

// TestCommittedExternalIndexedNestedWriteRefuses keeps a write below an inline
// element field a refusal: it changes the stored element in place, and the
// summary does not model the nested store. Each write is made with an inert
// callback in the element, so each case would publish if the write were read
// as a read; the same shape without the write publishes.
//
// MUTATION: (externalrangeelements.go, `			if reason := w.belowElementField(use, parent, depth+1); reason != "" {`, `			if reason := w.belowElementField(use, parent, depth+1); false {`): the index writes are judged reads, and those cases publish.
// MUTATION: (externalrangeelements.go, `		if below := e.belowField(declared, true, parent, occurrence.parents[1:], depth); below != "" {`, `		if below := e.belowField(declared, true, parent, occurrence.parents[1:], depth); false {`): the pointer write is judged a read, and the pointer case publishes.
// MUTATION: (externalrangeelements.go, `				return "collection element field " + name + " is written below the field"`, `				return ""`): the assigned and op-assigned cases publish.
// MUTATION: (externalrangeelements.go, `	return "collection element field " + name + " is used below the field past the retained context"`, `	return ""`): the deep case publishes.
func TestCommittedExternalIndexedNestedWriteRefuses(t *testing.T) {
	const shape = parcelNestedTypes + `func parcelHooks() map[string]func() {
	hooks := make(map[string]func())
	states := []parcelNest{{typ: 1, inner: parcelInner{hook: func() {}}}}
	st := &states[0]
	_ = st.typ
	WRITE
	hooks["a"] = states[0].inner.hook
	return hooks
}
`
	for _, scenario := range []struct{ name, write, reason string }{
		{"assigned", "states[0].inner.hook = " + parcelInstallingHook, "collection element field inner is written below the field"},
		{"assigned inert", "states[0].inner.hook = " + parcelInertHook, "collection element field inner is written below the field"},
		{"pointer", "st.inner.hook = " + parcelInstallingHook, "collection element field inner is written below the field"},
		{"op-assigned", "states[0].inner.n += 1", "collection element field inner is written below the field"},
		{"incremented", "states[0].inner.n++", "collection element field inner is incremented below the field"},
		{"deep", "states[0].inner.deep.n = 1", "collection element field inner is used below the field past the retained context"},
	} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(scenario.name+"/"+kind, func(t *testing.T) {
				fixture := externalEffectsFixture(t, kind)
				control := strings.Replace(shape, "WRITE", "_ = 0", 1)
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(control, `_ = 0`+parcelHooksExposed))
				commitInventoryFixture(t, fixture.root)
				externalEffectsPositive(t, fixture)

				changed := strings.Replace(shape, "WRITE", scenario.write, 1)
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(changed, `_ = 0`+parcelHooksExposed))
				commitInventoryFixture(t, fixture.root)
				externalEffectsRefusal(t, fixture, scenario.reason)
			})
		}
	}
}

// TestCommittedExternalIndexedReferenceFieldWrite proves a write below a
// pointer field, states[0].p.hook = HOOK, is not judged by the inline-field
// refusal: it lands on the pointer's referent, an anonymous allocation no
// binding names, which the location model cannot place (storeLocation), so
// both the inert and the installing write refuse as an unresolved assignment
// location, and never with the inline-field reason.
//
// MUTATION: (externalrangeelements.go, `	case *ast.StarExpr, *ast.MapType, *ast.ChanType, *ast.FuncType, *ast.InterfaceType:`, `	case *ast.MapType, *ast.ChanType, *ast.FuncType, *ast.InterfaceType:`): the pointer field is judged inline, and the write refuses with the inline-field reason.
// MUTATION: (externaleffects.go, `	if len(location.locations) == 0 && location.unknown == "" && externalElementFieldTarget(target) {`, `	if len(location.locations) == 0 && location.unknown == "" {`): the write is stored into the collection's flattened union, which the field read never consults, and the installing write publishes.
func TestCommittedExternalIndexedReferenceFieldWrite(t *testing.T) {
	const write = parcelNestedTypes + `func parcelHooks() map[string]func() {
	hooks := make(map[string]func())
	states := []parcelNest{{typ: 1, p: &parcelInner{hook: func() {}}}}
	states[0].p.hook = HOOK
	hooks["a"] = states[0].p.hook
	return hooks
}
`
	for _, hook := range []struct{ name, body string }{{"inert", parcelInertHook}, {"installer", parcelInstallingHook}} {
		for _, kind := range []string{"plugin", "yang"} {
			t.Run(hook.name+"/"+kind, func(t *testing.T) {
				fixture := externalEffectsFixture(t, kind)
				program := strings.ReplaceAll(write, parcelStateHook, hook.body)
				fixture.goFile(t, callableOwnerCaller, externalEffectsSource(parcelStateType+program, `_ = 0`+parcelHooksExposed))
				commitInventoryFixture(t, fixture.root)
				externalEffectsRefusal(t, fixture, "assignment location ownership is unresolved")
				if _, err := Collect(fixture.root); err != nil && strings.Contains(err.Error(), "below the field") {
					t.Fatalf("pointer-field write met the inline-field refusal: %v", err)
				}
			})
		}
	}
}
