package yang_test

import (
	"slices"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/firewall"
	fwyang "github.com/ze-software/ze/internal/component/firewall/yang"
)

// TestRateSpecPatternNamesTheRateUnits ties the time units an operator can
// write to the one table that says what each unit means.
//
// The Go side is the declaration here: firewall.RateUnitSeconds pairs each unit
// with the seconds it stands for, which is what the nft backend hands the
// kernel as an nftables LimitTime and what the VPP backend divides by. The
// schema cannot hold that pairing, so the pattern in the rate-spec typedef is
// the copy, and this test is what keeps it honest.
//
// The schema side is read out of the module the loader reads, never out of a
// list written here.
//
// VALIDATES: the alternation in the rate-spec pattern names exactly the units
// firewall.RateUnitNames answers.
// PREVENTS: a unit an operator commits that ParseRateSpec then refuses, and a
// unit Ze accepts that the schema rejects before it is ever parsed.
func TestRateSpecPatternNamesTheRateUnits(t *testing.T) {
	body, ok := typedefBody(fwyang.ZeFirewallConfYANG, "rate-spec")
	if !ok {
		t.Fatal("ze-firewall-conf.yang no longer declares typedef rate-spec")
	}

	inPattern, ok := unitAlternation(body)
	if !ok {
		t.Fatalf("typedef rate-spec no longer carries a /(unit|unit) alternation: %s", body)
	}

	declared := firewall.RateUnitNames()
	if !slices.Equal(inPattern, declared) {
		t.Errorf("the rate-spec pattern accepts %v, and firewall accepts %v", inPattern, declared)
	}
}

// unitAlternation answers the sorted alternatives of the LAST parenthesized
// group of the typedef's pattern, which is the time unit after the slash.
func unitAlternation(body string) ([]string, bool) {
	open := strings.LastIndex(body, "/(")
	if open < 0 {
		return nil, false
	}
	group, _, closed := strings.Cut(body[open+len("/("):], ")")
	if !closed {
		return nil, false
	}
	units := strings.Split(group, "|")
	slices.Sort(units)
	return units, true
}
