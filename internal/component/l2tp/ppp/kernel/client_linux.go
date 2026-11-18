//go:build linux

// Design: docs/architecture/l2tp/cpe-1-pppoe-client.md -- one delayed client reader.
// Related: session_pair_linux.go -- original owner/file locks and atomic attach.
// Reference: https://github.com/ppp-project/ppp/blob/v2.5.2/pppd/sys-linux.c -- read_packet.

package kernel

import (
	"errors"
	"io"
	"os"

	"golang.org/x/sys/unix"
)

func productionClientPairOps() clientPairOps {
	return clientPairOps{prepare: prepareOriginalClientPair, connect: connectOriginalPairLocked,
		read: readOriginalClientPair, write: writeOriginalClientPair, unit: originalClientUnitOperation}
}

func prepareOriginalClientPair(pair *claimedSessionPair) error {
	if err := prepareOriginalSessionPair(pair); err != nil {
		return err
	}
	pair.mu.Lock()
	defer pair.mu.Unlock()
	return pair.withOriginalFiles(func(_ *os.File, channelFD, unitFD int) error {
		if err := unix.SetNonblock(channelFD, true); err != nil {
			return err
		}
		return unix.SetNonblock(unitFD, true)
	})
}

// rawOrderedClientAttempt is a PRIVATE descriptor algorithm, not a resource
// grant. Production calls it only while both original owner/RawConn leases
// are held. Real-socket tests may call it without fabricating public PPP owners.
func rawOrderedClientAttempt(channelFD, unitFD int, readUnit bool, buffer []byte) (int, error) {
	if len(buffer) == 0 {
		return 0, io.ErrShortBuffer
	}
	n, err := unix.Read(channelFD, buffer)
	if !errors.Is(err, unix.EAGAIN) {
		return originalClientReadResult(n, err)
	}
	if readUnit {
		n, err = unix.Read(unitFD, buffer)
		if !errors.Is(err, unix.EAGAIN) {
			return originalClientReadResult(n, err)
		}
	}
	fds := [2]unix.PollFd{{Fd: int32(channelFD), Events: unix.POLLIN}, {Fd: -1}}
	if readUnit {
		fds[1] = unix.PollFd{Fd: int32(unitFD), Events: unix.POLLIN}
	}
	if _, err := unix.Poll(fds[:], 100); err != nil {
		return 0, err
	}
	for _, fd := range fds {
		if fd.Revents&unix.POLLNVAL != 0 {
			return 0, unix.EBADF
		}
		if fd.Revents&unix.POLLERR != 0 {
			return 0, unix.EIO
		}
	}
	// Recheck channel first after readiness too, including its EOF/terminal.
	n, err = unix.Read(channelFD, buffer)
	if !errors.Is(err, unix.EAGAIN) {
		return originalClientReadResult(n, err)
	}
	if readUnit {
		n, err = unix.Read(unitFD, buffer)
		if !errors.Is(err, unix.EAGAIN) {
			return originalClientReadResult(n, err)
		}
	}
	for _, fd := range fds {
		if fd.Revents&unix.POLLHUP != 0 {
			return 0, io.EOF
		}
	}
	return 0, unix.EAGAIN
}

func originalClientReadResult(n int, err error) (int, error) {
	if err != nil {
		return n, err
	}
	if n == 0 {
		return 0, io.EOF
	}
	return n, nil
}

func readOriginalClientPair(pair *claimedSessionPair, readUnit bool, buffer []byte) (n int, err error) {
	err = pair.withOriginalFiles(func(_ *os.File, channelFD, unitFD int) error {
		var readErr error
		n, readErr = rawOrderedClientAttempt(channelFD, unitFD, readUnit, buffer)
		return readErr
	})
	return n, err
}

// One original os.File.Write preserves the old frame/short-write/error contract.
// Its internal file lease protects actual I/O after the short validation lease
// ends. No owner/pair lock may span Write: original Close must wake a write
// waiting for readiness. No duplicate, rewrap, reader or queue is introduced.
func writeOriginalClientPair(pair *claimedSessionPair, buffer []byte) (n int, err error) {
	var original *os.File
	pair.mu.Lock()
	if pair.channelClosed {
		pair.mu.Unlock()
		return 0, io.ErrClosedPipe
	}
	if pair.unitClosed {
		pair.mu.Unlock()
		return 0, io.ErrClosedPipe
	}
	err = pair.withOriginalFiles(func(file *os.File, _, _ int) error { original = file; return nil })
	pair.mu.Unlock()
	if err != nil {
		return 0, err
	}
	return original.Write(buffer)
}

func originalClientUnitOperation(pair *claimedSessionPair, request clientUnitRequest) error {
	return pair.withOriginalFiles(func(_ *os.File, _, unitFD int) error {
		switch request := request.(type) {
		case clientMRU:
			if request.mru == 0 {
				return errors.New("ppp: session receive MRU must be positive")
			}
			return realSetMRU(unitFD, request.mru)
		case clientIPv4Mode:
			return SetIPv4Mode(unitFD, request.mode)
		case clientIPv6Mode:
			return SetIPv6Mode(unitFD, request.mode)
		}
		panic("BUG: unknown sealed client unit operation")
	})
}
