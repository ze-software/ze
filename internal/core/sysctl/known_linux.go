//go:build linux

// Design: docs/architecture/core-design.md -- Linux sysctl known-keys
// Overview: known.go -- registry types and functions

package sysctl

func init() {
	// System-wide keys.
	MustRegister(KeyDef{
		Name: "net.ipv4.conf.all.forwarding", Type: TypeBool,
		Description: "Enable IPv4 forwarding globally", Platform: PlatformLinux,
		// net/ipv4/devinet.c inet_forward_change writes the new value into
		// every device's forwarding and into default.forwarding. Linux
		// forwards IPv4 only on a device whose own forwarding is 1
		// (ip_route_input_slow reads IN_DEV_FORWARD), so a unit with ipv4
		// forwarding false is re-applied after the copy.
		Propagates: []string{"net.ipv4.conf.<iface>.forwarding"},
	})
	MustRegister(KeyDef{
		Name: "net.ipv6.conf.all.forwarding", Type: TypeBool,
		Description: "Enable IPv6 forwarding globally", Platform: PlatformLinux,
		// net/ipv6/addrconf.c addrconf_forward_change writes the new value
		// into every device's forwarding and into default.forwarding, and
		// zeroes every device's force_forwarding when the value is 0. The
		// copy stands on a device nothing in Ze manages: with all.forwarding
		// 1 the kernel forwards IPv6 arriving there (ip6_forward), so the
		// device is a router. A device whose forwarding a unit or a config
		// value sets is re-applied.
		Propagates: []string{
			"net.ipv6.conf.<iface>.forwarding",
			"net.ipv6.conf.<iface>.force_forwarding",
		},
	})
	MustRegister(KeyDef{
		Name: "net.ipv4.conf.all.rp_filter", Type: TypeIntRange, Min: 0, Max: 2,
		Description: "Reverse path filter mode (0=off, 1=strict, 2=loose)", Platform: PlatformLinux,
	})
	MustRegister(KeyDef{
		Name: "net.ipv4.tcp_syncookies", Type: TypeBool,
		Description: "Enable TCP SYN cookies", Platform: PlatformLinux,
	})
	MustRegister(KeyDef{
		Name: "net.core.somaxconn", Type: TypeInt,
		Description: "Maximum listen backlog queue length", Platform: PlatformLinux,
	})
	MustRegister(KeyDef{
		Name: "net.ipv4.conf.all.log_martians", Type: TypeBool,
		Description: "Log packets with impossible source addresses", Platform: PlatformLinux,
	})

	// Per-interface templates.
	MustRegister(KeyDef{
		Name: "net.ipv4.conf.<iface>.forwarding", Type: TypeBool, Template: true,
		Description: "Enable IPv4 forwarding on interface", Platform: PlatformLinux,
	})
	MustRegister(KeyDef{
		Name: keyIPv4ARPFilter, Type: TypeBool, Template: true,
		Description: "Enable ARP filter on interface", Platform: PlatformLinux,
	})
	MustRegister(KeyDef{
		Name: "net.ipv4.conf.<iface>.arp_accept", Type: TypeBool, Template: true,
		Description: "Accept gratuitous ARP on interface", Platform: PlatformLinux,
	})
	MustRegister(KeyDef{
		Name: "net.ipv4.conf.<iface>.proxy_arp", Type: TypeBool, Template: true,
		Description: "Enable proxy ARP on interface", Platform: PlatformLinux,
	})
	MustRegister(KeyDef{
		Name: "net.ipv4.conf.<iface>.arp_announce", Type: TypeIntRange, Min: 0, Max: 2, Template: true,
		Description: "ARP announce mode (0=any, 1=restrict, 2=best)", Platform: PlatformLinux,
	})
	MustRegister(KeyDef{
		Name: "net.ipv4.conf.<iface>.arp_ignore", Type: TypeIntRange, Min: 0, Max: 2, Template: true,
		Description: "ARP ignore mode (0=any, 1=local, 2=scope)", Platform: PlatformLinux,
	})
	MustRegister(KeyDef{
		Name: "net.ipv4.conf.<iface>.rp_filter", Type: TypeIntRange, Min: 0, Max: 2, Template: true,
		Description: "Per-interface reverse path filter", Platform: PlatformLinux,
	})
	MustRegister(KeyDef{
		Name: "net.ipv4.conf.<iface>.log_martians", Type: TypeBool, Template: true,
		Description: "Log packets with impossible source addresses", Platform: PlatformLinux,
	})
	MustRegister(KeyDef{
		Name: "net.ipv6.conf.<iface>.autoconf", Type: TypeBool, Template: true,
		Description: "Enable IPv6 SLAAC on interface", Platform: PlatformLinux,
	})
	MustRegister(KeyDef{
		Name: "net.ipv6.conf.<iface>.accept_ra", Type: TypeIntRange, Min: 0, Max: 2, Template: true,
		Description: "Accept Router Advertisements (0=off, 1=if-forwarding-off, 2=always)", Platform: PlatformLinux,
	})
	MustRegister(KeyDef{
		Name: "net.ipv6.conf.<iface>.forwarding", Type: TypeBool, Template: true,
		Description: "Enable IPv6 forwarding on interface", Platform: PlatformLinux,
	})

	// Path MTU aging. A PMTU learned from an ICMP Fragmentation Needed or Packet
	// Too Big expires after this many seconds and the route's MTU returns to the
	// device MTU (__ip_rt_update_pmtu in net/ipv4/route.c, rt6_do_update_pmtu in
	// net/ipv6/route.c). Each value is per network namespace. The PMTU of an
	// IPsec tunnel-mode SA lives on its outer route, so these keys are the period
	// RFC 4301 Section 8.2.2 asks to be configurable: "The period SHOULD be
	// configurable." The kernel converts seconds to jiffies (proc_dointvec_jiffies)
	// and refuses a value over INT_MAX/HZ, so the maximum holds for any HZ up to
	// 1000. The minimum is 1 because RFC 4301 Section 8.2.2 requires that "A given
	// PMTU has to remain in place long enough for a packet to get from the source
	// of the SA to the peer, and to propagate an ICMP error message if the current
	// PMTU is too big.", and 0 or less expires a PMTU the moment it is learned.
	MustRegister(KeyDef{
		Name: "net.ipv4.route.mtu_expires", Type: TypeIntRange, Min: 1, Max: 2147483,
		Description: "Seconds a learned IPv4 path MTU is kept before it returns to the link MTU", Platform: PlatformLinux,
	})
	MustRegister(KeyDef{
		Name: "net.ipv6.route.mtu_expires", Type: TypeIntRange, Min: 1, Max: 2147483,
		Description: "Seconds a learned IPv6 path MTU is kept before it returns to the link MTU", Platform: PlatformLinux,
	})

	// MPLS sysctl keys (RFC 3031, RFC 3032).
	MustRegister(KeyDef{
		Name: "net.mpls.platform_labels", Type: TypeIntRange, Min: 0, Max: 1048575,
		Description: "Maximum MPLS label value supported by the platform", Platform: PlatformLinux,
	})
	MustRegister(KeyDef{
		Name: "net.mpls.ip_ttl_propagate", Type: TypeBool,
		Description: "Propagate IP TTL into MPLS header", Platform: PlatformLinux,
	})
	MustRegister(KeyDef{
		Name: "net.mpls.default_ttl", Type: TypeIntRange, Min: 1, Max: 255,
		Description: "Default TTL for MPLS-encapsulated packets", Platform: PlatformLinux,
	})
	MustRegister(KeyDef{
		Name: "net.mpls.conf.<iface>.input", Type: TypeBool, Template: true,
		Description: "Enable MPLS input on interface", Platform: PlatformLinux,
	})
}
