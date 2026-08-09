// Design: docs/architecture/firewall/fw-6-firewall-vpp.md -- Linux-only logger accessor

//go:build linux

package firewallvpp

import "log/slog"

func logger() *slog.Logger { return loggerPtr.Load() }
