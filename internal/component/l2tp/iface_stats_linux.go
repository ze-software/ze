// Design: docs/architecture/l2tp/bng-1-radius-attributes.md -- idle timeout traffic detection
//go:build linux

package l2tp

import (
	"os"
	"strconv"
	"strings"
)

// readIfaceRXBytes returns the current RX byte counter for the named
// interface by reading /sys/class/net/<iface>/statistics/rx_bytes.
// Returns 0 on any error (interface gone, non-existent, permission).
func readIfaceRXBytes(iface string) uint64 {
	data, err := os.ReadFile("/sys/class/net/" + iface + "/statistics/rx_bytes") //nolint:gosec // fixed /sys/class/net base plus a kernel interface name; read-only sysfs
	if err != nil {
		return 0
	}
	v, err := strconv.ParseUint(strings.TrimSpace(string(data)), 10, 64)
	if err != nil {
		return 0
	}
	return v
}
