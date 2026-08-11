// Design: docs/features/interfaces.md -- Netlink interface backend
// Detail: backend_linux.go -- the netlinkBackend type every method hangs off

// Package ifacenetlink implements the netlink-based interface management
// backend for Linux. It registers itself with the iface component as the
// "netlink" backend via iface.RegisterBackend.
//
// On non-Linux platforms, stub implementations return "not supported" errors.
package ifacenetlink

import (
	"github.com/ze-software/ze/internal/core/slogutil"
)

// logger returns the package logger. This package is a backend registered
// through iface.RegisterBackend (register.go), not a plugin with its own
// registry.Registration, so no ConfigureEngineLogger callback reaches it: it
// names its own subsystem, as internal/plugins/traffic/vpp names "traffic.vpp"
// and internal/component/firewall/plugins/irr names "firewall.irr".
//
// The subsystem is "iface.netlink". CanonicalSubsystemName
// (internal/component/plugin/inprocess.go) turns the iface-dhcp and iface-ra
// plugin names into "iface.dhcp" and "iface.ra", and getLogEnv walks a dotted
// subsystem from the most specific key down, so ze.log.iface covers all three.
//
// slogutil.LazyLogger defers creation to the first log call, so a level the
// config file sets, which main applies after package init, still reaches it.
// A logger built in init() reads the environment before that and keeps the
// level it read there for the life of the process.
var logger = slogutil.LazyLogger("iface.netlink")
