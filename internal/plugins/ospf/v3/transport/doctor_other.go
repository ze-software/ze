//go:build !linux

// Design: docs/architecture/ospf/ospfv3-3-ipv6-transport.md -- raw-socket probe (non-Linux stub)
//
// The OSPFv3 raw IPv6 transport opens its raw socket on Linux alone:
// backend_other.go refuses to open one anywhere else. So off Linux the probe
// reports the transport unavailable and names the host's OS, and the doctor
// check warns instead of reporting healthy for a transport that cannot run.

package transport

import (
	"errors"
	"runtime"
)

// checkRawSocket reports why the OSPFv3 raw transport cannot run on
// this host: it is Linux-only, and this host is not Linux.
func checkRawSocket() error {
	return errors.New("OSPFv3 cannot run on this host, so it forms no adjacencies here: " +
		"its raw IPv6 transport is Linux-only, and this host runs " + runtime.GOOS +
		". Run ze on Linux to use OSPFv3")
}
