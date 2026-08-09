//go:build !linux

// Design: docs/architecture/ospf/ospf-3-ip-transport.md -- non-Linux raw-socket probe stub

package transport

func rawSocketAvailable() bool { return true }
