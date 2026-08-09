//go:build !linux

// Design: docs/architecture/ospf/ospfv3-3-ipv6-transport.md -- non-Linux raw IPv6 socket probe stub

package transport

// rawSocketAvailable reports the raw IPv6 socket as available on non-Linux
// platforms so config and unit tests do not flag a capability the daemon never
// uses there (the raw transport itself is Linux-only).
func rawSocketAvailable() bool { return true }
