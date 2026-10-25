// Design: docs/architecture/appliance/docker-host.md -- who may change a unit
// Related: systemctl.go -- the control socket that asks

//go:build !linux && !darwin

package dockerhost

import (
	"errors"
	"net"
	"runtime"
)

// errPeerUIDUnsupported is the refusal on a platform with no peer-credential
// read here. The guest is Linux and the development hosts are Linux and macOS,
// so this file exists only for the package to compile where the shipped tree
// is type-checked (`./le go vet-platforms`).
var errPeerUIDUnsupported = errors.New("reading the caller's uid is implemented on linux and darwin only, not on " + runtime.GOOS)

// peerUID refuses: with no caller uid, the shim answers every request with an
// error and changes no unit, rather than trusting a caller it cannot name.
func peerUID(_ *net.UnixConn) (uint32, error) {
	return 0, errPeerUIDUnsupported
}
