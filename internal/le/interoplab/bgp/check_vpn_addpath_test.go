// Related: check_vpn_addpath.go -- the FRR predicates these cases pin.
package bgp

import "testing"

// TestRequireVPNAddPathRoute pins the bgp-vpn-addpath-frr table predicate. It
// feeds FRR "rd all" table shapes and checks that only the exact RD, prefix and
// sending neighbor, as the one route in the table, pass. A route that FRR
// decoded four octets out of place lands under another RD or prefix, so those
// shapes must fail.
func TestRequireVPNAddPathRoute(t *testing.T) {
	const neighbor = "172.30.0.2"
	cases := []struct {
		name   string
		output string
		pass   bool
	}{
		{"exact route", `{"totalRoutes":1,"routes":{"routeDistinguishers":{"65001:100":{"10.99.0.0/24":[{"peerId":"172.30.0.2"}]}}}}`, true},
		{"other RD", `{"totalRoutes":1,"routes":{"routeDistinguishers":{"0:1000":{"10.99.0.0/24":[{"peerId":"172.30.0.2"}]}}}}`, false},
		{"other prefix", `{"totalRoutes":1,"routes":{"routeDistinguishers":{"65001:100":{"10.99.0.0/16":[{"peerId":"172.30.0.2"}]}}}}`, false},
		{"other neighbor", `{"totalRoutes":1,"routes":{"routeDistinguishers":{"65001:100":{"10.99.0.0/24":[{"peerId":"172.30.0.9"}]}}}}`, false},
		{"extra route", `{"totalRoutes":2,"routes":{"routeDistinguishers":{"65001:100":{"10.99.0.0/24":[{"peerId":"172.30.0.2"}]}}}}`, false},
		{"empty table", `{}`, false},
		{"not JSON", `% Unknown command`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := requireVPNAddPathRoute(tc.output, neighbor)
			if tc.pass && err != nil {
				t.Fatalf("want pass, got %v", err)
			}
			if !tc.pass && err == nil {
				t.Fatal("want failure, got pass")
			}
		})
	}
}

// TestRequireVPNAddPathReceive pins the negotiation predicate: only an ipv4Vpn
// entry whose receive side FRR advertised and received passes.
func TestRequireVPNAddPathReceive(t *testing.T) {
	cases := []struct {
		name   string
		output string
		pass   bool
	}{
		{"negotiated", `{"172.30.0.2":{"neighborCapabilities":{"addPath":{"ipv4Vpn":{"rxAdvertisedAndReceived":true}}}}}`, true},
		{"advertised only", `{"172.30.0.2":{"neighborCapabilities":{"addPath":{"ipv4Vpn":{"rxAdvertised":true,"rxAdvertisedAndReceived":false}}}}}`, false},
		{"unicast only", `{"172.30.0.2":{"neighborCapabilities":{"addPath":{"ipv4Unicast":{"rxAdvertisedAndReceived":true}}}}}`, false},
		{"no add-path", `{"172.30.0.2":{"neighborCapabilities":{}}}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := requireVPNAddPathReceive(tc.output)
			if tc.pass && err != nil {
				t.Fatalf("want pass, got %v", err)
			}
			if !tc.pass && err == nil {
				t.Fatal("want failure, got pass")
			}
		})
	}
}
