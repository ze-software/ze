// Design: docs/features/interfaces.md — IPv6 SLAAC control
// Overview: iface.go — shared types and topic constants

package iface

// EnableSLAAC enables IPv6 stateless autoconfiguration on an interface.
// Sets net.ipv6.conf.<iface>.autoconf = 1.
// Kernel SLAAC addresses are detected by the monitor and published as
// interface/addr/added events.
func EnableSLAAC(ifaceName string) error {
	return SetIPv6Autoconf(ifaceName, true)
}

// DisableSLAAC disables IPv6 stateless autoconfiguration on an interface.
// Sets net.ipv6.conf.<iface>.autoconf = 0.
func DisableSLAAC(ifaceName string) error {
	return SetIPv6Autoconf(ifaceName, false)
}
