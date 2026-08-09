//go:build !linux

// Design: docs/architecture/isis/isis-3-l2-transport.md -- raw-socket probe (non-Linux stub)
//
// The IS-IS raw L2 transport only opens an AF_PACKET socket on Linux, so off
// Linux there is no raw-socket dependency to warn about; the probe reports
// available so the doctor check stays quiet.

package transport

// rawSocketAvailable is a stub on non-Linux: there is no raw-socket dependency.
func rawSocketAvailable() bool { return true }
