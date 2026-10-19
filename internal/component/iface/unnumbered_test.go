package iface

import (
	"net/netip"
	"strings"
	"testing"

	"github.com/ze-software/ze/pkg/plugin/sdk"
)

// unnumberedConfig builds an interface section with a dummy donor holding
// donorAddrs on unit 0 and an ethernet eth0 whose unit 0 carries unitV4.
func unnumberedConfig(donorAddrs, unitV4 string) string {
	return `{"interface": {
		"dummy": {"dummy0": {"unit": {"0": {"ipv4": {"address": [` + donorAddrs + `]}}}}},
		"ethernet": {"eth0": {"unit": {"0": {"ipv4": {` + unitV4 + `}}}}}
	}}`
}

// TestUnnumberedInstallsDonorSlash32 proves the approved shape end to end in
// the config layer: an unnumbered unit borrows the donor's first address, or
// the chosen source, and the apply path's desired state carries it as a /32
// on the unnumbered device.
func TestUnnumberedInstallsDonorSlash32(t *testing.T) {
	cases := []struct {
		name   string
		unitV4 string
		want   string
	}{
		{"first donor address", `"unnumbered": "dummy0"`, "10.255.0.1/32"},
		{"chosen source", `"unnumbered": "dummy0", "unnumbered-source": "10.255.0.2"`, "10.255.0.2/32"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg, err := parseIfaceConfig(unnumberedConfig(`"10.255.0.1/32", "10.255.0.2/32"`, tc.unitV4))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			addrs, _, _ := cfg.desiredState(map[string]string{"eth0": "eth0", "dummy0": "dummy0"})
			if !addrs["eth0"][tc.want] {
				t.Fatalf("eth0 desired addresses %v, want %s", addrs["eth0"], tc.want)
			}
			if len(addrs["eth0"]) != 1 {
				t.Fatalf("eth0 desired addresses %v, want only %s", addrs["eth0"], tc.want)
			}
		})
	}
}

// TestUnnumberedLoopbackDonor proves the loopback block answers to the donor
// name "loopback", the usual Cisco and Junos donor.
func TestUnnumberedLoopbackDonor(t *testing.T) {
	cfg, err := parseIfaceConfig(`{"interface": {
		"loopback": {"unit": {"0": {"ipv4": {"address": ["192.0.2.1/32"]}}}},
		"ethernet": {"eth0": {"unit": {"0": {"ipv4": {"unnumbered": "loopback"}}}}}
	}}`)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if got := cfg.Ethernet[0].Units[0].IPv4.UnnumberedAddress; got != netip.MustParseAddr("192.0.2.1") {
		t.Fatalf("borrowed %v, want 192.0.2.1", got)
	}
}

// TestUnnumberedCommitRefusals walks every refusal the owner approved: an
// own address, a missing, unnumbered or address-less donor, and a source the
// donor does not hold. Each one is a parse error, so verify refuses the commit.
func TestUnnumberedCommitRefusals(t *testing.T) {
	cases := []struct {
		name   string
		config string
		want   string
	}{
		{"with address", unnumberedConfig(`"10.255.0.1/32"`, `"unnumbered": "dummy0", "address": ["10.0.0.1/30"]`), "both set"},
		{"with dhcp", unnumberedConfig(`"10.255.0.1/32"`, `"unnumbered": "dummy0", "dhcp": {"enabled": "true"}`), "ipv4 dhcp"},
		{"missing donor", unnumberedConfig(`"10.255.0.1/32"`, `"unnumbered": "nosuch"`), "no interface of that name"},
		{"address-less donor", unnumberedConfig(``, `"unnumbered": "dummy0"`), "holds no IPv4 address"},
		{"source not on donor", unnumberedConfig(`"10.255.0.1/32"`, `"unnumbered": "dummy0", "unnumbered-source": "10.9.9.9"`), "does not hold this address"},
		{"source alone", unnumberedConfig(`"10.255.0.1/32"`, `"unnumbered-source": "10.255.0.1"`), "without ipv4 unnumbered"},
		{"unnumbered donor", `{"interface": {
			"dummy": {"dummy0": {"unit": {"0": {"ipv4": {"unnumbered": "eth1"}}}}},
			"ethernet": {
				"eth0": {"unit": {"0": {"ipv4": {"unnumbered": "dummy0"}}}},
				"eth1": {"unit": {"0": {"ipv4": {"address": ["10.1.1.1/32"]}}}}
			}
		}}`, "itself unnumbered"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := parseIfaceConfig(tc.config)
			if err == nil {
				t.Fatal("parse accepted the config, want a refusal")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("error %q, want it to contain %q", err, tc.want)
			}
		})
	}
}

// TestUnnumberedDevicesNamesOSDevice proves the exported view another
// component reads at verify names the unit's OS device and the borrowed address.
func TestUnnumberedDevicesNamesOSDevice(t *testing.T) {
	data := `{"interface": {
		"dummy": {"dummy0": {"unit": {"0": {"ipv4": {"address": ["10.255.0.1/32"]}}}}},
		"ethernet": {"eth0": {"unit": {"0": {"vlan-id": "7", "ipv4": {"unnumbered": "dummy0"}}}}}
	}}`
	got, err := UnnumberedDevices([]sdk.ConfigSection{{Root: configRootInterface, Data: data}})
	if err != nil {
		t.Fatalf("UnnumberedDevices: %v", err)
	}
	if len(got) != 1 || got["eth0.7"] != netip.MustParseAddr("10.255.0.1") {
		t.Fatalf("UnnumberedDevices = %v, want eth0.7 -> 10.255.0.1", got)
	}
}
