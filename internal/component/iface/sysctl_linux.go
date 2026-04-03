// Design: plan/spec-iface-0-umbrella.md — Per-interface sysctl management
// Overview: iface.go — shared types and topic constants

package iface

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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

// SetIPv6AcceptRA sets the accept_ra level on an interface.
// Level 0: disable. Level 1: accept if not forwarding. Level 2: accept even
// if forwarding (required when IPv6 forwarding is active).
func SetIPv6AcceptRA(ifaceName string, level int) error {
	if err := validateIfaceName(ifaceName); err != nil {
		return err
	}
	if level < 0 || level > 2 {
		return fmt.Errorf("iface: accept-ra level %d not in [0, 2]", level)
	}
	path := fmt.Sprintf("net/ipv6/conf/%s/accept_ra", ifaceName)
	return writeSysctl(path, strconv.Itoa(level))
}

// SetIPv4ProxyARP enables or disables proxy ARP on an interface.
// When enabled, the interface responds to ARP requests for addresses on
// other interfaces, acting as an ARP proxy.
func SetIPv4ProxyARP(ifaceName string, enabled bool) error {
	if err := validateIfaceName(ifaceName); err != nil {
		return err
	}
	path := fmt.Sprintf("net/ipv4/conf/%s/proxy_arp", ifaceName)
	return writeSysctl(path, boolToSysctl(enabled))
}

// SetIPv4ArpAnnounce sets the ARP announce level on an interface.
// Level 0: any local address. Level 1: prefer address matching target's subnet.
// Level 2: use best local address for target's subnet only.
func SetIPv4ArpAnnounce(ifaceName string, level int) error {
	if err := validateIfaceName(ifaceName); err != nil {
		return err
	}
	if level < 0 || level > 2 {
		return fmt.Errorf("iface: arp-announce level %d not in [0, 2]", level)
	}
	path := fmt.Sprintf("net/ipv4/conf/%s/arp_announce", ifaceName)
	return writeSysctl(path, strconv.Itoa(level))
}

// SetIPv4ArpIgnore sets the ARP ignore level on an interface.
// Level 0: reply to any local address. Level 1: reply only if target is
// configured on the incoming interface. Level 2: same as 1 plus check source.
func SetIPv4ArpIgnore(ifaceName string, level int) error {
	if err := validateIfaceName(ifaceName); err != nil {
		return err
	}
	if level < 0 || level > 2 {
		return fmt.Errorf("iface: arp-ignore level %d not in [0, 2]", level)
	}
	path := fmt.Sprintf("net/ipv4/conf/%s/arp_ignore", ifaceName)
	return writeSysctl(path, strconv.Itoa(level))
}

// SetIPv4RPFilter sets reverse path filtering on an interface.
// Level 0: disabled. Level 1: strict (source must be reachable via same iface).
// Level 2: loose (source must be reachable via any iface).
func SetIPv4RPFilter(ifaceName string, level int) error {
	if err := validateIfaceName(ifaceName); err != nil {
		return err
	}
	if level < 0 || level > 2 {
		return fmt.Errorf("iface: rp_filter level %d not in [0, 2]", level)
	}
	path := fmt.Sprintf("net/ipv4/conf/%s/rp_filter", ifaceName)
	return writeSysctl(path, strconv.Itoa(level))
}

// SetIPv6Forwarding enables or disables IPv6 forwarding on an interface.
func SetIPv6Forwarding(ifaceName string, enabled bool) error {
	if err := validateIfaceName(ifaceName); err != nil {
		return err
	}
	path := fmt.Sprintf("net/ipv6/conf/%s/forwarding", ifaceName)
	return writeSysctl(path, boolToSysctl(enabled))
}
