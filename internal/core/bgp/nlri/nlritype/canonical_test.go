package nlritype

import (
	"bytes"
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
)

// canonicalTestFam is a family nlrisplit frames by type and length (MUP: a 4-octet
// header whose last octet is the Length), so Retain can carve it.
var canonicalTestFam = family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIMUP}

// growType3 is a stand-in canonical form: an NLRI of route type 3 grows by one
// trailing 0xEE octet, its Length one larger.
var growType3 = Canonical{
	Grow: func(one []byte, _ bool) int {
		if one[2] == 3 {
			return 1
		}
		return 0
	},
	WriteTo: func(buf []byte, off int, one []byte, _ bool) int {
		n := copy(buf[off:], one)
		buf[off+3]++
		buf[off+n] = 0xee
		return n + 1
	},
}

// TestRetainCanonicalForm covers RegisterCanonical and its application by Retain.
//
// VALIDATES: a canonical form needs a ruling first; Retain writes each kept route the
// form grows in that form, counts it, and leaves its siblings as received, in an
// announcement and a withdrawal; a section none of whose routes grows comes back as
// the input itself; a removed route is not canonicalized.
// PREVENTS: a registered form never applied, a current-form section copied, and a
// discarded route counted as rewritten.
func TestRetainCanonicalForm(t *testing.T) {
	t.Cleanup(SnapshotForTest())

	if err := RegisterCanonical(canonicalTestFam, growType3); !errors.Is(err, ErrNoRuling) {
		t.Fatalf("RegisterCanonical with no ruling: %v, want ErrNoRuling", err)
	}
	keepBelow9 := func(one []byte, _ bool) Verdict {
		if one[2] < 9 {
			return Keep
		}
		return Discard
	}
	if err := Register(canonicalTestFam, keepBelow9); err != nil {
		t.Fatal(err)
	}
	if err := RegisterCanonical(canonicalTestFam, growType3); err != nil {
		t.Fatal(err)
	}
	if err := RegisterCanonical(canonicalTestFam, growType3); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("second RegisterCanonical: %v, want ErrDuplicate", err)
	}

	grows := []byte{1, 0, 3, 1, 0xaa}
	stays := []byte{1, 0, 1, 1, 0xbb}
	dropped := []byte{1, 0, 9, 1, 0xcc}
	scratch := make([]byte, 64)

	for _, announce := range []bool{true, false} {
		section := append(append(append([]byte{}, grows...), stays...), dropped...)
		r, err := Retain(keepBelow9, canonicalTestFam, section, false, announce, scratch)
		if err != nil {
			t.Fatal(err)
		}
		want := []byte{1, 0, 3, 2, 0xaa, 0xee, 1, 0, 1, 1, 0xbb}
		if !bytes.Equal(r.Kept, want) {
			t.Fatalf("announce %v: kept %x, want %x", announce, r.Kept, want)
		}
		if r.Canonicalized != 1 || r.Discarded != 1 {
			t.Fatalf("announce %v: canonicalized %d discarded %d, want 1 and 1", announce, r.Canonicalized, r.Discarded)
		}
	}

	current := append([]byte{}, stays...)
	r, err := Retain(keepBelow9, canonicalTestFam, current, false, true, scratch)
	if err != nil {
		t.Fatal(err)
	}
	if &r.Kept[0] != &current[0] || r.Canonicalized != 0 {
		t.Fatalf("a section with nothing to rewrite was copied or counted: %+v", r)
	}
}
