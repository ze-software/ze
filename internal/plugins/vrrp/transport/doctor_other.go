//go:build !linux

// Design: docs/architecture/vrrp/vrrp-first-hop-redundancy.md -- raw-socket probe (non-Linux stub)
//
// The VRRP raw IP transport opens its raw socket on Linux alone:
// backend_other.go refuses to open one anywhere else. So off Linux the probe
// reports the transport unavailable and names the host's OS, and the doctor
// check warns instead of reporting healthy for a transport that cannot run.

package transport

import (
	"errors"
	"runtime"
)

// checkRawSocket reports why the VRRP raw transport cannot run on
// this host: it is Linux-only, and this host is not Linux.
func checkRawSocket() error {
	return errors.New("VRRP cannot run on this host, so this router can neither send nor hear " +
		"VRRP advertisements: its raw IP transport is Linux-only, and this host runs " +
		runtime.GOOS + ". Run ze on Linux to use VRRP")
}
