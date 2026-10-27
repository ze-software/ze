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

	// The input-hook base chain is the one the kernel walks for every packet;
	// a regular chain is reached only by a jump from it, so the base chain's
	// last term is the one every other source meets.
	var input *firewall.Chain
	for i := range table.Chains {
		if !table.Chains[i].IsBase {
			continue
		}
		if input != nil {
			t.Fatalf("table %s: two base chains, %s and %s", table.Name, input.Name, table.Chains[i].Name)
		}
		input = &table.Chains[i]
	}
	if input == nil {
		t.Fatalf("table %s: no base chain", table.Name)
	}
	if input.Hook != firewall.HookInput {
		t.Fatalf("table %s: base chain %s hooks %v, want input", table.Name, input.Name, input.Hook)
	}
	terms := input.Terms
	if len(terms) == 0 {
		t.Fatalf("table %s: chain %s has no term", table.Name, input.Name)
	}
	last := terms[len(terms)-1]
	if !dropsTCPToPort(last, defaultPort) {
		t.Errorf("last term %q = %+v, want a drop of TCP to port %d from any source", last.Name, last, defaultPort)
	}
}

// setElementPrefixes reads every set as the prefixes it holds. A set without
// the interval flag holds addresses, each read as its host prefix. An interval
// set holds a start element, then the exclusive end element marked
// IntervalEnd, or no end when the interval runs to the top of the address
// space; each interval MUST be exactly one prefix.
func setElementPrefixes(t *testing.T, sets []firewall.Set) []netip.Prefix {
	t.Helper()

	var prefixes []netip.Prefix
	for _, set := range sets {
		elements := set.Elements
		for i := 0; i < len(elements); i++ {
			start, err := netip.ParseAddr(elements[i].Value)
			if err != nil {
				t.Fatalf("set %s: element %q is not an address", set.Name, elements[i].Value)
			}
			if elements[i].IntervalEnd {
				t.Fatalf("set %s: element %q ends an interval nothing started", set.Name, elements[i].Value)
			}
			if set.Flags&firewall.SetFlagInterval == 0 {
				prefixes = append(prefixes, netip.PrefixFrom(start, start.BitLen()))
				continue
			}
			var after netip.Addr
			if i+1 < len(elements) && elements[i+1].IntervalEnd {
				i++
				after, err = netip.ParseAddr(elements[i].Value)
				if err != nil {
					t.Fatalf("set %s: interval end %q is not an address", set.Name, elements[i].Value)
				}
			}
			prefixes = append(prefixes, intervalPrefix(t, set.Name, start, after))
		}
	}
	return prefixes
}

// intervalPrefix answers the prefix whose addresses are exactly [start, after),
// where an invalid after means the interval runs to the top of the space.
func intervalPrefix(t *testing.T, setName string, start, after netip.Addr) netip.Prefix {
	t.Helper()

	// Widen from the host prefix: the first prefix starting at start whose
	// last address is the one before after (or the top of the space) is the
	// interval, and a prefix that no longer starts at start ends the search.
	for bits := start.BitLen(); bits >= 0; bits-- {
		prefix := netip.PrefixFrom(start, bits)
		if prefix.Masked().Addr() != start {
			break
		}
		last := testLastAddress(prefix)
		if last.Next() == after {
			return prefix
		}
	}
	t.Fatalf("set %s: interval [%s, %s) is not one prefix", setName, start, after)
	return netip.Prefix{}
}

// testLastAddress sets every host bit of prefix. It is written here rather
// than borrowed from translate.go, so the reading does not share a defect with
// the producer it judges.
func testLastAddress(prefix netip.Prefix) netip.Addr {
	octets := prefix.Masked().Addr().AsSlice()
	for bit := prefix.Bits(); bit < len(octets)*8; bit++ {
		octets[bit/8] |= 0x80 >> (bit % 8)
	}
	addr, ok := netip.AddrFromSlice(octets)
	if !ok {
		panic("BUG: AsSlice of a valid address is 4 or 16 octets")
	}
	return addr
}

// dropsTCPToPort reports whether a term drops TCP to port whatever its source:
// it matches protocol tcp and a destination port range holding port, names no
// source, and its actions are a drop, optionally counted first. No other
// action may ride the term, so it can admit nothing.
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
	actions := term.Actions
	if len(actions) == 2 {
		if _, counted := actions[0].(firewall.Counter); !counted {
			return false
		}
		actions = actions[1:]
	}
	if len(actions) != 1 {
		return false
	}
	_, drop := actions[0].(firewall.Drop)
	return tcp && toPort && drop
}
