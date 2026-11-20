// Design: docs/architecture/ike/ipsec-8-ikev2-child-xfrm.md -- SPD dispositions and their order
// Related: spd_order.go -- verifySPDOrder, the refusal under test
// RFC: rfc/short/rfc4301.md -- SPD ordering (Section 4.4.1)

package engine

import (
	"errors"
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/ike/dataplane"
	"github.com/ze-software/ze/internal/component/ike/ipsec"
	"github.com/ze-software/ze/internal/core/opererr"
)

// orderTotalEntry is a bypass entry, both directions, UDP, any ports, between local
// and remote at order.
func orderTotalEntry(t *testing.T, name, local, remote string, order uint32) ipsec.SPDPolicy {
	t.Helper()
	return ipsec.SPDPolicy{
		Name:         name,
		Action:       dataplane.SPActionBypass,
		Order:        order,
		Direction:    ipsec.SPDDirBoth,
		Protocol:     protoUDP,
		LocalPrefix:  mustCIDR(t, local),
		LocalPort:    ipsec.AnyPort(),
		RemotePrefix: mustCIDR(t, remote),
		RemotePort:   ipsec.AnyPort(),
	}
}

// orderTotalPeer is a policy-based peer whose one traffic selector is local to remote,
// at policy-priority 2000, bound to the XFRM interface vti when vti is not empty.
func orderTotalPeer(t *testing.T, local, remote, vti string) ipsec.SiteToSitePeer {
	t.Helper()
	return ipsec.SiteToSitePeer{
		PolicyPriority: 2000,
		VTIBind:        vti,
		TrafficSelectors: []ipsec.TrafficSelectorPolicy{{
			Number:      "1",
			LocalPrefix: mustCIDR(t, local), LocalPort: ipsec.AnyPort(),
			RemotePrefix: mustCIDR(t, remote), RemotePort: ipsec.AnyPort(),
		}},
	}
}

// VALIDATES: AC-15. Two operator entries that overlap in selector and direction and
// share an order, two left at the default 1000 included, are refused at commit, and so
// is an operator entry that shares an overlapping peer's policy-priority; the refusal
// names both, the shared order, and the `set vpn ipsec policy <name> order <n>` that
// separates them. Entries that do not overlap, or overlap at distinct orders, and a
// peer bound to an XFRM interface (an SPD of its own) are accepted.
// METHOD: run verifySPDOrder, the step validateIPsecSections runs at every commit door,
// over each configuration and read the operator error.
// PREVENTS: the D-2 defect, where ValidateSPDPolicies judged each entry alone, so two
// overlapping entries at the default order were installed and the kernel decided
// between them by the sequence it received them in.
//
// RFC requirement: RFC4301-4.4.1-4 negative -- two operator entries that overlap in selector and direction at one order (both at the default 1000 included), or an operator entry at an overlapping peer's policy-priority, are refused with both names, the shared order and the `set vpn ipsec policy <name> order <n>` fix; entries that do not overlap, or overlap at distinct orders, are accepted.
func TestSPDPolicyEqualOrderOnOverlapRefused(t *testing.T) {
	for _, tc := range []struct {
		name     string
		policies []ipsec.SPDPolicy
		peers    map[string]ipsec.SiteToSitePeer
		// names are the parts the refusal must carry; nil means accepted.
		names []string
		fix   string
	}{
		{
			name: "two overlapping entries at the default order",
			policies: []ipsec.SPDPolicy{
				orderTotalEntry(t, "wide", "10.0.0.0/8", "192.0.2.0/24", 1000),
				orderTotalEntry(t, "narrow", "10.1.0.0/16", "192.0.2.0/25", 1000),
			},
			names: []string{"vpn ipsec policy narrow", "vpn ipsec policy wide", "share order 1000"},
			fix:   "set vpn ipsec policy wide order 1001",
		},
		{
			name: "two overlapping entries at distinct orders",
			policies: []ipsec.SPDPolicy{
				orderTotalEntry(t, "wide", "10.0.0.0/8", "192.0.2.0/24", 1000),
				orderTotalEntry(t, "narrow", "10.1.0.0/16", "192.0.2.0/25", 900),
			},
		},
		{
			name: "two disjoint entries at the default order",
			policies: []ipsec.SPDPolicy{
				orderTotalEntry(t, "first", "10.1.0.0/16", "192.0.2.0/24", 1000),
				orderTotalEntry(t, "second", "10.2.0.0/16", "192.0.2.0/24", 1000),
			},
		},
		{
			name: "an entry at an overlapping peer's policy-priority",
			policies: []ipsec.SPDPolicy{
				orderTotalEntry(t, "hole", "10.1.0.0/16", "192.0.2.0/24", 2000),
			},
			peers: map[string]ipsec.SiteToSitePeer{
				"branch": orderTotalPeer(t, "10.0.0.0/8", "192.0.2.0/24", ""),
			},
			names: []string{"vpn ipsec policy hole", "site-to-site peer branch", "share order 2000"},
			fix:   "set vpn ipsec policy hole order 2001",
		},
		{
			name: "an entry ranked apart from an overlapping peer",
			policies: []ipsec.SPDPolicy{
				orderTotalEntry(t, "hole", "10.1.0.0/16", "192.0.2.0/24", 1000),
			},
			peers: map[string]ipsec.SiteToSitePeer{
				"branch": orderTotalPeer(t, "10.0.0.0/8", "192.0.2.0/24", ""),
			},
		},
		{
			name: "an entry at the priority of a peer bound to an XFRM interface",
			policies: []ipsec.SPDPolicy{
				orderTotalEntry(t, "hole", "10.1.0.0/16", "192.0.2.0/24", 2000),
			},
			peers: map[string]ipsec.SiteToSitePeer{
				"routed": orderTotalPeer(t, "10.0.0.0/8", "192.0.2.0/24", "vti0"),
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := &ipsec.IPsecConfig{Policies: map[string]ipsec.SPDPolicy{}, Peers: tc.peers}
			for _, p := range tc.policies {
				cfg.Policies[p.Name] = p
			}
			err := verifySPDOrder(cfg)
			if tc.names == nil {
				if err != nil {
					t.Fatalf("an order the configuration decides was refused: %v", err)
				}
				return
			}
			refusal, ok := errors.AsType[*opererr.Error](err)
			if !ok {
				t.Fatalf("a tie the configuration leaves undecided answered %v, want an operator error", err)
			}
			for _, want := range tc.names {
				if !strings.Contains(refusal.Problem(), want) {
					t.Fatalf("the refusal %q does not carry %q", refusal.Problem(), want)
				}
			}
			if refusal.Fix() != tc.fix {
				t.Fatalf("the refusal's fix is %q, want %q", refusal.Fix(), tc.fix)
			}
		})
	}
}

// VALIDATES: two operator entries that install one selector in one direction are
// refused whatever their orders, and two that differ in direction are not.
// METHOD: verifySPDOrder over an out entry and a both entry with the same prefixes at
// distinct orders, then over an out entry and an in entry.
// PREVENTS: the second install replacing the first in the dataplane, which holds one
// entry per selector, so the entry the operator ranked first might not act at all.
func TestSPDPolicySameSelectorRefused(t *testing.T) {
	out := orderTotalEntry(t, "out", "10.1.0.0/16", "192.0.2.0/24", 1000)
	out.Direction = ipsec.SPDDirOut
	both := orderTotalEntry(t, "both", "10.1.0.0/16", "192.0.2.0/24", 1500)
	err := verifySPDOrder(&ipsec.IPsecConfig{Policies: map[string]ipsec.SPDPolicy{"out": out, "both": both}})
	refusal, ok := errors.AsType[*opererr.Error](err)
	if !ok {
		t.Fatalf("two entries with one outbound selector answered %v, want an operator error", err)
	}
	if !strings.Contains(refusal.Problem(), "same selector") || refusal.Fix() != "delete vpn ipsec policy out" {
		t.Fatalf("the refusal is %q with fix %q", refusal.Problem(), refusal.Fix())
	}

	in := orderTotalEntry(t, "in", "10.1.0.0/16", "192.0.2.0/24", 1500)
	in.Direction = ipsec.SPDDirIn
	if err := verifySPDOrder(&ipsec.IPsecConfig{Policies: map[string]ipsec.SPDPolicy{"out": out, "in": in}}); err != nil {
		t.Fatalf("an out entry and an in entry over the same prefixes were refused: %v", err)
	}
}
