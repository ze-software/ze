// Design: docs/architecture/firewall/fw-6-firewall-vpp.md -- exact-or-reject NAT44 source-address match refusal
// Related: verify.go -- verifyNATSourceMatches

package firewallvpp

import (
	"errors"
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/core/opererr"
)

// VALIDATES: AC-6 -- under firewall backend vpp, snat and masquerade terms
// carrying a source-address match are refused naming table, chain and term,
// stating that VPP NAT44 applies to the whole inside interface, and offering
// the fix to delete the match or switch to firewall backend kernel; the same
// snat/masquerade terms without a source match, and dnat with a source match,
// stay accepted.
// PREVENTS: silent widening of a source-scoped NAT rule to every inside source
// on VPP NAT44.
func TestVerifyNATSourceMatchRefused(t *testing.T) {
	srcMatch := firewall.MatchSourceAddress{Prefix: netip.MustParsePrefix("10.0.0.0/24")}

	refusedCases := []struct {
		name   string
		term   string
		action firewall.Action
	}{
		{
			name:   "snat with source address match",
			term:   "snat-lan",
			action: firewall.SNAT{Address: netip.MustParseAddr("198.51.100.1")},
		},
		{
			name:   "masquerade with source address match",
			term:   "masq-lan",
			action: firewall.Masquerade{},
		},
	}

	for _, tc := range refusedCases {
		t.Run(tc.name, func(t *testing.T) {
			tables := natTable(natChain(firewall.HookPostrouting, firewall.Term{
				Name:    tc.term,
				Matches: []firewall.Match{srcMatch},
				Actions: []firewall.Action{tc.action},
			}))

			err := Verify(tables)
			if err == nil {
				t.Fatalf("%s must be refused by backend vpp", tc.name)
			}
			oe, ok := errors.AsType[*opererr.Error](err)
			if !ok {
				t.Fatalf("expected *opererr.Error in chain, got %T: %v", err, err)
			}
			msg := err.Error()
			for _, want := range []string{
				`table "nat"`,
				`chain "natchain"`,
				`term "` + tc.term + `"`,
				"whole inside interface",
				"firewall backend kernel",
			} {
				if !strings.Contains(msg, want) {
					t.Errorf("refusal missing %q:\n%s", want, msg)
				}
			}
			if !strings.Contains(oe.Fix(), "firewall backend kernel") {
				t.Errorf("fix = %q, want mention of firewall backend kernel", oe.Fix())
			}
		})
	}

	t.Run("snat without source match is accepted", func(t *testing.T) {
		tables := natTable(natChain(firewall.HookPostrouting, firewall.Term{
			Name:    "snat-all",
			Actions: []firewall.Action{firewall.SNAT{Address: netip.MustParseAddr("198.51.100.1")}},
		}))
		if err := Verify(tables); err != nil {
			t.Fatalf("snat without source match should be accepted, got %v", err)
		}
	})

	t.Run("masquerade without source match is accepted", func(t *testing.T) {
		tables := natTable(natChain(firewall.HookPostrouting, firewall.Term{
			Name:    "masq-all",
			Actions: []firewall.Action{firewall.Masquerade{}},
		}))
		if err := Verify(tables); err != nil {
			t.Fatalf("masquerade without source match should be accepted, got %v", err)
		}
	})

	t.Run("dnat with source match stays accepted", func(t *testing.T) {
		tables := natTable(natChain(firewall.HookPrerouting, firewall.Term{
			Name: "dnat-scoped",
			Matches: []firewall.Match{
				srcMatch,
				firewall.MatchProtocol{Protocol: "tcp"},
				firewall.MatchDestinationPort{Ranges: []firewall.PortRange{{Lo: 443, Hi: 443}}},
			},
			Actions: []firewall.Action{firewall.DNAT{Address: netip.MustParseAddr("10.0.0.10"), Port: 8443}},
		}))
		if err := Verify(tables); err != nil {
			t.Fatalf("dnat with source match should stay accepted, got %v", err)
		}
	})
}
