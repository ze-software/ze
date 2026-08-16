// VALIDATES: spec-ospf-ext-10 AC-1, AC-1b, AC-14 -- the per-interface `bfd` container parses
// on BOTH the IPv4 interface list and the address-family ipv6 interface list (one
// parseInterface path); milliseconds convert to stored microseconds; 0 / out-of-range timers
// and multipliers are rejected.
// PREVENTS: a BFD block silently ignored on one family, a unit mix-up (ms vs us), or an
// unusable session from a 0 timer/multiplier.
package ospf

import (
	"fmt"
	"testing"
)

func bfdIface(t *testing.T, cfg ospfConfig, name string) interfaceConfig {
	t.Helper()
	for _, ic := range cfg.Interfaces {
		if ic.Name == name {
			return ic
		}
	}
	t.Fatalf("interface %q not parsed", name)
	return interfaceConfig{}
}

func TestParseInterfaceBFD(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","bfd":{"enabled":"true","min-tx":"50","min-rx":"50","multiplier":"3"}}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	ic := bfdIface(t, cfg, "eth0")
	if !ic.BFD.Enabled {
		t.Fatal("BFD.Enabled = false, want true")
	}
	if ic.BFD.MinTxUs != 50000 || ic.BFD.MinRxUs != 50000 {
		t.Fatalf("BFD timers = tx %d / rx %d us, want 50000 / 50000 (50 ms)", ic.BFD.MinTxUs, ic.BFD.MinRxUs)
	}
	if ic.BFD.Multiplier != 3 {
		t.Fatalf("BFD.Multiplier = %d, want 3", ic.BFD.Multiplier)
	}
}

func TestParseInterfaceBFDDefaultsAndAbsent(t *testing.T) {
	// A bare `bfd { enabled true }` inherits the 50 ms / mult 3 defaults; no block -> disabled.
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","bfd":{"enabled":"true"}},"eth1":{"area":"0"}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	on := bfdIface(t, cfg, "eth0")
	if !on.BFD.Enabled || on.BFD.MinTxUs != DefaultBFDMinTxUs || on.BFD.MinRxUs != DefaultBFDMinRxUs || on.BFD.Multiplier != DefaultBFDMultiplier {
		t.Fatalf("eth0 defaults = %+v, want enabled with %d/%d us mult %d", on.BFD, DefaultBFDMinTxUs, DefaultBFDMinRxUs, DefaultBFDMultiplier)
	}
	if off := bfdIface(t, cfg, "eth1"); off.BFD.Enabled {
		t.Fatal("eth1 BFD.Enabled = true with no bfd block, want false")
	}
}

func TestParseInterfaceBFDv6(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","address-family":{"ipv6":{"areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","bfd":{"enabled":"true","min-tx":"50","min-rx":"50","multiplier":"3"}}}}}}}}`), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	if cfg.V6 == nil {
		t.Fatal("address-family ipv6 not parsed")
	}
	ic := bfdIface(t, *cfg.V6, "eth0")
	if !ic.BFD.Enabled || ic.BFD.MinTxUs != 50000 || ic.BFD.MinRxUs != 50000 || ic.BFD.Multiplier != 3 {
		t.Fatalf("v6 BFD = %+v, want enabled with 50000/50000 us mult 3", ic.BFD)
	}
}

func TestParseInterfaceBFDBoundary(t *testing.T) {
	tmpl := `{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},"interfaces":{"interface":{"eth0":{"area":"0","bfd":%s}}}}}`
	cases := []struct {
		bfd    string
		accept bool
	}{
		{`{"enabled":"true","min-tx":"0"}`, false},
		{`{"enabled":"true","min-rx":"0"}`, false},
		{`{"enabled":"true","min-tx":"10000","min-rx":"10000"}`, true},
		{`{"enabled":"true","min-tx":"10001"}`, false},
		{`{"enabled":"true","multiplier":"0"}`, false},
		{`{"enabled":"true","multiplier":"255"}`, true},
		{`{"enabled":"true","multiplier":"256"}`, false},
	}
	for _, c := range cases {
		_, err := parseOSPFConfig(ospfSec(fmt.Sprintf(tmpl, c.bfd)), nil)
		if c.accept && err != nil {
			t.Fatalf("bfd=%s: expected acceptance, got %v", c.bfd, err)
		}
		if !c.accept && err == nil {
			t.Fatalf("bfd=%s: expected rejection, got none", c.bfd)
		}
	}
}
