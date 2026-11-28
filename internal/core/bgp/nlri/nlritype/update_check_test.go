package nlritype

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/core/bgp/attribute"
	"github.com/ze-software/ze/internal/core/bgp/nlri/nlrisplit"
	"github.com/ze-software/ze/internal/core/family"
)

// These tests drive the UPDATE-level check through RetainUpdate with a stand-in check,
// so they see only what Retain does with its answer. The MUP plugin's real check is
// tested in its own package and through the reactor.

// prefixSIDAttrs is ORIGIN IGP, an empty AS_PATH, then a Prefix-SID attribute (type 40)
// holding one Label-Index TLV.
var prefixSIDAttrs = []byte{
	0x40, 0x01, 0x01, 0x00,
	0x40, 0x02, 0x00,
	0xc0, 0x28, 0x0a, 0x01, 0x00, 0x07, 0x00, 0x00, 0x00, 0x00, 0x00, 0x03, 0x09,
}

// withdrawWithoutPrefixSID answers Withdraw for every NLRI whose UPDATE carries no
// Prefix-SID attribute.
func withdrawWithoutPrefixSID(_ []byte, _ bool, attrs *UpdateAttrs) Verdict {
	if attrs.Has(attribute.AttrPrefixSID) {
		return Keep
	}
	return Withdraw
}

// keepAll is a recognizer that implements every type.
func keepAll([]byte, bool) Verdict { return Keep }

// VALIDATES: Has finds an attribute in the section, and answers absent for one that
// attribute-discard removes after the NLRIs are judged.
// PREVENTS: a check seeing a Prefix-SID the RFC 8669 Section 4 EBGP discard is about to
// remove, so the route it should withdraw is kept with no Prefix-SID.
func TestUpdateAttrsHasHonorsDiscarding(t *testing.T) {
	attrs := NewUpdateAttrs(prefixSIDAttrs)
	if !attrs.Has(attribute.AttrPrefixSID) {
		t.Fatal("the Prefix-SID is in the section")
	}
	if attrs.Has(attribute.AttrMED) {
		t.Fatal("no MED is in the section")
	}
	attrs.Discarding(attribute.AttrPrefixSID)
	if attrs.Has(attribute.AttrPrefixSID) {
		t.Fatal("a Prefix-SID being discarded is absent from the UPDATE that is installed")
	}
	if !attrs.Has(attribute.AttrOrigin) {
		t.Fatal("discarding one code hides only that code")
	}
	empty := UpdateAttrs{}
	if empty.Has(attribute.AttrOrigin) {
		t.Fatal("the zero value holds no attributes")
	}
}

// VALIDATES: an announced NLRI the check answers Withdraw for moves to Withdrawn; the same
// NLRI in an UPDATE whose attributes satisfy the check is kept, zero copy.
// PREVENTS: the check's verdict being ignored, or applied when it should not be.
func TestRetainUpdateAppliesCheckToAnnouncements(t *testing.T) {
	data := mupISD([]byte{10, 0, 0})
	var scratch [nlrisplit.PrefixKeyScratchSize]byte

	without := NewUpdateAttrs(prefixSIDAttrs[:7])
	r, err := RetainUpdate(keepAll, withdrawWithoutPrefixSID, &without, mupFamily, data, false, true, scratch[:])
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(r.Kept) != 0 || !bytes.Equal(r.Withdrawn, data) || r.WithdrawnCount != 1 {
		t.Fatalf("the route must be withdrawn, got %+v", r)
	}

	with := NewUpdateAttrs(prefixSIDAttrs)
	r, err = RetainUpdate(keepAll, withdrawWithoutPrefixSID, &with, mupFamily, data, false, true, scratch[:])
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Removed() || &r.Kept[0] != &data[0] {
		t.Fatalf("a route the check keeps must be returned unchanged, got %+v", r)
	}
}

// VALIDATES: the check is not consulted for a withdrawal section, nor for an NLRI the
// recognizer discarded.
// PREVENTS: a withdrawal of a route being dropped because the UPDATE carrying it has no
// Prefix-SID, and an unimplemented type being withdrawn instead of discarded.
func TestRetainUpdateCheckOnlyAfterRecognizerKeeps(t *testing.T) {
	data := mupISD([]byte{10, 0, 0})
	attrs := NewUpdateAttrs(nil)

	r, err := RetainUpdate(keepAll, withdrawWithoutPrefixSID, &attrs, mupFamily, data, false, false, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Removed() {
		t.Fatalf("a withdrawal section must be returned unchanged, got %+v", r)
	}

	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	discardAll := func([]byte, bool) Verdict { return Discard }
	r, err = RetainUpdate(discardAll, withdrawWithoutPrefixSID, &attrs, mupFamily, data, false, true, scratch[:])
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if r.Discarded != 1 || r.WithdrawnCount != 0 {
		t.Fatalf("the recognizer's discard must stand, got %+v", r)
	}
}

// VALIDATES: RegisterUpdateCheck refuses a family with no splitter and a second check,
// treats nil as no ruling, and Rulings answers both registries.
// PREVENTS: a check that can never run being accepted in silence.
func TestRegisterUpdateCheck(t *testing.T) {
	t.Cleanup(SnapshotForTest())

	noSplitter := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFI(250)}
	if err := RegisterUpdateCheck(noSplitter, withdrawWithoutPrefixSID); !errors.Is(err, ErrNoSplitter) {
		t.Fatalf("want ErrNoSplitter, got %v", err)
	}
	if err := RegisterUpdateCheck(mupFamily, nil); err != nil {
		t.Fatalf("nil is no ruling, got %v", err)
	}
	if _, check := Rulings(mupFamily); check != nil {
		t.Fatal("a nil registration must leave no check")
	}
	if err := RegisterUpdateCheck(mupFamily, withdrawWithoutPrefixSID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if err := RegisterUpdateCheck(mupFamily, withdrawWithoutPrefixSID); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("want ErrDuplicate, got %v", err)
	}
	if err := Register(mupFamily, keepAll); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	recognize, check := Rulings(mupFamily)
	if recognize == nil || check == nil {
		t.Fatal("Rulings must answer both the recognizer and the check")
	}
}

// VALIDATES: the per-NLRI verdict with a check consulted allocates nothing.
// PREVENTS: the attribute lookup costing an allocation per NLRI on the receive path.
func TestRetainUpdateVerdictAllocatesNothing(t *testing.T) {
	one := mupISD([]byte{10, 0, 0})
	var scratch [nlrisplit.PrefixKeyScratchSize]byte
	key := nlrisplit.GetPrefixKey(mupFamily)
	attrs := NewUpdateAttrs(prefixSIDAttrs[:7])
	r := rulings{recognize: keepAll, check: withdrawWithoutPrefixSID, attrs: &attrs}
	if n := testing.AllocsPerRun(100, func() {
		if judge(r, key, one, false, scratch[:]) != Withdraw {
			t.Fatal("a route with no Prefix-SID must be withdrawn")
		}
	}); n != 0 {
		t.Fatalf("judge allocated %v times", n)
	}
}

// VALIDATES: RetainUpdate invokes recognize and check strictly once per NLRI
// even when NLRIs are withdrawn or discarded, for both <=32 (stack) and >32
// (heap) NLRI sections.
// PREVENTS: pass-2 re-evaluation causing TOCTOU divergence against concurrent
// index updates or duplicate warning/report side effects.
func TestRetainUpdateInvokesJudgeOncePerNLRI(t *testing.T) {
	cases := []struct {
		name  string
		count int
	}{
		{name: "stack verdict buffer (<=32)", count: 8},
		{name: "heap verdict buffer (>32)", count: 40},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var section []byte
			var wantKept []byte
			var wantWithdrawn []byte
			var wantUnkeyable []byte
			wantDiscarded := 0
			wantWithdrawnCount := 0
			wantUnkeyableCount := 0
			wantCheckCalls := 0

			for i := range tc.count {
				var nlri []byte
				switch i % 4 {
				case 0:
					nlri = mupISD([]byte{10, 0, byte(i)})
					wantKept = append(wantKept, nlri...)
					wantCheckCalls++
				case 1:
					nlri = mupISD([]byte{10, 1, byte(i)})
					wantWithdrawn = append(wantWithdrawn, nlri...)
					wantWithdrawnCount++
					wantCheckCalls++
				case 2:
					nlri = mupISD([]byte{10, 2, byte(i)})
					wantDiscarded++
				case 3:
					nlri = mupISD([]byte{10, 3, byte(i)}, 0xff)
					wantUnkeyable = append(wantUnkeyable, nlri...)
					wantDiscarded++
					wantUnkeyableCount++
					wantCheckCalls++
				}
				section = append(section, nlri...)
			}

			recognizeCalls := 0
			checkCalls := 0
			recognize := func(nlriBytes []byte, _ bool) Verdict {
				recognizeCalls++
				if recognizeCalls > tc.count {
					t.Errorf("recognizer called more than once per NLRI (%d > %d)", recognizeCalls, tc.count)
				}
				// Prefix byte 1 is at offset 4 (header) + 8 (RD) + 1 (prefixLen) + 1 = 14.
				if nlriBytes[14] == 2 {
					return Discard
				}
				return Keep
			}
			check := func(nlriBytes []byte, _ bool, _ *UpdateAttrs) Verdict {
				checkCalls++
				if checkCalls > wantCheckCalls {
					t.Errorf("check called more than once per kept NLRI (%d > %d)", checkCalls, wantCheckCalls)
					return Keep
				}
				if nlriBytes[14] == 0 {
					return Keep
				}
				return Withdraw
			}

			attrs := NewUpdateAttrs(prefixSIDAttrs)
			var scratch [nlrisplit.PrefixKeyScratchSize]byte
			r, err := RetainUpdate(recognize, check, &attrs, mupFamily, section, false, true, scratch[:])
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if recognizeCalls != tc.count {
				t.Fatalf("recognize called %d times, want %d", recognizeCalls, tc.count)
			}
			if checkCalls != wantCheckCalls {
				t.Fatalf("check called %d times, want %d", checkCalls, wantCheckCalls)
			}
			if !bytes.Equal(r.Kept, wantKept) {
				t.Fatalf("Kept mismatch: got %x, want %x", r.Kept, wantKept)
			}
			if !bytes.Equal(r.Withdrawn, wantWithdrawn) {
				t.Fatalf("Withdrawn mismatch: got %x, want %x", r.Withdrawn, wantWithdrawn)
			}
			if !bytes.Equal(r.Unkeyable, wantUnkeyable) {
				t.Fatalf("Unkeyable mismatch: got %x, want %x", r.Unkeyable, wantUnkeyable)
			}
			if r.WithdrawnCount != wantWithdrawnCount {
				t.Fatalf("WithdrawnCount = %d, want %d", r.WithdrawnCount, wantWithdrawnCount)
			}
			if r.Discarded != wantDiscarded {
				t.Fatalf("Discarded = %d, want %d", r.Discarded, wantDiscarded)
			}
			if r.UnkeyableCount != wantUnkeyableCount {
				t.Fatalf("UnkeyableCount = %d, want %d", r.UnkeyableCount, wantUnkeyableCount)
			}
		})
	}
}
