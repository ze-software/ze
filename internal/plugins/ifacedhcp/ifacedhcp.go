// Design: docs/features/interfaces.md -- DHCP client plugin

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
