// Design: plan/spec-iface-0-umbrella.md — Per-interface sysctl management
// Overview: iface.go — shared types and topic constants

package iface

import (
	"fmt"
	"os"
	"path/filepath"
)

// sysctlRoot is the base path for sysctl writes. Tests override this to
// a temporary directory so that no real kernel tunables are modified.
var sysctlRoot = "/proc/sys"

// writeSysctl writes a value to a sysctl path.
// path is relative to sysctlRoot (e.g., "net/ipv4/conf/eth0/forwarding").
func writeSysctl(path, value string) error {
	full := filepath.Join(sysctlRoot, path)
	if err := os.WriteFile(full, []byte(value), 0o600); err != nil {
		return fmt.Errorf("sysctl write %s=%s: %w", path, value, err)
	}
	return nil
}

// boolToSysctl returns "1" for true and "0" for false.
func boolToSysctl(enabled bool) string {
	if enabled {
		return "1"
	}
	return "0"
}

// SetIPv4Forwarding enables or disables IPv4 forwarding on an interface.
func SetIPv4Forwarding(ifaceName string, enabled bool) error {
	if err := validateIfaceName(ifaceName); err != nil {
		return err
	}
	path := fmt.Sprintf("net/ipv4/conf/%s/forwarding", ifaceName)
	return writeSysctl(path, boolToSysctl(enabled))
}

// SetIPv4ArpFilter enables or disables ARP filtering on an interface.
func SetIPv4ArpFilter(ifaceName string, enabled bool) error {
	if err := validateIfaceName(ifaceName); err != nil {
		return err
	}
	path := fmt.Sprintf("net/ipv4/conf/%s/arp_filter", ifaceName)
	return writeSysctl(path, boolToSysctl(enabled))
}

// SetIPv4ArpAccept enables or disables gratuitous ARP acceptance on an interface.
func SetIPv4ArpAccept(ifaceName string, enabled bool) error {
	if err := validateIfaceName(ifaceName); err != nil {
		return err
	}
	path := fmt.Sprintf("net/ipv4/conf/%s/arp_accept", ifaceName)
	return writeSysctl(path, boolToSysctl(enabled))
}

// SetIPv6Autoconf enables or disables SLAAC on an interface.
func SetIPv6Autoconf(ifaceName string, enabled bool) error {
	if err := validateIfaceName(ifaceName); err != nil {
		return err
	}
	path := fmt.Sprintf("net/ipv6/conf/%s/autoconf", ifaceName)
	return writeSysctl(path, boolToSysctl(enabled))
}

// SetIPv6AcceptRA configures RA acceptance on an interface.
// When forwardingEnabled is true, accept_ra must be set to 2 (not 1) to
// still accept Router Advertisements. The kernel ignores accept_ra=1 when
// forwarding is active on the interface.
func SetIPv6AcceptRA(ifaceName string, enabled, forwardingEnabled bool) error {
	if err := validateIfaceName(ifaceName); err != nil {
		return err
	}
	path := fmt.Sprintf("net/ipv6/conf/%s/accept_ra", ifaceName)
	if !enabled {
		return writeSysctl(path, "0")
	}
	if forwardingEnabled {
		return writeSysctl(path, "2")
	}
	return writeSysctl(path, "1")
}

// SetIPv6Forwarding enables or disables IPv6 forwarding on an interface.
func SetIPv6Forwarding(ifaceName string, enabled bool) error {
	if err := validateIfaceName(ifaceName); err != nil {
		return err
	}
	path := fmt.Sprintf("net/ipv6/conf/%s/forwarding", ifaceName)
	return writeSysctl(path, boolToSysctl(enabled))
}
