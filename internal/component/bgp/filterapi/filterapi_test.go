// Design: docs/architecture/api/architecture.md -- BGP route filter pipeline
//
// Tests moved from internal/component/plugin/registry/registry_test.go when
// the BGP filter pipeline relocated to this BGP-owned seam package. The
// assertions are unchanged: ordering semantics (stage, then priority, then
// name), ModAccumulator behavior, and attr-mod handler registration must
// stay identical to the pre-move registry behavior.

package filterapi

import (
	"bytes"
	"reflect"
	"testing"
)

// --- RS forwarding capability tests ---

// VALIDATES: P1 AC-2 -- the reactor RS fast-path forwarding capability is inert
// (disabled) unless a plugin activates it. A binary that does not link the rs
// plugin (like this test binary) never calls EnableRSForwarding, so the flag
// stays false and the reactor gate is inert.
// PREVENTS: the reactor forwarding RS UPDATEs when no rs plugin is present.
func TestRSForwardingDefaultDisabled(t *testing.T) {
	snap := Snapshot()
	defer Restore(snap)
	ResetForTest()
	if RSForwardingEnabled() {
		t.Fatal("RSForwardingEnabled() = true after ResetForTest, want false (no plugin activated it)")
	}
}

// VALIDATES: P1 AC-1 -- a plugin activating the capability at registration makes
// the reactor fast path eligible to run.
// PREVENTS: the reactor ignoring an activated rs plugin.
func TestRSForwardingEnable(t *testing.T) {
	snap := Snapshot()
	defer Restore(snap)
	ResetForTest()
	EnableRSForwarding()
	if !RSForwardingEnabled() {
		t.Fatal("RSForwardingEnabled() = false after EnableRSForwarding, want true")
	}
}

// VALIDATES: Snapshot/Restore and ResetForTest round-trip the capability flag so
// tests that toggle it do not leak state into other tests.
// PREVENTS: cross-test contamination of the global RS-forwarding flag.
func TestRSForwardingSnapshotRestore(t *testing.T) {
	outer := Snapshot()
	defer Restore(outer)

	ResetForTest()
	EnableRSForwarding()
	enabledSnap := Snapshot()

	ResetForTest()
	if RSForwardingEnabled() {
		t.Fatal("RSForwardingEnabled() = true after ResetForTest, want false")
	}
	Restore(enabledSnap)
	if !RSForwardingEnabled() {
		t.Fatal("RSForwardingEnabled() = false after Restore of enabled snapshot, want true")
	}
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

// VALIDATES: OpCopy stores stack-owned bytes in accumulator-owned storage.
// PREVENTS: AttrOp.Buf pointing at caller scratch that is mutated or goes out of scope.
func TestModAccumulator_OpCopyOwnsBuffer(t *testing.T) {
	var mods ModAccumulator
	src := []byte{0x20, 0x01, 0x0d, 0xb8}
	mods.OpCopy(14, AttrModSet, src)
	src[0] = 0xff

	ops := mods.Ops()
	if len(ops) != 1 {
		t.Fatalf("Ops() len = %d, want 1", len(ops))
	}
	if bytes.Equal(ops[0].Buf, src) {
		t.Fatal("OpCopy retained caller buffer")
	}
	want := []byte{0x20, 0x01, 0x0d, 0xb8}
	if !bytes.Equal(ops[0].Buf, want) {
		t.Fatalf("OpCopy Buf = %x, want %x", ops[0].Buf, want)
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

// VALIDATES: rib-arch-8 — SetNLRIRewrite / SetWithdrawnRewrite accumulate,
// count toward HasModifications (but not Len), and clear on Reset.
// PREVENTS: NLRI rewrite leaking across per-peer iterations or being missed by
// the forward-path modification gate.
func TestModAccumulator_NLRIRewrite(t *testing.T) {
	var mods ModAccumulator
	if mods.HasModifications() || mods.NLRIRewrite() != nil || mods.WithdrawnRewrite() != nil {
		t.Fatal("empty ModAccumulator reports a rewrite")
	}

	nlri := []byte{24, 10, 0, 0} // 10.0.0.0/24
	mods.SetNLRIRewrite(nlri)
	if !bytes.Equal(mods.NLRIRewrite(), nlri) {
		t.Fatalf("NLRIRewrite() = %x, want %x", mods.NLRIRewrite(), nlri)
	}
	if !mods.HasModifications() {
		t.Fatal("HasModifications() false after SetNLRIRewrite")
	}
	if mods.Len() != 0 {
		t.Fatalf("Len() = %d, want 0 (rewrite is not an attribute op)", mods.Len())
	}

	wd := []byte{24, 172, 16, 0} // 172.16.0.0/24
	mods.SetWithdrawnRewrite(wd)
	if !bytes.Equal(mods.WithdrawnRewrite(), wd) {
		t.Fatalf("WithdrawnRewrite() = %x, want %x", mods.WithdrawnRewrite(), wd)
	}

	mods.Reset()
	if mods.NLRIRewrite() != nil || mods.WithdrawnRewrite() != nil || mods.HasModifications() {
		t.Fatal("Reset did not clear NLRI/withdrawn rewrites")
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
	handler := AttrModHandler(func(p *AttrPlan) {
		called = true
		p.Drop()
	})

	RegisterAttrModHandler(35, handler)
	t.Cleanup(func() { unregisterAttrModHandler(35) })

	got := AttrModHandlerFor(35)
	if got == nil {
		t.Fatal("AttrModHandlerFor returned nil for registered code")
	}

	// A handler plans rather than writes, so the call needs a planner. The
	// attribute is absent from the source (nil src), which is the shape a
	// created attribute has.
	var mods ModAccumulator
	edit := mods.EditSet()
	edit.Begin()
	got(edit.Attr(35, nil, 0, 0, 0, 0, nil, 0))
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
	h := AttrModHandler(func(p *AttrPlan) { p.Drop() })

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

// TestPeerChainStageSortsLast verifies that the stage constants strictly
// increase, so FilterStagePeerChain orders after every other stage and the
// reactor's per-peer policy chain (bound at that stage) runs LAST, after the
// engine's protocol step. TestLessOrderMatchesSort proves the comparator reads
// the stage first.
//
// VALIDATES: spec-unify-filters R-1 / AC-2 -- policy chain runs after OTC (Annotation).
// PREVENTS: a regression that places the external chain before Annotation.
func TestPeerChainStageSortsLast(t *testing.T) {
	if FilterStageProtocol >= FilterStagePolicy ||
		FilterStagePolicy >= FilterStageAnnotation ||
		FilterStageAnnotation >= FilterStagePeerChain {
		t.Fatalf("stage constants not strictly increasing: protocol=%d policy=%d annotation=%d peerchain=%d",
			FilterStageProtocol, FilterStagePolicy, FilterStageAnnotation, FilterStagePeerChain)
	}
}

// TestLessOrderMatchesSort verifies the exported comparator is stage, then
// priority, then name, the key the reactor sorts its engine steps with.
func TestLessOrderMatchesSort(t *testing.T) {
	// Stage dominates.
	if !LessOrder("z", FilterStagePolicy, 99, "a", FilterStageAnnotation, 0) {
		t.Error("lower stage must sort first regardless of priority/name")
	}
	// Priority breaks a stage tie.
	if !LessOrder("z", FilterStagePolicy, 0, "a", FilterStagePolicy, 10) {
		t.Error("lower priority must sort first within a stage")
	}
	// Name breaks a stage+priority tie.
	if !LessOrder("alpha", FilterStagePolicy, 0, "bravo", FilterStagePolicy, 0) {
		t.Error("name must break a stage+priority tie")
	}
	// The peer-chain stage sorts after Annotation.
	if !LessOrder("otc", FilterStageAnnotation, 0, "chain", FilterStagePeerChain, 0) {
		t.Error("Annotation must sort before FilterStagePeerChain")
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

// --- T1-3: Reset as the per-destination isolation boundary ---

// VALIDATES: AC-7, A-4 -- Reset clears every field a later destination can read.
// After a Reset the accumulator is indistinguishable from a fresh value through
// its whole public surface, and a refill with FEWER operations exposes nothing
// from the richer previous destination.
//
// PREVENTS: The cross-peer leak the hoist makes possible. The forward rails now
// share ONE accumulator across every destination, so a field Reset forgets is
// one peer receiving another peer's attributes -- the highest-risk failure in
// spec-hotpath-alloc-round-4's Security Review.
//
// The reflection sweep is the load-bearing part: it fails when someone ADDS a
// field to ModAccumulator and does not clear it, which is the way this decays.
func TestAccumulatorResetClearsEverything(t *testing.T) {
	// Destination 1: fill every field.
	var mods ModAccumulator
	mods.Op(35, AttrModSet, []byte{0x00, 0x00, 0xFD, 0xE8})
	mods.Op(8, AttrModAdd, []byte{0xFF, 0xFF, 0x00, 0x01})
	mods.OpCopy(16, AttrModRemove, []byte{0xAA, 0xBB, 0xCC, 0xDD})
	mods.SetWithdraw()
	mods.SetNLRIRewrite([]byte{24, 10, 0, 0})
	mods.SetWithdrawnRewrite([]byte{24, 172, 16, 0})

	if mods.Len() != 3 || !mods.IsWithdraw() || !mods.HasModifications() {
		t.Fatal("fixture did not fill the accumulator")
	}

	// Fill the EDIT SET too, or the sweep below is vacuous: an edit set that was
	// never planned into is already the zero value, so every field passes whether
	// Reset clears it or not. Planning one attribute makes the sweep test what it
	// claims to (ai/rules/testing.md, mutation-verify).
	srcAttr := []byte{0x40, 5, 4, 0, 0, 0, 100}
	edit := mods.EditSet()
	edit.Begin()
	planOps := []AttrOp{{Code: 5, Action: AttrModSet, Buf: []byte{0, 0, 0, 200}}}
	p := edit.Attr(5, srcAttr, 0, len(srcAttr), 3, 4, planOps, 0)
	p.Keep(0, 4)
	p.New([]byte{0xAA})
	p.Op(0)
	p.Emit(0x40, 5)
	if id := edit.Commit(p); edit.SlotFailed(id) {
		t.Fatal("fixture did not fill the edit set")
	}
	if edit.SlotCount() == 0 {
		t.Fatal("fixture did not fill the edit set")
	}

	mods.Reset()

	// Public surface: identical to a fresh value.
	var fresh ModAccumulator
	if mods.Len() != fresh.Len() {
		t.Fatalf("after Reset, Len() = %d, want %d", mods.Len(), fresh.Len())
	}
	if len(mods.Ops()) != 0 {
		t.Fatalf("after Reset, Ops() len = %d, want 0", len(mods.Ops()))
	}
	if mods.IsWithdraw() {
		t.Fatal("after Reset, IsWithdraw() is true -- destination 2 would be withdrawn by destination 1's policy")
	}
	if mods.NLRIRewrite() != nil {
		t.Fatalf("after Reset, NLRIRewrite() = %x, want nil", mods.NLRIRewrite())
	}
	if mods.WithdrawnRewrite() != nil {
		t.Fatalf("after Reset, WithdrawnRewrite() = %x, want nil", mods.WithdrawnRewrite())
	}
	if mods.HasModifications() {
		t.Fatal("after Reset, HasModifications() is true -- the forward path would rebuild a payload from nothing")
	}

	// Every field, including ones added after this test was written. `inline` is
	// the one deliberate exception: it is a bump arena made unreachable by
	// inlineOff, and re-zeroing it is exactly the cost the hoist exists to
	// remove. `ops` keeps its backing array by design; its LENGTH must be zero.
	rv := reflect.ValueOf(&mods).Elem()
	rt := rv.Type()
	for i := range rt.NumField() {
		name := rt.Field(i).Name
		f := rv.Field(i)
		switch name {
		case "inline":
			continue // deliberate: see TestAccumulatorResetIsConstantTime
		case "edit":
			// The edit set resets in O(1) for exactly the reason `inline` does:
			// it re-slices its three backing arrays to zero length instead of
			// re-zeroing them, so it is EMPTY without being the zero VALUE.
			// Sweep it the same way one level down, with its arrays and its
			// reusable planner as the deliberate exceptions, so a field added
			// to EditSet and left uncleared still fails here.
			ev := f
			et := ev.Type()
			for j := range et.NumField() {
				en := et.Field(j).Name
				ef := ev.Field(j)
				switch en {
				case "slotsArr", "fragsArr", "arenaArr":
					continue // backing arrays: unreachable at zero length, never re-zeroed
				// "cur", the reusable planner, is NOT exempt. It is the one part
				// of the edit set that holds pointers -- the source attribute,
				// its value and the operation slice -- so leaving it set keeps
				// the PREVIOUS destination's payload buffer alive for the whole
				// of the next one. Attr overwrites it before every handler call,
				// so nothing stale is ever readable, but reachable-for-the-GC is
				// still a boundary this type claims not to cross. It falls
				// through to the zero check below.
				case "slots", "frags", "arena":
					if ef.Len() != 0 {
						t.Fatalf("after Reset, edit set field %q has len %d, want 0 -- destination 1's plan is still readable", en, ef.Len())
					}
				default:
					if !ef.IsZero() {
						t.Fatalf("after Reset, edit set field %q is not zero -- a later destination can read destination 1's plan. "+
							"If this field was just added, clear it in EditSet.reset", en)
					}
				}
			}
		case "ops":
			if f.Len() != 0 {
				t.Fatalf("after Reset, field %q has len %d, want 0", name, f.Len())
			}
		default:
			if !f.IsZero() {
				t.Fatalf("after Reset, field %q is not zero -- a later destination can read destination 1's state. "+
					"If this field was just added, clear it in ModAccumulator.Reset", name)
			}
		}
	}

	// Destination 2: refill with FEWER operations than destination 1 had. This
	// is the shape that exposes a partial reset -- a stale third op would still
	// be reachable through Ops() if Reset had not truncated the slice.
	mods.Op(9, AttrModSet, []byte{0x0A, 0x0B, 0x0C, 0x0D})

	ops := mods.Ops()
	if len(ops) != 1 {
		t.Fatalf("destination 2 sees %d ops, want exactly its own 1 -- destination 1's operations survived", len(ops))
	}
	if ops[0].Code != 9 || ops[0].Action != AttrModSet {
		t.Fatalf("destination 2 op = {code %d, action %d}, want {9, AttrModSet}", ops[0].Code, ops[0].Action)
	}
	if !bytes.Equal(ops[0].Buf, []byte{0x0A, 0x0B, 0x0C, 0x0D}) {
		t.Fatalf("destination 2 op buf = %x, want 0a0b0c0d", ops[0].Buf)
	}

	// The arena is a bump allocator: destination 2's OpCopy must hand back a
	// window holding ONLY destination 2's bytes, never a tail of destination 1's.
	mods.OpCopy(17, AttrModSet, []byte{0x11, 0x22})
	copied := mods.Ops()[1].Buf
	if !bytes.Equal(copied, []byte{0x11, 0x22}) {
		t.Fatalf("arena window after Reset = %x, want 1122 -- destination 1's bytes are readable", copied)
	}
}

// VALIDATES: Reset's cost is independent of the inline arena size, so the
// roughly eightfold arena growth in spec-wire-edit-2-edit-apply cannot turn the
// per-destination Reset back into the per-destination zeroing this item removed.
//
// PREVENTS: A well-meaning "clear everything" edit to Reset that re-zeroes the
// arena. That edit would look safer, pass every isolation test above, and
// silently undo T1-3 -- the hoist would then buy nothing at all.
//
// The assertion is structural, not a timing measurement: Reset is O(1) exactly
// when it writes nothing to the arena, so the test proves the arena bytes
// survive it. A wall-clock comparison would be a flaky way to assert the same
// property (ai/rules/completion.md).
func TestAccumulatorResetIsConstantTime(t *testing.T) {
	var mods ModAccumulator

	// Fill the whole arena so any zeroing is unmissable.
	full := make([]byte, modAccumulatorInlineBytes)
	for i := range full {
		full[i] = byte(i + 1)
	}
	mods.OpCopy(8, AttrModSet, full)
	if mods.inlineOff != modAccumulatorInlineBytes {
		t.Fatalf("fixture did not fill the arena: inlineOff = %d, want %d", mods.inlineOff, modAccumulatorInlineBytes)
	}

	mods.Reset()

	if mods.inlineOff != 0 {
		t.Fatalf("after Reset, inlineOff = %d, want 0", mods.inlineOff)
	}
	if !bytes.Equal(mods.inline[:], full) {
		t.Fatal("Reset wrote to the inline arena; its cost now scales with the arena size, " +
			"which is the per-destination zeroing T1-3 removed")
	}

	// O(1) also means allocation-free, whatever the accumulator held.
	mods.Op(1, AttrModSet, []byte{0x00})
	mods.SetNLRIRewrite([]byte{24, 10, 0, 0})
	if n := testing.AllocsPerRun(100, mods.Reset); n != 0 {
		t.Fatalf("Reset allocates %v times per call, want 0", n)
	}
}

// BenchmarkAccumulatorReset records the per-destination cost the forward rails
// now pay instead of constructing a fresh accumulator per destination.
func BenchmarkAccumulatorReset(b *testing.B) {
	var mods ModAccumulator
	value := []byte{0xFF, 0xFF, 0x00, 0x01}
	for b.Loop() {
		mods.Op(8, AttrModAdd, value)
		mods.OpCopy(16, AttrModRemove, value)
		mods.SetWithdraw()
		mods.Reset()
	}
}
