//go:build !linux

// Design: plan/spec-ospf-3-ip-transport.md -- non-Linux raw-socket probe stub

package transport

func rawSocketAvailable() bool { return true }
