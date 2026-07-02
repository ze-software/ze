// VALIDATES: spec-ospf-ext-6 -- the `fast-reroute` config container parses into
// the resolved policy (enable, mode lfa|ti-lfa, node-protection) and is inherited
// into the OSPFv3 address family as a router-wide policy.
// PREVENTS: fast-reroute config being silently dropped, or the v6 engine not
// getting the base-LFA policy.
package ospf

import "testing"

func TestFastRerouteConfigParse(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1",`+
		`"fast-reroute":{"enable":true,"mode":"ti-lfa","node-protection":false},`+
		`"address-family":{"ipv6":{"instance-id":0,"areas":{"area":{"0":{"area-id":"0"}}}}}`+
		`}}`), nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !cfg.FastReroute.Enabled || !cfg.FastReroute.TILFA || cfg.FastReroute.NodeProtection {
		t.Fatalf("v4 fast-reroute = %+v, want enabled ti-lfa node-protection=false", cfg.FastReroute)
	}
	// Router-wide policy inherited into the OSPFv3 address family.
	if cfg.V6 == nil {
		t.Fatal("v6 address family not parsed")
	}
	if !cfg.V6.FastReroute.Enabled || !cfg.V6.FastReroute.TILFA {
		t.Fatalf("v6 fast-reroute not inherited: %+v", cfg.V6.FastReroute)
	}
}

func TestFastRerouteDisabledByDefault(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1"}}`), nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if cfg.FastReroute.Enabled {
		t.Fatal("fast-reroute enabled with no config; must default off")
	}
}

func TestFastRerouteModeDefaultsToLFA(t *testing.T) {
	cfg, err := parseOSPFConfig(ospfSec(`{"ospf":{"router-id":"10.0.0.1","fast-reroute":{"enable":true}}}`), nil)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !cfg.FastReroute.Enabled || cfg.FastReroute.TILFA {
		t.Fatalf("mode default = %+v, want base lfa (TILFA false)", cfg.FastReroute)
	}
	if !cfg.FastReroute.NodeProtection {
		t.Fatal("node-protection should default true")
	}
}
