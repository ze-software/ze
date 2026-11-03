//go:build linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- descriptor receive ownership.
// Related: receive.go -- dispatch handshake; dialer.go -- descriptor lifetime lock.
// Reference: https://github.com/ppp-project/ppp/blob/v2.5.2/pppd/sys-linux.c -- read_packet.

package pppoeclient

import (
	"errors"
	"io"

	"golang.org/x/sys/unix"
)

// readPacket checks the channel before the connected unit on each read, as
// pppd's read_packet does. It MUST be called by the sole frameReader worker.
// Poll and Read retain the lifecycle lock so Close cannot recycle either fd.
// Each poll waits at most 100ms before releasing the lock. This avoids the
// runtime epoll path. The loop lasts until a packet, terminal error, or Close.
func (s *sessionLink) readPacket(buf []byte) (int, error) {
	for {
		s.mu.Lock()
		if s.closed {
			s.mu.Unlock()
			return 0, io.ErrClosedPipe
		}
		n, err := unix.Read(s.channelFD, buf)
		if errors.Is(err, unix.EINTR) {
			s.mu.Unlock()
			continue
		}
		if !errors.Is(err, unix.EAGAIN) {
			s.mu.Unlock()
			return n, err
		}
		if s.unitConnected {
			n, err = unix.Read(s.unitFD, buf)
			if !errors.Is(err, unix.EAGAIN) {
				if !errors.Is(err, unix.EINTR) {
					s.mu.Unlock()
					return n, err
				}
			}
		}
		fds := [2]unix.PollFd{{Fd: int32(s.channelFD), Events: unix.POLLIN}, {Fd: -1}}
		if s.unitConnected {
			fds[1] = unix.PollFd{Fd: int32(s.unitFD), Events: unix.POLLIN}
		}
		_, err = unix.Poll(fds[:], 100)
		s.mu.Unlock()
		if err != nil {
			if !errors.Is(err, unix.EINTR) {
				return 0, err
			}
		}
		for _, fd := range fds {
			if fd.Revents&unix.POLLNVAL != 0 {
				return 0, unix.EBADF
			}
		}
	}
}

// prepareClientDescriptors leaves file ownership with Dial. Both descriptors
// MUST be protected by sessionLink's lifecycle lock. Cleanup MUST join the
// reader before releasing the unit. These raw reads do not use epoll.
func prepareClientDescriptors(channelFD, unitFD int) error {
	if err := unix.SetNonblock(channelFD, true); err != nil {
		return err
	}
	return unix.SetNonblock(unitFD, true)
}
