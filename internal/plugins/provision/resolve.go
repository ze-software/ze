// Design: docs/architecture/cli/plugin-modes.md -- provision interface auto-config
// Related: iface_linux.go -- ensureAddress/removeAddress

package provision

import (
	"fmt"
	"log/slog"
	"net/netip"
)

func resolveOrConfigureIP(ifaceName, override, network string) (serverIP, addedCIDR string, err error) {
	serverIP, err = resolveServerIP(ifaceName, override)
	if err == nil {
		return serverIP, "", nil
	}

	prefix, parseErr := netip.ParsePrefix(network)
	if parseErr != nil {
		return "", "", err
	}

	serverIP, addedCIDR = serverAddrFromPrefix(prefix)

	if cfgErr := ensureAddress(ifaceName, addedCIDR); cfgErr != nil { //nolint:staticcheck // SA4023 holds only on darwin: the !linux stub always errors, the linux implementation can return nil
		return "", "", fmt.Errorf("configure %s on %s: %w", addedCIDR, ifaceName, cfgErr)
	}

	slog.Info("provision: configured interface", "cidr", addedCIDR, "interface", ifaceName)

	return serverIP, addedCIDR, nil
}

func serverAddrFromPrefix(prefix netip.Prefix) (serverIP, cidr string) {
	addr := prefix.Addr()
	if addr == prefix.Masked().Addr() {
		addr = addr.Next()
	}
	return addr.String(), netip.PrefixFrom(addr, prefix.Bits()).String()
}

func cleanupAddress(ifaceName, cidr string) {
	if err := removeAddress(ifaceName, cidr); err != nil { //nolint:staticcheck // SA4023 holds only on darwin: the !linux stub always errors, the linux implementation can return nil
		slog.Warn("provision: remove address", "cidr", cidr, "interface", ifaceName, "error", err)
	} else {
		slog.Info("provision: removed address", "cidr", cidr, "interface", ifaceName)
	}
}
