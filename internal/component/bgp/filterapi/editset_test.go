package filterapi

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildAttr lays out one wire attribute: flags, code, length, value.
func buildAttr(flags, code byte, val []byte) []byte {
	if len(val) > 255 || flags&0x10 != 0 {
		out := make([]byte, 4+len(val))
		out[0] = flags | 0x10
		out[1] = code
		out[2] = byte(len(val) >> 8)
		out[3] = byte(len(val))
		copy(out[4:], val)
		return out
	}
	out := make([]byte, 3+len(val))
	out[0] = flags
	out[1] = code
	out[2] = byte(len(val))
	copy(out[3:], val)
	return out
}

// planOne drives one handler over a single-attribute section and returns the
// emitted bytes. section is the attribute itself, so a verbatim slot resolves
// against the same bytes the handler was shown.
func planOne(t *testing.T, e *EditSet, code uint8, srcAttr []byte, ops []AttrOp, h func(*AttrPlan)) (SlotID, []byte) {
	t.Helper()
	var src []byte
	srcOff, srcLen, valOff, valLen := 0, 0, 0, 0
	if len(srcAttr) > 0 {
		hdr := 3
		if srcAttr[0]&0x10 != 0 {
			hdr = 4
		}
		src = srcAttr
		srcLen = len(srcAttr)
		valOff, valLen = hdr, len(srcAttr)-hdr
	}
	p := e.Attr(code, src, srcOff, srcLen, valOff, valLen, ops, 0)
	h(p)
	id := e.Commit(p)
	if e.SlotFailed(id) {
		return id, nil
	}
	n := e.SlotSize(id)
	buf := make([]byte, n+8) // deliberate slack: the write must not use it
	got := e.SlotWrite(id, srcAttr, ops, buf, 0)
	require.Equal(t, n, got, "SlotWrite must return exactly off+SlotSize")
	return id, buf[:n]
}

// VALIDATES: AC-2, AC-11 — the size query equals the bytes the writer then
// writes, for every slot kind and both header size classes.
// PREVENTS: the one invariant this design exists to guarantee silently drifting.
// A size that is an upper bound rather than an exact count is what forced the
// old slack buffer, and with it the overflow branch that abandoned every
// modification and forwarded the route unchanged.
func TestEditSizeIsExact(t *testing.T) {
	short := bytes.Repeat([]byte{0xAB}, 4)
	// 255 and 256 are the header size class boundary: RFC 4271 Section 4.3 needs
	// the Extended Length header from 256 octets up.
	at255 := bytes.Repeat([]byte{0xCD}, 255)
	at256 := bytes.Repeat([]byte{0xCE}, 256)

	cases := []struct {
		name    string
		srcAttr []byte
		ops     []AttrOp
		plan    func(*AttrPlan)
		wantLen int
		wantHdr int
	}{
		{
			name:    "fragments over the source value",
			srcAttr: buildAttr(0x40, 5, short),
			plan:    func(p *AttrPlan) { p.Keep(0, 4); p.Emit(0x40, 5) },
			wantLen: 3 + 4,
			wantHdr: 3,
		},
		{
			name:    "value from an operation buffer",
			srcAttr: nil,
			ops:     []AttrOp{{Code: 4, Action: AttrModSet, Buf: short}},
			plan:    func(p *AttrPlan) { p.Op(0); p.Emit(0x80, 4) },
			wantLen: 3 + 4,
			wantHdr: 3,
		},
		{
			name:    "synthesized arena bytes",
			srcAttr: nil,
			plan:    func(p *AttrPlan) { p.New(short); p.NewByte(0x01); p.Emit(0xC0, 9) },
			wantLen: 3 + 5,
			wantHdr: 3,
		},
		{
			name:    "verbatim keeps the source header class",
			srcAttr: buildAttr(0x40, 5, short),
			plan:    func(p *AttrPlan) { p.KeepAll() },
			wantLen: 3 + 4,
			wantHdr: 3,
		},
		{
			name:    "last value of the short header class",
			srcAttr: buildAttr(0x40, 5, at255),
			plan:    func(p *AttrPlan) { p.Keep(0, 255); p.Emit(0x40, 5) },
			wantLen: 3 + 255,
			wantHdr: 3,
		},
		{
			name:    "first value of the extended header class",
			srcAttr: buildAttr(0x40, 5, at256),
			plan:    func(p *AttrPlan) { p.Keep(0, 256); p.Emit(0x40, 5) },
			wantLen: 4 + 256,
			wantHdr: 4,
		},
		{
			name:    "extended forced on a short value",
			srcAttr: buildAttr(0xC0, 8, short),
			plan:    func(p *AttrPlan) { p.Keep(0, 4); p.EmitExtended(0xC0, 8) },
			wantLen: 4 + 4,
			wantHdr: 4,
		},
		{
			name:    "mixed sources in one value",
			srcAttr: buildAttr(0x80, 14, bytes.Repeat([]byte{0x11}, 10)),
			ops:     []AttrOp{{Code: 14, Action: AttrModSet, Buf: short}},
			plan: func(p *AttrPlan) {
				p.Keep(0, 3)
				p.NewByte(4)
				p.Op(0)
				p.Keep(4, 6)
				p.Emit(0x80, 14)
			},
			wantLen: 3 + 3 + 1 + 4 + 6,
			wantHdr: 3,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var mods ModAccumulator
			e := mods.EditSet()
			e.Begin()
			id, out := planOne(t, e, 5, tc.srcAttr, tc.ops, tc.plan)

			assert.Equal(t, tc.wantLen, e.SlotSize(id), "the size query must be exact")
			require.Len(t, out, tc.wantLen, "the writer must produce exactly the queried size")

			if e.SlotVerbatim(id) {
				assert.Equal(t, tc.srcAttr, out, "a verbatim slot emits the source unchanged")
				return
			}
			// The declared length field must agree with the bytes that follow it.
			if tc.wantHdr == 4 {
				assert.Equal(t, byte(0x10), out[0]&0x10, "the extended length flag must be set")
				declared := int(out[2])<<8 | int(out[3])
				assert.Equal(t, tc.wantLen-4, declared, "the 2-octet length must match the value")
				return
			}
			assert.Zero(t, out[0]&0x10, "the extended length flag must be clear")
			assert.Equal(t, tc.wantLen-3, int(out[2]), "the 1-octet length must match the value")
		})
	}
}

// VALIDATES: a fragment over bytes that already exist never enters the arena.
// PREVENTS: the arena quietly becoming a second copy of the payload, which is
// the intermediate buffer this whole model exists to remove.
func TestArenaHoldsOnlyNewBytes(t *testing.T) {
	src := buildAttr(0xC0, 8, bytes.Repeat([]byte{0x22}, 32))
	opBuf := bytes.Repeat([]byte{0x33}, 8)

	var mods ModAccumulator
	e := mods.EditSet()
	e.Begin()

	_, out := planOne(t, e, 8, src, []AttrOp{{Code: 8, Action: AttrModAdd, Buf: opBuf}},
		func(p *AttrPlan) {
			p.Keep(0, 32) // already on the wire
			p.Op(0)       // already in the accumulator
			p.EmitExtended(0xC0, 8)
		})

	assert.Len(t, out, 4+40, "both sources contributed")
	assert.Empty(t, e.arena, "neither Keep nor Op may copy into the arena")

	// Only a byte that exists nowhere else lands there.
	e.Begin()
	_, synth := planOne(t, e, 14, src, nil, func(p *AttrPlan) {
		p.NewByte(0x07)
		p.Emit(0x80, 14)
	})
	assert.Equal(t, []byte{0x80, 14, 1, 0x07}, synth, "the synthesized byte reached the wire")
	assert.Equal(t, []byte{0x07}, e.arena, "a synthesized byte is the arena's only job")
}

// VALIDATES: AC-10 — reset cost does not scale with inline capacity.
// PREVENTS: a larger inline edit set turning the hoist into a regression. Reset
// runs once per DESTINATION, so a reset that re-zeroed its arrays would cost
// more on a wide fan-out than the copies the fragments save.
func TestResetIsConstantTime(t *testing.T) {
	var mods ModAccumulator
	e := mods.EditSet()
	e.Begin()

	id, _ := planOne(t, e, 9, nil, nil, func(p *AttrPlan) {
		p.New([]byte{0xDE, 0xAD, 0xBE, 0xEF})
		p.Emit(0x80, 9)
	})
	require.False(t, e.SlotFailed(id))
	require.Equal(t, 1, e.SlotCount())
	require.NotEmpty(t, e.arena)

	e.Begin()

	assert.Zero(t, e.SlotCount(), "reset clears the readable state")
	assert.Empty(t, e.arena, "reset clears the readable arena")
	assert.False(t, e.Planned(9), "reset clears the presence bitset")

	// The backing arrays are deliberately NOT re-zeroed: they are unreachable
	// once the lengths are zero, and zeroing them is exactly the cost that would
	// scale with capacity. Asserting the old bytes survive is how this test tells
	// a cheap reset from an expensive one without measuring elapsed time
	// (ai/rules/completion.md bans a timing assertion here).
	assert.Equal(t, byte(0xDE), e.arenaArr[0], "reset must not re-zero the arena array")
	assert.Equal(t, uint8(9), e.slotsArr[0].code, "reset must not re-zero the slot array")
}

// VALIDATES: a handler that plans bytes outside the source value is refused, and
// the refusal is visible to the caller.
// PREVENTS: a fragment resolved against peer-controlled bytes slicing out of
// range at write time. The bound is checked once, where the fragment is named,
// rather than trusted where it is used (ai/rules/evidence.md).
func TestFragmentBoundsAreRefused(t *testing.T) {
	src := buildAttr(0x40, 5, []byte{1, 2, 3, 4})

	cases := map[string]func(*AttrPlan){
		"past the end of the value": func(p *AttrPlan) { p.Keep(0, 5); p.Emit(0x40, 5) },
		"negative offset":           func(p *AttrPlan) { p.Keep(-1, 2); p.Emit(0x40, 5) },
		"no source at all":          func(p *AttrPlan) { p.Keep(0, 1); p.Emit(0x40, 5) },
		"operation out of range":    func(p *AttrPlan) { p.Op(3); p.Emit(0x40, 5) },
	}
	for name, plan := range cases {
		t.Run(name, func(t *testing.T) {
			var mods ModAccumulator
			e := mods.EditSet()
			e.Begin()
			attr := src
			if name == "no source at all" {
				attr = nil
			}
			id, _ := planOne(t, e, 5, attr, nil, plan)
			assert.True(t, e.SlotFailed(id), "an out-of-range fragment must refuse the plan")
		})
	}
}

// VALIDATES: a handler that returns without finishing its plan is a refusal.
// PREVENTS: silence reading as consent. An unfinished plan has no size and no
// bytes, so treating it as "emit nothing" would drop an attribute the policy
// asked to change and report success.
func TestUnfinishedPlanRefuses(t *testing.T) {
	var mods ModAccumulator
	e := mods.EditSet()
	e.Begin()
	id, _ := planOne(t, e, 5, buildAttr(0x40, 5, []byte{1, 2, 3, 4}), nil, func(_ *AttrPlan) {})
	assert.True(t, e.SlotFailed(id), "a handler that plans nothing must not be read as a drop")
}

// VALIDATES: adjacent fragments over the same source coalesce.
// PREVENTS: a community strip that retains a run of consecutive values costing
// one fragment per value. Coalescing is what keeps the inline fragment capacity
// meaningful on a long list.
func TestAdjacentFragmentsCoalesce(t *testing.T) {
	src := buildAttr(0xC0, 8, bytes.Repeat([]byte{0x44}, 16))

	var mods ModAccumulator
	e := mods.EditSet()
	e.Begin()
	_, out := planOne(t, e, 8, src, nil, func(p *AttrPlan) {
		for i := range 4 {
			p.Keep(i*4, 4)
		}
		p.EmitExtended(0xC0, 8)
	})

	assert.Len(t, out, 4+16)
	assert.Len(t, e.frags, 1, "four adjacent value fragments are one run")
}

// VALIDATES: GroupedOps sorts by attribute code and keeps each code's operations
// in the order their producers recorded them.
// PREVENTS: the community handler's remove-then-add-then-set order breaking, and
// the per-code heap slice the previous grouping allocated on every destination.
func TestGroupedOpsIsStableAndAllocationFree(t *testing.T) {
	var mods ModAccumulator
	mods.Op(8, AttrModRemove, []byte{1})
	mods.Op(4, AttrModSet, []byte{2})
	mods.Op(8, AttrModAdd, []byte{3})
	mods.Op(2, AttrModPrepend, []byte{4})
	mods.Op(8, AttrModAdd, []byte{5})

	got := mods.GroupedOps()
	require.Len(t, got, 5)

	codes := make([]uint8, len(got))
	for i := range got {
		codes[i] = got[i].Code
	}
	assert.Equal(t, []uint8{2, 4, 8, 8, 8}, codes, "operations group by ascending code")

	// Within code 8 the producers' order survives, which is what the strip
	// before tag rule depends on.
	assert.Equal(t, []byte{1}, got[2].Buf)
	assert.Equal(t, []byte{3}, got[3].Buf)
	assert.Equal(t, []byte{5}, got[4].Buf)

	allocs := testing.AllocsPerRun(50, func() {
		mods.grouped = false
		_ = mods.GroupedOps()
	})
	assert.Zero(t, allocs, "grouping must not allocate; it replaced a per-code heap slice")
}
