// Design: docs/architecture/l2tp/bng-5-pppoe.md -- native PPPoX session transport
// Related: transport.go -- lifetime owner; factory.go -- request validation

//go:build linux

package kernel

import (
	"context"
	"errors"
	"fmt"
	"os"

	"golang.org/x/sys/unix"

	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
)

type socketOps struct {
	socket func(int, int, int) (int, error)
	// own is nil only for explicit logical syscall fixtures. Production wraps
	// the descriptor just returned by socket exactly once, before connect.
	own     func(int) *os.File
	connect func(int, unix.Sockaddr) error
	close   func(int) error
}

// refuse-by-default: guards Open -- this build-tagged implementation admits Linux syscall attempts; socket and connect still report actual kernel capability failures.
func requirePlatform() error { return nil }

func openTransport(ctx context.Context, request dataplane.Request) (*pppoxTransport, error) {
	return openTransportWithOps(ctx, request, socketOps{
		socket: unix.Socket, connect: unix.Connect, close: unix.Close,
		own: func(fd int) *os.File { return os.NewFile(uintptr(fd), "pppoe-transport") },
	})
}

func openTransportWithOps(ctx context.Context, request dataplane.Request, ops socketOps) (
	*pppoxTransport, error,
) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	// PPPoE connect binds local kernel state and has no network handshake. A
	// nonblocking socket bounds it without a goroutine that could outlive ctx.
	// A refused syscall is returned unchanged; no provider or blocking fallback.
	const protocolPPPoE = 0
	fd, err := ops.socket(unix.AF_PPPOX, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, protocolPPPoE)
	if err != nil {
		return nil, fmt.Errorf("pppoe: socket(AF_PPPOX): %w", err)
	}
	owner := &pppoxTransport{fd: fd, closeFD: ops.close}
	if ops.own != nil {
		owner.file = ops.own(fd)
	}
	if err := ctx.Err(); err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	if err := ops.connect(fd, &unix.SockaddrPPPoE{
		SID: request.SessionID, Remote: request.PeerMAC[:], Dev: request.Device,
	}); err != nil {
		return nil, errors.Join(fmt.Errorf("pppoe: connect session %d on %s: %w", request.SessionID, request.Device, err), owner.Close())
	}
	if err := ctx.Err(); err != nil {
		return nil, errors.Join(err, owner.Close())
	}
	return owner, nil
}
