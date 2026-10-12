//go:build !linux

// Design: docs/architecture/isis/isis-3-l2-transport.md -- raw-socket probe (non-Linux stub)
//
// The IS-IS raw L2 transport opens its raw socket on Linux alone:
// backend_other.go refuses to open one anywhere else. So off Linux the probe
// reports the transport unavailable and names the host's OS, and the doctor
// check warns instead of reporting healthy for a transport that cannot run.

package transport

import (
	"errors"
	"runtime"
)

// checkRawSocket reports why the IS-IS raw transport cannot run on
// this host: it is Linux-only, and this host is not Linux.
func checkRawSocket() error {
	return errors.New("IS-IS cannot run on this host, so it forms no adjacencies here: " +
		"its raw L2 transport is Linux-only, and this host runs " + runtime.GOOS +
		". Run ze on Linux to use IS-IS")
}
