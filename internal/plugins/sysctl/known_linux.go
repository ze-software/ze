//go:build linux

package sysctl

import sysctlreg "codeberg.org/thomas-mangin/ze/internal/core/sysctl"

func init() {
	// System-wide keys.
	sysctlreg.MustRegister(sysctlreg.KeyDef{
		Name: "net.ipv4.conf.all.forwarding", Type: sysctlreg.TypeBool,
		Description: "Enable IPv4 forwarding globally", Platform: sysctlreg.PlatformLinux,
	})
	sysctlreg.MustRegister(sysctlreg.KeyDef{
		Name: "net.ipv6.conf.all.forwarding", Type: sysctlreg.TypeBool,
		Description: "Enable IPv6 forwarding globally", Platform: sysctlreg.PlatformLinux,
	})
	sysctlreg.MustRegister(sysctlreg.KeyDef{
		Name: "net.ipv4.conf.all.rp_filter", Type: sysctlreg.TypeIntRange, Min: 0, Max: 2,
		Description: "Reverse path filter mode (0=off, 1=strict, 2=loose)", Platform: sysctlreg.PlatformLinux,
	})
	sysctlreg.MustRegister(sysctlreg.KeyDef{
		Name: "net.ipv4.tcp_syncookies", Type: sysctlreg.TypeBool,
		Description: "Enable TCP SYN cookies", Platform: sysctlreg.PlatformLinux,
	})
	sysctlreg.MustRegister(sysctlreg.KeyDef{
		Name: "net.core.somaxconn", Type: sysctlreg.TypeInt,
		Description: "Maximum listen backlog queue length", Platform: sysctlreg.PlatformLinux,
	})
	sysctlreg.MustRegister(sysctlreg.KeyDef{
		Name: "net.ipv4.conf.all.log_martians", Type: sysctlreg.TypeBool,
		Description: "Log packets with impossible source addresses", Platform: sysctlreg.PlatformLinux,
	})

	// Per-interface templates.
	sysctlreg.MustRegister(sysctlreg.KeyDef{
		Name: "net.ipv4.conf.<iface>.forwarding", Type: sysctlreg.TypeBool, Template: true,
		Description: "Enable IPv4 forwarding on interface", Platform: sysctlreg.PlatformLinux,
	})
	sysctlreg.MustRegister(sysctlreg.KeyDef{
		Name: "net.ipv4.conf.<iface>.arp_filter", Type: sysctlreg.TypeBool, Template: true,
		Description: "Enable ARP filter on interface", Platform: sysctlreg.PlatformLinux,
	})
	sysctlreg.MustRegister(sysctlreg.KeyDef{
		Name: "net.ipv4.conf.<iface>.arp_accept", Type: sysctlreg.TypeBool, Template: true,
		Description: "Accept gratuitous ARP on interface", Platform: sysctlreg.PlatformLinux,
	})
	sysctlreg.MustRegister(sysctlreg.KeyDef{
		Name: "net.ipv4.conf.<iface>.proxy_arp", Type: sysctlreg.TypeBool, Template: true,
		Description: "Enable proxy ARP on interface", Platform: sysctlreg.PlatformLinux,
	})
	sysctlreg.MustRegister(sysctlreg.KeyDef{
		Name: "net.ipv4.conf.<iface>.arp_announce", Type: sysctlreg.TypeIntRange, Min: 0, Max: 2, Template: true,
		Description: "ARP announce mode (0=any, 1=restrict, 2=best)", Platform: sysctlreg.PlatformLinux,
	})
	sysctlreg.MustRegister(sysctlreg.KeyDef{
		Name: "net.ipv4.conf.<iface>.arp_ignore", Type: sysctlreg.TypeIntRange, Min: 0, Max: 2, Template: true,
		Description: "ARP ignore mode (0=any, 1=local, 2=scope)", Platform: sysctlreg.PlatformLinux,
	})
	sysctlreg.MustRegister(sysctlreg.KeyDef{
		Name: "net.ipv4.conf.<iface>.rp_filter", Type: sysctlreg.TypeIntRange, Min: 0, Max: 2, Template: true,
		Description: "Per-interface reverse path filter", Platform: sysctlreg.PlatformLinux,
	})
	sysctlreg.MustRegister(sysctlreg.KeyDef{
		Name: "net.ipv6.conf.<iface>.autoconf", Type: sysctlreg.TypeBool, Template: true,
		Description: "Enable IPv6 SLAAC on interface", Platform: sysctlreg.PlatformLinux,
	})
	sysctlreg.MustRegister(sysctlreg.KeyDef{
		Name: "net.ipv6.conf.<iface>.accept_ra", Type: sysctlreg.TypeIntRange, Min: 0, Max: 2, Template: true,
		Description: "Accept Router Advertisements (0=off, 1=if-forwarding-off, 2=always)", Platform: sysctlreg.PlatformLinux,
	})
	sysctlreg.MustRegister(sysctlreg.KeyDef{
		Name: "net.ipv6.conf.<iface>.forwarding", Type: sysctlreg.TypeBool, Template: true,
		Description: "Enable IPv6 forwarding on interface", Platform: sysctlreg.PlatformLinux,
	})
}
