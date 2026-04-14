// Design: docs/features/interfaces.md -- DHCP client plugin logger

//go:build linux

package ifacedhcp

import (
	"log/slog"
	"sync/atomic"
)

// loggerPtr is the package-level logger, disabled by default.
// Linux-only because all consumers (dhcp_linux.go, register.go, etc.) are linux-only.
var loggerPtr atomic.Pointer[slog.Logger]
