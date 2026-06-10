// Design: docs/architecture/api/architecture.md -- BGP route filter pipeline
//
// Tests moved from internal/component/plugin/registry/registry_test.go when
// the BGP filter pipeline relocated to this BGP-owned seam package. The
// assertions are unchanged: ordering semantics (stage, then priority, then
// name), ModAccumulator behavior, and attr-mod handler registration must
// stay identical to the pre-move registry behavior.

package filterapi

import "testing"

// noopIngress is a stub ingress filter for registration tests.
func noopIngress(_ PeerFilterInfo, _ []byte, _ map[string]any) (bool, []byte) { return true, nil }

// noopEgress is a stub egress filter for registration tests.
func noopEgress(_, _ PeerFilterInfo, _ []byte, _ map[string]any, _ *ModAccumulator) bool {
	return true
}

// --- ModAccumulator tests ---

// VALIDATES: AC-5 — ModAccumulator.Len() returns 0 when empty, no allocation.
// PREVENTS: Accidental allocation on the zero-mod path.
func TestModAccumulator_LazyAlloc(t *testing.T) {
	var mods ModAccumulator
	if mods.Len() != 0 {
		t.Fatalf("empty ModAccumulator.Len() = %d, want 0", mods.Len())
	}
	if ops := mods.Ops(); len(ops) != 0 {
		t.Fatalf("empty ModAccumulator.Ops() returned %d ops", len(ops))
	}
	// Op triggers allocation.
	mods.Op(35, AttrModSet, []byte{0x00, 0x00, 0x00, 0x64})
	if mods.Len() != 1 {
		t.Fatalf("after Op, Len() = %d, want 1", mods.Len())
	}
}

// VALIDATES: AC-6 — Multiple Op calls accumulated, all retrievable.
// PREVENTS: Overwrite or loss of mods from different filters.
func TestModAccumulator_MultipleOps(t *testing.T) {
	var mods ModAccumulator
	mods.Op(35, AttrModSet, []byte{0x00, 0x00, 0x00, 0x00})
	mods.Op(8, AttrModAdd, []byte{0xFF, 0xFF, 0x00, 0x01})
	mods.Op(8, AttrModRemove, []byte{0xFF, 0xFF, 0xFF, 0x03})

	if mods.Len() != 3 {
		t.Fatalf("Len() = %d, want 3", mods.Len())
	}

	ops := mods.Ops()
	if len(ops) != 3 {
		t.Fatalf("Ops() len = %d, want 3", len(ops))
	}
	if ops[0].Code != 35 || ops[0].Action != AttrModSet {
		t.Fatalf("ops[0] = {%d, %d}, want {35, AttrModSet}", ops[0].Code, ops[0].Action)
	}
	if ops[1].Code != 8 || ops[1].Action != AttrModAdd {
		t.Fatalf("ops[1] = {%d, %d}, want {8, AttrModAdd}", ops[1].Code, ops[1].Action)
	}
	if ops[2].Code != 8 || ops[2].Action != AttrModRemove {
		t.Fatalf("ops[2] = {%d, %d}, want {8, AttrModRemove}", ops[2].Code, ops[2].Action)
	}

	// Reset clears.
	mods.Reset()
	if mods.Len() != 0 {
		t.Fatalf("after Reset, Len() = %d, want 0", mods.Len())
	}
}

// --- AttrOp / AttrModHandler tests (v2 progressive build) ---

// VALIDATES: AC-11 — AttrOp holds code, action, buf fields.
// PREVENTS: Wrong structure for mod accumulation.
func TestAttrOpStructure(t *testing.T) {
	op := AttrOp{
		Code:   35,
		Action: AttrModSet,
		Buf:    []byte{0x00, 0x00, 0xFD, 0xE8}, // ASN 65000
	}
	if op.Code != 35 {
		t.Fatalf("Code = %d, want 35", op.Code)
	}
	if op.Action != AttrModSet {
		t.Fatalf("Action = %d, want AttrModSet", op.Action)
	}
	if len(op.Buf) != 4 {
		t.Fatalf("Buf len = %d, want 4", len(op.Buf))
	}
}

// VALIDATES: AC-11 — ModAccumulator.Op() stores AttrOp entries, Len() reflects count.
// PREVENTS: Op() not accumulating, or Len() wrong.
func TestModAccumulatorOp(t *testing.T) {
	var mods ModAccumulator
	if mods.Len() != 0 {
		t.Fatalf("empty Len() = %d, want 0", mods.Len())
	}

	mods.Op(35, AttrModSet, []byte{0x00, 0x00, 0xFD, 0xE8})
	if mods.Len() != 1 {
		t.Fatalf("after Op, Len() = %d, want 1", mods.Len())
	}

	// Multiple ops on same code accumulate separately.
	mods.Op(8, AttrModAdd, []byte{0xFF, 0xFF, 0x00, 0x01})
	mods.Op(8, AttrModRemove, []byte{0xFF, 0xFF, 0xFF, 0x03})
	if mods.Len() != 3 {
		t.Fatalf("after 3 ops, Len() = %d, want 3", mods.Len())
	}

	ops := mods.Ops()
	if len(ops) != 3 {
		t.Fatalf("Ops() len = %d, want 3", len(ops))
	}
	if ops[0].Code != 35 || ops[0].Action != AttrModSet {
		t.Fatalf("ops[0] = {%d, %d}, want {35, AttrModSet}", ops[0].Code, ops[0].Action)
	}
	if ops[1].Code != 8 || ops[1].Action != AttrModAdd {
		t.Fatalf("ops[1] = {%d, %d}, want {8, AttrModAdd}", ops[1].Code, ops[1].Action)
	}
	if ops[2].Code != 8 || ops[2].Action != AttrModRemove {
		t.Fatalf("ops[2] = {%d, %d}, want {8, AttrModRemove}", ops[2].Code, ops[2].Action)
	}
}

// VALIDATES: AC-11 — ModAccumulator.Reset() clears ops for reuse.
// PREVENTS: Stale ops leaking between peers.
func TestModAccumulatorOpReset(t *testing.T) {
	var mods ModAccumulator
	mods.Op(35, AttrModSet, []byte{0x00, 0x00, 0xFD, 0xE8})
	mods.Reset()
	if mods.Len() != 0 {
		t.Fatalf("after Reset, Len() = %d, want 0", mods.Len())
	}
	if ops := mods.Ops(); len(ops) != 0 {
		t.Fatalf("after Reset, Ops() len = %d, want 0", len(ops))
	}
}

// VALIDATES: AC-12 — AttrModHandler registered by attr code, retrievable.
// PREVENTS: Handler registration lost or wrong code mapping.
func TestAttrModHandlerRegistration(t *testing.T) {
	called := false
	handler := AttrModHandler(func(src []byte, ops []AttrOp, buf []byte, off int) int {
		called = true
		return off
	})

	RegisterAttrModHandler(35, handler)
	t.Cleanup(func() { unregisterAttrModHandler(35) })

	got := AttrModHandlerFor(35)
	if got == nil {
		t.Fatal("AttrModHandlerFor returned nil for registered code")
	}

	buf := make([]byte, 64)
	got(nil, nil, buf, 0)
	if !called {
		t.Fatal("handler was not called")
	}
}

// VALIDATES: AC-18 — Unknown attr code returns nil handler.
// PREVENTS: Panic on unregistered code lookup.
func TestAttrModHandlerNotFound(t *testing.T) {
	got := AttrModHandlerFor(99)
	if got != nil {
		t.Fatal("AttrModHandlerFor returned non-nil for unregistered code")
	}
}

// VALIDATES: AC-12 — AttrModHandlers returns snapshot for reactor startup.
// PREVENTS: Reactor sharing mutable reference with registry.
func TestAttrModHandlersSnapshot(t *testing.T) {
	h := AttrModHandler(func(src []byte, ops []AttrOp, buf []byte, off int) int { return off })

	RegisterAttrModHandler(200, h)
	RegisterAttrModHandler(201, h)
	t.Cleanup(func() {
		unregisterAttrModHandler(200)
		unregisterAttrModHandler(201)
	})

	snap := AttrModHandlers()
	if snap[200] == nil || snap[201] == nil {
		t.Fatal("snapshot missing registered handlers")
	}

	// Mutating snapshot must not affect registry.
	delete(snap, 200)
	if AttrModHandlerFor(200) == nil {
		t.Fatal("deleting from snapshot affected the registry")
	}
}

// VALIDATES: RegisterAttrModHandler ignores nil handler.
// PREVENTS: Nil handler registered leading to panic in progressive build.
func TestRegisterAttrModHandlerNil(t *testing.T) {
	RegisterAttrModHandler(250, nil)
	t.Cleanup(func() { unregisterAttrModHandler(250) })

	got := AttrModHandlerFor(250)
	if got != nil {
		t.Fatal("nil handler should not be registered")
	}
}

// --- Filter chain registration tests ---

// registerNoop registers a noop ingress filter under name with the given
// stage and priority, failing the test on error.
func registerNoop(t *testing.T, name string, stage, priority int) {
	t.Helper()
	if err := Register(Filter{Name: name, Stage: stage, Priority: priority, Ingress: noopIngress}); err != nil {
		t.Fatal(err)
	}
}

// TestFilterPriorityOrdering verifies that IngressFilters and EgressFilters return
// filters sorted by stage first, then priority, then by plugin name.
//
// VALIDATES: AC-12 -- filters execute in stage+priority order.
// PREVENTS: Non-deterministic filter ordering from map iteration.
func TestFilterPriorityOrdering(t *testing.T) {
	snap := Snapshot()
	t.Cleanup(func() { Restore(snap) })
	ResetForTest()

	// Register four plugins across stages and priorities.
	// "community" stage Policy priority 0, "loop" stage Protocol priority 0,
	// "otc" stage Annotation priority 0, "prefix" stage Policy priority 10.
	// Expected order: loop (Protocol/0), community (Policy/0), prefix (Policy/10), otc (Annotation/0).
	registerNoop(t, "community", FilterStagePolicy, 0)
	registerNoop(t, "loop", FilterStageProtocol, 0)
	registerNoop(t, "otc", FilterStageAnnotation, 0)
	registerNoop(t, "prefix", FilterStagePolicy, 10)

	names := ingressFilterNames()
	if len(names) != 4 {
		t.Fatalf("ingressFilterNames() len = %d, want 4", len(names))
	}
	want := []string{"loop", "community", "prefix", "otc"}
	for i, w := range want {
		if names[i] != w {
			t.Errorf("names[%d] = %q, want %q (full order: %v)", i, names[i], w, names)
		}
	}
	if got := len(IngressFilters()); got != 4 {
		t.Fatalf("IngressFilters() len = %d, want 4", got)
	}
}

// TestFilterSameStageNameBreaksTie verifies that name is the tiebreaker
// when both stage and priority are equal.
//
// VALIDATES: Deterministic ordering within identical stage+priority.
// PREVENTS: Random ordering from map iteration when priorities match.
func TestFilterSameStageNameBreaksTie(t *testing.T) {
	snap := Snapshot()
	t.Cleanup(func() { Restore(snap) })
	ResetForTest()

	for _, name := range []string{"charlie", "alpha", "bravo"} {
		registerNoop(t, name, FilterStagePolicy, 0)
	}

	names := ingressFilterNames()
	want := []string{"alpha", "bravo", "charlie"}
	for i, w := range want {
		if names[i] != w {
			t.Errorf("names[%d] = %q, want %q", i, names[i], w)
		}
	}
}

// TestEgressFilterOrdering verifies the egress chain uses the same
// stage/priority/name ordering as the ingress chain.
//
// VALIDATES: Egress chain ordering matches ingress semantics.
// PREVENTS: Divergent ordering between the two chains.
func TestEgressFilterOrdering(t *testing.T) {
	snap := Snapshot()
	t.Cleanup(func() { Restore(snap) })
	ResetForTest()

	for _, f := range []Filter{
		{Name: "annotate", Stage: FilterStageAnnotation, Egress: noopEgress},
		{Name: "policy", Stage: FilterStagePolicy, Egress: noopEgress},
	} {
		if err := Register(f); err != nil {
			t.Fatal(err)
		}
	}

	names := egressFilterNames()
	want := []string{"policy", "annotate"}
	if len(names) != len(want) {
		t.Fatalf("egressFilterNames() = %v, want %v", names, want)
	}
	for i, w := range want {
		if names[i] != w {
			t.Errorf("names[%d] = %q, want %q", i, names[i], w)
		}
	}
	if got := len(EgressFilters()); got != 2 {
		t.Fatalf("EgressFilters() len = %d, want 2", got)
	}
}

// TestRegisterValidation verifies Register rejects invalid filters.
//
// VALIDATES: Empty name, no funcs, and duplicate name all error.
// PREVENTS: Silent acceptance of broken filter registrations.
func TestRegisterValidation(t *testing.T) {
	snap := Snapshot()
	t.Cleanup(func() { Restore(snap) })
	ResetForTest()

	if err := Register(Filter{Name: "", Ingress: noopIngress}); err == nil {
		t.Error("Register with empty name succeeded, want error")
	}
	if err := Register(Filter{Name: "no-funcs"}); err == nil {
		t.Error("Register with nil Ingress and nil Egress succeeded, want error")
	}
	if err := Register(Filter{Name: "dup", Ingress: noopIngress}); err != nil {
		t.Fatal(err)
	}
	if err := Register(Filter{Name: "dup", Ingress: noopIngress}); err == nil {
		t.Error("duplicate Register succeeded, want error")
	}
}

// TestPeerFilterInfoFields verifies that PeerFilterInfo has Name and GroupName fields.
//
// VALIDATES: AC-20 -- PeerFilterInfo includes peer identity fields.
// PREVENTS: Filters building their own address-to-name lookup maps.
func TestPeerFilterInfoFields(t *testing.T) {
	info := PeerFilterInfo{
		Name:      "upstream-1",
		GroupName: "transit",
	}
	if info.Name != "upstream-1" {
		t.Errorf("Name = %q, want %q", info.Name, "upstream-1")
	}
	if info.GroupName != "transit" {
		t.Errorf("GroupName = %q, want %q", info.GroupName, "transit")
	}
}
