package nlrisyntax

import (
	"errors"
	"testing"

	"github.com/ze-software/ze/internal/core/family"
)

// VALIDATES: a rule registered for a family is found by that family and no other;
// a second rule for the same family, a rule with no answer, and a rule with no
// citation are refused; an unruled family answers "no rule".
// PREVENTS: two rulings for one family at one gate (ai/rules/no-layering.md), and a
// lookup that invents a rule for a family nobody ruled on.
func TestNLRISyntaxRegistry(t *testing.T) {
	t.Cleanup(SnapshotForTest())

	ruled := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFISRPolicy}
	other := family.Family{AFI: family.AFIIPv6, SAFI: family.SAFISRPolicy}
	unicast := family.Family{AFI: family.AFIIPv4, SAFI: family.SAFIUnicast}

	judged := 0
	rule := Rule{
		Citation: "RFC 9830 Section 5",
		Judge: func([]byte, bool) Judgement {
			judged++
			return WellFormed
		},
	}
	if err := Register(ruled, rule); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := Register(ruled, rule); !errors.Is(err, ErrDuplicate) {
		t.Fatalf("second Register: got %v, want ErrDuplicate", err)
	}
	if err := Register(other, Rule{Citation: "x"}); !errors.Is(err, ErrEmpty) {
		t.Fatalf("empty rule: got %v, want ErrEmpty", err)
	}
	if err := Register(other, Rule{Judge: rule.Judge}); !errors.Is(err, ErrNoCitation) {
		t.Fatalf("uncited rule: got %v, want ErrNoCitation", err)
	}

	got, ok := Get(ruled)
	if !ok {
		t.Fatal("Get(ruled): no rule")
	}
	if got.Judge(nil, false) != WellFormed || judged != 1 {
		t.Fatalf("Get(ruled) returned a different rule (judged %d)", judged)
	}
	if err := Register(other, Rule{Citation: "x", Retain: func(s []byte, _ bool) ([]byte, int, bool) { return s, 0, true }}); !errors.Is(err, ErrNoRemovedMessage) {
		t.Fatalf("Retain without message: got %v, want ErrNoRemovedMessage", err)
	}
	if _, ok := Get(other); ok {
		t.Fatal("Get(other): a refused registration left a rule behind")
	}
	// Moved from message.TestRetainWellFormedNLRILeavesOtherFamiliesAlone: an IPv4
	// unicast section finds no rule, so no family walk is ever applied to bytes that
	// are not that family's layout.
	if _, ok := Get(unicast); ok {
		t.Fatal("Get(unicast): an unruled family must answer no rule")
	}
}
