package migration

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// TestIsIPv6Prefix verifies IPv6 prefix detection.
//
// VALIDATES: Prefixes containing ":" are detected as IPv6.
//
// PREVENTS: IPv6 routes being misclassified as IPv4.
func TestIsIPv6Prefix(t *testing.T) {
	tests := []struct {
		prefix string
		isIPv6 bool
	}{
		// IPv4 prefixes
		{"10.0.0.0/8", false},
		{"192.168.1.0/24", false},
		{"172.16.0.0/12", false},
		{"0.0.0.0/0", false},
		{"224.0.0.0/4", false},

		// IPv6 prefixes
		{"2001:db8::/32", true},
		{"::/0", true},
		{"fe80::/10", true},
		{"ff00::/8", true},
		{"::1/128", true},

		// IPv4-mapped IPv6 (treated as IPv6)
		{"::ffff:192.0.2.1/128", true},
		{"::ffff:10.0.0.0/104", true},
	}

	for _, tc := range tests {
		t.Run(tc.prefix, func(t *testing.T) {
			got := isIPv6Prefix(tc.prefix)
			require.Equal(t, tc.isIPv6, got, "isIPv6Prefix(%q)", tc.prefix)
		})
	}
}

// TestIsMulticastPrefix verifies multicast prefix detection.
//
// VALIDATES: IPv4 224.0.0.0/4 and IPv6 ff00::/8 detected as multicast.
//
// PREVENTS: Multicast routes being classified as unicast.
func TestIsMulticastPrefix(t *testing.T) {
	tests := []struct {
		prefix      string
		isMulticast bool
	}{
		// IPv4 unicast
		{"10.0.0.0/8", false},
		{"192.168.1.0/24", false},
		{"0.0.0.0/0", false},

		// IPv4 multicast (224.0.0.0/4)
		{"224.0.0.0/4", true},
		{"224.0.0.1/32", true},
		{"239.255.255.255/32", true},
		{"232.0.0.0/8", true},

		// IPv6 unicast
		{"2001:db8::/32", false},
		{"::/0", false},
		{"fe80::/10", false},

		// IPv6 multicast (ff00::/8)
		{"ff00::/8", true},
		{"ff02::1/128", true},
		{"ff05::1:3/128", true},

		// Edge cases
		{"223.255.255.255/32", false}, // Just below multicast range
		{"240.0.0.0/4", false},        // Reserved, not multicast
	}

	for _, tc := range tests {
		t.Run(tc.prefix, func(t *testing.T) {
			got := isMulticastPrefix(tc.prefix)
			require.Equal(t, tc.isMulticast, got, "isMulticastPrefix(%q)", tc.prefix)
		})
	}
}

// TestDetectSAFI verifies SAFI detection from route attributes.
//
// VALIDATES: SAFI correctly detected from prefix range and attributes.
//
// PREVENTS: Routes being placed in wrong AFI/SAFI container.
func TestDetectSAFI(t *testing.T) {
	tests := []struct {
		name     string
		prefix   string
		hasRD    bool
		hasLabel bool
		want     string
	}{
		// Unicast (default)
		{"ipv4/unicast", "10.0.0.0/8", false, false, "unicast"},
		{"ipv6/unicast", "2001:db8::/32", false, false, "unicast"},

		// Multicast (from prefix range)
		{"ipv4/multicast", "224.0.0.0/4", false, false, "multicast"},
		{"ipv6/multicast", "ff02::1/128", false, false, "multicast"},

		// MPLS-VPN (from rd attribute)
		{"ipv4/mpls-vpn with rd", "10.0.0.0/8", true, false, "mpls-vpn"},
		{"ipv6/mpls-vpn with rd", "2001:db8::/32", true, false, "mpls-vpn"},

		// MPLS-VPN (from label only)
		{"ipv4/mpls-vpn with label", "10.0.0.0/8", false, true, "mpls-vpn"},
		{"ipv6/mpls-vpn with label", "2001:db8::/32", false, true, "mpls-vpn"},

		// MPLS-VPN (both rd and label)
		{"ipv4/mpls-vpn with both", "10.0.0.0/8", true, true, "mpls-vpn"},

		// Multicast takes precedence over attributes (unusual but possible)
		{"multicast ignores rd", "224.0.0.0/4", true, false, "multicast"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := detectSAFI(tc.prefix, tc.hasRD, tc.hasLabel)
			require.Equal(t, tc.want, got)
		})
	}
}
