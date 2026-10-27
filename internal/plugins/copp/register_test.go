package copp

import (
	"net/netip"
	"slices"
	"sync"
	"testing"

	"github.com/ze-software/ze/internal/component/firewall"
	"github.com/ze-software/ze/internal/component/plugin/registry"
	sdk "github.com/ze-software/ze/pkg/plugin/sdk"
)

func TestCoppRegistration(t *testing.T) {
	reg := registry.Lookup("copp")
	if reg == nil {
		t.Fatal("copp plugin not registered")
	}
	if reg.Name != "copp" {
		t.Errorf("name = %q, want copp", reg.Name)
	}
	if reg.RunEngine == nil {
		t.Error("RunEngine is nil")
	}
	if len(reg.ConfigRoots) != 1 || reg.ConfigRoots[0] != "control-plane-protection" {
		t.Errorf("ConfigRoots = %v, want [control-plane-protection]", reg.ConfigRoots)
	}
	// bgp is READ, never owned: a config that holds only bgp must not load
	// copp, which keeps CoPP opt-in (spec-bcp194-3-session D-9).
	if !slices.Equal(reg.ConfigReads, []string{"bgp"}) {
		t.Errorf("ConfigReads = %v, want [bgp]", reg.ConfigReads)
	}
}

func TestCoppRegistrationHasDoctorCheck(t *testing.T) {
	reg := registry.Lookup("copp")
	if reg == nil {
		t.Fatal("copp plugin not registered")
	}
	if len(reg.DoctorChecks) == 0 {
		t.Error("DoctorChecks is empty, want at least 1")
	}
	found := false
	for _, dc := range reg.DoctorChecks {
		if dc.Name == "copp-input-chain" {
			found = true
			break
		}
	}
	if !found {
		t.Error("doctor check copp-input-chain not found")
	}
}

// TestCoppDropTermWiring is the wiring proof for the CoPP ACL: the two config
// sections the SDK delivers (control-plane-protection, and the bgp root copp
// reads) go through the same verify and apply seam the plugin's handlers run,
// and the table the apply installs holds the configured neighbors in a set
// and ends with a drop of TCP to the protected port.
//
// VALIDATES: stageVerifiedPolicy -> parseCoppSections -> BGPNeighborSources
// -> applyStagedPolicy -> translatePolicy, with a static peer and a dynamic
// group's range in the bgp section.
// PREVENTS: an ACL that never learns the neighbors, or that admits every
// source because nothing drops the rest (RFC 7454 Section 4).
func TestCoppDropTermWiring(t *testing.T) {
	sections := []sdk.ConfigSection{
		{Root: configRoot, Data: `{"control-plane-protection": {"bgp": {"rate": "10/second"}}}`},
		{Root: bgpConfigRoot, Data: `{"bgp": {
			"peer": {"edge": {"connection": {"remote": {"ip": "192.0.2.1"}}}},
			"group": {"ixp": {"connection": {"remote": {"ip": "dynamic", "range": ["10.10.0.0/16"], "connect": "false"}}}}
		}}`},
	}

	var mu sync.Mutex
	var pending pendingPolicy
	if err := stageVerifiedPolicy(sections, &pending, &mu); err != nil {
		t.Fatalf("verify: %v", err)
	}

	var installed *coppPolicy
	_, err := applyStagedPolicy(&pending, &mu, nil, func(p *coppPolicy) error {
		installed = p
		return nil
	})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if installed == nil {
		t.Fatal("apply installed no policy")
	}

	staticPeer := netip.MustParsePrefix("192.0.2.1/32")
	dynamicRange := netip.MustParsePrefix("10.10.0.0/16")
	if !slices.Equal(installed.Neighbors, []netip.Prefix{dynamicRange, staticPeer}) {
		t.Fatalf("staged neighbors = %v, want [%s %s]", installed.Neighbors, dynamicRange, staticPeer)
	}

	table := translatePolicy(*installed)

	inSet := setElementPrefixes(t, table.Sets)
	for _, want := range []netip.Prefix{staticPeer, dynamicRange} {
		if !slices.Contains(inSet, want) {
			t.Errorf("table %s: no set holds %s; set elements %v", table.Name, want, inSet)
		}
	}

	if len(table.Chains) != 1 {
		t.Fatalf("table %s: %d chains, want 1", table.Name, len(table.Chains))
	}
	terms := table.Chains[0].Terms
	if len(terms) == 0 {
		t.Fatalf("table %s: chain %s has no term", table.Name, table.Chains[0].Name)
	}
	last := terms[len(terms)-1]
	if !dropsTCPToPort(last, defaultPort) {
		t.Errorf("last term %q = %+v, want a drop of TCP to port %d from any source", last.Name, last, defaultPort)
	}
}

// setElementPrefixes reads every element of every set as a prefix: an
// interval element as written, a lone address as its host prefix.
func setElementPrefixes(t *testing.T, sets []firewall.Set) []netip.Prefix {
	t.Helper()

	var prefixes []netip.Prefix
	for _, set := range sets {
		for _, element := range set.Elements {
			if prefix, err := netip.ParsePrefix(element.Value); err == nil {
				prefixes = append(prefixes, prefix)
				continue
			}
			addr, err := netip.ParseAddr(element.Value)
			if err != nil {
				t.Fatalf("set %s: element %q is neither a prefix nor an address", set.Name, element.Value)
			}
			prefixes = append(prefixes, netip.PrefixFrom(addr, addr.BitLen()))
		}
	}
	return prefixes
}

// dropsTCPToPort reports whether a term drops TCP to port whatever its source:
// it matches protocol tcp and a destination port range holding port, names no
// source, and its only action is a drop.
func dropsTCPToPort(term firewall.Term, port uint16) bool {
	tcp, toPort := false, false
	for _, match := range term.Matches {
		switch m := match.(type) {
		case firewall.MatchProtocol:
			tcp = m.Protocol == "tcp"
		case firewall.MatchDestinationPort:
			for _, r := range m.Ranges {
				if r.Lo <= port && port <= r.Hi {
					toPort = true
				}
			}
		case firewall.MatchSourceAddress, firewall.MatchInSet:
			return false
		}
	}
	if len(term.Actions) != 1 {
		return false
	}
	_, drop := term.Actions[0].(firewall.Drop)
	return tcp && toPort && drop
}
