// Design: docs/research/l2tpv2-ze-integration.md -- original PPPoX channel capture
// Related: created_channel.go -- original attached file lifetime
// Related: devppp_namespace_linux.go -- unit creation in the socket's namespace

//go:build linux

package kernel

import (
	"errors"
	"fmt"
	"os"
	"time"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	pppiocGTransportOwner     = 0xc014742c
	pppiocAttChanOwner        = 0x4010742d
	pppTransportOwnerABI      = 1
	pppTransportOwnerFeatures = uint32(1<<8 | 1<<6 | 1<<7)
	channelCaptureRetryLimit  = 4
	channelCaptureRetryDelay  = 20 * time.Millisecond
)

// These native-endian UAPI records have only 32-bit fields: sizes20/16, align4.
// Channel and unit in a query reply are diagnostic labels, never lookup keys.
type pppTransportOwner struct {
	abi      uint32
	features uint32
	channel  int32
	unit     int32
	reserved uint32
}

type pppTransportCapture struct {
	abi      uint32
	fd       int32
	flags    uint32
	reserved uint32
}

// CreatePPPChannel borrows the SAME original socket file; the transport owner
// MUST retain it through this call. A SyscallConn lease prevents Close/reuse
// while the original kernel po->chan is captured into a fresh unbound file.
// The input is never closed, duplicated or reconstructed from its descriptor.
// Every nonnil result owns its partial file and MUST be closed even on error.
// Only a pre-bind EBUSY can retry on the retained files. No legacy ioctl,
// numeric lookup, identity recheck or unsupported-kernel fallback runs.
func CreatePPPChannel(pppox *os.File) (channel *CreatedChannel, err error) {
	if pppox == nil {
		return nil, errors.New("ppp: missing original PPPoX socket file")
	}
	source, err := pppox.SyscallConn()
	if err != nil {
		return nil, fmt.Errorf("ppp: original PPPoX socket lease: %w", err)
	}
	var operationErr error
	defer func() {
		if failure := recover(); failure != nil {
			err = errors.Join(err, operationErr, fmt.Errorf("ppp: original channel capture panicked: %w", sessionPanicFailure(failure)))
		}
		if err != nil {
			if channel != nil {
				channel.ready = false
			}
		}
	}()
	sourceLeased := false
	leaseErr := source.Control(func(sourceFD uintptr) {
		sourceLeased = true
		// Open before SIOCGSKNS so descriptor0 can be the actual fresh file.
		file, openErr := os.OpenFile("/dev/ppp", os.O_RDWR, 0)
		if openErr != nil {
			operationErr = fmt.Errorf("ppp: original channel file: %w", openErr)
			return
		}
		channel = &CreatedChannel{file: file, poll: pollOriginalPPPUnit}
		channel.self = channel
		target, targetErr := file.SyscallConn()
		if targetErr != nil {
			operationErr = targetErr
			return
		}
		channel.namespace, operationErr = originalPPPChannelNamespace(int(sourceFD))
		if operationErr != nil {
			return
		}
		targetErr = target.Control(func(targetFD uintptr) {
			operationErr = captureFreshPPPChannel(int(sourceFD), int(targetFD), queryPPPTransportOwner, capturePPPTransportOwner, time.Sleep)
		})
		operationErr = errors.Join(operationErr, targetErr)
	})
	if leaseErr != nil && !sourceLeased {
		// On Linux a nonnil os.File Control returns before its callback only
		// when its descriptor is closing. RawControl skips os.File's usual
		// conversion to os.ErrClosed, so expose both causes here.
		return nil, fmt.Errorf("ppp: original PPPoX socket lease: %w (%w)", os.ErrClosed, leaseErr)
	}
	if err := errors.Join(operationErr, leaseErr); err != nil {
		return channel, err
	}
	// Both leases have ended. There is no backend/network wait inside capture.
	if err := channel.poll(channel.file); err != nil {
		return channel, err
	}
	channel.ready = true
	return channel, nil
}

// captureFreshPPPChannel requires leases on the original socket and on a fresh,
// private destination file that has never been bound or exposed to another caller.
// The kernel returns query EBUSY before writing the reply. Attach EBUSY means
// lock contention here because nobody else can bind the fresh destination.
// Both ioctl calls release the socket lock before returning the refusal.
func captureFreshPPPChannel(socketFD, fileFD int,
	query func(int, *pppTransportOwner) error,
	capture func(int, *pppTransportCapture) error,
	pause func(time.Duration),
) error {
	remaining := channelCaptureRetryLimit
	retryBusy := func(ioctl func() error) error {
		for {
			err := ioctl()
			errno, raw := err.(unix.Errno) //nolint:errorlint // Only a raw errno proves a pre-bind refusal.
			if !raw || errno != unix.EBUSY || remaining == 0 {
				return err
			}
			remaining--
			// sleep(no-signal): the kernel's trylocks expose no wait event.
			pause(channelCaptureRetryDelay)
		}
	}
	return captureOriginalPPPChannel(socketFD, fileFD,
		func(fd int, request *pppTransportOwner) error {
			return retryBusy(func() error { return query(fd, request) })
		},
		func(fd int, request *pppTransportCapture) error {
			return retryBusy(func() error { return capture(fd, request) })
		})
}

// The namespace returned by SIOCGSKNS belongs to the original socket even when
// the calling thread has moved. It is a newly owned FD, never a borrowed label.
// Reuse the same retained proof type that unit.SessionConfig compares.
func originalPPPChannelNamespace(socketFD int) (*capturedSessionNamespace, error) {
	fd, err := unix.IoctlRetInt(socketFD, unix.SIOCGSKNS)
	if err != nil {
		return nil, fmt.Errorf("ppp: original PPPoX namespace is unknown: %w", err)
	}
	return &capturedSessionNamespace{file: os.NewFile(uintptr(fd), "ppp.transport.netns")}, nil
}

// This private wire seam creates no owner or admission grant. Its injected
// operations let pure tests inspect exact ABI/refusal/error handling only. It
// cannot establish exclusive target ownership, so it invokes each operation once.
func captureOriginalPPPChannel(socketFD, fileFD int,
	query func(int, *pppTransportOwner) error,
	capture func(int, *pppTransportCapture) error,
) error {
	if fileFD < 0 {
		return errors.New("ppp: original capture target descriptor is negative")
	}
	if uint64(fileFD) > uint64(^uint32(0)>>1) {
		return errors.New("ppp: original capture target descriptor exceeds the native ABI")
	}
	owner := pppTransportOwner{abi: pppTransportOwnerABI}
	if err := query(socketFD, &owner); err != nil {
		return fmt.Errorf("ppp: original transport ownership is unknown: %w", err)
	}
	if err := validatePPPTransportOwner(owner); err != nil {
		return err
	}
	request := pppTransportCapture{abi: pppTransportOwnerABI, fd: int32(fileFD)}
	if err := capture(socketFD, &request); err != nil {
		return fmt.Errorf("ppp: original transport channel capture is unconfirmed: %w", err)
	}
	return nil
}

func validatePPPTransportOwner(owner pppTransportOwner) error {
	if owner.abi != pppTransportOwnerABI {
		return errors.New("ppp: original transport ownership ABI is unsupported")
	}
	if owner.reserved != 0 {
		return errors.New("ppp: original transport ownership reply has reserved data")
	}
	if owner.features&pppTransportOwnerFeatures != pppTransportOwnerFeatures {
		return errors.New("ppp: original transport does not support capture, connect and bridge ownership")
	}
	if owner.channel < 0 {
		return errors.New("ppp: original transport query has no live channel")
	}
	if owner.unit < -1 {
		return errors.New("ppp: original transport query has an invalid unit label")
	}
	return nil
}

func queryPPPTransportOwner(fd int, request *pppTransportOwner) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), pppiocGTransportOwner,
		uintptr(unsafe.Pointer(request))) //nolint:gosec // ioctl requires the exact native UAPI record.
	if errno != 0 {
		return errno
	}
	return nil
}

func capturePPPTransportOwner(fd int, request *pppTransportCapture) error {
	_, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), pppiocAttChanOwner,
		uintptr(unsafe.Pointer(request))) //nolint:gosec // ioctl requires the exact native UAPI record.
	if errno != 0 {
		return errno
	}
	return nil
}
