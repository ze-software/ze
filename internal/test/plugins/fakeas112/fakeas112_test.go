// Detail: fakeas112.go / store.go -- family selector (#7) and per-family
// del fidelity (#12) for the AS112 redistribute test producer.

package fakeas112

import (
	"testing"

	"github.com/ze-software/ze/internal/core/family"
)

func TestParseFamily(t *testing.T) {
	cases := []struct {
		tok     string
		wantLen int
		err     bool
	}{
		{"", 2, false},     // omitted -> both
		{"both", 2, false}, // both
		{"ipv4", 1, false},
		{"ipv6", 1, false},
		{"bogus", 0, true},
	}
	for _, c := range cases {
		got, err := parseFamily(c.tok)
		if c.err {
			if err == nil {
				t.Errorf("parseFamily(%q): want error", c.tok)
			}
			continue
		}
		if err != nil {
			t.Errorf("parseFamily(%q): %v", c.tok, err)
			continue
		}
		if len(got) != c.wantLen {
			t.Errorf("parseFamily(%q) = %v, want %d families", c.tok, got, c.wantLen)
		}
	}
}

// TestParseEmitArgs_FamilySelector validates the #7 fix: `family <af>` selects a
// single family, and it defaults to both when omitted (preserving the existing
// .ci behavior).
func TestParseEmitArgs_FamilySelector(t *testing.T) {
	ea, err := parseEmitArgs([]string{"add", "family", "ipv4", "asn", "65001"})
	if err != nil {
		t.Fatalf("parseEmitArgs: %v", err)
	}
	if len(ea.families) != 1 || ea.families[0] != family.IPv4Unicast {
		t.Fatalf("families = %v, want [ipv4]", ea.families)
	}
	if ea.asn != 65001 {
		t.Fatalf("asn = %d, want 65001", ea.asn)
	}

	ea2, err := parseEmitArgs([]string{"add"})
	if err != nil {
		t.Fatalf("parseEmitArgs(bare): %v", err)
	}
	if len(ea2.families) != 2 {
		t.Fatalf("default families = %v, want both", ea2.families)
	}
}

// TestStore_DelWithdrawsOnlyAnnounced is the #12 fidelity guard: a del after a
// single-family add withdraws ONLY the family that was announced, never the
// family that was never added.
func TestStore_DelWithdrawsOnlyAnnounced(t *testing.T) {
	resetStore()
	t.Cleanup(resetStore)

	store.applyAdd([]family.Family{family.IPv4Unicast}, 112, nil)

	removed, _, _ := store.applyDel(nil) // no request -> all announced (only v4)
	if len(removed) != 1 || removed[0] != family.IPv4Unicast {
		t.Fatalf("applyDel removed %v, want [ipv4] only (v6 was never added)", removed)
	}
	if fams, _, _ := store.snapshot(); len(fams) != 0 {
		t.Fatalf("after del, snapshot = %v, want empty", fams)
	}
}

// TestStore_DelSpecificFamilyKeepsOther validates a per-family del withdraws only
// the requested family and leaves the other announced with its attributes.
func TestStore_DelSpecificFamilyKeepsOther(t *testing.T) {
	resetStore()
	t.Cleanup(resetStore)

	store.applyAdd([]family.Family{family.IPv4Unicast, family.IPv6Unicast}, 112, []uint32{0xFFFFFF01})

	removed, _, _ := store.applyDel([]family.Family{family.IPv6Unicast})
	if len(removed) != 1 || removed[0] != family.IPv6Unicast {
		t.Fatalf("applyDel(v6) removed %v, want [ipv6]", removed)
	}
	fams, asn, comm := store.snapshot()
	if len(fams) != 1 || fams[0] != family.IPv4Unicast {
		t.Fatalf("snapshot fams = %v, want [ipv4] still announced", fams)
	}
	if asn != 112 || len(comm) != 1 || comm[0] != 0xFFFFFF01 {
		t.Fatalf("v4 attrs lost after v6 del: asn=%d comm=%v", asn, comm)
	}
}

// TestStore_AddIsUnion validates adds accumulate families rather than replacing.
func TestStore_AddIsUnion(t *testing.T) {
	resetStore()
	t.Cleanup(resetStore)

	store.applyAdd([]family.Family{family.IPv4Unicast}, 112, nil)
	store.applyAdd([]family.Family{family.IPv6Unicast}, 112, nil)
	if fams, _, _ := store.snapshot(); len(fams) != 2 {
		t.Fatalf("after two single-family adds, snapshot = %v, want both", fams)
	}
}
