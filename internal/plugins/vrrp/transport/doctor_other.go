//go:build !linux

// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- raw-socket probe (non-Linux stub)
//
// The VRRP raw transport only opens raw sockets on Linux, so off Linux there is
// no raw-socket dependency to warn about; the probe reports available so the
// doctor check stays quiet.

package transport

// rawSocketAvailable is a stub on non-Linux: there is no raw-socket dependency.
func rawSocketAvailable() bool { return true }
