// Design: docs/features/interfaces.md -- DHCP client plugin
// Detail: dhcp_linux.go -- DHCPClient lifecycle and Start/Stop
// Detail: dhcp_v4_linux.go -- DHCPv4 DORA worker
// Detail: dhcp_v6_linux.go -- DHCPv6 SARR worker
// Detail: resolv_linux.go -- DNS resolv.conf writer

// Package ifacedhcp implements DHCPv4/DHCPv6 client functionality as a
// separate plugin. It publishes lease events on the Bus using topic
// constants from the iface component.
package ifacedhcp

import (
	"log/slog"
	"sync/atomic"
)

// loggerPtr is the package-level logger, disabled by default.
var loggerPtr atomic.Pointer[slog.Logger]

// DHCPConfig holds optional DHCP client parameters parsed from config.
// Defined in this platform-independent file so register.go (no build tag)
// can reference it when compiling on non-Linux (e.g., macOS build host).
type DHCPConfig struct {
	Hostname string // DHCPv4 option 12
	ClientID string // DHCPv4 option 61
	PDLength int    // DHCPv6 requested prefix delegation length (0 = server decides)
	DUID     string // DHCPv6 DUID override
}
