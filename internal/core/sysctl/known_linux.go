//go:build linux

package sysctl

func init() {
	// System-wide keys.
	MustRegister(KeyDef{
		Name: "net.ipv4.conf.all.forwarding", Type: TypeBool,
		Description: "Enable IPv4 forwarding globally", Platform: PlatformLinux,
	})
	MustRegister(KeyDef{
		Name: "net.ipv6.conf.all.forwarding", Type: TypeBool,
		Description: "Enable IPv6 forwarding globally", Platform: PlatformLinux,
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
		Name: "net.ipv4.conf.<iface>.arp_filter", Type: TypeBool, Template: true,
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
}
