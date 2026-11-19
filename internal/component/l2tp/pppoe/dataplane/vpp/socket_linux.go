// Design: docs/architecture/l2tp/bng-5-pppoe.md -- polled socket without /dev/ppp
// Related: socket.go -- sole endpoint owner; factory.go -- rollback authority
// API: Linux AF_PPPOX/PX_PROTO_OE (include/uapi/linux/if_pppox.h), as used by
// accel-ppp-ng non-dev-ppp PPPoE; golang.org/x/sys/unix and netns are vendored.

//go:build linux

package vpp

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"runtime"
	"syscall"
	"time"

	"github.com/vishvananda/netns"
	"github.com/ze-software/ze/internal/component/l2tp/pppoe/dataplane"
	"github.com/ze-software/ze/internal/core/diagnostic"
	"github.com/ze-software/ze/internal/core/report"
	"golang.org/x/sys/unix"
)

type socketFile struct {
	file *os.File
	raw  syscall.RawConn
}

var receiveControlDatagram = unix.Recvmsg

// Read uses the File's runtime poller, but MSG_TRUNC preserves a datagram's
// original length. An overlong PPP frame is refused, never accepted truncated.
func (s *socketFile) Read(buf []byte) (int, error) {
	var count, flags int
	var readErr error
	err := s.raw.Read(func(fd uintptr) bool {
		count, _, flags, _, readErr = receiveControlDatagram(int(fd), buf, nil, unix.MSG_DONTWAIT|unix.MSG_TRUNC)
		if errors.Is(readErr, unix.EAGAIN) {
			return false
		}
		if errors.Is(readErr, unix.EINTR) {
			return false
		}
		return true
	})
	if err != nil {
		return 0, err
	}
	if readErr != nil {
		return 0, readErr
	}
	if flags&unix.MSG_TRUNC != 0 {
		return 0, errors.New("pppoe-vpp: truncated control datagram")
	}
	if count > len(buf) {
		return 0, errors.New("pppoe-vpp: oversized control datagram")
	}
	return count, nil
}
func (s *socketFile) Write(buf []byte) (int, error) { return s.file.Write(buf) }
func (s *socketFile) SetReadDeadline(deadline time.Time) error {
	return s.file.SetReadDeadline(deadline)
}

// os.File.Close first deregisters the poll descriptor, unblocks its readers,
// then releases the native fd after in-flight syscalls leave their reference.
func (s *socketFile) Close() error { return s.file.Close() }

type socketResult struct {
	socket *controlSocket
	err    error
}

// openControlSocket performs one session's creation lifecycle in a dedicated
// locked thread. A failed namespace restore exits that goroutine still locked,
// which discards the contaminated thread; no caller goroutine changes netns.
// The result channel transfers ownership only to a receiver; cancellation
// before that handoff closes the unpublished owner in this worker.
func openControlSocket(ctx context.Context, request dataplane.Request) (*controlSocket, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	if request.Namespace == "" {
		return openSocketHere(ctx, request)
	}
	result, _ := startControlSocketThread(ctx, request, openSocketInNamespace)
	select {
	case answer := <-result:
		return answer.socket, answer.err
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// startControlSocketThread owns one namespace creation attempt. The opener MUST
// set restored only when the original namespace is restored. Receiving result
// transfers the socket; cancellation leaves cleanup with this worker. A caller
// that retains attempt bookkeeping MUST wait for done before releasing it.
// done follows the worker's deferred handling, not kernel thread retirement.
// Concurrent calls are safe when their supplied opener is safe.
func startControlSocketThread(ctx context.Context, request dataplane.Request,
	open func(context.Context, dataplane.Request, *bool) (*controlSocket, error),
) (<-chan socketResult, <-chan struct{}) {
	result := make(chan socketResult)
	done := make(chan struct{})
	go func() {
		defer close(done)
		runtime.LockOSThread()
		restored := false
		defer func() {
			if restored {
				runtime.UnlockOSThread()
			}
		}()
		socket, err := open(ctx, request, &restored)
		select {
		case result <- socketResult{socket: socket, err: err}:
		case <-ctx.Done():
			if socket != nil {
				if err := socket.Close(); err != nil {
					subject := fmt.Sprintf("socket/%p/%d", socket, time.Now().UnixNano())
					report.RaiseWarningFix(reconciliationSource, diagnostic.CodeDoctorPPPoEVPPReconcile, subject,
						"Canceled PPPoE VPP control socket cleanup failed", reconciliationFix,
						map[string]any{"error": err.Error(), "access": request.AccessName, "namespace": request.Namespace})
					slog.Default().Warn("pppoe-vpp: canceled control socket cleanup failed",
						"code", diagnostic.CodeDoctorPPPoEVPPReconcile, "fix", reconciliationFix, "error", err)
				}
			}
		}
	}()
	return result, done
}

func openSocketInNamespace(ctx context.Context, request dataplane.Request, restored *bool) (socket *controlSocket, err error) {
	return openSocketInNamespaceWithOps(ctx, request, restored, socketNamespaceOps{
		current: netns.Get, target: netns.GetFromName, set: netns.Set,
		close: func(handle netns.NsHandle) error { return handle.Close() },
		open:  openSocketHere,
	})
}

// socketNamespaceOps captures the complete native namespace operation set for
// one attempt. Tests supply a complete set and never borrow native descriptors.
type socketNamespaceOps struct {
	current func() (netns.NsHandle, error)
	target  func(string) (netns.NsHandle, error)
	set     func(netns.NsHandle) error
	close   func(netns.NsHandle) error
	open    func(context.Context, dataplane.Request) (*controlSocket, error)
}

func openSocketInNamespaceWithOps(ctx context.Context, request dataplane.Request, restored *bool,
	ops socketNamespaceOps,
) (socket *controlSocket, err error) {
	defer func() {
		if err != nil {
			if socket != nil {
				err = errors.Join(err, socket.Close())
				socket = nil
			}
		}
	}()
	original, err := ops.current()
	if err != nil {
		*restored = true
		return nil, err
	}
	defer func() { err = errors.Join(err, ops.close(original)) }()
	target, err := ops.target(request.Namespace)
	if err != nil {
		*restored = true
		return nil, err
	}
	defer func() { err = errors.Join(err, ops.close(target)) }()
	if err = ops.set(target); err != nil {
		*restored = true
		return nil, err
	}
	defer func() {
		restoreErr := ops.set(original)
		if restoreErr != nil {
			if socket != nil {
				err = errors.Join(err, socket.Close())
				socket = nil
			}
			err = errors.Join(err, fmt.Errorf("pppoe-vpp: restore network namespace: %w", restoreErr))
			return
		}
		*restored = true
	}()
	return ops.open(ctx, request)
}

func openSocketHere(ctx context.Context, request dataplane.Request) (socket *controlSocket, err error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	fd, err := unix.Socket(unix.AF_PPPOX, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC|unix.SOCK_NONBLOCK, 0)
	if err != nil {
		return nil, fmt.Errorf("pppoe-vpp: create AF_PPPOX socket: %w", err)
	}
	rawOwned := true
	defer func() {
		if rawOwned {
			err = errors.Join(err, unix.Close(fd))
		}
	}()
	if err = unix.Connect(fd, &unix.SockaddrPPPoE{SID: request.SessionID, Remote: request.PeerMAC[:], Dev: request.ControlDevice}); err != nil {
		return nil, fmt.Errorf("pppoe-vpp: connect control socket on %q: %w", request.ControlDevice, err)
	}
	if err = ctx.Err(); err != nil {
		return nil, err
	}
	file := os.NewFile(uintptr(fd), "pppoe-vpp-control")
	if file == nil {
		return nil, errors.New("pppoe-vpp: wrap control descriptor")
	}
	rawOwned = false
	raw, rawErr := file.SyscallConn()
	if rawErr != nil {
		return nil, errors.Join(rawErr, file.Close())
	}
	socket = newControlSocket(&socketFile{file: file, raw: raw})
	return socket, nil
}
