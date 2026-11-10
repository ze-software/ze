// Design: docs/architecture/l2tp/followup-l2tp-call.md -- realm relay handoff (LAC role)
// Related: relay_handoff.go -- Driver.QuiesceSession wakes this reader
// Related: frame_linux.go -- the plain blocking file every other session uses

//go:build linux

package ppp

import (
	"errors"
	"fmt"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

// errReadInterrupted is what an interruptible channel file returns from Read
// once InterruptRead ran. It is sticky: every later Read returns it too.
var errReadInterrupted = errors.New("ppp: channel read interrupted for a relay handoff")

// interruptibleFDFile is the channel file of a session that may be handed
// to a realm relay. A blocking read(2) on /dev/ppp is woken only by a frame,
// the channel unregistering, or a signal (ppp_read), so closing the
// descriptor cannot stop it. This file instead waits in poll(2) on the
// channel and on an eventfd, and InterruptRead makes the eventfd readable:
// the waiting Read returns errReadInterrupted, as does every later Read.
//
// Both descriptors are held as *os.File and used through RawConn, so a
// Close racing a Read never lets the Read touch a recycled descriptor
// number: os.File keeps the descriptor until in-flight calls return.
// Neither is registered with the runtime poller (frame_linux.go says why).
//
// Safe for concurrent use by one reader, writers and one closer.
type interruptibleFDFile struct {
	channel *os.File
	wake    *os.File
}

// newInterruptibleFDFile wraps the channel descriptor fd. The caller MUST
// NOT close fd after this call; Close releases it.
func newInterruptibleFDFile(fd int, name string) (io.ReadWriteCloser, error) {
	if err := unix.SetNonblock(fd, false); err != nil {
		return nil, fmt.Errorf("ppp: %s: blocking mode: %w", name, err)
	}
	wakeFD, err := unix.Eventfd(0, unix.EFD_CLOEXEC)
	if err != nil {
		return nil, fmt.Errorf("ppp: %s: eventfd for the relay handoff: %w", name, err)
	}
	return &interruptibleFDFile{
		channel: os.NewFile(uintptr(fd), name),
		wake:    os.NewFile(uintptr(wakeFD), name+".wake"),
	}, nil
}

// Read waits until the channel holds a frame or InterruptRead ran, then
// reads one frame. A channel that unregistered reads as io.EOF.
func (f *interruptibleFDFile) Read(buf []byte) (int, error) {
	wakeConn, err := f.wake.SyscallConn()
	if err != nil {
		return 0, err
	}
	channelConn, err := f.channel.SyscallConn()
	if err != nil {
		return 0, err
	}
	var (
		n       int
		readErr error
	)
	ctlErr := wakeConn.Control(func(wakeFD uintptr) {
		if err := channelConn.Control(func(channelFD uintptr) {
			n, readErr = pollThenRead(int(channelFD), int(wakeFD), buf)
		}); err != nil {
			readErr = err
		}
	})
	if ctlErr != nil {
		return 0, ctlErr
	}
	return n, readErr
}

// pollThenRead is Read's body over the two raw descriptors. The poll has no
// timeout: the reader waits for a frame or the wake, nothing else.
func pollThenRead(channelFD, wakeFD int, buf []byte) (int, error) {
	fds := []unix.PollFd{
		{Fd: int32(channelFD), Events: unix.POLLIN},
		{Fd: int32(wakeFD), Events: unix.POLLIN},
	}
	for {
		_, err := unix.Poll(fds, -1)
		if errors.Is(err, unix.EINTR) {
			continue
		}
		if err != nil {
			return 0, fmt.Errorf("ppp: poll on the relay channel: %w", err)
		}
		if fds[1].Revents&unix.POLLIN != 0 {
			return 0, errReadInterrupted
		}
		if fds[0].Revents == 0 {
			continue
		}
		n, err := unix.Read(channelFD, buf)
		if err != nil {
			return 0, err
		}
		if n == 0 {
			return 0, io.EOF
		}
		return n, nil
	}
}

// InterruptRead wakes a Read waiting in poll and makes every later Read
// return errReadInterrupted. Idempotent: the eventfd counter only grows.
func (f *interruptibleFDFile) InterruptRead() error {
	var one [8]byte
	one[0] = 1 // eventfd counters are host-endian; every Linux target Ze runs on is little-endian
	if _, err := f.wake.Write(one[:]); err != nil {
		return fmt.Errorf("ppp: waking the relay channel reader: %w", err)
	}
	return nil
}

// Write sends one frame on the channel.
func (f *interruptibleFDFile) Write(buf []byte) (int, error) {
	return f.channel.Write(buf)
}

// Close wakes any reader, then releases both descriptors.
func (f *interruptibleFDFile) Close() error {
	werr := f.InterruptRead()
	cerr := f.channel.Close()
	wcloseErr := f.wake.Close()
	return errors.Join(werr, cerr, wcloseErr)
}

var _ readInterrupter = (*interruptibleFDFile)(nil)
