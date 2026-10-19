package ospf

import (
	"strings"
	"testing"

	"github.com/ze-software/ze/internal/plugins/ospf/types"
)

// TestHelloNetworkMaskUnnumbered proves an unnumbered point-to-point Hello
// carries 0.0.0.0 (RFC 2328 Section 9.5) and every other shape keeps its mask.
func TestHelloNetworkMaskUnnumbered(t *testing.T) {
	host := [4]byte{255, 255, 255, 255}
	subnet := [4]byte{255, 255, 255, 252}
	cases := []struct {
		network networkType
		mask    [4]byte
		want    [4]byte
	}{
		{types.NetworkPointToPoint, host, [4]byte{}},
		{types.NetworkPointToPoint, subnet, subnet},
		{types.NetworkBroadcast, host, host},
	}
	for _, tc := range cases {
		if got := helloNetworkMask(tc.network, tc.mask); got != tc.want {
			t.Errorf("helloNetworkMask(%s, %v) = %v, want %v", tc.network, tc.mask, got, tc.want)
		}
	}
}

// TestUnnumberedNetworkTypeRefusedAtCommit proves the OSPF verifier, reading
// the candidate interface section, refuses an OSPF interface on an unnumbered
// unit unless it is point-to-point, and accepts it as point-to-point.
func TestUnnumberedNetworkTypeRefusedAtCommit(t *testing.T) {
	iface := configSection{Root: "interface", Data: `{"interface": {
		"loopback": {"unit": {"0": {"ipv4": {"address": ["192.0.2.1/32"]}}}},
		"ethernet": {"eth0": {"unit": {"0": {"ipv4": {"unnumbered": "loopback"}}}}}
	}}`}
	ospf := func(network string) configSection {
		return configSection{Root: "ospf", Data: `{"ospf":{"router-id":"10.0.0.1","areas":{"area":{"0":{"area-id":"0"}}},` +
			`"interfaces":{"interface":{"eth0":{"area":"0"` + network + `}}}}}`}
	}
	for _, tc := range []struct {
		name    string
		network string
		refused bool
	}{
		{"default broadcast", ``, true},
		{"nbma", `,"network-type":"nbma"`, true},
		{"point-to-point", `,"network-type":"point-to-point"`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			sections := []configSection{ospf(tc.network), iface}
			cfg, err := parseOSPFConfig(sections, nil)
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			err = checkUnnumberedNetworkTypes(cfg, sections)
			if !tc.refused {
				if err != nil {
					t.Fatalf("refused point-to-point: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), "unnumbered") {
				t.Fatalf("err = %v, want the unnumbered refusal", err)
			}
		})
	}
}
