//go:build !linux

// Design: plan/learned/957-ospf-3-ip-transport.md -- non-Linux raw-socket probe stub

package transport

func rawSocketAvailable() bool { return true }
