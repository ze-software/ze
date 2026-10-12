//go:build !linux

// Design: docs/architecture/ospf/ospf-3-ip-transport.md -- raw-socket probe (non-Linux stub)
//
// The OSPF raw IP transport opens its raw socket on Linux alone:
// backend_other.go refuses to open one anywhere else. So off Linux the probe
// reports the transport unavailable and names the host's OS, and the doctor
// check warns instead of reporting healthy for a transport that cannot run.

package transport

import (
	"errors"
	"runtime"
)

// checkRawSocket reports why the OSPF raw transport cannot run on
// this host: it is Linux-only, and this host is not Linux.
func checkRawSocket() error {
	return errors.New("OSPF cannot run on this host, so it forms no adjacencies here: " +
		"its raw IP transport is Linux-only, and this host runs " + runtime.GOOS +
		". Run ze on Linux to use OSPF")
}
