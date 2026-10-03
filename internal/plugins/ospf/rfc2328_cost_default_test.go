// Design: docs/architecture/ospf/ospf-4-component-config.md -- OSPF interface output cost.
// Related: interface_cost.go -- interfaceCost, the one producer of an interface's cost.
// Related: rfc2328_config_interface_validate_test.go -- the explicit cost 0 refusal.
//
// VALIDATES: RFC 2328 Appendix C.3 "Interface output cost ... must always be greater than
// 0" for a cost that is NOT configured: every interface resolved from the configuration
// text with no `cost` leaf gets a cost of at least 1, whatever the link speed and the
// reference bandwidth are.
// PREVENTS: an auto-cost derivation that divides down to 0 (a link faster than the
// reference bandwidth) or defaults an unknown speed to 0.
package ospf

import "testing"

// rfc2328DefaultedCosts resolves a configuration whose interfaces carry no `cost` leaf and
// returns the output cost interfaceCost gives each one.
func rfc2328DefaultedCosts(t *testing.T, referenceBandwidth string) map[string]uint16 {
	t.Helper()
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"1.1.1.1","reference-bandwidth":"`+referenceBandwidth+`",`+
		`"areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{`+
		`"eth0":{"name":"eth0","area":"0"},"eth1":{"name":"eth1","area":"0"},"eth2":{"name":"eth2","area":"0"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	costs := map[string]uint16{}
	for _, ic := range cfg.Interfaces {
		if ic.HasCost {
			t.Fatalf("interface %s has a cost leaf, want none", ic.Name)
		}
		costs[ic.Name] = interfaceCost(ic, cfg.ReferenceBandwidth)
	}
	return costs
}

// RFC requirement: RFC2328-C.3-1 positive -- an interface resolved from configuration with
// no cost leaf takes a cost greater than 0: 100 for a 1 Gbit/s link under a 100 Gbit/s
// reference bandwidth.
// RFC requirement: RFC2328-C.3-1 negative -- the inputs that divide down to 0 still give a
// cost of 1: a link faster than the reference bandwidth (100 Gbit/s under 1 Mbit/s, where
// integer division is 0) and a link whose speed the kernel does not report.
func TestRFC2328DefaultedInterfaceCostIsPositive(t *testing.T) {
	// Goal: "must always be greater than 0" for every source of the cost. Method: stub the
	// link speeds, resolve interfaces with no cost leaf, read interfaceCost for each.
	stubLinkSpeed(t, map[string]uint64{"eth0": 1000, "eth1": 100000, "eth2": 0})

	if got := rfc2328DefaultedCosts(t, "100000")["eth0"]; got != 100 {
		t.Fatalf("1 Gbit/s link under a 100 Gbit/s reference: cost = %d, want 100", got)
	}
	costs := rfc2328DefaultedCosts(t, "1")
	if costs["eth1"] != 1 {
		t.Fatalf("100 Gbit/s link under a 1 Mbit/s reference: cost = %d, want 1 (never 0)", costs["eth1"])
	}
	if costs["eth2"] != 1 {
		t.Fatalf("link with no reported speed: cost = %d, want 1 (never 0)", costs["eth2"])
	}
}
