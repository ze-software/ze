package policychain

import (
	"errors"
	"slices"
	"testing"

	"github.com/ze-software/ze/internal/component/policy"
	"github.com/ze-software/ze/internal/core/family"
)

// TestIntentsFold proves how the chain folds the named intents of two
// members: prepends add up, the later remove-private mode wins, NLRI drops
// accumulate and come back ascending and each once, and a family stripped
// twice is stripped once.
func TestIntentsFold(t *testing.T) {
	var folded, first, second Sets
	mustOK := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	mustOK(first.PrependASPath(2))
	mustOK(first.RemovePrivateAS(RemovePrivateStrip))
	mustOK(first.DropNLRI(NLRISectionMPReach, 7))
	mustOK(first.StripFamily(family.IPv6Unicast))
	mustOK(second.PrependASPath(3))
	mustOK(second.RemovePrivateAS(RemovePrivatePeerAS))
	mustOK(second.DropNLRI(NLRISectionMPReach, 0))
	mustOK(second.DropNLRI(NLRISectionMPReach, 7))
	mustOK(second.StripFamily(family.IPv6Unicast))
	mustOK(folded.merge(&first))
	mustOK(folded.merge(&second))

	if got := folded.ASPathPrepend(); got != 5 {
		t.Errorf("prepend = %d, want 5", got)
	}
	if got := folded.RemovePrivate(); got != RemovePrivatePeerAS {
		t.Errorf("remove-private = %d, want peer-as", got)
	}
	if got := folded.NLRIDrops(NLRISectionMPReach); !slices.Equal(got, []uint16{0, 7}) {
		t.Errorf("drops = %v, want [0 7]", got)
	}
	if got := folded.NLRIDrops(NLRISectionAnnounce); len(got) != 0 {
		t.Errorf("announce drops = %v, want none", got)
	}
	if !folded.StripsFamily(family.IPv6Unicast) || folded.StripsFamily(family.IPv4Unicast) {
		t.Errorf("strips ipv6 %v ipv4 %v, want true false", folded.StripsFamily(family.IPv6Unicast), folded.StripsFamily(family.IPv4Unicast))
	}
	if folded.intents.familiesN != 1 {
		t.Errorf("families stored = %d, want 1", folded.intents.familiesN)
	}
}

// TestIntentsBounds proves each intent's bound and invalid input: a prepend
// count of 0, a total past ASPathPrependMax within one member and across the
// fold, an unspecified remove-private mode, an unspecified NLRI section, and
// a fourth stripped family are refused, so the route fails under on-error.
func TestIntentsBounds(t *testing.T) {
	var s Sets
	if err := s.PrependASPath(0); err == nil {
		t.Error("a prepend of 0 is accepted")
	}
	if err := s.PrependASPath(ASPathPrependMax); err != nil {
		t.Fatalf("a prepend of %d: %v", ASPathPrependMax, err)
	}
	if err := s.PrependASPath(1); err == nil {
		t.Error("a prepend past the maximum is accepted")
	}
	var folded, later Sets
	if err := folded.PrependASPath(30); err != nil {
		t.Fatal(err)
	}
	if err := later.PrependASPath(3); err != nil {
		t.Fatal(err)
	}
	if err := folded.merge(&later); err == nil {
		t.Error("a folded prepend past the maximum is accepted")
	}
	if err := s.RemovePrivateAS(RemovePrivateUnspecified); err == nil {
		t.Error("an unspecified remove-private mode is accepted")
	}
	if err := s.DropNLRI(NLRISectionUnspecified, 0); err == nil {
		t.Error("an unspecified NLRI section is accepted")
	}
	fams := []family.Family{family.IPv4Unicast, family.IPv6Unicast, family.IPv4Multicast}
	for _, fam := range fams {
		if err := s.StripFamily(fam); err != nil {
			t.Fatalf("strip %s: %v", fam, err)
		}
	}
	if err := s.StripFamily(family.IPv6Multicast); !errors.Is(err, ErrTooManyFamilies) {
		t.Errorf("a fourth family = %v, want ErrTooManyFamilies", err)
	}
}

// TestIntentsResetKeepsStorage proves Reset forgets every intent and keeps the
// drop lists' storage, so a pooled Sets reused for the next route does not
// allocate again for the same number of drops.
func TestIntentsResetKeepsStorage(t *testing.T) {
	var s Sets
	for off := range uint16(8) {
		if err := s.DropNLRI(NLRISectionAnnounce, off); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.PrependASPath(1); err != nil {
		t.Fatal(err)
	}
	allocs := testing.AllocsPerRun(100, func() {
		s.Reset()
		for off := range uint16(8) {
			_ = s.DropNLRI(NLRISectionAnnounce, off) //nolint:errcheck // the section is valid
		}
	})
	if allocs != 0 {
		t.Errorf("reused drops allocate %v times, want 0", allocs)
	}
	s.Reset()
	if !s.Empty() || s.ASPathPrepend() != 0 || len(s.NLRIDrops(NLRISectionAnnounce)) != 0 {
		t.Errorf("Reset left intents behind")
	}
}

// TestIntentsMarkTheirFields proves each intent writes the vocabulary field
// D-9 checks it against, so a member that did not declare as-path, prefix or
// family is refused for the intent.
func TestIntentsMarkTheirFields(t *testing.T) {
	tests := []struct {
		name  string
		ask   func(*Sets) error
		field policy.Field
	}{
		{"prepend", func(s *Sets) error { return s.PrependASPath(1) }, policy.FieldASPath},
		{"remove-private", func(s *Sets) error { return s.RemovePrivateAS(RemovePrivateStrip) }, policy.FieldASPath},
		{"drop", func(s *Sets) error { return s.DropNLRI(NLRISectionAnnounce, 0) }, policy.FieldPrefix},
		{"strip", func(s *Sets) error { return s.StripFamily(family.IPv6Unicast) }, policy.FieldFamily},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var s Sets
			if err := tt.ask(&s); err != nil {
				t.Fatal(err)
			}
			if s.written != maskOf([]policy.Field{tt.field}) {
				t.Errorf("written = %b, want the bit of %s", s.written, tt.field)
			}
			undeclared := member{settable: maskOf([]policy.Field{policy.FieldLocalPreference})}
			if err := undeclared.refuseUndeclared(&s); !errors.Is(err, errUndeclaredSet) {
				t.Errorf("an undeclared %s = %v, want errUndeclaredSet", tt.field, err)
			}
		})
	}
}
