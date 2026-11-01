// Design: docs/architecture/testing/interop.md -- Container Addresses
// Related: prepare.go -- needsIPv6, zeRunsOSPFv3
package bgp

import (
	"path/filepath"
	"testing"
)

// VALIDATES: a Ze config that runs OSPFv3 selects an IPv6 lab network in
// every spelling the parser accepts: `address-family ipv6 { ... }` and the
// nested `address-family { ipv6 { ... } }`, and every family the
// `address-family` container holds, because each one is an OSPFv3 instance
// (RFC 5838 carries IPv4 over OSPFv3 too), whose packets need an IPv6
// link-local on eth0.
// Method: needsIPv6 over fixtures in each spelling, over configs that run no
// OSPFv3, and over the repository scenarios written in the nested form.
// PREVENTS: an OSPFv3 scenario starting on an IPv4-only Docker network, where
// Docker disables IPv6 on eth0 and Ze logs "no usable link-local source"
// (nightly 41298258262: ospfv3-gr-frr, ospfv3-gr-fib-retention).
func TestOSPFv3ConfigSelectsIPv6Network(t *testing.T) {
	for _, scenario := range []struct {
		name     string
		zeConfig string
		want     bool
	}{
		{"flat-ipv6", "ospf {\n\taddress-family ipv6 {\n\t\tareas { }\n\t}\n}\n", true},
		{"nested-ipv6", "ospf {\n\taddress-family {\n\t\tipv6 {\n\t\t\tareas { }\n\t\t}\n\t}\n}\n", true},
		{"nested-ipv4-unicast", "ospf {\n\taddress-family {\n\t\tipv4-unicast {\n\t\t\tinstance-id 64\n\t\t}\n\t}\n}\n", true},
		{"nested-second-family", "ospf {\n\taddress-family {\n\t\tipv4-unicast { instance-id 64 }\n\t\tipv6-unicast { instance-id 0 }\n\t}\n}\n", true},
		{"ospfv2-only", "ospf {\n\trouter-id 172.30.0.2\n\tareas { area 0.0.0.0 { area-type normal } }\n}\n", false},
		{"bgp-address-family", "bgp {\n\tpeer p {\n\t\taddress-family { ipv4 { } }\n\t}\n}\n", false},
		{"comment-only", "# ospf address-family ipv6 is not configured here.\nbgp {\n}\n", false},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			source := t.TempDir()
			writeFixture(t, filepath.Join(source, zeConfigFile), scenario.zeConfig)
			got, err := needsIPv6(source)
			if err != nil {
				t.Fatal(err)
			}
			if got != scenario.want {
				t.Fatalf("needsIPv6 = %v, want %v for:\n%s", got, scenario.want, scenario.zeConfig)
			}
		})
	}
	for _, name := range []string{"ospfv3-gr-frr", "ospfv3-gr-fib-retention", "ospf-multiaf-frr", "ospf-multiaf-v4-frr"} {
		t.Run(name, func(t *testing.T) {
			source := filepath.Join("..", "..", "..", "..", "test", "interop", "scenarios", name)
			got, err := needsIPv6(source)
			if err != nil {
				t.Fatal(err)
			}
			if !got {
				t.Fatalf("scenario %s runs OSPFv3 and does not select an IPv6 lab network", name)
			}
		})
	}
}
