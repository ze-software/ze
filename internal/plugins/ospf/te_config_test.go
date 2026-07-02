// VALIDATES: spec-ospf-ext-2 traffic-engineering config -- the per-interface
// `traffic-engineering` block and the top-level `router-address` parse into typed config;
// the TE metric is independent of the OSPF cost; the inter-as sub-block resolves remote
// AS / ASBR / scope; and validateConfig rejects an inter-as block missing remote-as or
// any remote-asbr, an out-of-range te-metric, or a bad router-address.
// PREVENTS: TE config that silently drops attributes or accepts an incomplete inter-as link.
package ospf

import (
	"errors"
	"testing"
)

func parseTECfg(t *testing.T, json string) ospfConfig {
	t.Helper()
	cfg, err := parseOSPFConfig(ospfSec(json), nil)
	if err != nil {
		t.Fatalf("parseOSPFConfig: %v", err)
	}
	return cfg
}

func TestTEConfigParsesInterfaceBlock(t *testing.T) {
	const j = `{"ospf":{"router-id":"1.1.1.1","router-address":"9.9.9.9","opaque":true,
	  "areas":{"area":{"0":{"area-id":"0"}}},
	  "interfaces":{"interface":{"eth0":{"name":"eth0","area":"0","cost":"10",
	    "traffic-engineering":{"enable":true,"te-metric":"50","max-bandwidth":"1250000000",
	      "max-reservable-bandwidth":"1000000000","admin-group":"5"}}}}}}`
	cfg := parseTECfg(t, j)
	if !cfg.HasTERouterAddress || cfg.TERouterAddress != [4]byte{9, 9, 9, 9} {
		t.Fatalf("router-address = %v/%v", cfg.HasTERouterAddress, cfg.TERouterAddress)
	}
	if len(cfg.Interfaces) != 1 {
		t.Fatalf("interfaces = %d", len(cfg.Interfaces))
	}
	ic := cfg.Interfaces[0]
	te := ic.TE
	if !te.Enabled || !te.active() {
		t.Fatalf("TE not enabled: %+v", te)
	}
	// RFC 3630 sec 2.5.5: the TE metric is a separate value from the OSPF cost.
	if ic.Cost != 10 || !te.HasMetric || te.Metric != 50 {
		t.Fatalf("cost=%d te-metric=%d (must be independent)", ic.Cost, te.Metric)
	}
	if !te.HasMaxBandwidth || te.MaxBandwidth != 1250000000 {
		t.Fatalf("max-bandwidth = %v/%g", te.HasMaxBandwidth, te.MaxBandwidth)
	}
	if !te.HasMaxReservable || te.MaxReservable != 1000000000 {
		t.Fatalf("max-reservable = %v/%g", te.HasMaxReservable, te.MaxReservable)
	}
	if !te.HasAdminGroup || te.AdminGroup != 5 {
		t.Fatalf("admin-group = %v/%d", te.HasAdminGroup, te.AdminGroup)
	}
	if te.InterAS != nil {
		t.Fatalf("no inter-as expected: %+v", te.InterAS)
	}
}

func TestTEConfigParsesInterAS(t *testing.T) {
	const j = `{"ospf":{"router-id":"1.1.1.1","opaque":true,
	  "areas":{"area":{"0":{"area-id":"0"}}},
	  "interfaces":{"interface":{"eth0":{"name":"eth0","area":"0",
	    "traffic-engineering":{"enable":true,"inter-as":{"remote-as":"65001",
	      "remote-asbr-ipv4":"203.0.113.9","remote-asbr-ipv6":"2001:db8::1","scope":"as"}}}}}}}`
	cfg := parseTECfg(t, j)
	ia := cfg.Interfaces[0].TE.InterAS
	if ia == nil {
		t.Fatalf("inter-as not parsed")
	}
	if !ia.HasRemoteAS || ia.RemoteAS != 65001 {
		t.Fatalf("remote-as = %v/%d", ia.HasRemoteAS, ia.RemoteAS)
	}
	if !ia.HasRemoteASBRv4 || ia.RemoteASBRv4 != [4]byte{203, 0, 113, 9} {
		t.Fatalf("remote-asbr-ipv4 = %v/%v", ia.HasRemoteASBRv4, ia.RemoteASBRv4)
	}
	if !ia.HasRemoteASBRv6 {
		t.Fatalf("remote-asbr-ipv6 not parsed")
	}
	if ia.Scope != OpaqueScopeAS {
		t.Fatalf("scope = %v, want as", ia.Scope)
	}
}

func TestTEConfigInterASScopeDefaultsArea(t *testing.T) {
	const j = `{"ospf":{"router-id":"1.1.1.1","opaque":true,
	  "areas":{"area":{"0":{"area-id":"0"}}},
	  "interfaces":{"interface":{"eth0":{"name":"eth0","area":"0",
	    "traffic-engineering":{"inter-as":{"remote-as":"65001","remote-asbr-ipv4":"203.0.113.9"}}}}}}}`
	cfg := parseTECfg(t, j)
	ia := cfg.Interfaces[0].TE.InterAS
	if ia == nil || ia.Scope != OpaqueScopeArea {
		t.Fatalf("inter-as scope default = %v, want area (RFC 5392 sec 3.1.1)", ia)
	}
}

func TestTEConfigValidateInterASRequiresRemoteAS(t *testing.T) {
	const j = `{"ospf":{"router-id":"1.1.1.1","opaque":true,
	  "areas":{"area":{"0":{"area-id":"0"}}},
	  "interfaces":{"interface":{"eth0":{"name":"eth0","area":"0",
	    "traffic-engineering":{"inter-as":{"remote-asbr-ipv4":"203.0.113.9"}}}}}}}`
	cfg := parseTECfg(t, j)
	if err := validateConfig(cfg); !errors.Is(err, ErrTEInterASRemoteAS) {
		t.Fatalf("validate err = %v, want ErrTEInterASRemoteAS", err)
	}
}

func TestTEConfigValidateInterASRequiresASBR(t *testing.T) {
	const j = `{"ospf":{"router-id":"1.1.1.1","opaque":true,
	  "areas":{"area":{"0":{"area-id":"0"}}},
	  "interfaces":{"interface":{"eth0":{"name":"eth0","area":"0",
	    "traffic-engineering":{"inter-as":{"remote-as":"65001"}}}}}}}`
	cfg := parseTECfg(t, j)
	if err := validateConfig(cfg); !errors.Is(err, ErrTEInterASRemoteASBR) {
		t.Fatalf("validate err = %v, want ErrTEInterASRemoteASBR", err)
	}
}

func TestTEConfigRejectsBadRouterAddress(t *testing.T) {
	const j = `{"ospf":{"router-id":"1.1.1.1","router-address":"not-an-ip","areas":{"area":{"0":{"area-id":"0"}}}}}`
	if _, err := parseOSPFConfig(ospfSec(j), nil); !errors.Is(err, ErrTERouterAddress) {
		t.Fatalf("parse err = %v, want ErrTERouterAddress", err)
	}
}

func TestTEConfigRejectsBadScope(t *testing.T) {
	const j = `{"ospf":{"router-id":"1.1.1.1","opaque":true,"areas":{"area":{"0":{"area-id":"0"}}},
	  "interfaces":{"interface":{"eth0":{"name":"eth0","area":"0",
	    "traffic-engineering":{"inter-as":{"remote-as":"65001","remote-asbr-ipv4":"203.0.113.9","scope":"planet"}}}}}}}`
	if _, err := parseOSPFConfig(ospfSec(j), nil); !errors.Is(err, ErrTEInterASScope) {
		t.Fatalf("parse err = %v, want ErrTEInterASScope", err)
	}
}
