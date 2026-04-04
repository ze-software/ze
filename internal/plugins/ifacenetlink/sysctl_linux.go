// Design: docs/features/interfaces.md -- Per-interface sysctl management
// Overview: ifacenetlink.go -- package hub

//go:build linux

package ifacenetlink

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	"codeberg.org/thomas-mangin/ze/internal/component/iface"
)

// sysctlRoot is the base path for sysctl writes. Tests override this to
// a temporary directory so that no real kernel tunables are modified.
var sysctlRoot = "/proc/sys"

func writeSysctl(path, value string) error {
	full := filepath.Join(sysctlRoot, path)
	if err := os.WriteFile(full, []byte(value), 0o600); err != nil {
		return fmt.Errorf("sysctl write %s=%s: %w", path, value, err)
	}
	return nil
}

func boolToSysctl(enabled bool) string {
	if enabled {
		return "1"
	}
	return "0"
}

func (b *netlinkBackend) SetIPv4Forwarding(ifaceName string, enabled bool) error {
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
		return err
	}
	return writeSysctl(fmt.Sprintf("net/ipv4/conf/%s/forwarding", ifaceName), boolToSysctl(enabled))
}

func (b *netlinkBackend) SetIPv4ArpFilter(ifaceName string, enabled bool) error {
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
		return err
	}
	return writeSysctl(fmt.Sprintf("net/ipv4/conf/%s/arp_filter", ifaceName), boolToSysctl(enabled))
}

func (b *netlinkBackend) SetIPv4ArpAccept(ifaceName string, enabled bool) error {
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
		return err
	}
	return writeSysctl(fmt.Sprintf("net/ipv4/conf/%s/arp_accept", ifaceName), boolToSysctl(enabled))
}

func (b *netlinkBackend) SetIPv4ProxyARP(ifaceName string, enabled bool) error {
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
		return err
	}
	return writeSysctl(fmt.Sprintf("net/ipv4/conf/%s/proxy_arp", ifaceName), boolToSysctl(enabled))
}

func (b *netlinkBackend) SetIPv4ArpAnnounce(ifaceName string, level int) error {
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
		return err
	}
	if level < 0 || level > 2 {
		return fmt.Errorf("iface: arp-announce level %d not in [0, 2]", level)
	}
	return writeSysctl(fmt.Sprintf("net/ipv4/conf/%s/arp_announce", ifaceName), strconv.Itoa(level))
}

func (b *netlinkBackend) SetIPv4ArpIgnore(ifaceName string, level int) error {
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
		return err
	}
	if level < 0 || level > 2 {
		return fmt.Errorf("iface: arp-ignore level %d not in [0, 2]", level)
	}
	return writeSysctl(fmt.Sprintf("net/ipv4/conf/%s/arp_ignore", ifaceName), strconv.Itoa(level))
}

func (b *netlinkBackend) SetIPv4RPFilter(ifaceName string, level int) error {
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
		return err
	}
	if level < 0 || level > 2 {
		return fmt.Errorf("iface: rp_filter level %d not in [0, 2]", level)
	}
	return writeSysctl(fmt.Sprintf("net/ipv4/conf/%s/rp_filter", ifaceName), strconv.Itoa(level))
}

func (b *netlinkBackend) SetIPv6Autoconf(ifaceName string, enabled bool) error {
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
		return err
	}
	return writeSysctl(fmt.Sprintf("net/ipv6/conf/%s/autoconf", ifaceName), boolToSysctl(enabled))
}

func (b *netlinkBackend) SetIPv6AcceptRA(ifaceName string, level int) error {
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
		return err
	}
	if level < 0 || level > 2 {
		return fmt.Errorf("iface: accept-ra level %d not in [0, 2]", level)
	}
	return writeSysctl(fmt.Sprintf("net/ipv6/conf/%s/accept_ra", ifaceName), strconv.Itoa(level))
}

func (b *netlinkBackend) SetIPv6Forwarding(ifaceName string, enabled bool) error {
	if err := iface.ValidateIfaceName(ifaceName); err != nil {
		return err
	}
	return writeSysctl(fmt.Sprintf("net/ipv6/conf/%s/forwarding", ifaceName), boolToSysctl(enabled))
}
