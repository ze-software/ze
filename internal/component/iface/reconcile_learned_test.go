// Design: docs/features/interfaces.md -- what the address reconcile removes.
// Related: config_apply.go -- learnedAddrSet, addrLearned.

package iface

import "testing"

// TestLearnedAddrSet feeds learnedAddrSet one listing holding every kind of
// address a configured unit can carry, and checks which ones it spares.
//
// VALIDATES: a lease or SLAAC address (finite valid lifetime) and the kernel's
// IPv6 link-local are spared; a link-local the replaced config named, a
// permanent global address, and an IPv4 169.254 address are not.
// PREVENTS: the "remove extra addresses" pass deleting a DHCP lease, SLAAC, or
// fe80:: from a unit, or keeping an address the operator dropped from config.
// The kernel round trip is TestIntegrationReconcileKeepsLearnedAddresses.
func TestLearnedAddrSet(t *testing.T) {
	infos := []InterfaceInfo{{
		Name: "eth0",
		Addresses: []AddrInfo{
			{Address: "10.0.0.1", PrefixLength: 24, Family: "ipv4"},
			{Address: "10.0.1.5", PrefixLength: 24, Family: "ipv4", ValidLifetime: 3600},
			{Address: "2001:db8::5", PrefixLength: 64, Family: "ipv6", ValidLifetime: 3600},
			{Address: "2001:db8::1", PrefixLength: 64, Family: "ipv6"},
			{Address: "fe80::1c00:ff:fe00:1", PrefixLength: 64, Family: "ipv6"},
			{Address: "fe80::99", PrefixLength: 64, Family: "ipv6"},
			{Address: "169.254.7.7", PrefixLength: 16, Family: "ipv4"},
		},
	}}
	previous := map[string]map[string]bool{"eth0": {"fe80::99/64": true}}

	got := learnedAddrSet(infos, previous)["eth0"]
	want := map[string]bool{
		"10.0.0.1/24":             false,
		"10.0.1.5/24":             true,
		"2001:db8::5/64":          true,
		"2001:db8::1/64":          false,
		"fe80::1c00:ff:fe00:1/64": true,
		"fe80::99/64":             false,
		"169.254.7.7/16":          false,
	}
	for cidr, learned := range want {
		if got[cidr] != learned {
			t.Errorf("learnedAddrSet[%s] = %v, want %v", cidr, got[cidr], learned)
		}
	}
	if len(got) != 3 {
		t.Errorf("learnedAddrSet holds %d addresses, want 3: %v", len(got), got)
	}

	// With no replaced config (startup, vpp connect), the link-local the old
	// config named is indistinguishable from the kernel's and is spared.
	if !learnedAddrSet(infos, nil)["eth0"]["fe80::99/64"] {
		t.Error("with no previous config, fe80::99/64 must be spared")
	}
}
