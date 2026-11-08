// Design: docs/architecture/policyroute/policy-routing.md -- netlink ip rule and route management

package iprule

import (
	"math"
	"net/netip"
	"testing"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"
)

// VALIDATES: a source rule is encoded in the prefix's family, from the masked
// prefix, into the requested table, at the requested priority, and starts from
// netlink.NewRule (the -1 "unset" fields stay unset, so no FRA_GOTO or FRA_FLOW
// reaches the kernel). An invalid prefix and a 4-in-6 prefix are refused.
// Method: encode an IPv4 and an IPv6 source rule and compare each field with a
// fresh netlink.NewRule's.
// PREVENTS: an IPv6 guard rule installed as IPv4 (matching nothing), and the EINVAL
// that a zero-valued netlink.Rule literal draws from the kernel.
func TestSourceRuleEncodesBothFamilies(t *testing.T) {
	seed := netlink.NewRule()
	cases := []struct {
		from   string
		family int
		src    string
	}{
		{"192.0.2.7/24", unix.AF_INET, "192.0.2.0/24"},
		{"2001:db8::1/128", unix.AF_INET6, "2001:db8::1/128"},
	}
	for _, tc := range cases {
		rule, err := Source{Priority: 100, From: netip.MustParsePrefix(tc.from), Table: IPsecSourceGuardTables.First}.Rule()
		if err != nil {
			t.Fatalf("%s: %v", tc.from, err)
		}
		if rule.Family != tc.family {
			t.Errorf("%s: family %d, want %d", tc.from, rule.Family, tc.family)
		}
		if rule.Src == nil || rule.Src.String() != tc.src {
			t.Errorf("%s: source %v, want %s", tc.from, rule.Src, tc.src)
		}
		if rule.Table != int(IPsecSourceGuardTables.First) || rule.Priority != 100 {
			t.Errorf("%s: table %d priority %d", tc.from, rule.Table, rule.Priority)
		}
		if rule.Goto != seed.Goto || rule.Flow != seed.Flow || rule.SuppressIfgroup != seed.SuppressIfgroup || rule.SuppressPrefixlen != seed.SuppressPrefixlen {
			t.Errorf("%s: the rule did not start from netlink.NewRule: %+v", tc.from, rule)
		}
	}
	if _, err := (Source{From: netip.Prefix{}, Table: 3000}).Rule(); err == nil {
		t.Error("an invalid prefix was encoded")
	}
	if _, err := (Source{From: netip.MustParsePrefix("::ffff:192.0.2.1/128"), Table: 3000}).Rule(); err == nil {
		t.Error("a 4-in-6 prefix was encoded")
	}
}

// VALIDATES: the fwmark rule keeps the encoding policyroute shipped: IPv4, mark and
// mask, table, priority, seeded by netlink.NewRule (AC-11 at the encoder).
// PREVENTS: the move changing the rules policyroute installs.
func TestMarkRuleEncoding(t *testing.T) {
	rule, err := Mark{Priority: 200, Mark: 0x50001, Mask: math.MaxUint32, Table: 2000}.Rule()
	if err != nil {
		t.Fatal(err)
	}
	seed := netlink.NewRule()
	if rule.Family != unix.AF_INET || rule.Mark != 0x50001 || rule.Mask == nil || *rule.Mask != math.MaxUint32 {
		t.Errorf("mark rule %+v", rule)
	}
	if rule.Table != 2000 || rule.Priority != 200 || rule.Goto != seed.Goto || rule.Flow != seed.Flow {
		t.Errorf("mark rule %+v", rule)
	}
}
