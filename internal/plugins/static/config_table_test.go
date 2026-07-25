package static

import (
	"net/netip"
	"testing"

	"github.com/ze-software/ze/internal/core/routingtable"
)

func testRegistry() *routingtable.Registry {
	return routingtable.New(map[string]uint32{
		"lns":         100,
		"surfprotect": 200,
	})
}

func defaultOnlyRegistry() *routingtable.Registry {
	return routingtable.New(nil)
}

func TestExtractRouteInTable(t *testing.T) {
	input := `{"static":{"table":{"lns":{"route":{"0.0.0.0/0":{"next":{"hop":{"10.0.0.1":{}}}}}}}}}` //nolint:lll // JSON test fixture
	routes, err := parseStaticConfig(input, testRegistry())
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatalf("got %d routes, want 1", len(routes))
	}
	if routes[0].Table != 100 {
		t.Errorf("table = %d, want 100", routes[0].Table)
	}
	if routes[0].Prefix != netip.MustParsePrefix("0.0.0.0/0") {
		t.Errorf("prefix = %s, want 0.0.0.0/0", routes[0].Prefix)
	}
}

func TestExtractRouteInDefaultTable(t *testing.T) {
	input := `{"static":{"table":{"default":{"route":{"10.0.0.0/8":{"next":{"hop":{"192.168.1.1":{}}}}}}}}}` //nolint:lll // JSON test fixture
	routes, err := parseStaticConfig(input, defaultOnlyRegistry())
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatalf("got %d routes, want 1", len(routes))
	}
	if routes[0].Table != 0 {
		t.Errorf("table = %d, want 0", routes[0].Table)
	}
}

func TestExtractInterfaceOnlyRoute(t *testing.T) {
	input := `{"static":{"table":{"default":{"route":{"0.0.0.0/0":{"next":{"interface":{"pppoe0":{}}}}}}}}}` //nolint:lll // JSON test fixture
	routes, err := parseStaticConfig(input, defaultOnlyRegistry())
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatalf("got %d routes, want 1", len(routes))
	}
	r := routes[0]
	if r.Action != actionForward {
		t.Errorf("action = %s, want forward", r.Action)
	}
	if len(r.NextHops) != 1 {
		t.Fatalf("got %d next-hops, want 1", len(r.NextHops))
	}
	nh := r.NextHops[0]
	if nh.Address.IsValid() {
		t.Errorf("address = %s, want zero (interface-only)", nh.Address)
	}
	if nh.Interface != "pppoe0" {
		t.Errorf("interface = %q, want %q", nh.Interface, "pppoe0")
	}
	if nh.Weight != 1 {
		t.Errorf("weight = %d, want 1", nh.Weight)
	}
}

func TestExtractTableAndInterface(t *testing.T) {
	input := `{"static":{"table":{"lns":{"route":{"0.0.0.0/0":{"next":{"interface":{"tun100":{}}}}}}}}}` //nolint:lll // JSON test fixture
	routes, err := parseStaticConfig(input, testRegistry())
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatalf("got %d routes, want 1", len(routes))
	}
	if routes[0].Table != 100 {
		t.Errorf("table = %d, want 100", routes[0].Table)
	}
	if routes[0].NextHops[0].Interface != "tun100" {
		t.Errorf("interface = %q, want %q", routes[0].NextHops[0].Interface, "tun100")
	}
}

func TestExtractMixedECMP(t *testing.T) {
	input := `{"static":{"table":{"default":{"route":{"0.0.0.0/0":{"next":{"hop":{"10.0.0.1":{"weight":3}},"interface":{"pppoe0":{"weight":1}}}}}}}}}` //nolint:lll // JSON test fixture
	routes, err := parseStaticConfig(input, defaultOnlyRegistry())
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatalf("got %d routes, want 1", len(routes))
	}
	r := routes[0]
	if len(r.NextHops) != 2 {
		t.Fatalf("got %d next-hops, want 2 (mixed ECMP)", len(r.NextHops))
	}
	var hasGateway, hasInterface bool
	for _, nh := range r.NextHops {
		if nh.Address.IsValid() {
			hasGateway = true
		}
		if nh.Interface == "pppoe0" && !nh.Address.IsValid() {
			hasInterface = true
		}
	}
	if !hasGateway {
		t.Error("missing gateway next-hop in mixed ECMP")
	}
	if !hasInterface {
		t.Error("missing interface-only next-hop in mixed ECMP")
	}
}

func TestExtractSamePrefixDifferentTables(t *testing.T) {
	input := `{"static":{"table":{"default":{"route":{"0.0.0.0/0":{"next":{"hop":{"10.0.0.1":{}}}}}},"lns":{"route":{"0.0.0.0/0":{"next":{"interface":{"tun100":{}}}}}}}}}` //nolint:lll // JSON test fixture
	routes, err := parseStaticConfig(input, testRegistry())
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 2 {
		t.Fatalf("got %d routes, want 2 (same prefix, different tables)", len(routes))
	}
	tables := map[uint32]bool{}
	for _, r := range routes {
		tables[r.Table] = true
	}
	if !tables[0] || !tables[100] {
		t.Errorf("expected routes in table 0 and 100, got tables %v", tables)
	}
}

func TestRejectNextHopNoAddressNoInterface(t *testing.T) {
	input := `{"static":{"table":{"default":{"route":{"10.0.0.0/8":{}}}}}}`
	_, err := parseStaticConfig(input, defaultOnlyRegistry())
	if err == nil {
		t.Fatal("expected error for route with no action")
	}
}

func TestRejectBFDOnInterfaceOnly(t *testing.T) {
	input := `{"static":{"table":{"default":{"route":{"10.0.0.0/8":{"next":{"interface":{"eth0":{"bfd-profile":"fast"}}}}}}}}}` //nolint:lll // JSON test fixture
	_, err := parseStaticConfig(input, defaultOnlyRegistry())
	if err == nil {
		t.Fatal("expected error for BFD profile on interface-only next-hop")
	}
}

func TestRejectUnknownTableName(t *testing.T) {
	input := `{"static":{"table":{"nonexistent":{"route":{"10.0.0.0/8":{"next":{"hop":{"10.0.0.1":{}}}}}}}}}` //nolint:lll // JSON test fixture
	_, err := parseStaticConfig(input, defaultOnlyRegistry())
	if err == nil {
		t.Fatal("expected error for unknown table name")
	}
}

func TestExistingGatewayRouteUnchanged(t *testing.T) {
	input := `{"static":{"table":{"default":{"route":{"10.0.0.0/8":{"next":{"hop":{"192.168.1.1":{}}}}}}}}}` //nolint:lll // JSON test fixture
	routes, err := parseStaticConfig(input, defaultOnlyRegistry())
	if err != nil {
		t.Fatal(err)
	}
	if len(routes) != 1 {
		t.Fatalf("got %d routes, want 1", len(routes))
	}
	r := routes[0]
	if r.Table != 0 {
		t.Errorf("table = %d, want 0", r.Table)
	}
	if r.Action != actionForward {
		t.Errorf("action = %s, want forward", r.Action)
	}
	if r.NextHops[0].Address != netip.MustParseAddr("192.168.1.1") {
		t.Errorf("next-hop = %s, want 192.168.1.1", r.NextHops[0].Address)
	}
	if r.NextHops[0].Weight != 1 {
		t.Errorf("weight = %d, want 1", r.NextHops[0].Weight)
	}
}

func TestInterfaceOnlyWithWeight(t *testing.T) {
	input := `{"static":{"table":{"default":{"route":{"0.0.0.0/0":{"next":{"interface":{"pppoe0":{"weight":5}}}}}}}}}` //nolint:lll // JSON test fixture
	routes, err := parseStaticConfig(input, defaultOnlyRegistry())
	if err != nil {
		t.Fatal(err)
	}
	if routes[0].NextHops[0].Weight != 5 {
		t.Errorf("weight = %d, want 5", routes[0].NextHops[0].Weight)
	}
}
