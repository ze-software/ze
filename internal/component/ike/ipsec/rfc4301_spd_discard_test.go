// Design: docs/architecture/ike/ipsec-3-data-model.md -- IPsec data model
// Related: spd_policy.go -- parseSPDPolicy and ValidateSPDPolicies, the producers
// RFC: rfc/short/rfc4301.md -- SPD dispositions (Sections 4.4.1, 7.4)
package ipsec

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/component/config"
	"github.com/ze-software/ze/internal/component/ike/dataplane"
)

// spdTree builds vpn { ipsec { policy <name> { ... } } } from the leaves a test sets.
// A leaf absent from the map is absent from the tree, which is how the defaults the
// parser resolves are put under test.
func spdTree(name string, leaves, local, remote map[string]string) *config.Tree {
	tree := config.NewTree()
	ipsec := tree.GetOrCreateContainer("vpn").GetOrCreateContainer("ipsec")

	entry := config.NewTree()
	for k, v := range leaves {
		entry.Set(k, v)
	}
	for side, values := range map[string]map[string]string{"local": local, "remote": remote} {
		if len(values) == 0 {
			continue
		}
		sub := entry.GetOrCreateContainer(side)
		for k, v := range values {
			sub.Set(k, v)
		}
	}
	ipsec.AddListEntry("policy", name, entry)
	return tree
}

// wideSPD is the smallest valid entry: one disposition and the two mandatory prefixes.
func wideSPD(name, action string) *config.Tree {
	return spdTree(name,
		map[string]string{"action": action},
		map[string]string{"prefix": "192.0.2.0/24"},
		map[string]string{"prefix": "198.51.100.0/24"})
}

// VALIDATES: RFC4301-7.4-1. An operator can write a DISCARD entry, and it reaches the
// typed model as dataplane.SPActionDiscard.
// PREVENTS: the disposition existing in the dataplane and being unreachable from a
// configuration. RFC 4301 Section 7.4 asks for DISCARD "using the normal SPD packet
// classification mechanisms", and a disposition no administrator can write is not a
// mechanism they have.
// RFC requirement: RFC4301-7.4-1 positive -- an administrator can write a discard entry.
func TestSPDPolicyCarriesTheDiscardDisposition(t *testing.T) {
	cfg, err := ParseIPsecConfig(wideSPD("drop-guest", "discard"))
	if err != nil {
		t.Fatalf("ParseIPsecConfig: %v", err)
	}
	p, ok := cfg.Policies["drop-guest"]
	if !ok {
		t.Fatalf("the policy list parsed no entry named drop-guest; got %v", cfg.Policies)
	}
	if p.Action != dataplane.SPActionDiscard {
		t.Errorf("Action = %d, want SPActionDiscard (%d)", p.Action, dataplane.SPActionDiscard)
	}
	// The defaults the parser resolves rather than leaving to a Go zero value.
	if p.Direction != SPDDirBoth {
		t.Errorf("Direction = %v, want both", p.Direction)
	}
	if p.Order != 1000 {
		t.Errorf("Order = %d, want the YANG default of 1000", p.Order)
	}
	if err := cfg.ValidateSPDPolicies(); err != nil {
		t.Errorf("a minimal discard entry was refused: %v", err)
	}
}

// VALIDATES: RFC4301-7.4-1. The BYPASS disposition of the same list stays a bypass,
// so the two are told apart by what the operator wrote.
// PREVENTS: one disposition being parsed for the other. SPActionProtect is the zero
// value of SPAction, so a parser that failed to read the leaf would produce neither
// of the two words the list offers, and the entry would demand a transform it has no
// peer to negotiate.
// RFC requirement: RFC4301-7.4-1 negative -- a bypass entry is not read as a discard.
func TestSPDPolicyBypassIsNotADiscard(t *testing.T) {
	cfg, err := ParseIPsecConfig(wideSPD("pass-mgmt", "bypass"))
	if err != nil {
		t.Fatalf("ParseIPsecConfig: %v", err)
	}
	if got := cfg.Policies["pass-mgmt"].Action; got != dataplane.SPActionBypass {
		t.Errorf("Action = %d, want SPActionBypass (%d)", got, dataplane.SPActionBypass)
	}
}

// VALIDATES: an entry with no action leaf, and one naming a disposition this list does
// not carry, are both refused at parse.
// PREVENTS: the SPAction zero value reading as a disposition. SPActionProtect is 0, so
// an unread action leaf would produce a template-free PROTECT entry: the backend would
// refuse it at install time, on a running daemon, where the operator cannot see it.
func TestSPDPolicyRefusesAnAbsentOrUnknownAction(t *testing.T) {
	noAction := spdTree("nameless", nil,
		map[string]string{"prefix": "192.0.2.0/24"},
		map[string]string{"prefix": "198.51.100.0/24"})
	if _, err := ParseIPsecConfig(noAction); err == nil {
		t.Error("an entry with no action was accepted; the disposition has no safe default")
	}

	protect := wideSPD("wrong", "protect")
	_, err := ParseIPsecConfig(protect)
	if err == nil {
		t.Fatal("action protect was accepted; a protect entry names a transform and a peer")
	}
	if !strings.Contains(err.Error(), "site-to-site peer") {
		t.Errorf("the refusal does not say where a protect entry is written: %v", err)
	}
}

// VALIDATES: RFC4301-4.4.1-4. An operator SPD entry ranked at or above the IKE
// control-plane bypass is refused at commit.
// PREVENTS: an entry that captures the IKE exchange. A DISCARD at rank 100 or below
// matches ze's own IKE datagrams before the bypass does, so every tunnel on the node
// stops being built, rekeyed and torn down, and the kernel reports nothing because it
// picked the entry the operator ranked highest, exactly as asked.
// RFC requirement: RFC4301-4.4.1-4 negative -- an order that captures IKE is refused.
func TestSPDPolicyOrderInsideBypassBandRefused(t *testing.T) {
	for _, order := range []string{"1", "99", "100"} {
		tree := spdTree("greedy",
			map[string]string{"action": "discard", "order": order},
			map[string]string{"prefix": "192.0.2.0/24"},
			map[string]string{"prefix": "198.51.100.0/24"})
		cfg, err := ParseIPsecConfig(tree)
		if err != nil {
			t.Fatalf("ParseIPsecConfig for order %s: %v", order, err)
		}
		if err := cfg.ValidateSPDPolicies(); err == nil {
			t.Errorf("order %s was accepted; it ranks at or above the IKE bypass (%d)",
				order, dataplane.PriorityIKEBypass)
		}
	}

	// The positive half: the first rank above the band is accepted, so the refusal is
	// a boundary rather than a blanket ban on ordering.
	tree := spdTree("polite",
		map[string]string{"action": "discard", "order": "101"},
		map[string]string{"prefix": "192.0.2.0/24"},
		map[string]string{"prefix": "198.51.100.0/24"})
	cfg, err := ParseIPsecConfig(tree)
	if err != nil {
		t.Fatalf("ParseIPsecConfig for order 101: %v", err)
	}
	if err := cfg.ValidateSPDPolicies(); err != nil {
		t.Errorf("order 101 was refused, and it is the first rank below the IKE bypass: %v", err)
	}
}

// VALIDATES: a port selector without a protocol that carries ports is refused, and one
// with TCP, UDP or SCTP is accepted.
// PREVENTS: a rule matching on a field the classifier was never told is present. RFC
// 4301 Section 4.4.1.1 lists the port selectors under the next-layer protocol, and a
// port under protocol 0 would be compared against whatever the transport decode left
// in the flow key.
func TestSPDPolicyPortNeedsAProtocolThatCarriesPorts(t *testing.T) {
	withPort := func(proto string) *IPsecConfig {
		t.Helper()
		leaves := map[string]string{"action": "discard"}
		if proto != "" {
			leaves["protocol"] = proto
		}
		tree := spdTree("ports", leaves,
			map[string]string{"prefix": "192.0.2.0/24", "port": "443"},
			map[string]string{"prefix": "198.51.100.0/24"})
		cfg, err := ParseIPsecConfig(tree)
		if err != nil {
			t.Fatalf("ParseIPsecConfig for protocol %q: %v", proto, err)
		}
		return cfg
	}

	if err := withPort("").ValidateSPDPolicies(); err == nil {
		t.Error("a port selector under the default protocol 0 was accepted")
	}
	if err := withPort("1").ValidateSPDPolicies(); err == nil {
		t.Error("a port selector under ICMP (1) was accepted, and ICMP carries no ports")
	}
	for _, proto := range []string{"6", "17", "132"} {
		if err := withPort(proto).ValidateSPDPolicies(); err != nil {
			t.Errorf("a port selector under protocol %s was refused, and that protocol carries ports: %v", proto, err)
		}
	}
}

// VALIDATES: an entry whose two prefixes are different address families is refused.
// PREVENTS: a selector with no faithful projection. XfrmSelector carries ONE family
// field, so the backend would have to pick one and install a rule over addresses the
// operator did not write.
func TestSPDPolicyRefusesAMixedAddressFamilyPair(t *testing.T) {
	tree := spdTree("mixed",
		map[string]string{"action": "discard"},
		map[string]string{"prefix": "192.0.2.0/24"},
		map[string]string{"prefix": "2001:db8::/32"})
	cfg, err := ParseIPsecConfig(tree)
	if err != nil {
		t.Fatalf("ParseIPsecConfig: %v", err)
	}
	if err := cfg.ValidateSPDPolicies(); err == nil {
		t.Error("a mixed-family prefix pair was accepted; one policy selector carries one family")
	}

	// The negative half: a matched IPv6 pair is accepted, so the refusal is about the
	// MIX rather than about IPv6.
	v6 := spdTree("v6",
		map[string]string{"action": "discard"},
		map[string]string{"prefix": "2001:db8:1::/48"},
		map[string]string{"prefix": "2001:db8:2::/48"})
	cfg6, err := ParseIPsecConfig(v6)
	if err != nil {
		t.Fatalf("ParseIPsecConfig for the IPv6 pair: %v", err)
	}
	if err := cfg6.ValidateSPDPolicies(); err != nil {
		t.Errorf("a matched IPv6 pair was refused: %v", err)
	}
}

// VALIDATES: an entry missing either mandatory prefix is refused.
// PREVENTS: a nil prefix reaching the backend. netlink derives a selector family from
// the destination prefix, so a nil one installs a rule over the wildcard of a family
// nobody chose.
func TestSPDPolicyNeedsBothPrefixes(t *testing.T) {
	for _, missing := range []string{"local", "remote"} {
		local := map[string]string{"prefix": "192.0.2.0/24"}
		remote := map[string]string{"prefix": "198.51.100.0/24"}
		if missing == "local" {
			local = nil
		} else {
			remote = nil
		}
		cfg, err := ParseIPsecConfig(spdTree("half", map[string]string{"action": "discard"}, local, remote))
		if err != nil {
			t.Fatalf("ParseIPsecConfig missing the %s prefix: %v", missing, err)
		}
		if err := cfg.ValidateSPDPolicies(); err == nil {
			t.Errorf("an entry with no %s prefix was accepted", missing)
		}
	}
}

// VALIDATES: a configuration with no policy list parses to no entries and installs
// nothing.
// PREVENTS: the new list changing what an existing configuration does. Every IPsec
// configuration written before this list existed carries no policy node, and it must
// keep installing exactly the policies it installed before.
func TestSPDPolicyAbsentListInstallsNothing(t *testing.T) {
	cfg, err := ParseIPsecConfig(makeESPTree("ESP-1", "3600", "", map[string][2]string{"10": {"aes256", "sha256"}}))
	if err != nil {
		t.Fatalf("ParseIPsecConfig: %v", err)
	}
	if len(cfg.Policies) != 0 {
		t.Errorf("a configuration with no policy list parsed %d entries", len(cfg.Policies))
	}
	if err := cfg.ValidateSPDPolicies(); err != nil {
		t.Errorf("a configuration with no policy list was refused: %v", err)
	}
}
