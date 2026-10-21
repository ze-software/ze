// Design: docs/architecture/appliance/docker-host.md -- who may change a unit
// Related: systemctl.go -- the control socket that asks

package dockerhost

import (
	"fmt"
	"net"

	"golang.org/x/sys/unix"
)

// peerUID answers the uid of the process on the other end of conn, from the
// kernel (SO_PEERCRED), never from anything the client sends.
func peerUID(conn *net.UnixConn) (uint32, error) {
	raw, err := conn.SyscallConn()
	if err != nil {
		return 0, fmt.Errorf("socket: %w", err)
	}
	var cred *unix.Ucred
	var credErr error
	if err := raw.Control(func(fd uintptr) {
		cred, credErr = unix.GetsockoptUcred(int(fd), unix.SOL_SOCKET, unix.SO_PEERCRED)
	}); err != nil {
		return 0, fmt.Errorf("socket control: %w", err)
	}
	if credErr != nil {
		return 0, fmt.Errorf("SO_PEERCRED: %w", credErr)
	}
	return cred.Uid, nil
}
