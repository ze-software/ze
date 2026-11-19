// Design: docs/architecture/appliance/kernel-profiles.md -- creator-only PPP IPv6 control constructor
// Related: created_unit_control_socket.go -- original-unit admission and ownership contract

//go:build linux

package kernel

import (
	"context"
	"errors"
	"os"
	"runtime"
	"unsafe"

	"golang.org/x/sys/unix"
)

const (
	createdUnitControlFeature = 1 << 9
	createdUnitControlSocket  = 0x4020742b
)

// Fixed 32-byte, four-byte-aligned input-only ioctl43 layout. Success returns
// the new owned fd, including zero; it does not overwrite an input descriptor.
type createdUnitControlRequest struct {
	abi      uint32
	kind     uint32
	unit     int32
	index    int32
	flags    uint32
	reserved [3]uint32
}

// Both directions refuse size or alignment drift at target compilation.
var (
	_ [32 - int(unsafe.Sizeof(createdUnitControlRequest{}))]byte
	_ [int(unsafe.Sizeof(createdUnitControlRequest{})) - 32]byte
	_ [4 - int(unsafe.Alignof(createdUnitControlRequest{}))]byte
	_ [int(unsafe.Alignof(createdUnitControlRequest{})) - 4]byte
)

func (unit *CreatedUnit) openIPv6ControlSocket(ctx context.Context, kind IPv6ControlSocketKind, index, number int32) (*os.File, error) {
	// Primary's existing joined namespace helper surrounds this method. Pin
	// this thread so query51 and constructor43 cannot observe different task
	// namespaces; query51 itself refuses a caller outside the original netns.
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()
	var owned *os.File
	var operationErr error
	borrowErr := unit.WithDescriptor(func(fd int) error {
		if err := ctx.Err(); err != nil {
			operationErr = err
			return err
		}
		owner := pppPairUnitOwner{abi: pppPairOwnerABI, ifindex: index, unit: number}
		if err := pppPairReadUnitOwner(fd, &owner); err != nil {
			operationErr = err
			return err
		}
		if err := validateCreatedUnitControlOwner(owner, index, number); err != nil {
			operationErr = err
			return err
		}
		if err := ctx.Err(); err != nil {
			operationErr = err
			return err
		}
		request := createdUnitControlRequest{abi: pppPairOwnerABI,
			kind: uint32(kind), unit: number, index: index}
		result, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), createdUnitControlSocket,
			uintptr(unsafe.Pointer(&request))) //nolint:gosec // exact input-only original-unit constructor43 pointer.
		if errno != 0 {
			operationErr = errno
			return errno
		}
		if result == uintptr(fd) {
			operationErr = errors.New("ppp: original control constructor returned its borrowed creator descriptor")
			return operationErr
		}
		if uint64(result) > uint64(1<<31-1) {
			operationErr = errors.New("ppp: original control constructor returned an invalid native descriptor")
			return operationErr
		}
		// This wraps only the actual NEW owned result, never the borrowed unit
		// descriptor. It remains returned even if borrow settlement or caller
		// cancellation later fails. The caller owns its mandatory Close.
		owned = os.NewFile(result, "ze-original-ppp-ipv6-control")
		return nil
	})
	return owned, errors.Join(operationErr, borrowErr, ctx.Err())
}

func validateCreatedUnitControlOwner(owner pppPairUnitOwner, index, number int32) error {
	if owner.abi != pppPairOwnerABI {
		return errors.New("ppp: original IPv6 control query returned another ABI")
	}
	if owner.ifindex != index {
		return errors.New("ppp: original IPv6 control query changed the captured native index")
	}
	if owner.unit != number {
		return errors.New("ppp: original IPv6 control query changed the captured unit")
	}
	if owner.reserved != 0 {
		return errors.New("ppp: original IPv6 control query returned nonzero reserved fields")
	}
	if owner.features&createdUnitControlFeature == 0 {
		return errors.New("ppp: original creator lacks complete IPv6 control socket capability")
	}
	return nil
}
